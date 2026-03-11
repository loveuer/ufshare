package tests

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/pkg/database"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
	npmsvc "gitea.loveuer.com/loveuer/ufshare/v2/internal/service/npm"
	ocisvc "gitea.loveuer.com/loveuer/ufshare/v2/internal/service/oci"
)

const (
	testJWTSecret = "test-jwt-secret-for-npm-integration-tests"
	testAdminUser = "admin"
	testAdminPass = "admin123"
)

type testServer struct {
	BaseURL string
	Token   string
}

// newTestServer 启动一个完整的 ufshare HTTP 服务用于集成测试，测试结束自动清理。
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	dataDir := t.TempDir()

	db, err := database.Connect("sqlite", filepath.Join(dataDir, "test.db"), false)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}

	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	authSvc := service.NewAuthService(db, testJWTSecret, 24*time.Hour)
	userSvc := service.NewUserService(db)
	fileSvc := service.NewFileService(db, dataDir)
	settingSvc := service.NewSettingService(db)
	npmSvc := npmsvc.New(db, dataDir, settingSvc)
	ociSvc := ocisvc.New(db, dataDir, settingSvc)

	// 创建管理员用户
	bgCtx := context.Background()
	user, err := authSvc.Register(bgCtx, testAdminUser, testAdminPass, "admin@test.local")
	if err != nil {
		t.Fatalf("register admin: %v", err)
	}
	if err := userSvc.SetAdmin(bgCtx, user.ID, true); err != nil {
		t.Fatalf("set admin: %v", err)
	}

	// 找空闲端口
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	baseURL := "http://" + addr

	router := api.NewRouter(authSvc, userSvc, fileSvc, npmSvc, ociSvc, settingSvc, nil)
	app := ursa.New(ursa.Config{BodyLimit: -1})
	router.Setup(app, nil)

	go func() { _ = app.Run(addr) }()

	// 等待服务就绪（最多 5 秒）
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/npm/-/ping")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// 获取 token
	token, _, err := authSvc.Login(bgCtx, testAdminUser, testAdminPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	return &testServer{BaseURL: baseURL, Token: token}
}

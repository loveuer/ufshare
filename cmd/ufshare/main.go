package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/loveuer/ursa"
	"github.com/spf13/cobra"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api/handler"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api/middleware"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/pkg/config"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/pkg/database"
	pkgserver "gitea.loveuer.com/loveuer/ufshare/v2/internal/pkg/server"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
	npmsvc "gitea.loveuer.com/loveuer/ufshare/v2/internal/service/npm"
	"gitea.loveuer.com/loveuer/ufshare/v2/web"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	log.Printf("[ROOTCMD] Creating root command")
	cfg := config.Load()

	cmd := &cobra.Command{
		Use:   "ufshare",
		Short: "UFShare - Artifact Repository Manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Printf("[RUNE] RunE called")
			if err := cfg.Validate(); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			return run(cfg)
		},
	}

	cmd.Flags().StringVar(&cfg.Address, "address", cfg.Address, "监听地址 (e.g. 0.0.0.0:8000)")
	cmd.Flags().StringVar(&cfg.Data, "data", cfg.Data, "数据目录，存放文件和数据库")
	cmd.Flags().BoolVar(&cfg.Debug, "debug", false, "开启 debug 模式（打印 GORM 日志及详细流程）")
	cmd.Flags().StringVar(&cfg.NpmAddr, "npm-addr", "", "npm 专用端口（可选，如 0.0.0.0:4873），空则仅通过主端口 /npm/ 访问")
	cmd.Flags().StringVar(&cfg.FileAddr, "file-addr", "", "file-store 专用端口（可选，如 0.0.0.0:8001），空则仅通过主端口 /file-store/ 访问")

	return cmd
}

func run(cfg *config.Config) error {
	log.Printf("[RUN] Starting run function")
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: %v", r)
			panic(r) // re-panic to show stack
		}
	}()

	// 确保数据目录存在
	if err := os.MkdirAll(cfg.Data, 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	// SQLite 默认放在 data 目录下
	if cfg.Database.DSN == "" {
		cfg.Database.DSN = filepath.Join(cfg.Data, "ufshare.db")
	}

	// 连接数据库
	db, err := database.Connect(cfg.Database.Driver, cfg.Database.DSN, cfg.Debug)
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}
	time.Sleep(500 * time.Millisecond) // 等待可能的后台 goroutine 启动

	// 自动迁移
	if err := model.AutoMigrate(db); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// 初始化服务
	authService    := service.NewAuthService(db, cfg.JWT.Secret, cfg.JWT.Expire)
	userService    := service.NewUserService(db)
	fileService    := service.NewFileService(db, cfg.Data)
	settingService := service.NewSettingService(db)
	npmService     := npmsvc.New(db, cfg.Data, settingService)

	// 创建默认管理员用户
	if err := createDefaultAdmin(authService, userService); err != nil {
		log.Printf("Warning: failed to create default admin: %v", err)
	}

	// ── 独立端口服务器 ─────────────────────────────────────────────────────────

	npmHandler  := handler.NewNpmHandler(npmService, authService)
	fileHandler := handler.NewFileHandler(fileService)

	// npm 专用端口：同时注册两套路由：
	//   - 无前缀：供 npm 客户端直接使用（npm set registry http://host:4873）
	//   - /npm 前缀：tarball URL 由 rewriteTarballURL 写入 /npm/ 路径，需在此端口也能访问
	npmDedicated := pkgserver.New("npm", func(app *ursa.App) {
		api.RegisterNpmRoutes(app, npmHandler, authService, "")
		api.RegisterNpmRoutes(app, npmHandler, authService, "/npm")
	})
	// file-store 专用端口：手动注册无前缀路由（避免 RegisterFileRoutes 的问题）
	fileDedicated := pkgserver.New("file", func(app *ursa.App) {
		app.Get("/*path", fileHandler.Download)
		app.Put("/*path", middleware.Auth(authService), fileHandler.Upload)
		app.Delete("/*path", middleware.Auth(authService), fileHandler.Delete)
	})

	// 将启动参数中的端口写入 settings（仅当 DB 中尚未配置时，启动参数作为初始值）
	// 注意：必须在 Dedicated 创建之后再调用 Set，避免触发未初始化的回调
	if cfg.NpmAddr != "" && settingService.GetNpmAddr() == "" {
		_ = settingService.Set(service.SettingNpmAddr, cfg.NpmAddr)
		_ = settingService.Set(service.SettingNpmEnabled, "true")
	}
	// if cfg.FileAddr != "" && settingService.GetFileAddr() == "" {
	// 	_ = settingService.Set(service.SettingFileAddr, cfg.FileAddr)
	// 	_ = settingService.Set(service.SettingFileEnabled, "true")
	// }

	// tryStartDedicated 根据 enabled + addr 决定是否启动/停止独立端口
	tryStartDedicated := func(d *pkgserver.Dedicated, enabled bool, addr string) {
		if enabled && addr != "" {
			d.Start(addr)
		} else {
			d.Stop()
		}
	}

	// 启动时根据已保存的配置决定是否启动独立端口
	log.Printf("[INIT] trying to start npm: enabled=%v addr=%q", settingService.GetNpmEnabled(), settingService.GetNpmAddr())
	tryStartDedicated(npmDedicated, settingService.GetNpmEnabled(), settingService.GetNpmAddr())
	log.Printf("[INIT] trying to start file: enabled=%v addr=%q", settingService.GetFileEnabled(), settingService.GetFileAddr())
	tryStartDedicated(fileDedicated, settingService.GetFileEnabled(), settingService.GetFileAddr())

	// 监听 setting 变更，动态热重启独立端口
	// npm: enabled 或 addr 任一变更都重新评估
	log.Printf("[MAIN] registering npm listeners")
	settingService.OnChange(service.SettingNpmEnabled, func(_ string) {
		log.Printf("[CALLBACK] npm enabled changed")
		tryStartDedicated(npmDedicated, settingService.GetNpmEnabled(), settingService.GetNpmAddr())
	})
	settingService.OnChange(service.SettingNpmAddr, func(_ string) {
		log.Printf("[CALLBACK] npm addr changed")
		tryStartDedicated(npmDedicated, settingService.GetNpmEnabled(), settingService.GetNpmAddr())
	})
	// file: enabled 或 addr 任一变更都重新评估
	log.Printf("[MAIN] registering file listeners")
	settingService.OnChange(service.SettingFileEnabled, func(_ string) {
		log.Printf("[CALLBACK] file enabled changed")
		tryStartDedicated(fileDedicated, settingService.GetFileEnabled(), settingService.GetFileAddr())
	})
	settingService.OnChange(service.SettingFileAddr, func(_ string) {
		log.Printf("[CALLBACK] file addr changed")
		tryStartDedicated(fileDedicated, settingService.GetFileEnabled(), settingService.GetFileAddr())
	})

	// ── 主端口 ────────────────────────────────────────────────────────────────

	router := api.NewRouter(authService, userService, fileService, npmService, settingService, web.FS())

	appConfig := ursa.Config{
		BodyLimit: -1,
	}
	if spaHandler := router.SPAHandler(); spaHandler != nil {
		appConfig.NotFoundHandler = spaHandler
	}
	app := ursa.New(appConfig)
	router.Setup(app)

	log.Printf("data dir : %s", cfg.Data)
	log.Printf("database : %s", cfg.Database.DSN)
	log.Printf("listening: %s", cfg.Address)
	if addr := settingService.GetNpmAddr(); addr != "" {
		log.Printf("npm dedicated: %s", addr)
	}
	if addr := settingService.GetFileAddr(); addr != "" {
		log.Printf("file dedicated: %s", addr)
	}

	return app.Run(cfg.Address)
}

func createDefaultAdmin(authService *service.AuthService, userService *service.UserService) error {
	user, err := authService.Register("admin", "admin123", "admin@ufshare.local")
	if err != nil {
		if err == service.ErrUserExists {
			return nil
		}
		return err
	}

	if err := userService.SetAdmin(user.ID, true); err != nil {
		return err
	}

	log.Println("Default admin user created: admin/admin123")
	return nil
}

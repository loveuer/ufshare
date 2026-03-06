package main

import (
	"fmt"
	"log"

	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/pkg/config"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/pkg/database"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 连接数据库
	db, err := database.Connect(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}

	// 自动迁移
	if err := model.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// 初始化服务
	authService := service.NewAuthService(db, cfg.JWT.Secret, cfg.JWT.Expire)
	userService := service.NewUserService(db)
	permService := service.NewPermissionService(db)

	// 创建默认管理员用户
	if err := createDefaultAdmin(authService, userService); err != nil {
		log.Printf("Warning: failed to create default admin: %v", err)
	}

	// 创建 Ursa 应用
	app := ursa.New()

	// 设置路由
	router := api.NewRouter(authService, userService, permService)
	router.Setup(app)

	// 启动服务
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	if err := app.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func createDefaultAdmin(authService *service.AuthService, userService *service.UserService) error {
	user, err := authService.Register("admin", "admin123", "admin@ufshare.local")
	if err != nil {
		if err == service.ErrUserExists {
			return nil // 已存在，忽略
		}
		return err
	}

	// 设置为管理员
	if err := userService.SetAdmin(user.ID, true); err != nil {
		return err
	}

	log.Println("Default admin user created: admin/admin123")
	return nil
}

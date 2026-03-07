package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/loveuer/ursa"
	"github.com/spf13/cobra"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/pkg/config"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/pkg/database"
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
	cfg := config.Load()

	cmd := &cobra.Command{
		Use:   "ufshare",
		Short: "UFShare - Artifact Repository Manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg)
		},
	}

	cmd.Flags().StringVar(&cfg.Address, "address", cfg.Address, "监听地址 (e.g. 0.0.0.0:8000)")
	cmd.Flags().StringVar(&cfg.Data, "data", cfg.Data, "数据目录，存放文件和数据库")

	return cmd
}

func run(cfg *config.Config) error {
	// 确保数据目录存在
	if err := os.MkdirAll(cfg.Data, 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	// SQLite 默认放在 data 目录下
	if cfg.Database.DSN == "" {
		cfg.Database.DSN = filepath.Join(cfg.Data, "ufshare.db")
	}

	// 连接数据库
	db, err := database.Connect(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	// 自动迁移
	if err := model.AutoMigrate(db); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// 初始化服务
	authService := service.NewAuthService(db, cfg.JWT.Secret, cfg.JWT.Expire)
	userService := service.NewUserService(db)
	fileService := service.NewFileService(db, cfg.Data)
	npmService := npmsvc.New(db, cfg.Data)

	// 创建默认管理员用户
	if err := createDefaultAdmin(authService, userService); err != nil {
		log.Printf("Warning: failed to create default admin: %v", err)
	}

	// 创建 Ursa 应用
	router := api.NewRouter(authService, userService, fileService, npmService, web.FS())

	appConfig := ursa.Config{}
	if spaHandler := router.SPAHandler(); spaHandler != nil {
		appConfig.NotFoundHandler = spaHandler
	}
	app := ursa.New(appConfig)

	// 设置路由
	router.Setup(app)

	log.Printf("data dir : %s", cfg.Data)
	log.Printf("database : %s", cfg.Database.DSN)
	log.Printf("listening: %s", cfg.Address)

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

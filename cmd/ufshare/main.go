package main

import (
	"fmt"
	"log"
	"os"

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
	cfg := config.Load()

	cmd := &cobra.Command{
		Use:   "ufshare",
		Short: "UFShare - Artifact Repository Manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Finalize()
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
	cmd.Flags().StringVar(&cfg.NpmAddr, "npm-addr", "", "npm 专用端口（可选，如 0.0.0.0:4873）")
	cmd.Flags().StringVar(&cfg.FileAddr, "file-addr", "", "file-store 专用端口（可选，如 0.0.0.0:8001）")

	return cmd
}

func run(cfg *config.Config) error {
	if err := os.MkdirAll(cfg.Data, 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	db, err := database.Connect(cfg.Database.Driver, cfg.Database.DSN, cfg.Debug)
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	if err := model.AutoMigrate(db); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	authService    := service.NewAuthService(db, cfg.JWT.Secret, cfg.JWT.Expire)
	userService    := service.NewUserService(db)
	fileService    := service.NewFileService(db, cfg.Data)
	settingService := service.NewSettingService(db)
	npmService     := npmsvc.New(db, cfg.Data, settingService)

	if err := createDefaultAdmin(authService, userService); err != nil {
		log.Printf("warning: failed to create default admin: %v", err)
	}

	// ── 独立端口服务器 ─────────────────────────────────────────────────────────

	npmHandler  := handler.NewNpmHandler(npmService, authService)
	fileHandler := handler.NewFileHandler(fileService)

	npmDedicated := pkgserver.New("npm", func(app *ursa.App) {
		api.RegisterNpmRoutes(app, npmHandler, authService, "")
		api.RegisterNpmRoutes(app, npmHandler, authService, "/npm")
	})
	fileDedicated := pkgserver.New("file", func(app *ursa.App) {
		app.Get("/*path", fileHandler.Download)
		app.Put("/*path", middleware.Auth(authService), fileHandler.Upload)
		app.Delete("/*path", middleware.Auth(authService), fileHandler.Delete)
	})

	// 启动参数写入 settings（仅当 DB 中尚未配置时作为初始值）
	if cfg.NpmAddr != "" && settingService.GetNpmAddr() == "" {
		_ = settingService.Set(service.SettingNpmAddr, cfg.NpmAddr)
		_ = settingService.Set(service.SettingNpmEnabled, "true")
	}
	if cfg.FileAddr != "" && settingService.GetFileAddr() == "" {
		_ = settingService.Set(service.SettingFileAddr, cfg.FileAddr)
		_ = settingService.Set(service.SettingFileEnabled, "true")
	}

	// tryDedicated 根据 enabled + addr 决定启动/停止独立端口
	tryDedicated := func(d *pkgserver.Dedicated, enabled bool, addr string) {
		if enabled && addr != "" {
			d.Restart(addr)
		} else {
			d.Stop()
		}
	}

	// 启动时读取已保存配置
	tryDedicated(npmDedicated, settingService.GetNpmEnabled(), settingService.GetNpmAddr())
	tryDedicated(fileDedicated, settingService.GetFileEnabled(), settingService.GetFileAddr())

	// 监听配置变更，动态热重启独立端口
	settingService.OnChange(service.SettingNpmEnabled, func(_ string) {
		tryDedicated(npmDedicated, settingService.GetNpmEnabled(), settingService.GetNpmAddr())
	})
	settingService.OnChange(service.SettingNpmAddr, func(_ string) {
		tryDedicated(npmDedicated, settingService.GetNpmEnabled(), settingService.GetNpmAddr())
	})
	settingService.OnChange(service.SettingFileEnabled, func(_ string) {
		tryDedicated(fileDedicated, settingService.GetFileEnabled(), settingService.GetFileAddr())
	})
	settingService.OnChange(service.SettingFileAddr, func(_ string) {
		tryDedicated(fileDedicated, settingService.GetFileEnabled(), settingService.GetFileAddr())
	})

	// ── 主端口 ────────────────────────────────────────────────────────────────

	router := api.NewRouter(authService, userService, fileService, npmService, settingService, web.FS())

	appConfig := ursa.Config{BodyLimit: -1}
	if spaHandler := router.SPAHandler(); spaHandler != nil {
		appConfig.NotFoundHandler = spaHandler
	}
	app := ursa.New(appConfig)
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

	log.Println("default admin user created: admin/admin123")
	return nil
}

package main

import (
	"context"
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
	gosvc "gitea.loveuer.com/loveuer/ufshare/v2/internal/service/goproxy"
	mavensvc "gitea.loveuer.com/loveuer/ufshare/v2/internal/service/maven"
	npmsvc "gitea.loveuer.com/loveuer/ufshare/v2/internal/service/npm"
	ocisvc "gitea.loveuer.com/loveuer/ufshare/v2/internal/service/oci"
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

	cmd.Flags().StringVar(&cfg.Address, "address", cfg.Address, "监听地址 (e.g. 0.0.0.0:9817)")
	cmd.Flags().StringVar(&cfg.Data, "data", cfg.Data, "数据目录，存放文件和数据库")
	cmd.Flags().StringVar(&cfg.DBPath, "db", "", "SQLite 数据库文件路径（可选，默认为 <data>/ufshare.db）")
	cmd.Flags().BoolVar(&cfg.Debug, "debug", false, "开启 debug 模式（打印 GORM 日志及详细流程）")
	cmd.Flags().StringVar(&cfg.NpmAddr, "npm-addr", "", "npm 专用端口（可选，如 0.0.0.0:4873）")
	cmd.Flags().StringVar(&cfg.FileAddr, "file-addr", "", "file-store 专用端口（可选，如 0.0.0.0:8001）")
	cmd.Flags().StringVar(&cfg.GoAddr, "go-addr", "", "go 模块代理专用端口（可选，如 0.0.0.0:8081）")
	cmd.Flags().StringVar(&cfg.OciAddr, "oci-addr", "", "OCI/Docker 镜像代理专用端口（可选，如 0.0.0.0:5000）")
	cmd.Flags().StringVar(&cfg.MavenAddr, "maven-addr", "", "Maven 仓库专用端口（可选，如 0.0.0.0:8082）")

	// 添加子命令
	cmd.AddCommand(newInstallCmd())

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
	goService      := gosvc.New(cfg.Data, settingService)
	ociService     := ocisvc.New(db, cfg.Data, settingService)
	mavenService   := mavensvc.New(db, cfg.Data, settingService)

	if err := createDefaultAdmin(authService, userService); err != nil {
		log.Printf("warning: failed to create default admin: %v", err)
	}

	// ── 独立端口服务器 ─────────────────────────────────────────────────────────

	npmHandler   := handler.NewNpmHandler(npmService, authService)
	fileHandler  := handler.NewFileHandler(fileService)
	goHandler    := handler.NewGoHandler(goService, authService)
	ociHandler   := handler.NewOciHandler(ociService, authService)
	mavenHandler := handler.NewMavenHandler(mavenService, authService)

	npmDedicated := pkgserver.New("npm", cfg.BodySize, func(app *ursa.App) {
		api.RegisterNpmRoutes(app, npmHandler, authService, "")
		api.RegisterNpmRoutes(app, npmHandler, authService, "/npm")
	})
	fileDedicated := pkgserver.New("file", cfg.BodySize, func(app *ursa.App) {
		app.Get("/*path", fileHandler.Download)
		app.Put("/*path", middleware.Auth(authService), fileHandler.Upload)
		app.Delete("/*path", middleware.Auth(authService), fileHandler.Delete)
	})
	goDedicated := pkgserver.New("go", cfg.BodySize, func(app *ursa.App) {
		handler.RegisterGoRoutes(app, goHandler, authService, "")
	})
	ociDedicated := pkgserver.New("oci", cfg.BodySize, func(app *ursa.App) {
		api.RegisterOciRoutes(app, ociHandler, authService, "")
	})
	mavenDedicated := pkgserver.New("maven", cfg.BodySize, func(app *ursa.App) {
		api.RegisterMavenRoutes(app, mavenHandler, authService, "")
	})

	// CLI flag 显式指定时强制覆盖 DB 中的值，保证每次启动 flag 均生效
	bgCtx := context.Background()
	if cfg.NpmAddr != "" {
		_ = settingService.Set(bgCtx, service.SettingNpmAddr, cfg.NpmAddr)
		_ = settingService.Set(bgCtx, service.SettingNpmEnabled, "true")
	}
	if cfg.FileAddr != "" {
		_ = settingService.Set(bgCtx, service.SettingFileAddr, cfg.FileAddr)
		_ = settingService.Set(bgCtx, service.SettingFileEnabled, "true")
	}
	if cfg.GoAddr != "" {
		_ = settingService.Set(bgCtx, service.SettingGoAddr, cfg.GoAddr)
		_ = settingService.Set(bgCtx, service.SettingGoEnabled, "true")
	}
	if cfg.OciAddr != "" {
		_ = settingService.Set(bgCtx, service.SettingOciAddr, cfg.OciAddr)
		_ = settingService.Set(bgCtx, service.SettingOciEnabled, "true")
	}
	if cfg.MavenAddr != "" {
		_ = settingService.Set(bgCtx, service.SettingMavenAddr, cfg.MavenAddr)
		_ = settingService.Set(bgCtx, service.SettingMavenEnabled, "true")
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
	tryDedicated(goDedicated, settingService.GetGoEnabled(), settingService.GetGoAddr())
	tryDedicated(ociDedicated, settingService.GetOciEnabled(), settingService.GetOciAddr())
	tryDedicated(mavenDedicated, settingService.GetMavenEnabled(), settingService.GetMavenAddr())

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
	settingService.OnChange(service.SettingGoEnabled, func(_ string) {
		tryDedicated(goDedicated, settingService.GetGoEnabled(), settingService.GetGoAddr())
	})
	settingService.OnChange(service.SettingGoAddr, func(_ string) {
		tryDedicated(goDedicated, settingService.GetGoEnabled(), settingService.GetGoAddr())
	})
	settingService.OnChange(service.SettingOciEnabled, func(_ string) {
		tryDedicated(ociDedicated, settingService.GetOciEnabled(), settingService.GetOciAddr())
	})
	settingService.OnChange(service.SettingOciAddr, func(_ string) {
		tryDedicated(ociDedicated, settingService.GetOciEnabled(), settingService.GetOciAddr())
	})
	settingService.OnChange(service.SettingMavenEnabled, func(_ string) {
		tryDedicated(mavenDedicated, settingService.GetMavenEnabled(), settingService.GetMavenAddr())
	})
	settingService.OnChange(service.SettingMavenAddr, func(_ string) {
		tryDedicated(mavenDedicated, settingService.GetMavenEnabled(), settingService.GetMavenAddr())
	})

	// ── 主端口 ────────────────────────────────────────────────────────────────

	router := api.NewRouter(authService, userService, fileService, npmService, ociService, mavenService, settingService, web.FS())

	appConfig := ursa.Config{BodyLimit: cfg.BodySize}
	if spaHandler := router.SPAHandler(); spaHandler != nil {
		appConfig.NotFoundHandler = spaHandler
	}
	app := ursa.New(appConfig)
	router.Setup(app, goHandler)

	log.Printf("data dir : %s", cfg.Data)
	log.Printf("database : %s", cfg.Database.DSN)
	log.Printf("body limit: %s", formatBodySize(cfg.BodySize))
	log.Printf("listening: %s", cfg.Address)

	return app.Run(cfg.Address)
}

func formatBodySize(n int64) string {
	if n < 0 {
		return "unlimited"
	}
	units := []struct {
		thresh int64
		label  string
	}{
		{1 << 40, "TiB"}, {1 << 30, "GiB"}, {1 << 20, "MiB"}, {1 << 10, "KiB"},
	}
	for _, u := range units {
		if n >= u.thresh {
			return fmt.Sprintf("%.2g %s", float64(n)/float64(u.thresh), u.label)
		}
	}
	return fmt.Sprintf("%d B", n)
}

func createDefaultAdmin(authService *service.AuthService, userService *service.UserService) error {
	ctx := context.Background()
	user, err := authService.Register(ctx, "admin", "admin123", "admin@ufshare.local")
	if err != nil {
		if err == service.ErrUserExists {
			return nil
		}
		return err
	}

	if err := userService.SetAdmin(ctx, user.ID, true); err != nil {
		return err
	}

	log.Println("default admin user created: admin/admin123")
	return nil
}

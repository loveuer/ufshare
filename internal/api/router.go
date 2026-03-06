package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api/handler"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api/middleware"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

type Router struct {
	authService *service.AuthService
	userService *service.UserService
	permService *service.PermissionService
	webFS       fs.FS
}

func NewRouter(authService *service.AuthService, userService *service.UserService, permService *service.PermissionService, webFS fs.FS) *Router {
	return &Router{
		authService: authService,
		userService: userService,
		permService: permService,
		webFS:       webFS,
	}
}

func (r *Router) Setup(app *ursa.App) {
	// Handlers
	authHandler := handler.NewAuthHandler(r.authService)
	userHandler := handler.NewUserHandler(r.userService)
	moduleHandler := handler.NewModuleHandler(r.permService)
	permHandler := handler.NewPermissionHandler(r.permService)

	// 公开路由
	api := app.Group("/api/v1")

	// 认证
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)

	// 需要认证的路由
	protected := api.Group("", middleware.Auth(r.authService))
	protected.Get("/auth/me", authHandler.Me)

	// 管理员路由
	admin := api.Group("/admin", middleware.Auth(r.authService), middleware.AdminOnly())

	// 用户管理
	admin.Get("/users", userHandler.List)
	admin.Get("/users/:id", userHandler.Get)
	admin.Put("/users/:id", userHandler.Update)
	admin.Delete("/users/:id", userHandler.Delete)

	// 模块管理
	admin.Post("/modules", moduleHandler.Create)
	admin.Get("/modules", moduleHandler.List)
	admin.Get("/modules/:id", moduleHandler.Get)
	admin.Put("/modules/:id", moduleHandler.Update)
	admin.Delete("/modules/:id", moduleHandler.Delete)

	// 权限管理
	admin.Post("/permissions/grant", permHandler.Grant)
	admin.Post("/permissions/revoke", permHandler.Revoke)
	admin.Get("/permissions/user/:user_id", permHandler.GetUserPermissions)

	// 前端静态文件 + SPA fallback (通过 NoRoute 避免与 /api 路由冲突)
	if r.webFS != nil {
		fileServer := http.FileServer(http.FS(r.webFS))
		app.NoRoute(func(c *ursa.Ctx) error {
			path := c.Request.URL.Path
			// assets 及静态资源直接走 fileServer
			if strings.HasPrefix(path, "/assets/") || path == "/favicon.ico" {
				fileServer.ServeHTTP(c.Writer, c.Request)
				return nil
			}
			// SPA fallback：其余路径均返回 index.html
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
			return nil
		})
	}
}

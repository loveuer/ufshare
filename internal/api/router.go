package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api/handler"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api/middleware"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
	npmsvc "gitea.loveuer.com/loveuer/ufshare/v2/internal/service/npm"
)

type Router struct {
	authService *service.AuthService
	userService *service.UserService
	fileService *service.FileService
	npmService  *npmsvc.Service
	webFS       fs.FS
}

func NewRouter(
	authService *service.AuthService,
	userService *service.UserService,
	fileService *service.FileService,
	npmService *npmsvc.Service,
	webFS fs.FS,
) *Router {
	return &Router{
		authService: authService,
		userService: userService,
		fileService: fileService,
		npmService:  npmService,
		webFS:       webFS,
	}
}

// SPAHandler 返回用于 ursa.Config.NotFoundHandler 的 SPA fallback 处理器
func (r *Router) SPAHandler() ursa.HandlerFunc {
	if r.webFS == nil {
		return nil
	}
	fileServer := http.FileServer(http.FS(r.webFS))
	return func(c *ursa.Ctx) error {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/assets/") || path == "/favicon.ico" {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return nil
		}
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
		return nil
	}
}

func (r *Router) Setup(app *ursa.App) {
	authHandler := handler.NewAuthHandler(r.authService)
	userHandler := handler.NewUserHandler(r.userService)
	fileHandler := handler.NewFileHandler(r.fileService)
	npmHandler := handler.NewNpmHandler(r.npmService, r.authService)

	// ── REST API (/api/v1) ───────────────────────────────────────────────────

	api := app.Group("/api/v1")

	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	api.Get("/auth/me", middleware.Auth(r.authService), authHandler.Me)

	admin := api.Group("/admin", middleware.Auth(r.authService), middleware.AdminOnly())
	admin.Get("/users", userHandler.List)
	admin.Post("/users", userHandler.Create)
	admin.Get("/users/:id", userHandler.Get)
	admin.Put("/users/:id", userHandler.Update)
	admin.Delete("/users/:id", userHandler.Delete)

	// npm 管理接口（供前端使用，需认证）
	npmAdmin := api.Group("/npm", middleware.Auth(r.authService))
	npmAdmin.Get("/packages", npmHandler.ListPackages)
	npmAdmin.Get("/packages/:name", npmHandler.ListVersions)

	// ── file-store (读公开，写需认证) ─────────────────────────────────────────
	//   GET    /file-store          列出文件
	//   GET    /file-store/*path    下载
	//   PUT    /file-store/*path    上传
	//   DELETE /file-store/*path    删除
	app.Get("/file-store", fileHandler.List)
	app.Get("/file-store/*path", fileHandler.Download)
	app.Put("/file-store/*path", middleware.Auth(r.authService), fileHandler.Upload)
	app.Delete("/file-store/*path", middleware.Auth(r.authService), fileHandler.Delete)

	// ── npm registry (/npm) ───────────────────────────────────────────────────
	//
	// 注意：/-/ 前缀的特殊路由必须在参数路由之前注册，
	// 否则 :package 可能匹配到 "-"。
	//
	// 基础端点（公开）
	app.Get("/npm/-/ping", npmHandler.Ping)
	app.Get("/npm/-/whoami", middleware.Auth(r.authService), npmHandler.Whoami)
	// npm login: PUT /npm/-/user/org.couchdb.user:<username>
	app.Put("/npm/-/user/:id", npmHandler.Login)

	// 普通包（unscoped）
	//   GET    /npm/:package                   packument
	//   GET    /npm/:package/:version           version metadata
	//   GET    /npm/:package/-/:file            tarball 下载（本地缓存 + 代理）
	//   PUT    /npm/:package                    npm publish（需认证）
	app.Get("/npm/:package/-/:file", npmHandler.GetTarball)
	app.Get("/npm/:package/:version", npmHandler.GetVersion)
	app.Get("/npm/:package", npmHandler.GetPackument)
	app.Put("/npm/:package", middleware.Auth(r.authService), npmHandler.Publish)

	// Scoped 包（@scope/name）
	//   GET    /npm/@:scope/:name                packument
	//   GET    /npm/@:scope/:name/:version        version metadata
	//   GET    /npm/@:scope/:name/-/:file         tarball 下载
	//   PUT    /npm/@:scope/:name                 npm publish（需认证）
	app.Get("/npm/@:scope/:name/-/:file", npmHandler.GetTarball)
	app.Get("/npm/@:scope/:name/:version", npmHandler.GetVersion)
	app.Get("/npm/@:scope/:name", npmHandler.GetPackument)
	app.Put("/npm/@:scope/:name", middleware.Auth(r.authService), npmHandler.Publish)

	// ── 前端静态文件 + SPA fallback（由 ursa.Config.NotFoundHandler 处理）──────
}

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
	authService    *service.AuthService
	userService    *service.UserService
	fileService    *service.FileService
	npmService     *npmsvc.Service
	settingService *service.SettingService
	webFS          fs.FS
}

func NewRouter(
	authService *service.AuthService,
	userService *service.UserService,
	fileService *service.FileService,
	npmService *npmsvc.Service,
	settingService *service.SettingService,
	webFS fs.FS,
) *Router {
	return &Router{
		authService:    authService,
		userService:    userService,
		fileService:    fileService,
		npmService:     npmService,
		settingService: settingService,
		webFS:          webFS,
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
	authHandler    := handler.NewAuthHandler(r.authService)
	userHandler    := handler.NewUserHandler(r.userService)
	fileHandler    := handler.NewFileHandler(r.fileService)
	npmHandler     := handler.NewNpmHandler(r.npmService, r.authService)
	settingHandler := handler.NewSettingHandler(r.settingService)

	// ── REST API (/api/v1) ───────────────────────────────────────────────────

	api := app.Group("/api/v1")

	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	api.Get("/auth/me", middleware.Auth(r.authService), authHandler.Me)
	api.Put("/auth/password", middleware.Auth(r.authService), authHandler.ChangePassword)

	admin := api.Group("/admin", middleware.Auth(r.authService), middleware.AdminOnly())
	admin.Get("/users", userHandler.List)
	admin.Post("/users", userHandler.Create)
	admin.Get("/users/:id", userHandler.Get)
	admin.Put("/users/:id", userHandler.Update)
	admin.Put("/users/:id/password", userHandler.ResetPassword)
	admin.Delete("/users/:id", userHandler.Delete)

	admin.Get("/settings", settingHandler.GetAll)
	admin.Put("/settings", settingHandler.Update)

	// npm 管理接口（供前端使用，需认证）
	npmAdmin := api.Group("/npm", middleware.Auth(r.authService))
	npmAdmin.Get("/packages", npmHandler.ListPackages)
	npmAdmin.Get("/packages/:name", npmHandler.ListVersions)

	// ── file-store（主端口，带 /file-store 前缀）──────────────────────────────
	RegisterFileRoutes(app, fileHandler, r.authService, "/file-store")

	// ── npm registry（主端口，带 /npm 前缀）──────────────────────────────────
	RegisterNpmRoutes(app, npmHandler, r.authService, "/npm")

	// ── 前端静态文件 + SPA fallback（由 ursa.Config.NotFoundHandler 处理）──────
}

func RegisterNpmRoutes(app *ursa.App, npmHandler *handler.NpmHandler, auth *service.AuthService, prefix string) {
	// 基础端点（公开）
	app.Get(prefix+"/-/ping", npmHandler.Ping)
	app.Get(prefix+"/-/whoami", npmHandler.Whoami) // 移除 middleware
	// npm login: PUT {prefix}/-/user/org.couchdb.user:<username>
	app.Put(prefix+"/-/user/:id", npmHandler.Login)

	// Scoped 包（@scope/name）—— 必须在普通包路由之前注册
	//   GET    {prefix}/@:scope/:name                packument
	//   GET    {prefix}/@:scope/:name/:version        version metadata
	//   GET    {prefix}/@:scope/:name/-/:file         tarball 下载
	//   PUT    {prefix}/@:scope/:name                 npm publish（需认证）
	app.Get(prefix+"/@:scope/:name/-/:file", npmHandler.GetTarball)
	app.Get(prefix+"/@:scope/:name/:version", npmHandler.GetVersion)
	app.Get(prefix+"/@:scope/:name", npmHandler.GetPackument)
	app.Put(prefix+"/@:scope/:name", middleware.Auth(auth), npmHandler.Publish)

	// 普通包（unscoped）
	//   GET    {prefix}/:package                   packument
	//   GET    {prefix}/:package/:version           version metadata
	//   GET    {prefix}/:package/-/:file            tarball 下载（本地缓存 + 代理）
	//   PUT    {prefix}/:package                    npm publish（需认证）
	app.Get(prefix+"/:package/-/:file", npmHandler.GetTarball)
	app.Get(prefix+"/:package/:version", npmHandler.GetVersion)
	app.Get(prefix+"/:package", npmHandler.GetPackument)
	app.Put(prefix+"/:package", middleware.Auth(auth), npmHandler.Publish)
}

// RegisterFileRoutes 在 app 上以 prefix 为前缀注册 file-store 路由。
// prefix="/file-store" → 主端口用法（注册 /file-store 和 /file-store/*path）
// prefix=""            → 独立端口用法（只注册 /*path，因为 / 会被 /*path 覆盖）
func RegisterFileRoutes(app *ursa.App, fileHandler *handler.FileHandler, auth *service.AuthService, prefix string) {
	if fileHandler == nil {
		panic("fileHandler is nil!")
	}
	if prefix == "" {
		// 独立端口：只注册通配符路由
		app.Get("/*path", fileHandler.Download)
		app.Put("/*path", middleware.Auth(auth), fileHandler.Upload)
		app.Delete("/*path", middleware.Auth(auth), fileHandler.Delete)
	} else {
		// 主端口：注册具体路径 + 通配符
		app.Get(prefix, fileHandler.List)
		app.Get(prefix+"/*path", fileHandler.Download)
		app.Put(prefix+"/*path", middleware.Auth(auth), fileHandler.Upload)
		app.Delete(prefix+"/*path", middleware.Auth(auth), fileHandler.Delete)
	}
}

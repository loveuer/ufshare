package handler

import (
	"strings"

	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api/middleware"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
	gosvc "gitea.loveuer.com/loveuer/ufshare/v2/internal/service/goproxy"
)

// GoHandler 处理 Go 模块代理请求
type GoHandler struct {
	goSvc *gosvc.Service
	auth  *service.AuthService
}

// NewGoHandler 创建 Go 模块处理器
func NewGoHandler(goSvc *gosvc.Service, auth *service.AuthService) *GoHandler {
	return &GoHandler{goSvc: goSvc, auth: auth}
}

// Proxy 处理所有 Go 模块代理请求（用于主端口 /go 前缀）
// Go 模块代理协议包括:
//   - /github.com/gin-gonic/gin/@v/list: 获取版本列表
//   - /github.com/gin-gonic/gin/@v/{version}.info: 获取版本信息
//   - /github.com/gin-gonic/gin/@v/{version}.mod: 获取 go.mod 文件
//   - /github.com/gin-gonic/gin/@v/{version}.zip: 获取模块源码 zip
//   - /sumdb/*: 校验和数据库代理
func (h *GoHandler) Proxy(c *ursa.Ctx) error {
	// 当使用 /go 前缀时，需要去除前缀再传递给 goproxy
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/go/") {
		c.Request.URL.Path = path[3:] // 去除 "/go" 前缀，保留后面的 /
	}
	h.goSvc.ServeHTTP(c.Writer, c.Request)
	return nil
}

// ProxyDirect 直接代理请求（用于专用端口，不需要去除前缀）
func (h *GoHandler) ProxyDirect(c *ursa.Ctx) error {
	h.goSvc.ServeHTTP(c.Writer, c.Request)
	return nil
}

// GetStats 获取缓存统计信息（需要认证）
func (h *GoHandler) GetStats(c *ursa.Ctx) error {
	stats, err := h.goSvc.GetCacheStats()
	if err != nil {
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": err.Error()})
	}
	return c.JSON(ursa.Map{"code": 0, "message": "success", "data": stats})
}

// CleanCache 清理缓存（需要管理员权限）
func (h *GoHandler) CleanCache(c *ursa.Ctx) error {
	if err := h.goSvc.CleanCache(); err != nil {
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": err.Error()})
	}
	return c.JSON(ursa.Map{"code": 0, "message": "cache cleaned"})
}

// RegisterGoRoutes 注册 Go 模块代理路由
// prefix="/go" → 主端口用法
// prefix=""    → 独立端口用法
func RegisterGoRoutes(app *ursa.App, handler *GoHandler, auth *service.AuthService, prefix string) {
	if prefix == "" {
		// 独立端口：所有请求都走代理（使用通配符捕获所有路径）
		// 专用端口直接使用 ProxyDirect，不需要去除前缀
		app.Get("/*path", handler.ProxyDirect)
	} else {
		// 主端口：只注册 Go 模块代理端点（公开访问）
		// 管理接口通过 /api/v1/go/* 注册，避免路由冲突
		app.Get(prefix+"/*path", handler.Proxy)
	}
}

// RegisterGoAdminRoutes 注册 Go 模块管理接口（在 /api/v1 下）
func RegisterGoAdminRoutes(api *ursa.RouterGroup, handler *GoHandler, auth *service.AuthService) {
	// Go 模块管理接口
	goAdmin := api.Group("/go", middleware.Auth(auth))
	goAdmin.Get("/stats", handler.GetStats)
	goAdmin.Delete("/cache", middleware.AdminOnly(), handler.CleanCache)
}

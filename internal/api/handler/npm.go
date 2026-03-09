package handler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api/middleware"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
	npmsvc "gitea.loveuer.com/loveuer/ufshare/v2/internal/service/npm"
)

// NpmHandler 处理所有 npm registry 兼容端点
type NpmHandler struct {
	npm  *npmsvc.Service
	auth *service.AuthService
}

func NewNpmHandler(npm *npmsvc.Service, auth *service.AuthService) *NpmHandler {
	return &NpmHandler{npm: npm, auth: auth}
}

// ── 管理接口（供前端使用）────────────────────────────────────────────────────

// ListPackages GET /api/v1/npm/packages?page=1&page_size=20&search=
func (h *NpmHandler) ListPackages(c *ursa.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	search := c.Query("search")

	list, total, err := h.npm.ListPackages(page, pageSize, search)
	if err != nil {
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": err.Error()})
	}
	return c.JSON(ursa.Map{
		"code": 0, "message": "success",
		"data":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ListVersions GET /api/v1/npm/packages/:name
func (h *NpmHandler) ListVersions(c *ursa.Ctx) error {
	name := c.Param("name")
	// scoped 包通过 query 传入（避免路由歧义）
	if scope := c.Query("scope"); scope != "" {
		name = "@" + scope + "/" + name
	}
	versions, err := h.npm.ListVersions(name)
	if err != nil {
		if errors.Is(err, npmsvc.ErrPackageNotFound) {
			return c.Status(404).JSON(ursa.Map{"code": 404, "message": "package not found"})
		}
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": err.Error()})
	}
	return c.JSON(ursa.Map{"code": 0, "message": "success", "data": versions})
}

// ── 基础端点 ──────────────────────────────────────────────────────────────────

// Ping GET /npm/-/ping
func (h *NpmHandler) Ping(c *ursa.Ctx) error {
	return c.JSON(ursa.Map{})
}

// Login PUT /npm/-/user/:id  (npm login / npm adduser)
// npm 发送: {"name":"user","password":"pass","type":"user"}
// 我们复用现有 AuthService.Login，返回 JWT token
func (h *NpmHandler) Login(c *ursa.Ctx) error {
	var body struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ursa.Map{"error": "invalid request body"})
	}

	token, user, err := h.auth.Login(body.Name, body.Password)
	if err != nil {
		return c.Status(401).JSON(ursa.Map{"error": "unauthorized"})
	}

	return c.JSON(ursa.Map{
		"ok":    true,
		"id":    fmt.Sprintf("org.couchdb.user:%s", user.Username),
		"rev":   "_we_dont_use_revs_anymore",
		"token": token,
	})
}

// Whoami GET /npm/-/whoami
func (h *NpmHandler) Whoami(c *ursa.Ctx) error {
	return c.JSON(ursa.Map{"username": middleware.GetUsername(c)})
}

// ── 包元数据 ──────────────────────────────────────────────────────────────────

// GetPackument GET /npm/:package  (或 /npm/@:scope/:name)
// 本地优先，缺失时代理上游并缓存
func (h *NpmHandler) GetPackument(c *ursa.Ctx) error {
	name := resolvePkgName(c)
	baseURL := resolveBaseURL(c)
	abbreviated := strings.Contains(c.Request.Header.Get("Accept"), "vnd.npm.install-v1+json")

	pack, err := h.npm.GetPackument(name, baseURL, abbreviated)
	if err != nil {
		if errors.Is(err, npmsvc.ErrPackageNotFound) {
			return c.Status(404).JSON(ursa.Map{"error": "package not found"})
		}
		return c.Status(502).JSON(ursa.Map{"error": err.Error()})
	}

	if abbreviated {
		c.Set("Content-Type", "application/vnd.npm.install-v1+json")
	}
	return c.JSON(pack)
}

// GetVersion GET /npm/:package/:version  (或 /npm/@:scope/:name/:version)
func (h *NpmHandler) GetVersion(c *ursa.Ctx) error {
	name := resolvePkgName(c)
	version := c.Param("version")

	meta, err := h.npm.GetVersion(name, version)
	if err != nil {
		if errors.Is(err, npmsvc.ErrPackageNotFound) || errors.Is(err, npmsvc.ErrVersionNotFound) {
			return c.Status(404).JSON(ursa.Map{"error": "version not found"})
		}
		return c.Status(500).JSON(ursa.Map{"error": err.Error()})
	}

	c.Set("Content-Type", "application/json")
	_, err = c.Writer.Write(meta)
	return err
}

// ── tarball 下载 ──────────────────────────────────────────────────────────────

// GetTarball GET /npm/:package/-/:file  (或 /npm/@:scope/:name/-/:file)
// 已缓存时直接从磁盘返回；未缓存时流式从上游拉取（边下边传），同时写入本地缓存。
func (h *NpmHandler) GetTarball(c *ursa.Ctx) error {
	name := resolvePkgName(c)
	filename := c.Param("file")

	c.Set("Content-Type", "application/octet-stream")
	if err := h.npm.ServeTarball(name, filename, c.Writer); err != nil {
		if errors.Is(err, npmsvc.ErrTarballNotFound) {
			return c.Status(404).JSON(ursa.Map{"error": "tarball not found"})
		}
		return c.Status(502).JSON(ursa.Map{"error": err.Error()})
	}
	return nil
}

// ── 发布 ──────────────────────────────────────────────────────────────────────

// Publish PUT /npm/:package  (或 /npm/@:scope/:name)
// 解析 npm publish 请求体，保存 tarball + 元数据
func (h *NpmHandler) Publish(c *ursa.Ctx) error {
	var body npmsvc.PublishBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ursa.Map{"error": "invalid request body"})
	}

	uploaderID := middleware.GetUserID(c)
	uploaderName := middleware.GetUsername(c)

	if err := h.npm.Publish(&body, uploaderID, uploaderName); err != nil {
		if errors.Is(err, npmsvc.ErrVersionExists) {
			return c.Status(409).JSON(ursa.Map{"error": err.Error()})
		}
		return c.Status(500).JSON(ursa.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(ursa.Map{"ok": true})
}

// ── 路由辅助 ──────────────────────────────────────────────────────────────────

// resolvePkgName 从路由参数中还原包名。
// 普通包：:package → "lodash"
// Scoped 包：:scope + :name → "@babel/core"
func resolvePkgName(c *ursa.Ctx) string {
	if scope := c.Param("scope"); scope != "" {
		return "@" + scope + "/" + c.Param("name")
	}
	return c.Param("package")
}

// resolveBaseURL 从请求中推断 baseURL（用于改写 tarball URL）
func resolveBaseURL(c *ursa.Ctx) string {
	scheme := c.Request.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := c.Request.Host
	if fwdHost := c.Request.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}
	baseURL := scheme + "://" + host
	// 如果 url 包含端口或在反向代理后，去掉尾部斜杠
	return strings.TrimRight(baseURL, "/")
}

package handler

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
	ocisvc "gitea.loveuer.com/loveuer/ufshare/v2/internal/service/oci"
)

// OciHandler 处理 OCI Distribution API 和管理接口
type OciHandler struct {
	oci  *ocisvc.Service
	auth *service.AuthService
}

func NewOciHandler(oci *ocisvc.Service, auth *service.AuthService) *OciHandler {
	return &OciHandler{oci: oci, auth: auth}
}

// ── OCI Distribution API ──────────────────────────────────────────────────────

// V2Check GET /v2/
// Docker client 首先调用此端点检查 API 版本兼容性
func (h *OciHandler) V2Check(c *ursa.Ctx) error {
	c.Set("Docker-Distribution-API-Version", "registry/2.0")
	return c.JSON(ursa.Map{})
}

// DispatchGet GET /v2/*path
// 通配符路由，解析 path 后分发到对应 handler
func (h *OciHandler) DispatchGet(c *ursa.Ctx) error {
	return h.dispatch(c, false)
}

// DispatchHead HEAD /v2/*path
func (h *OciHandler) DispatchHead(c *ursa.Ctx) error {
	return h.dispatch(c, true)
}

func (h *OciHandler) dispatch(c *ursa.Ctx, headOnly bool) error {
	path := c.Param("path")

	// 调试：用 URL path 直接解析
	urlPath := c.Request.URL.Path
	// 去掉 /v2 前缀
	if idx := strings.Index(urlPath, "/v2"); idx >= 0 {
		path = urlPath[idx+3:] // 去掉 /v2
	}
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")

	// 空 path 等同于 /v2/ (版本检查)
	if path == "" {
		return h.V2Check(c)
	}

	// _catalog
	if path == "_catalog" {
		return h.catalog(c)
	}

	// 解析 path：<name>/manifests/<ref> 或 <name>/blobs/<digest> 或 <name>/tags/list
	// name 可能包含 /（如 library/nginx）
	// 从后往前找 action 关键词

	if idx := strings.LastIndex(path, "/manifests/"); idx > 0 {
		name := path[:idx]
		ref := path[idx+len("/manifests/"):]
		if headOnly {
			return h.headManifest(c, name, ref)
		}
		return h.getManifest(c, name, ref)
	}

	if idx := strings.LastIndex(path, "/blobs/"); idx > 0 {
		name := path[:idx]
		digest := path[idx+len("/blobs/"):]
		if headOnly {
			return h.headBlob(c, name, digest)
		}
		return h.getBlob(c, name, digest)
	}

	if strings.HasSuffix(path, "/tags/list") {
		name := path[:len(path)-len("/tags/list")]
		return h.listTags(c, name)
	}

	return c.Status(404).JSON(ursa.Map{"errors": []ursa.Map{{"code": "NAME_UNKNOWN", "message": "unknown endpoint"}}})
}

// Catalog GET /v2/_catalog
func (h *OciHandler) Catalog(c *ursa.Ctx) error {
	return h.catalog(c)
}

func (h *OciHandler) catalog(c *ursa.Ctx) error {
	repos, err := h.oci.ListCatalog(c.Request.Context())
	if err != nil {
		return c.Status(500).JSON(ociError("UNKNOWN", err.Error()))
	}
	if repos == nil {
		repos = []string{}
	}
	return c.JSON(ursa.Map{"repositories": repos})
}

// getManifest GET /v2/<name>/manifests/<reference>
func (h *OciHandler) getManifest(c *ursa.Ctx, name, reference string) error {
	ctx := c.Request.Context()

	// 先查本地
	content, mediaType, digest, err := h.oci.GetManifest(ctx, name, reference)
	if err != nil {
		// 本地没有，代理上游
		content, mediaType, digest, err = h.oci.ProxyManifest(ctx, name, reference)
		if err != nil {
			if errors.Is(err, ocisvc.ErrManifestNotFound) {
				return c.Status(404).JSON(ociError("MANIFEST_UNKNOWN", "manifest unknown"))
			}
			return c.Status(502).JSON(ociError("UNKNOWN", err.Error()))
		}
	}

	c.Set("Content-Type", mediaType)
	c.Set("Docker-Content-Digest", digest)
	c.Set("Content-Length", strconv.Itoa(len(content)))
	c.Set("Docker-Distribution-API-Version", "registry/2.0")
	_, writeErr := c.Writer.Write(content)
	return writeErr
}

// headManifest HEAD /v2/<name>/manifests/<reference>
func (h *OciHandler) headManifest(c *ursa.Ctx, name, reference string) error {
	ctx := c.Request.Context()

	size, mediaType, digest, ok := h.oci.ManifestExists(ctx, name, reference)
	if !ok {
		// 尝试代理
		content, mt, d, err := h.oci.ProxyManifest(ctx, name, reference)
		if err != nil {
			return c.Status(404).JSON(ociError("MANIFEST_UNKNOWN", "manifest unknown"))
		}
		size = int64(len(content))
		mediaType = mt
		digest = d
	}

	c.Set("Content-Type", mediaType)
	c.Set("Docker-Content-Digest", digest)
	c.Set("Content-Length", strconv.FormatInt(size, 10))
	c.Set("Docker-Distribution-API-Version", "registry/2.0")
	return c.SendStatus(200)
}

// getBlob GET /v2/<name>/blobs/<digest>
func (h *OciHandler) getBlob(c *ursa.Ctx, name, digest string) error {
	ctx := c.Request.Context()

	// 先查本地
	rc, size, err := h.oci.GetBlob(ctx, name, digest)
	if err == nil {
		defer rc.Close()
		c.Set("Content-Type", "application/octet-stream")
		c.Set("Docker-Content-Digest", digest)
		c.Set("Content-Length", strconv.FormatInt(size, 10))
		_, copyErr := io.Copy(c.Writer, rc)
		return copyErr
	}

	// 代理上游，流式返回
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Docker-Content-Digest", digest)
	blobSize, proxyErr := h.oci.ProxyBlob(ctx, name, digest, c.Writer)
	if proxyErr != nil {
		if errors.Is(proxyErr, ocisvc.ErrBlobNotFound) {
			return c.Status(404).JSON(ociError("BLOB_UNKNOWN", "blob unknown"))
		}
		return c.Status(502).JSON(ociError("UNKNOWN", proxyErr.Error()))
	}
	_ = blobSize
	return nil
}

// headBlob HEAD /v2/<name>/blobs/<digest>
func (h *OciHandler) headBlob(c *ursa.Ctx, name, digest string) error {
	size, ok := h.oci.BlobExists(c.Request.Context(), digest)
	if !ok {
		return c.Status(404).JSON(ociError("BLOB_UNKNOWN", "blob unknown"))
	}

	c.Set("Content-Type", "application/octet-stream")
	c.Set("Docker-Content-Digest", digest)
	c.Set("Content-Length", strconv.FormatInt(size, 10))
	return c.SendStatus(200)
}

// listTags GET /v2/<name>/tags/list
func (h *OciHandler) listTags(c *ursa.Ctx, name string) error {
	tags, err := h.oci.ListTags(c.Request.Context(), name)
	if err != nil {
		return c.Status(500).JSON(ociError("UNKNOWN", err.Error()))
	}
	if tags == nil {
		tags = []string{}
	}
	return c.JSON(ursa.Map{"name": name, "tags": tags})
}

// ── 管理 API ──────────────────────────────────────────────────────────────────

// ListRepositories GET /api/v1/oci/repositories
func (h *OciHandler) ListRepositories(c *ursa.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	search := c.Query("search")

	repos, total, err := h.oci.ListRepositories(c.Request.Context(), page, pageSize, search)
	if err != nil {
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": err.Error()})
	}
	return c.JSON(ursa.Map{
		"code": 0, "message": "success",
		"data":      repos,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ListRepoTags GET /api/v1/oci/repositories/:name/tags
func (h *OciHandler) ListRepoTags(c *ursa.Ctx) error {
	// name 可能含 /，通过 query param 传递
	name := c.Query("name")
	if name == "" {
		name = c.Param("name")
	}
	tags, err := h.oci.ListTagsForRepo(c.Request.Context(), name)
	if err != nil {
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": err.Error()})
	}
	return c.JSON(ursa.Map{"code": 0, "message": "success", "data": tags})
}

// DeleteRepository DELETE /api/v1/oci/repositories/:id
func (h *OciHandler) DeleteRepository(c *ursa.Ctx) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(ursa.Map{"code": 400, "message": "invalid id"})
	}
	if err := h.oci.DeleteRepository(c.Request.Context(), uint(id)); err != nil {
		if errors.Is(err, ocisvc.ErrRepoNotFound) {
			return c.Status(404).JSON(ursa.Map{"code": 404, "message": "repository not found"})
		}
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": err.Error()})
	}
	return c.JSON(ursa.Map{"code": 0, "message": "deleted"})
}

// GetStats GET /api/v1/oci/stats
func (h *OciHandler) GetStats(c *ursa.Ctx) error {
	stats, err := h.oci.GetStats(c.Request.Context())
	if err != nil {
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": err.Error()})
	}
	return c.JSON(ursa.Map{"code": 0, "message": "success", "data": stats})
}

// CleanCache DELETE /api/v1/oci/cache
func (h *OciHandler) CleanCache(c *ursa.Ctx) error {
	if err := h.oci.CleanCache(c.Request.Context()); err != nil {
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": err.Error()})
	}
	return c.JSON(ursa.Map{"code": 0, "message": "cache cleaned"})
}

// ── 辅助 ────────────────────────────────────────────────────────────────────

func ociError(code, message string) ursa.Map {
	return ursa.Map{
		"errors": []ursa.Map{{
			"code":    code,
			"message": message,
		}},
	}
}

package handler

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service/maven"
)

// MavenHandler Maven 仓库 HTTP 处理器
type MavenHandler struct {
	service *maven.Service
	authSvc *service.AuthService
}

// NewMavenHandler 创建 Maven 处理器
func NewMavenHandler(service *maven.Service, authSvc *service.AuthService) *MavenHandler {
	return &MavenHandler{
		service: service,
		authSvc: authSvc,
	}
}

// GetArtifact 处理 GET 请求，获取制品文件
// 路径格式: /maven/{group}/{artifact}/{version}/{filename}
func (h *MavenHandler) GetArtifact(c *ursa.Ctx) error {
	path := c.Request.URL.Path
	// 移除 /maven 前缀
	path = strings.TrimPrefix(path, "/maven")
	path = strings.TrimPrefix(path, "/")

	// 检查是否是 maven-metadata.xml
	if strings.HasSuffix(path, "maven-metadata.xml") {
		return h.getMetadata(c, path)
	}

	// 解析 GAV
	groupID, artifactID, version, filename, err := parseMavenPath(path)
	if err != nil {
		return c.Status(http.StatusBadRequest).SendString(err.Error())
	}

	// 检查是否是 SNAPSHOT 版本
	if maven.IsSnapshotVersion(version) {
		return h.getSnapshotArtifact(c, groupID, artifactID, version, filename)
	}

	// 获取文件（使用多仓库回退）
	reader, size, localPath, err := h.service.GetArtifactFileWithFallback(c.Request.Context(), path)
	if err != nil {
		if err == maven.ErrFileNotFound {
			return c.Status(http.StatusNotFound).SendString("Not found")
		}
		return c.Status(http.StatusInternalServerError).SendString(err.Error())
	}
	defer reader.Close()

	// 设置 Content-Type
	contentType := detectContentType(localPath)
	c.Set("Content-Type", contentType)
	c.Set("Content-Length", strconv.FormatInt(size, 10))

	// 传输文件
	_, err = io.Copy(c.Writer, reader)
	return err
}

// getSnapshotArtifact 获取 SNAPSHOT 版本文件
func (h *MavenHandler) getSnapshotArtifact(c *ursa.Ctx, groupID, artifactID, version, filename string) error {
	// 检查是否是 metadata 请求
	if filename == "maven-metadata.xml" {
		return h.getSnapshotMetadata(c, groupID, artifactID, version)
	}

	// 获取 SNAPSHOT 文件
	reader, size, localPath, err := h.service.GetSnapshotFile(c.Request.Context(), groupID, artifactID, version, filename)
	if err != nil {
		if err == maven.ErrFileNotFound {
			return c.Status(http.StatusNotFound).SendString("Not found")
		}
		return c.Status(http.StatusInternalServerError).SendString(err.Error())
	}
	defer reader.Close()

	// 设置 Content-Type
	contentType := detectContentType(localPath)
	c.Set("Content-Type", contentType)
	c.Set("Content-Length", strconv.FormatInt(size, 10))

	// 传输文件
	_, err = io.Copy(c.Writer, reader)
	return err
}

// getSnapshotMetadata 获取 SNAPSHOT metadata
func (h *MavenHandler) getSnapshotMetadata(c *ursa.Ctx, groupID, artifactID, version string) error {
	reader, size, err := h.service.GetSnapshotMetadata(c.Request.Context(), groupID, artifactID, version)
	if err != nil {
		// 尝试生成本地 metadata
		data, genErr := h.service.GenerateSnapshotMetadata(c.Request.Context(), groupID, artifactID, version)
		if genErr != nil {
			return c.Status(http.StatusNotFound).SendString("Not found")
		}
		c.Set("Content-Type", "application/xml")
		c.Set("Content-Length", strconv.Itoa(len(data)))
		_, err = c.Writer.Write(data)
		return err
	}
	defer reader.Close()

	c.Set("Content-Type", "application/xml")
	c.Set("Content-Length", strconv.FormatInt(size, 10))

	_, err = io.Copy(c.Writer, reader)
	return err
}

// getMetadata 获取 maven-metadata.xml
func (h *MavenHandler) getMetadata(c *ursa.Ctx, path string) error {
	// 解析路径获取 groupId 和 artifactId
	// 路径格式: group/id/artifact/maven-metadata.xml
	parts := strings.Split(strings.TrimSuffix(path, "/maven-metadata.xml"), "/")
	if len(parts) < 2 {
		return c.Status(http.StatusBadRequest).SendString("Invalid path")
	}

	artifactID := parts[len(parts)-1]
	groupParts := parts[:len(parts)-1]
	groupID := strings.Join(groupParts, ".")

	// 使用多仓库回退获取 metadata
	reader, size, err := h.service.GetMetadataWithFallback(c.Request.Context(), groupID, artifactID)
	if err != nil {
		// 尝试生成本地 metadata
		data, genErr := h.service.GenerateMetadata(c.Request.Context(), groupID, artifactID)
		if genErr != nil {
			if err == maven.ErrArtifactNotFound {
				return c.Status(http.StatusNotFound).SendString("Not found")
			}
			return c.Status(http.StatusInternalServerError).SendString(err.Error())
		}
		c.Set("Content-Type", "application/xml")
		c.Set("Content-Length", strconv.Itoa(len(data)))
		_, err = c.Writer.Write(data)
		return err
	}
	defer reader.Close()

	c.Set("Content-Type", "application/xml")
	c.Set("Content-Length", strconv.FormatInt(size, 10))

	_, err = io.Copy(c.Writer, reader)
	return err
}

// PutArtifact 处理 PUT 请求，上传制品文件
// 路径格式: /maven/{group}/{artifact}/{version}/{filename}
func (h *MavenHandler) PutArtifact(c *ursa.Ctx) error {
	// 解析认证信息
	userID, username, err := h.resolveAuth(c)
	if err != nil {
		c.Set("WWW-Authenticate", `Basic realm="UFShare Maven Repository"`)
		return c.Status(http.StatusUnauthorized).SendString("Unauthorized")
	}

	path := c.Request.URL.Path
	path = strings.TrimPrefix(path, "/maven")
	path = strings.TrimPrefix(path, "/")

	// 解析 GAV
	groupID, artifactID, version, filename, err := parseMavenPath(path)
	if err != nil {
		return c.Status(http.StatusBadRequest).SendString(err.Error())
	}

	// 解析 classifier 和 extension
	ext := filepath.Ext(filename)
	if ext != "" {
		ext = ext[1:]
	}

	classifier := ""
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	prefix := artifactID + "-" + version
	if strings.HasPrefix(baseName, prefix+"-") {
		classifier = strings.TrimPrefix(baseName, prefix+"-")
	}

	// 上传文件
	opts := maven.UploadOptions{
		GroupID:    groupID,
		ArtifactID: artifactID,
		Version:    version,
		Filename:   filename,
		Classifier: classifier,
		Extension:  ext,
		UploaderID: userID,
		Uploader:   username,
	}

	// 检查是否是 SNAPSHOT 版本
	if maven.IsSnapshotVersion(version) {
		if err := h.service.UploadSnapshot(c.Request.Context(), opts, c.Request.Body); err != nil {
			return c.Status(http.StatusInternalServerError).SendString(err.Error())
		}
	} else {
		if err := h.service.UploadFile(c.Request.Context(), opts, c.Request.Body); err != nil {
			if err == maven.ErrVersionExists {
				return c.Status(http.StatusConflict).SendString("Version already exists")
			}
			return c.Status(http.StatusInternalServerError).SendString(err.Error())
		}
	}

	// 如果是 POM 文件上传，更新 metadata
	if ext == "pom" {
		h.service.UpdateMetadata(c.Request.Context(), groupID, artifactID)
	}

	return c.Status(http.StatusCreated).SendString("Created")
}

// DeleteArtifact 处理 DELETE 请求，删除制品
func (h *MavenHandler) DeleteArtifact(c *ursa.Ctx) error {
	// 解析认证信息
	_, _, err := h.resolveAuth(c)
	if err != nil {
		c.Set("WWW-Authenticate", `Basic realm="UFShare Maven Repository"`)
		return c.Status(http.StatusUnauthorized).SendString("Unauthorized")
	}

	path := c.Request.URL.Path
	path = strings.TrimPrefix(path, "/maven")
	path = strings.TrimPrefix(path, "/")

	// 解析 GAV
	groupID, artifactID, version, filename, err := parseMavenPath(path)
	if err != nil {
		return c.Status(http.StatusBadRequest).SendString(err.Error())
	}

	// 如果指定了文件名，只删除该文件
	if filename != "" {
		err = h.service.DeleteArtifactFile(c.Request.Context(), groupID, artifactID, version, filename)
	} else {
		// 否则删除整个版本
		err = h.service.DeleteArtifact(c.Request.Context(), groupID, artifactID, version)
	}

	if err != nil {
		if err == maven.ErrArtifactNotFound || err == maven.ErrFileNotFound {
			return c.Status(http.StatusNotFound).SendString("Not found")
		}
		return c.Status(http.StatusInternalServerError).SendString(err.Error())
	}

	return c.Status(http.StatusNoContent).SendString("")
}

// HeadArtifact 处理 HEAD 请求，检查文件是否存在
func (h *MavenHandler) HeadArtifact(c *ursa.Ctx) error {
	path := c.Request.URL.Path
	path = strings.TrimPrefix(path, "/maven")
	path = strings.TrimPrefix(path, "/")

	// 检查是否是 maven-metadata.xml
	if strings.HasSuffix(path, "maven-metadata.xml") {
		// 解析路径获取 groupId 和 artifactId
		parts := strings.Split(strings.TrimSuffix(path, "/maven-metadata.xml"), "/")
		if len(parts) < 2 {
			return c.Status(http.StatusBadRequest).SendString("Invalid path")
		}

		artifactID := parts[len(parts)-1]
		groupParts := parts[:len(parts)-1]
		groupID := strings.Join(groupParts, ".")

		reader, size, err := h.service.GetMetadata(c.Request.Context(), groupID, artifactID)
		if err != nil {
			if err == maven.ErrArtifactNotFound {
				return c.Status(http.StatusNotFound).SendString("Not found")
			}
			return c.Status(http.StatusInternalServerError).SendString(err.Error())
		}
		reader.Close()

		c.Set("Content-Type", "application/xml")
		c.Set("Content-Length", strconv.FormatInt(size, 10))
		return c.Status(http.StatusOK).SendString("")
	}

	// 获取文件信息
	reader, size, _, err := h.service.GetArtifactFile(c.Request.Context(), path)
	if err != nil {
		if err == maven.ErrFileNotFound {
			return c.Status(http.StatusNotFound).SendString("Not found")
		}
		return c.Status(http.StatusInternalServerError).SendString(err.Error())
	}
	reader.Close()

	c.Set("Content-Length", strconv.FormatInt(size, 10))
	return c.Status(http.StatusOK).SendString("")
}

// ListArtifacts 列出制品（管理接口）
func (h *MavenHandler) ListArtifacts(c *ursa.Ctx) error {
	groupID := c.Query("group_id")
	artifactID := c.Query("artifact_id")
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	artifacts, total, err := h.service.ListArtifacts(c.Request.Context(), groupID, artifactID, page, pageSize)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ursa.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(ursa.Map{
		"data":      artifacts,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetArtifactDetail 获取制品详情（管理接口）
func (h *MavenHandler) GetArtifactDetail(c *ursa.Ctx) error {
	groupID := c.Query("group_id")
	artifactID := c.Query("artifact_id")
	version := c.Query("version")

	if groupID == "" || artifactID == "" || version == "" {
		return c.Status(http.StatusBadRequest).JSON(ursa.Map{
			"error": "group_id, artifact_id and version are required",
		})
	}

	artifact, err := h.service.GetArtifact(c.Request.Context(), groupID, artifactID, version)
	if err != nil {
		if err == maven.ErrArtifactNotFound {
			return c.Status(http.StatusNotFound).JSON(ursa.Map{
				"error": "Artifact not found",
			})
		}
		return c.Status(http.StatusInternalServerError).JSON(ursa.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(artifact)
}

// SearchArtifacts 搜索制品（管理接口）
func (h *MavenHandler) SearchArtifacts(c *ursa.Ctx) error {
	query := c.Query("q")
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	artifacts, total, err := h.service.SearchArtifacts(c.Request.Context(), query, page, pageSize)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ursa.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(ursa.Map{
		"data":      artifacts,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetVersions 获取制品的所有版本（管理接口）
func (h *MavenHandler) GetVersions(c *ursa.Ctx) error {
	groupID := c.Query("group_id")
	artifactID := c.Query("artifact_id")

	if groupID == "" || artifactID == "" {
		return c.Status(http.StatusBadRequest).JSON(ursa.Map{
			"error": "group_id and artifact_id are required",
		})
	}

	versions, err := h.service.GetVersions(c.Request.Context(), groupID, artifactID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ursa.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(ursa.Map{
		"group_id":    groupID,
		"artifact_id": artifactID,
		"versions":    versions,
	})
}

// ListRepositories 列出 Maven 仓库（管理接口）
func (h *MavenHandler) ListRepositories(c *ursa.Ctx) error {
	repos, err := h.service.GetRepositories(c.Request.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ursa.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(ursa.Map{
		"data": repos,
	})
}

// AddRepository 添加 Maven 仓库（管理接口）
func (h *MavenHandler) AddRepository(c *ursa.Ctx) error {
	var config maven.RepositoryConfig
	if err := c.BodyParser(&config); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ursa.Map{
			"error": "Invalid request body",
		})
	}

	if config.Name == "" || config.URL == "" {
		return c.Status(http.StatusBadRequest).JSON(ursa.Map{
			"error": "name and url are required",
		})
	}

	repo, err := h.service.AddRepository(c.Request.Context(), config)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ursa.Map{
			"error": err.Error(),
		})
	}

	return c.Status(http.StatusCreated).JSON(repo)
}

// UpdateRepository 更新 Maven 仓库（管理接口）
func (h *MavenHandler) UpdateRepository(c *ursa.Ctx) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(ursa.Map{
			"error": "Invalid repository ID",
		})
	}

	var config maven.RepositoryConfig
	if err := c.BodyParser(&config); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ursa.Map{
			"error": "Invalid request body",
		})
	}

	if err := h.service.UpdateRepository(c.Request.Context(), uint(id), config); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ursa.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(ursa.Map{
		"message": "Repository updated",
	})
}

// DeleteRepository 删除 Maven 仓库（管理接口）
func (h *MavenHandler) DeleteRepository(c *ursa.Ctx) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(ursa.Map{
			"error": "Invalid repository ID",
		})
	}

	if err := h.service.DeleteRepository(c.Request.Context(), uint(id)); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ursa.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(ursa.Map{
		"message": "Repository deleted",
	})
}

// resolveAuth 解析认证信息
func (h *MavenHandler) resolveAuth(c *ursa.Ctx) (uint, string, error) {
	auth := c.Request.Header.Get("Authorization")
	if auth == "" {
		return 0, "", fmt.Errorf("no authorization header")
	}

	// 支持 Basic 认证
	if strings.HasPrefix(auth, "Basic ") {
		username, password, ok := c.Request.BasicAuth()
		if !ok {
			return 0, "", fmt.Errorf("invalid basic auth")
		}

		user, err := h.authSvc.VerifyCredentials(c.Request.Context(), username, password)
		if err != nil {
			return 0, "", err
		}

		return user.ID, user.Username, nil
	}

	// 支持 Bearer 认证（JWT）
	if strings.HasPrefix(auth, "Bearer ") {
		token := auth[7:]
		claims, err := h.authSvc.ValidateToken(token)
		if err != nil {
			return 0, "", err
		}

		return claims.UserID, claims.Username, nil
	}

	return 0, "", fmt.Errorf("unsupported auth scheme")
}

// parseMavenPath 解析 Maven 路径
func parseMavenPath(path string) (groupID, artifactID, version, filename string, err error) {
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		return "", "", "", "", fmt.Errorf("invalid path: %s", path)
	}

	// 最后一部分是文件名
	filename = parts[len(parts)-1]
	version = parts[len(parts)-2]
	artifactID = parts[len(parts)-3]

	// 剩余部分是 groupId
	groupParts := parts[:len(parts)-3]
	groupID = strings.Join(groupParts, ".")

	return groupID, artifactID, version, filename, nil
}

// detectContentType 检测文件 Content-Type
func detectContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jar":
		return "application/java-archive"
	case ".pom":
		return "application/xml"
	case ".xml":
		return "application/xml"
	case ".sha1":
		return "text/plain"
	case ".md5":
		return "text/plain"
	case ".war":
		return "application/java-archive"
	case ".ear":
		return "application/java-archive"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	default:
		return "application/octet-stream"
	}
}

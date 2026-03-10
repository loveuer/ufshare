package handler

import (
	"errors"
	"net/http"
	"path/filepath"

	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api/middleware"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

type FileHandler struct {
	fileService *service.FileService
}

func NewFileHandler(fileService *service.FileService) *FileHandler {
	return &FileHandler{fileService: fileService}
}

// Upload PUT /file-store/*path  — 需要认证
func (h *FileHandler) Upload(c *ursa.Ctx) error {
	filePath := c.Param("path")
	uploaderID := middleware.GetUserID(c)
	uploaderName := middleware.GetUsername(c)

	entry, err := h.fileService.Upload(c.Request.Context(), filePath, c.Request.Body, uploaderID, uploaderName)
	if err != nil {
		if errors.Is(err, service.ErrInvalidPath) {
			return c.Status(400).JSON(ursa.Map{"code": 400, "message": "invalid path"})
		}
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": err.Error()})
	}

	return c.Status(201).JSON(ursa.Map{"code": 0, "message": "success", "data": entry})
}

// Download GET /file-store/*path  — 公开
func (h *FileHandler) Download(c *ursa.Ctx) error {
	filePath := c.Param("path")

	entry, diskPath, err := h.fileService.Download(c.Request.Context(), filePath)
	if err != nil {
		if errors.Is(err, service.ErrFileNotFound) {
			return c.Status(404).JSON(ursa.Map{"code": 404, "message": "file not found"})
		}
		if errors.Is(err, service.ErrInvalidPath) {
			return c.Status(400).JSON(ursa.Map{"code": 400, "message": "invalid path"})
		}
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": "internal server error"})
	}

	c.Set("Content-Type", entry.MimeType)
	c.Set("X-SHA256", entry.SHA256)
	c.Set("Content-Disposition", `attachment; filename="`+filepath.Base(entry.Path)+`"`)
	http.ServeFile(c.Writer, c.Request, diskPath)
	return nil
}

// Delete DELETE /file-store/*path  — 需要认证
func (h *FileHandler) Delete(c *ursa.Ctx) error {
	filePath := c.Param("path")

	if err := h.fileService.Delete(c.Request.Context(), filePath); err != nil {
		if errors.Is(err, service.ErrFileNotFound) {
			return c.Status(404).JSON(ursa.Map{"code": 404, "message": "file not found"})
		}
		if errors.Is(err, service.ErrInvalidPath) {
			return c.Status(400).JSON(ursa.Map{"code": 400, "message": "invalid path"})
		}
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": "internal server error"})
	}

	return c.JSON(ursa.Map{"code": 0, "message": "success"})
}

// List GET /file-store  — 公开
func (h *FileHandler) List(c *ursa.Ctx) error {
	prefix := c.Query("prefix")

	entries, err := h.fileService.List(c.Request.Context(), prefix)
	if err != nil {
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": "internal server error"})
	}

	return c.JSON(ursa.Map{"code": 0, "message": "success", "data": entries})
}

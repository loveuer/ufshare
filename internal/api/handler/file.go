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
	permService *service.PermissionService
}

func NewFileHandler(fileService *service.FileService, permService *service.PermissionService) *FileHandler {
	return &FileHandler{fileService: fileService, permService: permService}
}

// checkPerm 校验权限，返回 (canRead, canWrite, moduleID, error)
func (h *FileHandler) checkPerm(c *ursa.Ctx, moduleName string) (canRead, canWrite bool, moduleID uint, err error) {
	userID := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	canRead, canWrite, err = h.permService.CheckPermission(userID, moduleName, isAdmin)
	if err != nil {
		return
	}

	mod, e := h.permService.GetModule(moduleName)
	if e != nil {
		err = e
		return
	}
	moduleID = mod.ID
	return
}

// Upload PUT /files/:module/*path
func (h *FileHandler) Upload(c *ursa.Ctx) error {
	moduleName := c.Param("module")
	filePath := c.Param("path")

	_, canWrite, moduleID, err := h.checkPerm(c, moduleName)
	if err != nil {
		if errors.Is(err, service.ErrModuleNotFound) {
			return c.Status(404).JSON(ursa.Map{"code": 404, "message": "module not found"})
		}
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": "internal server error"})
	}
	if !canWrite {
		return c.Status(403).JSON(ursa.Map{"code": 403, "message": "write permission denied"})
	}

	uploaderID := middleware.GetUserID(c)
	uploaderName := middleware.GetUsername(c)

	entry, err := h.fileService.Upload(moduleID, moduleName, filePath, c.Request.Body, uploaderID, uploaderName)
	if err != nil {
		if errors.Is(err, service.ErrInvalidPath) {
			return c.Status(400).JSON(ursa.Map{"code": 400, "message": "invalid path"})
		}
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": err.Error()})
	}

	return c.Status(201).JSON(ursa.Map{
		"code":    0,
		"message": "success",
		"data":    entry,
	})
}

// Download GET /files/:module/*path
func (h *FileHandler) Download(c *ursa.Ctx) error {
	moduleName := c.Param("module")
	filePath := c.Param("path")

	canRead, _, moduleID, err := h.checkPerm(c, moduleName)
	if err != nil {
		if errors.Is(err, service.ErrModuleNotFound) {
			return c.Status(404).JSON(ursa.Map{"code": 404, "message": "module not found"})
		}
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": "internal server error"})
	}
	if !canRead {
		return c.Status(403).JSON(ursa.Map{"code": 403, "message": "read permission denied"})
	}

	entry, diskPath, err := h.fileService.Download(moduleID, moduleName, filePath)
	if err != nil {
		if errors.Is(err, service.ErrFileNotFound) {
			return c.Status(404).JSON(ursa.Map{"code": 404, "message": "file not found"})
		}
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": "internal server error"})
	}

	c.Set("Content-Type", entry.MimeType)
	c.Set("X-SHA256", entry.SHA256)
	c.Set("Content-Disposition", `attachment; filename="`+filepath.Base(entry.Path)+`"`)
	http.ServeFile(c.Writer, c.Request, diskPath)
	return nil
}

// Delete DELETE /files/:module/*path
func (h *FileHandler) Delete(c *ursa.Ctx) error {
	moduleName := c.Param("module")
	filePath := c.Param("path")

	_, canWrite, moduleID, err := h.checkPerm(c, moduleName)
	if err != nil {
		if errors.Is(err, service.ErrModuleNotFound) {
			return c.Status(404).JSON(ursa.Map{"code": 404, "message": "module not found"})
		}
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": "internal server error"})
	}
	if !canWrite {
		return c.Status(403).JSON(ursa.Map{"code": 403, "message": "write permission denied"})
	}

	if err := h.fileService.Delete(moduleID, moduleName, filePath); err != nil {
		if errors.Is(err, service.ErrFileNotFound) {
			return c.Status(404).JSON(ursa.Map{"code": 404, "message": "file not found"})
		}
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": "internal server error"})
	}

	return c.JSON(ursa.Map{"code": 0, "message": "success"})
}

// List GET /files/:module
func (h *FileHandler) List(c *ursa.Ctx) error {
	moduleName := c.Param("module")
	prefix := c.Query("prefix")

	canRead, _, moduleID, err := h.checkPerm(c, moduleName)
	if err != nil {
		if errors.Is(err, service.ErrModuleNotFound) {
			return c.Status(404).JSON(ursa.Map{"code": 404, "message": "module not found"})
		}
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": "internal server error"})
	}
	if !canRead {
		return c.Status(403).JSON(ursa.Map{"code": 403, "message": "read permission denied"})
	}

	entries, err := h.fileService.List(moduleID, prefix)
	if err != nil {
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": "internal server error"})
	}

	return c.JSON(ursa.Map{
		"code":    0,
		"message": "success",
		"data":    entries,
	})
}

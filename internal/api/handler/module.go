package handler

import (
	"strconv"

	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

type ModuleHandler struct {
	permService *service.PermissionService
}

func NewModuleHandler(permService *service.PermissionService) *ModuleHandler {
	return &ModuleHandler{permService: permService}
}

type CreateModuleRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	PublicRead  bool   `json:"public_read"`
	PublicWrite bool   `json:"public_write"`
}

// Create 创建模块
func (h *ModuleHandler) Create(c *ursa.Ctx) error {
	var req CreateModuleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid request body",
		})
	}

	if req.Name == "" || req.Type == "" {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "name and type are required",
		})
	}

	moduleType := model.ModuleType(req.Type)
	module, err := h.permService.CreateModule(req.Name, moduleType, req.Description, req.PublicRead, req.PublicWrite)
	if err != nil {
		if err == service.ErrModuleExists {
			return c.Status(409).JSON(ursa.Map{
				"code":    409,
				"message": "module already exists",
			})
		}
		return c.Status(500).JSON(ursa.Map{
			"code":    500,
			"message": "internal server error",
		})
	}

	return c.JSON(ursa.Map{
		"code":    0,
		"message": "success",
		"data":    module,
	})
}

// List 列出所有模块
func (h *ModuleHandler) List(c *ursa.Ctx) error {
	modules, err := h.permService.ListModules()
	if err != nil {
		return c.Status(500).JSON(ursa.Map{
			"code":    500,
			"message": "internal server error",
		})
	}

	return c.JSON(ursa.Map{
		"code":    0,
		"message": "success",
		"data":    modules,
	})
}

// Get 获取模块
func (h *ModuleHandler) Get(c *ursa.Ctx) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid module id",
		})
	}

	module, err := h.permService.GetModuleByID(uint(id))
	if err != nil {
		if err == service.ErrModuleNotFound {
			return c.Status(404).JSON(ursa.Map{
				"code":    404,
				"message": "module not found",
			})
		}
		return c.Status(500).JSON(ursa.Map{
			"code":    500,
			"message": "internal server error",
		})
	}

	return c.JSON(ursa.Map{
		"code":    0,
		"message": "success",
		"data":    module,
	})
}

type UpdateModuleRequest struct {
	Description *string `json:"description"`
	PublicRead  *bool   `json:"public_read"`
	PublicWrite *bool   `json:"public_write"`
}

// Update 更新模块
func (h *ModuleHandler) Update(c *ursa.Ctx) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid module id",
		})
	}

	var req UpdateModuleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid request body",
		})
	}

	updates := make(map[string]interface{})
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.PublicRead != nil {
		updates["public_read"] = *req.PublicRead
	}
	if req.PublicWrite != nil {
		updates["public_write"] = *req.PublicWrite
	}

	if len(updates) == 0 {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "no fields to update",
		})
	}

	if err := h.permService.UpdateModule(uint(id), updates); err != nil {
		return c.Status(500).JSON(ursa.Map{
			"code":    500,
			"message": "internal server error",
		})
	}

	return c.JSON(ursa.Map{
		"code":    0,
		"message": "success",
	})
}

// Delete 删除模块
func (h *ModuleHandler) Delete(c *ursa.Ctx) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid module id",
		})
	}

	if err := h.permService.DeleteModule(uint(id)); err != nil {
		return c.Status(500).JSON(ursa.Map{
			"code":    500,
			"message": "internal server error",
		})
	}

	return c.JSON(ursa.Map{
		"code":    0,
		"message": "success",
	})
}

package handler

import (
	"strconv"

	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

type PermissionHandler struct {
	permService *service.PermissionService
}

func NewPermissionHandler(permService *service.PermissionService) *PermissionHandler {
	return &PermissionHandler{permService: permService}
}

type GrantPermissionRequest struct {
	UserID   uint `json:"user_id"`
	ModuleID uint `json:"module_id"`
	CanRead  bool `json:"can_read"`
	CanWrite bool `json:"can_write"`
}

// Grant 授予权限
func (h *PermissionHandler) Grant(c *ursa.Ctx) error {
	var req GrantPermissionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid request body",
		})
	}

	if req.UserID == 0 || req.ModuleID == 0 {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "user_id and module_id are required",
		})
	}

	if err := h.permService.GrantPermission(req.UserID, req.ModuleID, req.CanRead, req.CanWrite); err != nil {
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

type RevokePermissionRequest struct {
	UserID   uint `json:"user_id"`
	ModuleID uint `json:"module_id"`
}

// Revoke 撤销权限
func (h *PermissionHandler) Revoke(c *ursa.Ctx) error {
	var req RevokePermissionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid request body",
		})
	}

	if req.UserID == 0 || req.ModuleID == 0 {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "user_id and module_id are required",
		})
	}

	if err := h.permService.RevokePermission(req.UserID, req.ModuleID); err != nil {
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

// GetUserPermissions 获取用户权限列表
func (h *PermissionHandler) GetUserPermissions(c *ursa.Ctx) error {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid user id",
		})
	}

	perms, err := h.permService.GetUserPermissions(uint(userID))
	if err != nil {
		return c.Status(500).JSON(ursa.Map{
			"code":    500,
			"message": "internal server error",
		})
	}

	return c.JSON(ursa.Map{
		"code":    0,
		"message": "success",
		"data":    perms,
	})
}

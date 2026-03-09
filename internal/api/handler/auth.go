package handler

import (
	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/api/middleware"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Register 用户注册
func (h *AuthHandler) Register(c *ursa.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid request body",
		})
	}

	if req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "username and password are required",
		})
	}

	user, err := h.authService.Register(req.Username, req.Password, req.Email)
	if err != nil {
		if err == service.ErrUserExists {
			return c.Status(409).JSON(ursa.Map{
				"code":    409,
				"message": "user already exists",
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
		"data":    user,
	})
}

// Login 用户登录
func (h *AuthHandler) Login(c *ursa.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid request body",
		})
	}

	if req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "username and password are required",
		})
	}

	token, user, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		switch err {
		case service.ErrUserNotFound, service.ErrInvalidCredentials:
			return c.Status(401).JSON(ursa.Map{
				"code":    401,
				"message": "invalid credentials",
			})
		case service.ErrUserDisabled:
			return c.Status(403).JSON(ursa.Map{
				"code":    403,
				"message": "user is disabled",
			})
		default:
			return c.Status(500).JSON(ursa.Map{
				"code":    500,
				"message": "internal server error",
			})
		}
	}

	return c.JSON(ursa.Map{
		"code":    0,
		"message": "success",
		"data": ursa.Map{
			"token": token,
			"user":  user,
		},
	})
}

// ChangePassword 用户自助修改密码，需要提供旧密码
func (h *AuthHandler) ChangePassword(c *ursa.Ctx) error {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ursa.Map{"code": 400, "message": "invalid request body"})
	}
	if req.OldPassword == "" || req.NewPassword == "" {
		return c.Status(400).JSON(ursa.Map{"code": 400, "message": "old_password and new_password are required"})
	}
	if len(req.NewPassword) < 6 {
		return c.Status(400).JSON(ursa.Map{"code": 400, "message": "new password must be at least 6 characters"})
	}

	userID := middleware.GetUserID(c)
	if err := h.authService.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		if err == service.ErrInvalidCredentials {
			return c.Status(400).JSON(ursa.Map{"code": 400, "message": "old password is incorrect"})
		}
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": "internal server error"})
	}

	return c.JSON(ursa.Map{"code": 0, "message": "password changed successfully"})
}

func (h *AuthHandler) Me(c *ursa.Ctx) error {
	userID := middleware.GetUserID(c)
	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		return c.Status(404).JSON(ursa.Map{
			"code":    404,
			"message": "user not found",
		})
	}

	return c.JSON(ursa.Map{
		"code":    0,
		"message": "success",
		"data":    user,
	})
}

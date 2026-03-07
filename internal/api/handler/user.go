package handler

import (
	"strconv"

	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	IsAdmin  bool   `json:"is_admin"`
}

// Create 创建用户
func (h *UserHandler) Create(c *ursa.Ctx) error {
	var req CreateUserRequest
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

	user, err := h.userService.CreateUser(req.Username, req.Password, req.Email, req.IsAdmin)
	if err != nil {
		return c.Status(500).JSON(ursa.Map{
			"code":    500,
			"message": "failed to create user: " + err.Error(),
		})
	}

	return c.Status(201).JSON(ursa.Map{
		"code":    0,
		"message": "success",
		"data":    user,
	})
}

// List 列出所有用户
func (h *UserHandler) List(c *ursa.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	users, total, err := h.userService.ListUsers(page, pageSize)
	if err != nil {
		return c.Status(500).JSON(ursa.Map{
			"code":    500,
			"message": "internal server error",
		})
	}

	return c.JSON(ursa.Map{
		"code":    0,
		"message": "success",
		"data": ursa.Map{
			"items": users,
			"total": total,
			"page":  page,
		},
	})
}

// Get 获取用户
func (h *UserHandler) Get(c *ursa.Ctx) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid user id",
		})
	}

	user, err := h.userService.GetUser(uint(id))
	if err != nil {
		if err == service.ErrUserNotFound {
			return c.Status(404).JSON(ursa.Map{
				"code":    404,
				"message": "user not found",
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

type UpdateUserRequest struct {
	Email    *string `json:"email"`
	Password *string `json:"password"`
	Status   *int    `json:"status"`
	IsAdmin  *bool   `json:"is_admin"`
}

// Update 更新用户
func (h *UserHandler) Update(c *ursa.Ctx) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid user id",
		})
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid request body",
		})
	}

	updates := make(map[string]interface{})
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Password != nil && *req.Password != "" {
		updates["password"] = *req.Password
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.IsAdmin != nil {
		updates["is_admin"] = *req.IsAdmin
	}

	if len(updates) == 0 {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "no fields to update",
		})
	}

	if err := h.userService.UpdateUser(uint(id), updates); err != nil {
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

// Delete 删除用户
func (h *UserHandler) Delete(c *ursa.Ctx) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(ursa.Map{
			"code":    400,
			"message": "invalid user id",
		})
	}

	if err := h.userService.DeleteUser(uint(id)); err != nil {
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

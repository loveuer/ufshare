package middleware

import (
	"strings"

	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

const (
	LocalsUserID   = "user_id"
	LocalsUsername = "username"
	LocalsIsAdmin  = "is_admin"
)

// Auth JWT 认证中间件
func Auth(authService *service.AuthService) ursa.HandlerFunc {
	return func(c *ursa.Ctx) error {
		// 从 Authorization header 获取 token
		auth := c.Get("Authorization")
		if auth == "" {
			return c.Status(401).JSON(ursa.Map{
				"code":    401,
				"message": "unauthorized",
			})
		}

		// 去除 Bearer 前缀
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth {
			return c.Status(401).JSON(ursa.Map{
				"code":    401,
				"message": "invalid authorization header",
			})
		}

		// 验证 token
		claims, err := authService.ValidateToken(token)
		if err != nil {
			return c.Status(401).JSON(ursa.Map{
				"code":    401,
				"message": "invalid token",
			})
		}

		// 存储用户信息到上下文
		c.Locals(LocalsUserID, claims.UserID)
		c.Locals(LocalsUsername, claims.Username)
		c.Locals(LocalsIsAdmin, claims.IsAdmin)

		return c.Next()
	}
}

// OptionalAuth 可选认证中间件 (不强制要求登录)
func OptionalAuth(authService *service.AuthService) ursa.HandlerFunc {
	return func(c *ursa.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" {
			return c.Next()
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth {
			return c.Next()
		}

		claims, err := authService.ValidateToken(token)
		if err != nil {
			return c.Next()
		}

		c.Locals(LocalsUserID, claims.UserID)
		c.Locals(LocalsUsername, claims.Username)
		c.Locals(LocalsIsAdmin, claims.IsAdmin)

		return c.Next()
	}
}

// AdminOnly 仅管理员中间件
func AdminOnly() ursa.HandlerFunc {
	return func(c *ursa.Ctx) error {
		isAdmin, ok := c.Locals(LocalsIsAdmin).(bool)
		if !ok || !isAdmin {
			return c.Status(403).JSON(ursa.Map{
				"code":    403,
				"message": "forbidden: admin only",
			})
		}
		return c.Next()
	}
}

// GetUserID 从上下文获取用户 ID
func GetUserID(c *ursa.Ctx) uint {
	if id, ok := c.Locals(LocalsUserID).(uint); ok {
		return id
	}
	return 0
}

// GetUsername 从上下文获取用户名
func GetUsername(c *ursa.Ctx) string {
	if name, ok := c.Locals(LocalsUsername).(string); ok {
		return name
	}
	return ""
}

// IsAdmin 从上下文获取管理员状态
func IsAdmin(c *ursa.Ctx) bool {
	if isAdmin, ok := c.Locals(LocalsIsAdmin).(bool); ok {
		return isAdmin
	}
	return false
}

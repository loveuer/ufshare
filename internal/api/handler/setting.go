package handler

import (
	"github.com/loveuer/ursa"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

type SettingHandler struct {
	settingService *service.SettingService
}

func NewSettingHandler(s *service.SettingService) *SettingHandler {
	return &SettingHandler{settingService: s}
}

// GetAll GET /api/v1/admin/settings
func (h *SettingHandler) GetAll(c *ursa.Ctx) error {
	settings, err := h.settingService.GetAll(c.Request.Context())
	if err != nil {
		return c.Status(500).JSON(ursa.Map{"code": 500, "message": "internal server error"})
	}
	// 转成 map 方便前端使用
	result := make(map[string]string, len(settings))
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	// 补充未写入 DB 的默认值
	if _, ok := result[service.SettingNpmUpstream]; !ok {
		result[service.SettingNpmUpstream] = service.DefaultNpmUpstream
	}
	// enabled/addr 字段默认值，确保前端能看到完整的 key 列表
	for _, key := range []string{
		service.SettingNpmEnabled, service.SettingNpmAddr,
		service.SettingFileEnabled, service.SettingFileAddr,
	} {
		if _, ok := result[key]; !ok {
			result[key] = ""
		}
	}
	return c.JSON(ursa.Map{"code": 0, "message": "success", "data": result})
}

// Update PUT /api/v1/admin/settings
func (h *SettingHandler) Update(c *ursa.Ctx) error {
	var body map[string]string
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ursa.Map{"code": 400, "message": "invalid request body"})
	}
	for key, value := range body {
		if err := h.settingService.Set(c.Request.Context(), key, value); err != nil {
			return c.Status(500).JSON(ursa.Map{"code": 500, "message": "failed to save setting: " + key})
		}
	}
	return c.JSON(ursa.Map{"code": 0, "message": "success"})
}

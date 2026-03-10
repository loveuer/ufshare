package service

import (
	"log"
	"sync"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
)

const (
	SettingNpmUpstream = "npm.upstream"
	SettingNpmEnabled  = "npm.enabled"
	SettingNpmAddr     = "npm.addr"
	SettingFileEnabled = "file.enabled"
	SettingFileAddr    = "file.addr"
	SettingGoUpstream  = "go.upstream"
	SettingGoPrivate   = "go.private"
	SettingGoEnabled   = "go.enabled"
	SettingGoAddr      = "go.addr"

	DefaultNpmUpstream = "https://registry.npmmirror.com"
	DefaultGoUpstream  = "https://goproxy.cn,direct"
)

type SettingService struct {
	db        *gorm.DB
	mu        sync.RWMutex
	listeners map[string][]func(string) // key → callbacks
}

func NewSettingService(db *gorm.DB) *SettingService {
	return &SettingService{
		db:        db,
		listeners: make(map[string][]func(string)),
	}
}

// OnChange 注册当 key 对应的配置变更时触发的回调
func (s *SettingService) OnChange(key string, fn func(newValue string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners[key] = append(s.listeners[key], fn)
}

// Get 获取配置值；key 不存在时返回空字符串
func (s *SettingService) Get(key string) string {
	var setting model.Setting
	if err := s.db.First(&setting, "key = ?", key).Error; err != nil {
		return ""
	}
	return setting.Value
}

// Set 写入配置项（upsert），并通知所有注册了该 key 的观察者
func (s *SettingService) Set(key, value string) error {
	err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&model.Setting{Key: key, Value: value}).Error
	if err != nil {
		return err
	}

	s.mu.RLock()
	fns := s.listeners[key]
	s.mu.RUnlock()

	for _, fn := range fns {
		go func(cb func(string)) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[setting] callback panic for key %q: %v", key, r)
				}
			}()
			cb(value)
		}(fn)
	}
	return nil
}

// GetNpmUpstream 返回 npm 代理上游地址，未配置时返回默认值
func (s *SettingService) GetNpmUpstream() string {
	if v := s.Get(SettingNpmUpstream); v != "" {
		return v
	}
	return DefaultNpmUpstream
}

// GetNpmEnabled 返回 npm 专用端口是否已启用
func (s *SettingService) GetNpmEnabled() bool {
	return s.Get(SettingNpmEnabled) == "true"
}

// GetNpmAddr 返回 npm 专用端口监听地址，未配置时返回空字符串
func (s *SettingService) GetNpmAddr() string {
	return s.Get(SettingNpmAddr)
}

// GetFileEnabled 返回 file-store 专用端口是否已启用
func (s *SettingService) GetFileEnabled() bool {
	return s.Get(SettingFileEnabled) == "true"
}

// GetFileAddr 返回 file-store 专用端口监听地址，未配置时返回空字符串
func (s *SettingService) GetFileAddr() string {
	return s.Get(SettingFileAddr)
}

// GetGoUpstream 返回 Go 模块代理上游地址，未配置时返回默认值
func (s *SettingService) GetGoUpstream() string {
	if v := s.Get(SettingGoUpstream); v != "" {
		return v
	}
	return DefaultGoUpstream
}

// GetGoPrivate 返回 Go 私有模块模式
func (s *SettingService) GetGoPrivate() string {
	return s.Get(SettingGoPrivate)
}

// GetGoEnabled 返回 Go 模块代理专用端口是否已启用
func (s *SettingService) GetGoEnabled() bool {
	return s.Get(SettingGoEnabled) == "true"
}

// GetGoAddr 返回 Go 模块代理专用端口监听地址，未配置时返回空字符串
func (s *SettingService) GetGoAddr() string {
	return s.Get(SettingGoAddr)
}

// GetAll 返回所有配置项
func (s *SettingService) GetAll() ([]model.Setting, error) {
	var settings []model.Setting
	return settings, s.db.Find(&settings).Error
}

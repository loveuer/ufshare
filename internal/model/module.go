package model

import (
	"time"

	"gorm.io/gorm"
)

// ModuleType 模块类型
type ModuleType string

const (
	ModuleTypeUser   ModuleType = "user"   // 用户模块
	ModuleTypeFile   ModuleType = "file"   // 普通文件模块
	ModuleTypeNPM    ModuleType = "npm"    // npm 模块
	ModuleTypeGo     ModuleType = "go"     // go 模块
	ModuleTypePyPI   ModuleType = "pypi"   // pypi 模块
	ModuleTypeMaven  ModuleType = "maven"  // maven 模块
)

// Module 模块定义
type Module struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Name        string     `json:"name" gorm:"uniqueIndex;size:64;not null"`
	Type        ModuleType `json:"type" gorm:"size:32;not null"`
	Description string     `json:"description" gorm:"size:256"`
	
	// 公有权限设置
	PublicRead  bool `json:"public_read" gorm:"default:false"`  // 是否公有读
	PublicWrite bool `json:"public_write" gorm:"default:false"` // 是否公有写
	
	// 模块配置 (JSON 格式存储)
	Config string `json:"config" gorm:"type:text"`
}

func (Module) TableName() string {
	return "modules"
}

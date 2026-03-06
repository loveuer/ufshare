package model

import (
	"time"

	"gorm.io/gorm"
)

// Permission 用户对模块的权限
type Permission struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	UserID   uint `json:"user_id" gorm:"index;not null"`
	ModuleID uint `json:"module_id" gorm:"index;not null"`

	CanRead  bool `json:"can_read" gorm:"default:false"`
	CanWrite bool `json:"can_write" gorm:"default:false"`

	// 关联
	User   User   `json:"-" gorm:"foreignKey:UserID"`
	Module Module `json:"-" gorm:"foreignKey:ModuleID"`
}

func (Permission) TableName() string {
	return "permissions"
}

// 创建唯一索引
func (Permission) Indexes() []string {
	return []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_user_module ON permissions(user_id, module_id)",
	}
}

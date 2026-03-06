package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Username string `json:"username" gorm:"uniqueIndex;size:64;not null"`
	Password string `json:"-" gorm:"size:128;not null"`
	Email    string `json:"email" gorm:"size:128"`
	IsAdmin  bool   `json:"is_admin" gorm:"default:false"`
	Status   int    `json:"status" gorm:"default:1"` // 1: active, 0: disabled
}

func (User) TableName() string {
	return "users"
}

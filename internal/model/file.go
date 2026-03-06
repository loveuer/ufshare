package model

import (
	"time"

	"gorm.io/gorm"
)

// FileEntry 文件元数据
type FileEntry struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	ModuleID uint   `json:"module_id" gorm:"index;not null"`
	Path     string `json:"path" gorm:"size:1024;not null"` // 相对于模块目录的路径，如 v1.0/app.tar.gz
	Size     int64  `json:"size" gorm:"not null"`
	MimeType string `json:"mime_type" gorm:"size:128"`
	SHA256   string `json:"sha256" gorm:"size:64"`

	UploaderID uint   `json:"uploader_id" gorm:"index"`
	Uploader   string `json:"uploader" gorm:"size:64"` // 冗余存用户名，方便展示

	Module Module `json:"-" gorm:"foreignKey:ModuleID"`
}

func (FileEntry) TableName() string {
	return "file_entries"
}

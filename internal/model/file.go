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

	Path     string `json:"path" gorm:"uniqueIndex;size:1024;not null"` // 文件相对路径，如 v1.0/app.tar.gz
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type" gorm:"size:128"`
	SHA256   string `json:"sha256" gorm:"size:64"`

	UploaderID uint   `json:"uploader_id"`
	Uploader   string `json:"uploader" gorm:"size:64"`
}

func (FileEntry) TableName() string {
	return "file_entries"
}

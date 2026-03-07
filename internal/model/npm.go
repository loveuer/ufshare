package model

import (
	"time"

	"gorm.io/gorm"
)

// NpmPackage 包级别元数据（一个包名对应一条记录）
type NpmPackage struct {
	ID        uint           `json:"id"         gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"          gorm:"index"`

	Name        string `gorm:"uniqueIndex;size:256;not null"`
	Description string `gorm:"size:1024"`
	Readme      string `gorm:"type:text"`
	// JSON 编码的 dist-tags，如 {"latest":"1.0.0"}
	DistTags string `gorm:"column:dist_tags;type:text"`

	Versions []NpmVersion `gorm:"foreignKey:PackageID"`
}

func (NpmPackage) TableName() string { return "npm_packages" }

// NpmVersion 版本级别元数据（每个版本一条记录）
type NpmVersion struct {
	ID        uint           `json:"id"         gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"          gorm:"index"`

	PackageID uint   `gorm:"index;not null"`
	Version   string `gorm:"size:64;not null"`

	// 完整的 package.json 元数据 JSON（dist.tarball 保存上游原始 URL，对外输出时改写）
	MetaJSON    string `gorm:"type:text;not null"`
	TarballName string `gorm:"size:256"` // e.g. lodash-4.17.21.tgz
	Shasum      string `gorm:"size:40"`
	Integrity   string `gorm:"size:128"`
	Size        int64

	// Cached=true 表示 tarball 已保存到本地磁盘
	Cached bool `gorm:"default:false"`

	// 本地发布者（为空表示从上游代理缓存）
	UploaderID uint   `gorm:"index"`
	Uploader   string `gorm:"size:64"`
}

func (NpmVersion) TableName() string { return "npm_versions" }

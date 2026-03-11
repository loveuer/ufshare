package model

import "time"

// OciRepository 镜像仓库（如 library/nginx）
type OciRepository struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Name     string `json:"name" gorm:"uniqueIndex;size:256;not null"` // e.g. "library/nginx"
	Upstream string `json:"upstream" gorm:"size:256"`                  // 来源 registry，如 "docker.io"

	// Push 相关信息
	IsPushed   bool   `json:"is_pushed" gorm:"default:false"` // 是否是本地推送的镜像
	PushedByID uint   `json:"pushed_by_id" gorm:"index"`      // 推送者用户ID
	PushedBy   string `json:"pushed_by" gorm:"size:64"`       // 推送者用户名

	Tags []OciTag `json:"-" gorm:"foreignKey:RepositoryID"`
}

func (OciRepository) TableName() string { return "oci_repositories" }

// OciTag 镜像标签
type OciTag struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	RepositoryID   uint   `json:"repository_id" gorm:"index;not null"`
	Tag            string `json:"tag" gorm:"size:256;not null"`
	ManifestDigest string `json:"manifest_digest" gorm:"size:128;not null"` // sha256:...

	// uniqueIndex: (repository_id, tag)
}

func (OciTag) TableName() string { return "oci_tags" }

// OciManifest 镜像 manifest（包括 manifest list 和单个 manifest）
type OciManifest struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	RepositoryID uint   `json:"repository_id" gorm:"index;not null"`
	Digest       string `json:"digest" gorm:"uniqueIndex;size:128;not null"` // sha256:...
	MediaType    string `json:"media_type" gorm:"size:128"`
	Content      string `json:"-" gorm:"type:text"`
	Size         int64  `json:"size"`
}

func (OciManifest) TableName() string { return "oci_manifests" }

// OciBlob 镜像层/配置 blob
type OciBlob struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`

	RepositoryID uint   `json:"repository_id" gorm:"index;not null"`
	Digest       string `json:"digest" gorm:"uniqueIndex;size:128;not null"` // sha256:...
	Size         int64  `json:"size"`
	Cached       bool   `json:"cached" gorm:"default:false"` // 文件是否在磁盘上
}

func (OciBlob) TableName() string { return "oci_blobs" }

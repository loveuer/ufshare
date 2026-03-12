package model

import (
	"time"

	"gorm.io/gorm"
)

// MavenRepository Maven 仓库（如 central, jcenter 等）
type MavenRepository struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Name        string `json:"name" gorm:"uniqueIndex;size:128;not null"` // 仓库名称
	Description string `json:"description" gorm:"size:512"`               // 描述
	Upstream    string `json:"upstream" gorm:"size:256"`                  // 上游地址，如 https://repo.maven.apache.org/maven2
	Enabled     bool   `json:"enabled" gorm:"default:true"`               // 是否启用

	// 存储路径（相对于 dataDir）
	StoragePath string `json:"storage_path" gorm:"size:256;default:'maven'"`
}

func (MavenRepository) TableName() string { return "maven_repositories" }

// MavenArtifact Maven 制品（GAV: groupId:artifactId:version）
type MavenArtifact struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// GAV 坐标
	GroupID    string `json:"group_id" gorm:"index;size:256;not null"`    // 如 com.example
	ArtifactID string `json:"artifact_id" gorm:"index;size:256;not null"` // 如 myapp
	Version    string `json:"version" gorm:"index;size:128;not null"`     // 如 1.0.0

	// 是否是 SNAPSHOT 版本
	IsSnapshot bool `json:"is_snapshot" gorm:"default:false;index"`

	// 所属仓库
	RepositoryID uint            `json:"repository_id" gorm:"index;not null"`
	Repository   MavenRepository `json:"-" gorm:"foreignKey:RepositoryID"`

	// 是否为本地上传的制品
	IsUploaded bool `json:"is_uploaded" gorm:"default:false"`

	// 上传者信息（本地上传时记录）
	UploaderID uint   `json:"uploader_id" gorm:"index"`
	Uploader   string `json:"uploader" gorm:"size:64"`

	// 文件列表（通过关联表）
	Files []MavenArtifactFile `json:"files,omitempty" gorm:"foreignKey:ArtifactID"`
}

func (MavenArtifact) TableName() string { return "maven_artifacts" }

// MavenArtifactFile Maven 制品文件（如 .jar, .pom, -sources.jar 等）
type MavenArtifactFile struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`

	// 所属制品
	ArtifactID uint          `json:"artifact_id" gorm:"index;not null"`
	Artifact   MavenArtifact `json:"-" gorm:"foreignKey:ArtifactID"`

	// 文件信息
	Filename string `json:"filename" gorm:"size:256;not null"` // 如 myapp-1.0.0.jar
	Path     string `json:"path" gorm:"size:512;not null"`     // 相对存储路径
	Size     int64  `json:"size"`                              // 文件大小
	Checksum string `json:"checksum" gorm:"size:64"`           // SHA1 或 MD5

	// 文件类型
	Classifier string `json:"classifier" gorm:"size:64"` // 如 sources, javadoc, tests
	Extension  string `json:"extension" gorm:"size:16"`  // 如 jar, pom, war

	// 是否已缓存到本地
	Cached bool `json:"cached" gorm:"default:false"`

	// 来源信息
	IsUploaded bool `json:"is_uploaded" gorm:"default:false"` // 是否为本地上传
}

func (MavenArtifactFile) TableName() string { return "maven_artifact_files" }

// MavenMetadata Maven 仓库元数据（maven-metadata.xml）
type MavenMetadata struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`

	// 坐标（不含 version，因为是同一 artifact 的所有版本）
	GroupID    string `json:"group_id" gorm:"index;size:256;not null"`
	ArtifactID string `json:"artifact_id" gorm:"index;size:256;not null"`

	// 所属仓库
	RepositoryID uint `json:"repository_id" gorm:"index;not null"`

	// 最新版本信息
	LatestVersion  string `json:"latest_version" gorm:"size:128"`
	ReleaseVersion string `json:"release_version" gorm:"size:128"`

	// 所有版本列表（JSON 数组）
	VersionsJSON string `json:"-" gorm:"column:versions;type:text"`

	// 最后更新时间
	LastUpdated string `json:"last_updated" gorm:"size:20"` // yyyyMMddHHmmss 格式
}

func (MavenMetadata) TableName() string { return "maven_metadata" }

// MavenSnapshotMetadata SNAPSHOT 版本元数据（maven-metadata.xml 中的 snapshot 部分）
type MavenSnapshotMetadata struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`

	// GAV 坐标
	GroupID    string `json:"group_id" gorm:"index;size:256;not null"`
	ArtifactID string `json:"artifact_id" gorm:"index;size:256;not null"`
	Version    string `json:"version" gorm:"index;size:128;not null"` // SNAPSHOT 版本号，如 1.0.0-SNAPSHOT

	// 时间戳版本信息（如 1.0.0-20240311.123456-1）
	Timestamp   string `json:"timestamp" gorm:"size:20"` // 如 20240311.123456
	BuildNumber int    `json:"build_number"`             // 构建号，如 1

	// 文件列表（JSON 数组，记录该 SNAPSHOT 的所有文件）
	FilesJSON string `json:"-" gorm:"column:files;type:text"`

	// 最后更新时间
	LastUpdated string `json:"last_updated" gorm:"size:20"` // yyyyMMddHHmmss 格式
}

func (MavenSnapshotMetadata) TableName() string { return "maven_snapshot_metadata" }

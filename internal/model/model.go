package model

import (
	"gorm.io/gorm"
)

// AutoMigrate 自动迁移数据库表
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&FileEntry{},
		&NpmPackage{},
		&NpmVersion{},
		&Setting{},
		&OciRepository{},
		&OciTag{},
		&OciManifest{},
		&OciBlob{},
		&MavenRepository{},
		&MavenArtifact{},
		&MavenArtifactFile{},
		&MavenMetadata{},
	)
}

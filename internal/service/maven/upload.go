package maven

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
	"gorm.io/gorm"
)

// UploadOptions 上传选项
type UploadOptions struct {
	GroupID    string
	ArtifactID string
	Version    string
	Filename   string
	Classifier string
	Extension  string
	UploaderID uint
	Uploader   string
}

// UploadFile 上传制品文件
func (s *Service) UploadFile(ctx context.Context, opts UploadOptions, reader io.Reader) error {
	// 验证必填字段
	if opts.GroupID == "" || opts.ArtifactID == "" || opts.Version == "" || opts.Filename == "" {
		return fmt.Errorf("groupId, artifactId, version and filename are required")
	}

	// 计算存储路径
	localPath := s.artifactPath(opts.GroupID, opts.ArtifactID, opts.Version, opts.Filename)

	// 检查文件是否已存在
	if fileExists(localPath) {
		return ErrVersionExists
	}

	// 确保目录存在
	if err := ensureDir(filepath.Dir(localPath)); err != nil {
		return err
	}

	// 创建临时文件
	tmpPath := localPath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	// 写入文件并计算 SHA1
	hash := sha1.New()
	teeReader := io.TeeReader(reader, hash)
	size, err := io.Copy(tmpFile, teeReader)
	if err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	// 重命名为最终文件
	if err := os.Rename(tmpPath, localPath); err != nil {
		return err
	}

	// 保存 SHA1 文件
	sha1Value := hex.EncodeToString(hash.Sum(nil))
	sha1Path := localPath + ".sha1"
	os.WriteFile(sha1Path, []byte(sha1Value), 0644)

	// 数据库事务
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 查找或创建 artifact
		var artifact model.MavenArtifact
		err := tx.Where(
			"group_id = ? AND artifact_id = ? AND version = ?",
			opts.GroupID, opts.ArtifactID, opts.Version,
		).First(&artifact).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				artifact = model.MavenArtifact{
					GroupID:    opts.GroupID,
					ArtifactID: opts.ArtifactID,
					Version:    opts.Version,
					IsUploaded: true,
					UploaderID: opts.UploaderID,
					Uploader:   opts.Uploader,
				}
				if err := tx.Create(&artifact).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

		// 创建文件记录
		file := model.MavenArtifactFile{
			ArtifactID: artifact.ID,
			Filename:   opts.Filename,
			Path:       localPath,
			Size:       size,
			Checksum:   sha1Value,
			Classifier: opts.Classifier,
			Extension:  opts.Extension,
			Cached:     true,
			IsUploaded: true,
		}

		return tx.Create(&file).Error
	})
}

// UploadPOM 上传 POM 文件并解析
func (s *Service) UploadPOM(ctx context.Context, opts UploadOptions, pomContent []byte) error {
	// 保存 POM 文件
	pomFilename := fmt.Sprintf("%s-%s.pom", opts.ArtifactID, opts.Version)
	opts.Filename = pomFilename
	opts.Extension = "pom"

	if err := s.UploadFile(ctx, opts, strings.NewReader(string(pomContent))); err != nil {
		return err
	}

	// 解析 POM 文件获取依赖信息（可选）
	// TODO: 解析 POM 中的 dependencies 信息

	return nil
}

// DeleteArtifact 删除制品
func (s *Service) DeleteArtifact(ctx context.Context, groupID, artifactID, version string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 查找 artifact
		var artifact model.MavenArtifact
		if err := tx.Where(
			"group_id = ? AND artifact_id = ? AND version = ?",
			groupID, artifactID, version,
		).First(&artifact).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return ErrArtifactNotFound
			}
			return err
		}

		// 查找所有文件
		var files []model.MavenArtifactFile
		if err := tx.Where("artifact_id = ?", artifact.ID).Find(&files).Error; err != nil {
			return err
		}

		// 删除物理文件
		for _, file := range files {
			os.Remove(file.Path)
			os.Remove(file.Path + ".sha1")
			os.Remove(file.Path + ".md5")
		}

		// 删除数据库记录
		if err := tx.Where("artifact_id = ?", artifact.ID).Delete(&model.MavenArtifactFile{}).Error; err != nil {
			return err
		}

		return tx.Delete(&artifact).Error
	})
}

// DeleteArtifactFile 删除单个文件
func (s *Service) DeleteArtifactFile(ctx context.Context, groupID, artifactID, version, filename string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 查找 artifact
		var artifact model.MavenArtifact
		if err := tx.Where(
			"group_id = ? AND artifact_id = ? AND version = ?",
			groupID, artifactID, version,
		).First(&artifact).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return ErrArtifactNotFound
			}
			return err
		}

		// 查找文件
		var file model.MavenArtifactFile
		if err := tx.Where(
			"artifact_id = ? AND filename = ?",
			artifact.ID, filename,
		).First(&file).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return ErrFileNotFound
			}
			return err
		}

		// 删除物理文件
		os.Remove(file.Path)
		os.Remove(file.Path + ".sha1")
		os.Remove(file.Path + ".md5")

		// 删除数据库记录
		return tx.Delete(&file).Error
	})
}

// ListArtifacts 列制品
func (s *Service) ListArtifacts(ctx context.Context, groupID, artifactID string, page, pageSize int) ([]model.MavenArtifact, int64, error) {
	var artifacts []model.MavenArtifact
	var total int64

	query := s.db.WithContext(ctx).Model(&model.MavenArtifact{})

	if groupID != "" {
		query = query.Where("group_id = ?", groupID)
	}
	if artifactID != "" {
		query = query.Where("artifact_id = ?", artifactID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&artifacts).Error; err != nil {
		return nil, 0, err
	}

	return artifacts, total, nil
}

// GetArtifact 获取制品详情
func (s *Service) GetArtifact(ctx context.Context, groupID, artifactID, version string) (*model.MavenArtifact, error) {
	var artifact model.MavenArtifact
	if err := s.db.WithContext(ctx).Where(
		"group_id = ? AND artifact_id = ? AND version = ?",
		groupID, artifactID, version,
	).Preload("Files").First(&artifact).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrArtifactNotFound
		}
		return nil, err
	}

	return &artifact, nil
}

// GenerateMetadata 生成 maven-metadata.xml
func (s *Service) GenerateMetadata(ctx context.Context, groupID, artifactID string) ([]byte, error) {
	// 查询所有版本
	var artifacts []model.MavenArtifact
	if err := s.db.WithContext(ctx).Where(
		"group_id = ? AND artifact_id = ?",
		groupID, artifactID,
	).Order("created_at DESC").Find(&artifacts).Error; err != nil {
		return nil, err
	}

	if len(artifacts) == 0 {
		return nil, ErrArtifactNotFound
	}

	// 构建 metadata XML
	var versions []string
	var latestVersion, releaseVersion string

	for i, artifact := range artifacts {
		versions = append(versions, artifact.Version)
		if i == 0 {
			latestVersion = artifact.Version
			if !strings.Contains(artifact.Version, "SNAPSHOT") {
				releaseVersion = artifact.Version
			}
		}
	}

	// 如果没有找到 release 版本，使用第一个
	if releaseVersion == "" && len(artifacts) > 0 {
		for _, artifact := range artifacts {
			if !strings.Contains(artifact.Version, "SNAPSHOT") {
				releaseVersion = artifact.Version
				break
			}
		}
	}
	if releaseVersion == "" {
		releaseVersion = latestVersion
	}

	now := time.Now().UTC().Format("20060102150405")

	metadata := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<metadata>
  <groupId>%s</groupId>
  <artifactId>%s</artifactId>
  <versioning>
    <latest>%s</latest>
    <release>%s</release>
    <versions>%s
    </versions>
    <lastUpdated>%s</lastUpdated>
  </versioning>
</metadata>`,
		groupID,
		artifactID,
		latestVersion,
		releaseVersion,
		func() string {
			result := ""
			for _, v := range versions {
				result += fmt.Sprintf("\n      <version>%s</version>", v)
			}
			return result
		}(),
		now,
	)

	return []byte(metadata), nil
}

// UpdateMetadata 更新本地 maven-metadata.xml
func (s *Service) UpdateMetadata(ctx context.Context, groupID, artifactID string) error {
	metadata, err := s.GenerateMetadata(ctx, groupID, artifactID)
	if err != nil {
		return err
	}

	localPath := s.metadataPath(groupID, artifactID)
	if err := ensureDir(filepath.Dir(localPath)); err != nil {
		return err
	}

	return os.WriteFile(localPath, metadata, 0644)
}

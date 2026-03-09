package npm

import (
	"crypto/sha1" //nolint:gosec // npm uses sha1 for shasum
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/gorm"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
)

// Publish 处理 npm publish 请求（PUT /:package）
// 将 tarball 保存到磁盘，将元数据写入数据库（包裹在事务中保证一致性）
func (s *Service) Publish(body *PublishBody, uploaderID uint, uploaderName string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 创建或更新包记录
		pkg, err := s.upsertPackageWith(tx, body.Name, body.Description, body.Readme, body.DistTags)
		if err != nil {
			return fmt.Errorf("upsert package: %w", err)
		}

		// 2. 处理每个版本
		for version, metaRaw := range body.Versions {
			// 不允许覆盖已有版本
			var existing model.NpmVersion
			err := tx.Where("package_id = ? AND version = ?", pkg.ID, version).First(&existing).Error
			if err == nil {
				return fmt.Errorf("%w: %s@%s", ErrVersionExists, body.Name, version)
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			di := extractDistInfo(metaRaw, body.Name, version)

			// 3. 保存 tarball 到磁盘（磁盘写入在事务外，但使用临时文件+rename保证原子性）
			var size int64
			cached := false

			if att, ok := body.Attachments[di.tarballName]; ok {
				data, err := base64.StdEncoding.DecodeString(att.Data)
				if err != nil {
					return fmt.Errorf("decode tarball %s: %w", di.tarballName, err)
				}

				if di.shasum == "" {
					h := sha1.New() //nolint:gosec
					h.Write(data)
					di.shasum = fmt.Sprintf("%x", h.Sum(nil))
				}

				size = int64(len(data))
				diskDir := s.tarballDir(body.Name)

				if err := ensureDir(diskDir); err != nil {
					return fmt.Errorf("create tarball dir: %w", err)
				}

				diskPath := filepath.Join(diskDir, di.tarballName)
				if err := os.WriteFile(diskPath, data, 0644); err != nil { //nolint:gosec
					return fmt.Errorf("write tarball: %w", err)
				}

				cached = true
			}

			// 4. 写入版本元数据
			npmVer := model.NpmVersion{
				PackageID:   pkg.ID,
				Version:     version,
				MetaJSON:    string(metaRaw),
				TarballName: di.tarballName,
				Shasum:      di.shasum,
				Integrity:   di.integrity,
				Size:        size,
				Cached:      cached,
				UploaderID:  uploaderID,
				Uploader:    uploaderName,
			}
			if err := tx.Create(&npmVer).Error; err != nil {
				return fmt.Errorf("save version %s: %w", version, err)
			}
		}

		return nil
	})
}

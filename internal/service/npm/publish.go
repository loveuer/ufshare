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

// preparedVersion 保存一个版本的准备数据，包括临时文件路径
type preparedVersion struct {
	version  string
	metaRaw  []byte
	di       distInfo
	tmpPath  string // 非空表示 tarball 已写入临时文件，rename 成功后清为空
	diskPath string
	size     int64
	cached   bool
}

// Publish 处理 npm publish 请求（PUT /:package）
// 流程：
//  1. 事务外：解码 tarball 并写入临时文件（磁盘操作独立于事务）
//  2. DB 事务：仅做数据库操作，事务失败时 defer 清理所有临时文件
//  3. 事务成功后：原子 rename 临时文件到最终路径
func (s *Service) Publish(body *PublishBody, uploaderID uint, uploaderName string) error {
	var prepared []preparedVersion

	// defer 保证：无论函数以何种方式退出，都清理所有未 rename 的临时文件
	defer func() {
		for _, pv := range prepared {
			if pv.tmpPath != "" {
				os.Remove(pv.tmpPath)
			}
		}
	}()

	// 1. 在事务外：解码 tarball，写入临时文件
	for version, metaRaw := range body.Versions {
		di := extractDistInfo(metaRaw, body.Name, version)
		pv := preparedVersion{version: version, metaRaw: metaRaw, di: di}

		if att, ok := body.Attachments[di.tarballName]; ok {
			data, err := base64.StdEncoding.DecodeString(att.Data)
			if err != nil {
				return fmt.Errorf("decode tarball %s: %w", di.tarballName, err)
			}

			if di.shasum == "" {
				h := sha1.New() //nolint:gosec
				h.Write(data)
				di.shasum = fmt.Sprintf("%x", h.Sum(nil))
				pv.di = di
			}

			diskDir := s.tarballDir(body.Name)
			if err := ensureDir(diskDir); err != nil {
				return fmt.Errorf("create tarball dir: %w", err)
			}

			diskPath := filepath.Join(diskDir, di.tarballName)
			tmpPath := diskPath + ".tmp"
			if err := os.WriteFile(tmpPath, data, 0644); err != nil { //nolint:gosec
				return fmt.Errorf("write tarball tmp: %w", err)
			}

			pv.tmpPath = tmpPath
			pv.diskPath = diskPath
			pv.size = int64(len(data))
			pv.cached = true
		}

		prepared = append(prepared, pv)
	}

	// 2. DB 事务：仅做数据库操作，不碰磁盘
	// 若事务失败，defer 负责清理所有临时文件
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		pkg, err := s.upsertPackageWith(tx, body.Name, body.Description, body.Readme, body.DistTags)
		if err != nil {
			return fmt.Errorf("upsert package: %w", err)
		}

		for _, pv := range prepared {
			var existing model.NpmVersion
			err := tx.Where("package_id = ? AND version = ?", pkg.ID, pv.version).First(&existing).Error
			if err == nil {
				return fmt.Errorf("%w: %s@%s", ErrVersionExists, body.Name, pv.version)
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			if err := tx.Create(&model.NpmVersion{
				PackageID:   pkg.ID,
				Version:     pv.version,
				MetaJSON:    string(pv.metaRaw),
				TarballName: pv.di.tarballName,
				Shasum:      pv.di.shasum,
				Integrity:   pv.di.integrity,
				Size:        pv.size,
				Cached:      pv.cached,
				UploaderID:  uploaderID,
				Uploader:    uploaderName,
			}).Error; err != nil {
				return fmt.Errorf("save version %s: %w", pv.version, err)
			}
		}

		return nil
	}); err != nil {
		return err
	}

	// 3. DB 事务成功后，原子 rename 临时文件到最终路径
	// rename 成功后将 tmpPath 清空，defer 跳过删除
	for i := range prepared {
		if prepared[i].tmpPath == "" {
			continue
		}
		if err := os.Rename(prepared[i].tmpPath, prepared[i].diskPath); err == nil {
			prepared[i].tmpPath = "" // 清空，避免 defer 误删已 rename 的文件
		}
		// rename 失败属极小概率（磁盘满/跨设备挂载），
		// DB 记录 cached=true 但文件不存在，ServeTarball 会在下次请求时
		// 从上游重新拉取并写入缓存，系统可自愈
	}

	return nil
}

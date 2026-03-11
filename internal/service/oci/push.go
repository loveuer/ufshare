package oci

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
	"gorm.io/gorm"
)

// PushManifest 处理推送的 manifest
func (s *Service) PushManifest(ctx context.Context, name, reference, mediaType string, content []byte, userID uint, username string) (digest string, err error) {
	name = s.normalizeImageName(name)

	// 计算 digest
	h := sha256.New()
	h.Write(content)
	digest = "sha256:" + hex.EncodeToString(h.Sum(nil))

	// 解析 manifest 获取 layers
	var manifest struct {
		Layers []struct {
			Digest string `json:"digest"`
			Size   int64  `json:"size"`
		} `json:"layers"`
		Config struct {
			Digest string `json:"digest"`
			Size   int64  `json:"size"`
		} `json:"config"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		return "", fmt.Errorf("parse manifest: %w", err)
	}

	// 事务处理
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 获取或创建仓库
		var repo model.OciRepository
		if err := tx.Where("name = ?", name).First(&repo).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				repo = model.OciRepository{
					Name:       name,
					Upstream:   "local", // 本地推送的镜像标记为 local
					IsPushed:   true,
					PushedByID: userID,
					PushedBy:   username,
				}
				if err := tx.Create(&repo).Error; err != nil {
					return fmt.Errorf("create repo: %w", err)
				}
			} else {
				return fmt.Errorf("get repo: %w", err)
			}
		} else {
			// 更新推送信息
			repo.IsPushed = true
			repo.PushedByID = userID
			repo.PushedBy = username
			repo.UpdatedAt = time.Now()
			if err := tx.Save(&repo).Error; err != nil {
				return fmt.Errorf("update repo: %w", err)
			}
		}

		// 2. 保存 manifest
		var mf model.OciManifest
		if err := tx.Where("digest = ?", digest).First(&mf).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				mf = model.OciManifest{
					RepositoryID: repo.ID,
					Digest:       digest,
					MediaType:    mediaType,
					Content:      string(content),
					Size:         int64(len(content)),
				}
				if err := tx.Create(&mf).Error; err != nil {
					return fmt.Errorf("create manifest: %w", err)
				}
			} else {
				return fmt.Errorf("get manifest: %w", err)
			}
		}

		// 3. 保存或更新 tag
		if !strings.HasPrefix(reference, "sha256:") {
			var tag model.OciTag
			result := tx.Where("repository_id = ? AND tag = ?", repo.ID, reference).First(&tag)
			if result.Error != nil {
				if result.Error == gorm.ErrRecordNotFound {
					tag = model.OciTag{
						RepositoryID:   repo.ID,
						Tag:            reference,
						ManifestDigest: digest,
					}
					if err := tx.Create(&tag).Error; err != nil {
						return fmt.Errorf("create tag: %w", err)
					}
				} else {
					return fmt.Errorf("get tag: %w", result.Error)
				}
			} else {
				tag.ManifestDigest = digest
				if err := tx.Save(&tag).Error; err != nil {
					return fmt.Errorf("update tag: %w", err)
				}
			}
		}

		// 4. 记录 blobs（如果还没有记录）
		allBlobs := append(manifest.Layers, struct {
			Digest string `json:"digest"`
			Size   int64  `json:"size"`
		}{Digest: manifest.Config.Digest, Size: manifest.Config.Size})

		for _, layer := range allBlobs {
			var blob model.OciBlob
			result := tx.Where("digest = ?", layer.Digest).First(&blob)
			if result.Error != nil {
				if result.Error == gorm.ErrRecordNotFound {
					blob = model.OciBlob{
						RepositoryID: repo.ID,
						Digest:       layer.Digest,
						Size:         layer.Size,
						Cached:       false, // 初始状态为未缓存，等待 blob 上传
					}
					if err := tx.Create(&blob).Error; err != nil {
						return fmt.Errorf("create blob record: %w", err)
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return digest, nil
}

// PushBlob 处理推送的 blob
func (s *Service) PushBlob(ctx context.Context, digest string, r io.Reader) error {
	// 确保 digest 格式正确
	digest = strings.TrimPrefix(digest, "sha256:")
	digest = "sha256:" + digest

	// 确保目录存在
	blobDir := s.blobDir()
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		return fmt.Errorf("create blob dir: %w", err)
	}

	// 检查 blob 是否已存在
	blobPath := filepath.Join(blobDir, digest)
	if _, err := os.Stat(blobPath); err == nil {
		// 已存在，直接返回成功
		return nil
	}

	// 创建临时文件
	tmpPath := blobPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpPath)

	// 计算 digest 并写入文件
	h := sha256.New()
	w := io.MultiWriter(f, h)

	size, err := io.Copy(w, r)
	if err != nil {
		f.Close()
		return fmt.Errorf("write blob: %w", err)
	}
	f.Close()

	// 验证 digest
	computedDigest := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if computedDigest != digest {
		os.Remove(tmpPath)
		return fmt.Errorf("digest mismatch: expected %s, got %s", digest, computedDigest)
	}

	// 移动到最终位置
	if err := os.Rename(tmpPath, blobPath); err != nil {
		return fmt.Errorf("rename blob: %w", err)
	}

	// 更新数据库中的 blob 状态
	s.db.Model(&model.OciBlob{}).Where("digest = ?", digest).Update("cached", true)

	log.Printf("[oci] blob saved: %s (%d bytes)", digest, size)
	return nil
}

// InitiateUpload 初始化 blob 上传，返回 upload URL
func (s *Service) InitiateUpload(ctx context.Context, name string) (uploadURL string, err error) {
	name = s.normalizeImageName(name)
	// 生成一个临时的 upload ID
	uploadID := generateUploadID()
	return fmt.Sprintf("/v2/%s/blobs/uploads/%s", name, uploadID), nil
}

func generateUploadID() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() % 256)
	}
	return hex.EncodeToString(b)
}

// CheckBlobExists 检查 blob 是否已存在
func (s *Service) CheckBlobExists(digest string) bool {
	digest = strings.TrimPrefix(digest, "sha256:")
	digest = "sha256:" + digest

	blobPath := filepath.Join(s.blobDir(), digest)
	_, err := os.Stat(blobPath)
	return err == nil
}

// DeleteManifest 删除 manifest
func (s *Service) DeleteManifest(ctx context.Context, name, reference string, userID uint) error {
	name = s.normalizeImageName(name)

	// 获取仓库信息
	var repo model.OciRepository
	if err := s.db.Where("name = ?", name).First(&repo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrManifestNotFound
		}
		return err
	}

	// 检查权限（只有管理员或推送者可以删除）
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return err
	}
	if !user.IsAdmin && repo.PushedByID != userID {
		return ErrForbidden
	}

	// 删除 tag
	if err := s.db.Where("repository_id = ? AND tag = ?", repo.ID, reference).Delete(&model.OciTag{}).Error; err != nil {
		return err
	}

	return nil
}

var ErrForbidden = errors.New("forbidden")

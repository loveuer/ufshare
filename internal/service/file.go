package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
)

var (
	ErrFileNotFound = errors.New("file not found")
	ErrInvalidPath  = errors.New("invalid file path")
)

type FileService struct {
	db      *gorm.DB
	dataDir string // {data}/file-store
}

func NewFileService(db *gorm.DB, dataDir string) *FileService {
	return &FileService{
		db:      db,
		dataDir: filepath.Join(dataDir, "file-store"),
	}
}

// Upload 上传文件，filePath 为相对路径如 v1.0/app.tar.gz
// 每次使用带时间戳的唯一临时文件名，不同路径并发无竞争，
// 同一路径并发时各自写各自的 tmp 后 rename，幂等（last-write-wins）
func (s *FileService) Upload(ctx context.Context, filePath string, src io.Reader, uploaderID uint, uploaderName string) (*model.FileEntry, error) {
	filePath = normalizePath(filePath)
	if err := validatePath(filePath); err != nil {
		return nil, err
	}
	return s.doUpload(ctx, filePath, src, uploaderID, uploaderName)
}

func (s *FileService) doUpload(ctx context.Context, filePath string, src io.Reader, uploaderID uint, uploaderName string) (*model.FileEntry, error) {
	diskPath := filepath.Join(s.dataDir, filePath)
	if err := os.MkdirAll(filepath.Dir(diskPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// 唯一临时文件名，避免并发上传同一路径时互相覆盖临时数据
	tmpPath := fmt.Sprintf("%s.tmp.%d", diskPath, time.Now().UnixNano())
	f, err := os.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), src)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	if err := os.Rename(tmpPath, diskPath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	sha256sum := fmt.Sprintf("%x", h.Sum(nil))
	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	var entry model.FileEntry
	err = s.db.WithContext(ctx).Where("path = ?", filePath).First(&entry).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	entry.Path = filePath
	entry.Size = size
	entry.SHA256 = sha256sum
	entry.MimeType = mimeType
	entry.UploaderID = uploaderID
	entry.Uploader = uploaderName

	if entry.ID == 0 {
		err = s.db.WithContext(ctx).Create(&entry).Error
	} else {
		err = s.db.WithContext(ctx).Save(&entry).Error
	}
	if err != nil {
		return nil, err
	}

	return &entry, nil
}

// Download 返回元数据和磁盘路径
func (s *FileService) Download(ctx context.Context, filePath string) (*model.FileEntry, string, error) {
	filePath = normalizePath(filePath)
	if err := validatePath(filePath); err != nil {
		return nil, "", err
	}

	var entry model.FileEntry
	if err := s.db.WithContext(ctx).Where("path = ?", filePath).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", ErrFileNotFound
		}
		return nil, "", err
	}

	diskPath := filepath.Join(s.dataDir, filePath)
	if _, err := os.Stat(diskPath); err != nil {
		return nil, "", ErrFileNotFound
	}

	return &entry, diskPath, nil
}

// Delete 删除文件
func (s *FileService) Delete(ctx context.Context, filePath string) error {
	filePath = normalizePath(filePath)
	if err := validatePath(filePath); err != nil {
		return err
	}

	var entry model.FileEntry
	if err := s.db.WithContext(ctx).Where("path = ?", filePath).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return err
	}

	_ = os.Remove(filepath.Join(s.dataDir, filePath))
	return s.db.WithContext(ctx).Delete(&entry).Error
}

// List 列出文件，可按前缀过滤
func (s *FileService) List(ctx context.Context, prefix string) ([]model.FileEntry, error) {
	query := s.db.WithContext(ctx).Model(&model.FileEntry{})
	if prefix != "" {
		query = query.Where("path LIKE ?", prefix+"%")
	}

	var entries []model.FileEntry
	if err := query.Order("path").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func validatePath(p string) error {
	if p == "" {
		return ErrInvalidPath
	}
	cleaned := filepath.Clean(p)
	if strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return ErrInvalidPath
	}
	return nil
}

func normalizePath(p string) string {
	return strings.TrimPrefix(p, "/")
}

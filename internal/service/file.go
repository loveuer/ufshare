package service

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
)

var (
	ErrFileNotFound   = errors.New("file not found")
	ErrInvalidPath    = errors.New("invalid file path")
)

type FileService struct {
	db      *gorm.DB
	dataDir string // {data}/files
}

func NewFileService(db *gorm.DB, dataDir string) *FileService {
	return &FileService{
		db:      db,
		dataDir: filepath.Join(dataDir, "files"),
	}
}

// Upload 上传文件
// moduleName: 模块名称
// filePath:   文件在模块内的相对路径，如 v1.0/app.tar.gz
// src:        文件内容
func (s *FileService) Upload(moduleID uint, moduleName, filePath string, src io.Reader, uploaderID uint, uploaderName string) (*model.FileEntry, error) {
	filePath = normalizePath(filePath)
	if err := validatePath(filePath); err != nil {
		return nil, err
	}

	// 磁盘路径: {dataDir}/{moduleName}/{filePath}
	diskPath := filepath.Join(s.dataDir, moduleName, filePath)
	if err := os.MkdirAll(filepath.Dir(diskPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// 先写入临时文件，计算 sha256 和 size
	tmpPath := diskPath + ".tmp"
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

	// upsert 元数据
	var entry model.FileEntry
	err = s.db.Where("module_id = ? AND path = ?", moduleID, filePath).First(&entry).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	entry.ModuleID = moduleID
	entry.Path = filePath
	entry.Size = size
	entry.SHA256 = sha256sum
	entry.MimeType = mimeType
	entry.UploaderID = uploaderID
	entry.Uploader = uploaderName

	if entry.ID == 0 {
		err = s.db.Create(&entry).Error
	} else {
		err = s.db.Save(&entry).Error
	}
	if err != nil {
		return nil, err
	}

	return &entry, nil
}

// Download 下载文件，返回磁盘路径和元数据
func (s *FileService) Download(moduleID uint, moduleName, filePath string) (*model.FileEntry, string, error) {
	filePath = normalizePath(filePath)
	if err := validatePath(filePath); err != nil {
		return nil, "", err
	}

	var entry model.FileEntry
	if err := s.db.Where("module_id = ? AND path = ?", moduleID, filePath).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", ErrFileNotFound
		}
		return nil, "", err
	}

	diskPath := filepath.Join(s.dataDir, moduleName, filePath)
	if _, err := os.Stat(diskPath); err != nil {
		return nil, "", ErrFileNotFound
	}

	return &entry, diskPath, nil
}

// Delete 删除文件
func (s *FileService) Delete(moduleID uint, moduleName, filePath string) error {
	filePath = normalizePath(filePath)
	if err := validatePath(filePath); err != nil {
		return err
	}

	var entry model.FileEntry
	if err := s.db.Where("module_id = ? AND path = ?", moduleID, filePath).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return err
	}

	diskPath := filepath.Join(s.dataDir, moduleName, filePath)
	_ = os.Remove(diskPath)

	return s.db.Delete(&entry).Error
}

// List 列出模块下的文件
func (s *FileService) List(moduleID uint, prefix string) ([]model.FileEntry, error) {
	query := s.db.Where("module_id = ?", moduleID)
	if prefix != "" {
		query = query.Where("path LIKE ?", prefix+"%")
	}

	var entries []model.FileEntry
	if err := query.Order("path").Find(&entries).Error; err != nil {
		return nil, err
	}

	return entries, nil
}

// validatePath 防止路径穿越攻击，同时去除前导斜杠（ursa *path 参数会携带）
func validatePath(p string) error {
	if p == "" {
		return ErrInvalidPath
	}
	p = strings.TrimPrefix(p, "/")
	cleaned := filepath.Clean(p)
	if strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return ErrInvalidPath
	}
	return nil
}

// normalizePath 返回去除前导斜杠后的路径
func normalizePath(p string) string {
	return strings.TrimPrefix(p, "/")
}

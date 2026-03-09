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

	"golang.org/x/sync/singleflight"
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
	sfGroup singleflight.Group
}

func NewFileService(db *gorm.DB, dataDir string) *FileService {
	return &FileService{
		db:      db,
		dataDir: filepath.Join(dataDir, "file-store"),
	}
}

// Upload 上传文件，filePath 为相对路径如 v1.0/app.tar.gz
// 同一路径的并发上传通过 singleflight 合并，只执行一次实际写入
func (s *FileService) Upload(filePath string, src io.Reader, uploaderID uint, uploaderName string) (*model.FileEntry, error) {
	filePath = normalizePath(filePath)
	if err := validatePath(filePath); err != nil {
		return nil, err
	}

	type result struct {
		entry *model.FileEntry
		err   error
	}
	v, _, _ := s.sfGroup.Do(filePath, func() (interface{}, error) {
		entry, err := s.doUpload(filePath, src, uploaderID, uploaderName)
		return &result{entry: entry, err: err}, nil
	})
	res := v.(*result)
	return res.entry, res.err
}

func (s *FileService) doUpload(filePath string, src io.Reader, uploaderID uint, uploaderName string) (*model.FileEntry, error) {	diskPath := filepath.Join(s.dataDir, filePath)
	if err := os.MkdirAll(filepath.Dir(diskPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

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

	var entry model.FileEntry
	err = s.db.Where("path = ?", filePath).First(&entry).Error
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
		err = s.db.Create(&entry).Error
	} else {
		err = s.db.Save(&entry).Error
	}
	if err != nil {
		return nil, err
	}

	return &entry, nil
}

// Download 返回元数据和磁盘路径
func (s *FileService) Download(filePath string) (*model.FileEntry, string, error) {
	filePath = normalizePath(filePath)
	if err := validatePath(filePath); err != nil {
		return nil, "", err
	}

	var entry model.FileEntry
	if err := s.db.Where("path = ?", filePath).First(&entry).Error; err != nil {
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
func (s *FileService) Delete(filePath string) error {
	filePath = normalizePath(filePath)
	if err := validatePath(filePath); err != nil {
		return err
	}

	var entry model.FileEntry
	if err := s.db.Where("path = ?", filePath).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return err
	}

	_ = os.Remove(filepath.Join(s.dataDir, filePath))
	return s.db.Delete(&entry).Error
}

// List 列出文件，可按前缀过滤
func (s *FileService) List(prefix string) ([]model.FileEntry, error) {
	query := s.db.Model(&model.FileEntry{})
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

package maven

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
	"gorm.io/gorm"
)

// SnapshotFile SNAPSHOT 文件信息
type SnapshotFile struct {
	Classifier string `json:"classifier" xml:"classifier"`
	Extension  string `json:"extension" xml:"extension"`
	Value      string `json:"value" xml:"value"`           // 时间戳版本文件名
	Updated    string `json:"updated" xml:"updated"`       // 更新时间
	Size       int64  `json:"size" xml:"size,omitempty"`   // 文件大小
}

// SnapshotVersioning SNAPSHOT 版本信息
type SnapshotVersioning struct {
	Snapshot struct {
		Timestamp   string `xml:"timestamp"`
		BuildNumber int    `xml:"buildNumber"`
	} `xml:"snapshot"`
	LastUpdated string         `xml:"lastUpdated"`
	SnapshotVersions []SnapshotFile `xml:"snapshotVersions>snapshotVersion"`
}

// IsSnapshotVersion 检查是否是 SNAPSHOT 版本
func IsSnapshotVersion(version string) bool {
	return strings.HasSuffix(version, "-SNAPSHOT")
}

// GetSnapshotMetadata 获取 SNAPSHOT 版本的 metadata
func (s *Service) GetSnapshotMetadata(ctx context.Context, groupID, artifactID, version string) (io.ReadCloser, int64, error) {
	if !IsSnapshotVersion(version) {
		return nil, 0, fmt.Errorf("not a snapshot version: %s", version)
	}

	// 本地路径
	groupPath := strings.ReplaceAll(groupID, ".", "/")
	localDir := filepath.Join(s.dataDir, groupPath, artifactID, version)
	localPath := filepath.Join(localDir, "maven-metadata.xml")

	// 检查本地缓存（SNAPSHOT 缓存时间较短，这里设置为 5 分钟）
	if info, err := os.Stat(localPath); err == nil {
		// 如果缓存时间在 5 分钟内，使用缓存
		if time.Since(info.ModTime()) < 5*time.Minute {
			file, err := os.Open(localPath)
			if err == nil {
				return file, info.Size(), nil
			}
		}
	}

	// 从上游获取
	upstream := s.upstream()
	upstreamURL := fmt.Sprintf("%s/%s/%s/%s/maven-metadata.xml", upstream, groupPath, artifactID, version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return nil, 0, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	// 读取内容
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, 0, err
	}

	// 缓存到本地
	if err := ensureDir(localDir); err == nil {
		os.WriteFile(localPath, data, 0644)
	}

	// 解析并保存到数据库
	go s.parseAndSaveSnapshotMetadata(groupID, artifactID, version, data)

	return io.NopCloser(strings.NewReader(string(data))), int64(len(data)), nil
}

// parseAndSaveSnapshotMetadata 解析 SNAPSHOT metadata 并保存到数据库
func (s *Service) parseAndSaveSnapshotMetadata(groupID, artifactID, version string, data []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var metadata struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Version    string `xml:"version"`
		Versioning struct {
			Snapshot struct {
				Timestamp   string `xml:"timestamp"`
				BuildNumber int    `xml:"buildNumber"`
			} `xml:"snapshot"`
			LastUpdated      string `xml:"lastUpdated"`
			SnapshotVersions []struct {
				Classifier string `xml:"classifier"`
				Extension  string `xml:"extension"`
				Value      string `xml:"value"`
				Updated    string `xml:"updated"`
			} `xml:"snapshotVersions>snapshotVersion"`
		} `xml:"versioning"`
	}

	if err := xml.Unmarshal(data, &metadata); err != nil {
		return
	}

	// 转换为 JSON
	files := make([]SnapshotFile, 0, len(metadata.Versioning.SnapshotVersions))
	for _, sv := range metadata.Versioning.SnapshotVersions {
		files = append(files, SnapshotFile{
			Classifier: sv.Classifier,
			Extension:  sv.Extension,
			Value:      sv.Value,
			Updated:    sv.Updated,
		})
	}

	filesJSON, _ := json.Marshal(files)

	// 保存到数据库
	var snapshotMeta model.MavenSnapshotMetadata
	err := s.db.WithContext(ctx).Where(
		"group_id = ? AND artifact_id = ? AND version = ?",
		groupID, artifactID, version,
	).First(&snapshotMeta).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			snapshotMeta = model.MavenSnapshotMetadata{
				GroupID:     groupID,
				ArtifactID:  artifactID,
				Version:     version,
				Timestamp:   metadata.Versioning.Snapshot.Timestamp,
				BuildNumber: metadata.Versioning.Snapshot.BuildNumber,
				FilesJSON:   string(filesJSON),
				LastUpdated: metadata.Versioning.LastUpdated,
			}
			s.db.WithContext(ctx).Create(&snapshotMeta)
		}
	} else {
		s.db.WithContext(ctx).Model(&snapshotMeta).Updates(map[string]interface{}{
			"timestamp":    metadata.Versioning.Snapshot.Timestamp,
			"build_number": metadata.Versioning.Snapshot.BuildNumber,
			"files":        string(filesJSON),
			"last_updated": metadata.Versioning.LastUpdated,
		})
	}
}

// GetSnapshotFile 获取 SNAPSHOT 版本的实际文件
// SNAPSHOT 文件名格式: artifactId-version-timestamp-buildNumber.classifier.extension
// 例如: myapp-1.0.0-20240311.123456-1.jar
func (s *Service) GetSnapshotFile(ctx context.Context, groupID, artifactID, version, filename string) (io.ReadCloser, int64, string, error) {
	if !IsSnapshotVersion(version) {
		return nil, 0, "", fmt.Errorf("not a snapshot version: %s", version)
	}

	groupPath := strings.ReplaceAll(groupID, ".", "/")
	localDir := filepath.Join(s.dataDir, groupPath, artifactID, version)
	localPath := filepath.Join(localDir, filename)

	// 检查本地缓存
	if info, err := os.Stat(localPath); err == nil {
		file, err := os.Open(localPath)
		if err == nil {
			return file, info.Size(), localPath, nil
		}
	}

	// 从上游获取
	upstream := s.upstream()
	upstreamURL := fmt.Sprintf("%s/%s/%s/%s/%s", upstream, groupPath, artifactID, version, filename)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return nil, 0, "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, "", err
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, 0, "", ErrFileNotFound
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, "", fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	// 确保目录存在
	if err := ensureDir(localDir); err != nil {
		resp.Body.Close()
		return nil, 0, "", err
	}

	// 创建临时文件
	tmpPath := localPath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		resp.Body.Close()
		return nil, 0, "", err
	}

	// 写入文件
	size, err := io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	resp.Body.Close()

	if err != nil {
		os.Remove(tmpPath)
		return nil, 0, "", err
	}

	// 重命名
	if err := os.Rename(tmpPath, localPath); err != nil {
		os.Remove(tmpPath)
		return nil, 0, "", err
	}

	// 记录到数据库
	go s.recordSnapshotArtifact(groupID, artifactID, version, filename, size)

	file, err := os.Open(localPath)
	if err != nil {
		return nil, 0, "", err
	}

	return file, size, localPath, nil
}

// recordSnapshotArtifact 记录 SNAPSHOT 制品
func (s *Service) recordSnapshotArtifact(groupID, artifactID, version, filename string, size int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 解析文件名获取 classifier 和 extension
	ext := filepath.Ext(filename)
	if ext != "" {
		ext = ext[1:]
	}

	classifier := ""
	// SNAPSHOT 文件名格式: artifactId-version-timestamp-buildNumber-classifier.extension
	// 例如: myapp-1.0.0-20240311.123456-1-sources.jar
	re := regexp.MustCompile(`-\d{8}\.\d{6}-\d+(-[^.]+)?\.[^.]+$`)
	match := re.FindString(filename)
	if match != "" {
		// 去掉时间戳部分和扩展名
		inner := strings.TrimPrefix(match, "-")
		parts := strings.SplitN(inner, "-", 3)
		if len(parts) >= 3 {
			classifier = parts[2]
			// 去掉扩展名
			if idx := strings.LastIndex(classifier, "."); idx > 0 {
				ext = classifier[idx+1:]
				classifier = classifier[:idx]
			}
		}
	}

	// 查找或创建 artifact
	var artifact model.MavenArtifact
	err := s.db.WithContext(ctx).Where(
		"group_id = ? AND artifact_id = ? AND version = ?",
		groupID, artifactID, version,
	).First(&artifact).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			artifact = model.MavenArtifact{
				GroupID:     groupID,
				ArtifactID:  artifactID,
				Version:     version,
				IsSnapshot:  true,
				IsUploaded:  false,
			}
			if err := s.db.WithContext(ctx).Create(&artifact).Error; err != nil {
				return
			}
		} else {
			return
		}
	}

	// 查找或创建文件记录
	var file model.MavenArtifactFile
	err = s.db.WithContext(ctx).Where(
		"artifact_id = ? AND filename = ?",
		artifact.ID, filename,
	).First(&file).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			file = model.MavenArtifactFile{
				ArtifactID: artifact.ID,
				Filename:   filename,
				Path:       filepath.Join(s.dataDir, strings.ReplaceAll(groupID, ".", "/"), artifactID, version, filename),
				Size:       size,
				Classifier: classifier,
				Extension:  ext,
				Cached:     true,
				IsUploaded: false,
			}
			s.db.WithContext(ctx).Create(&file)
		}
	}
}

// UploadSnapshot 上传 SNAPSHOT 版本
func (s *Service) UploadSnapshot(ctx context.Context, opts UploadOptions, reader io.Reader) error {
	if !IsSnapshotVersion(opts.Version) {
		return fmt.Errorf("not a snapshot version: %s", opts.Version)
	}

	// 生成时间戳版本号
	timestamp := time.Now().UTC().Format("20060102.150405")

	// 查询当前 buildNumber
	var snapshotMeta model.MavenSnapshotMetadata
	err := s.db.WithContext(ctx).Where(
		"group_id = ? AND artifact_id = ? AND version = ?",
		opts.GroupID, opts.ArtifactID, opts.Version,
	).First(&snapshotMeta).Error

	buildNumber := 1
	if err == nil {
		buildNumber = snapshotMeta.BuildNumber + 1
	}

	// 生成新的文件名
	// 原始: myapp-1.0.0-SNAPSHOT.jar
	// 新: myapp-1.0.0-20240311.123456-1.jar
	baseName := strings.TrimSuffix(opts.Filename, filepath.Ext(opts.Filename))
	ext := filepath.Ext(opts.Filename)

	// 移除 -SNAPSHOT 后缀
	baseName = strings.TrimSuffix(baseName, "-SNAPSHOT")

	// 添加时间戳和构建号
	timestampedName := fmt.Sprintf("%s-%s-%d%s", baseName, timestamp, buildNumber, ext)

	// 保存文件
	groupPath := strings.ReplaceAll(opts.GroupID, ".", "/")
	localDir := filepath.Join(s.dataDir, groupPath, opts.ArtifactID, opts.Version)
	localPath := filepath.Join(localDir, timestampedName)

	if err := ensureDir(localDir); err != nil {
		return err
	}

	tmpPath := localPath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	size, err := io.Copy(tmpFile, reader)
	tmpFile.Close()

	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, localPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

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
					GroupID:     opts.GroupID,
					ArtifactID:  opts.ArtifactID,
					Version:     opts.Version,
					IsSnapshot:  true,
					IsUploaded:  true,
					UploaderID:  opts.UploaderID,
					Uploader:    opts.Uploader,
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
			Filename:   timestampedName,
			Path:       localPath,
			Size:       size,
			Classifier: opts.Classifier,
			Extension:  opts.Extension,
			Cached:     true,
			IsUploaded: true,
		}

		if err := tx.Create(&file).Error; err != nil {
			return err
		}

		// 更新或创建 SNAPSHOT metadata
		var meta model.MavenSnapshotMetadata
		err = tx.Where(
			"group_id = ? AND artifact_id = ? AND version = ?",
			opts.GroupID, opts.ArtifactID, opts.Version,
		).First(&meta).Error

		filesJSON := "[]"
		if err == nil && meta.FilesJSON != "" {
			// 解析现有文件列表并添加新文件
			var files []SnapshotFile
			json.Unmarshal([]byte(meta.FilesJSON), &files)
			files = append(files, SnapshotFile{
				Classifier: opts.Classifier,
				Extension:  opts.Extension,
				Value:      timestampedName,
				Updated:    time.Now().UTC().Format("20060102150405"),
				Size:       size,
			})
			data, _ := json.Marshal(files)
			filesJSON = string(data)
		} else {
			files := []SnapshotFile{{
				Classifier: opts.Classifier,
				Extension:  opts.Extension,
				Value:      timestampedName,
				Updated:    time.Now().UTC().Format("20060102150405"),
				Size:       size,
			}}
			data, _ := json.Marshal(files)
			filesJSON = string(data)
		}

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				meta = model.MavenSnapshotMetadata{
					GroupID:     opts.GroupID,
					ArtifactID:  opts.ArtifactID,
					Version:     opts.Version,
					Timestamp:   timestamp,
					BuildNumber: buildNumber,
					FilesJSON:   filesJSON,
					LastUpdated: time.Now().UTC().Format("20060102150405"),
				}
				return tx.Create(&meta).Error
			}
			return err
		}

		return tx.Model(&meta).Updates(map[string]interface{}{
			"timestamp":    timestamp,
			"build_number": buildNumber,
			"files":        filesJSON,
			"last_updated": time.Now().UTC().Format("20060102150405"),
		}).Error
	})
}

// GenerateSnapshotMetadata 生成 SNAPSHOT 版本的 maven-metadata.xml
func (s *Service) GenerateSnapshotMetadata(ctx context.Context, groupID, artifactID, version string) ([]byte, error) {
	if !IsSnapshotVersion(version) {
		return nil, fmt.Errorf("not a snapshot version: %s", version)
	}

	// 查询 SNAPSHOT metadata
	var meta model.MavenSnapshotMetadata
	err := s.db.WithContext(ctx).Where(
		"group_id = ? AND artifact_id = ? AND version = ?",
		groupID, artifactID, version,
	).First(&meta).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 返回空的 metadata
			return s.generateEmptySnapshotMetadata(groupID, artifactID, version), nil
		}
		return nil, err
	}

	// 解析文件列表
	var files []SnapshotFile
	if meta.FilesJSON != "" {
		json.Unmarshal([]byte(meta.FilesJSON), &files)
	}

	// 构建 XML
	var snapshotVersions strings.Builder
	for _, f := range files {
		snapshotVersions.WriteString(fmt.Sprintf(`
      <snapshotVersion>
        <extension>%s</extension>
        <value>%s</value>
        <updated>%s</updated>
      </snapshotVersion>`, f.Extension, f.Value, f.Updated))
		if f.Classifier != "" {
			// 需要重新构建 XML 片段包含 classifier
		}
	}

	metadata := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<metadata modelVersion="1.1.0">
  <groupId>%s</groupId>
  <artifactId>%s</artifactId>
  <version>%s</version>
  <versioning>
    <snapshot>
      <timestamp>%s</timestamp>
      <buildNumber>%d</buildNumber>
    </snapshot>
    <lastUpdated>%s</lastUpdated>
    <snapshotVersions>%s
    </snapshotVersions>
  </versioning>
</metadata>`,
		groupID,
		artifactID,
		version,
		meta.Timestamp,
		meta.BuildNumber,
		meta.LastUpdated,
		snapshotVersions.String(),
	)

	return []byte(metadata), nil
}

// generateEmptySnapshotMetadata 生成空的 SNAPSHOT metadata
func (s *Service) generateEmptySnapshotMetadata(groupID, artifactID, version string) []byte {
	now := time.Now().UTC().Format("20060102150405")
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<metadata modelVersion="1.1.0">
  <groupId>%s</groupId>
  <artifactId>%s</artifactId>
  <version>%s</version>
  <versioning>
    <snapshot>
      <timestamp>%s</timestamp>
      <buildNumber>0</buildNumber>
    </snapshot>
    <lastUpdated>%s</lastUpdated>
    <snapshotVersions>
    </snapshotVersions>
  </versioning>
</metadata>`, groupID, artifactID, version, now[:8]+"."+now[8:], now))
}

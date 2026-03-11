package maven

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

var (
	ErrArtifactNotFound = errors.New("artifact not found")
	ErrFileNotFound     = errors.New("file not found")
	ErrVersionExists    = errors.New("version already exists")
)

// Service Maven 仓库服务
type Service struct {
	db         *gorm.DB
	dataDir    string
	settingSvc *service.SettingService
	httpClient *http.Client
}

// New 创建 Maven 服务实例
func New(db *gorm.DB, dataDir string, settingSvc *service.SettingService) *Service {
	return &Service{
		db:         db,
		dataDir:    filepath.Join(dataDir, "maven"),
		settingSvc: settingSvc,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// upstream 获取上游 Maven 仓库地址
func (s *Service) upstream() string {
	// 默认使用 Maven Central
	upstream := s.settingSvc.GetMavenUpstream()
	if upstream == "" {
		return "https://repo.maven.apache.org/maven2"
	}
	return upstream
}

// ensureDir 确保目录存在
func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// parseGAV 从路径解析 GAV 坐标
// 路径格式: /group/id/artifactId/version/filename
// 例如: /com/example/myapp/1.0.0/myapp-1.0.0.jar
func parseGAV(path string) (groupID, artifactID, version, filename string, err error) {
	// 移除前导斜杠
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		return "", "", "", "", fmt.Errorf("invalid path: %s", path)
	}

	// 最后两部分是 version 和 filename
	filename = parts[len(parts)-1]
	version = parts[len(parts)-2]
	artifactID = parts[len(parts)-3]

	// 剩余部分是 groupId
	groupParts := parts[:len(parts)-3]
	groupID = strings.Join(groupParts, ".")

	return groupID, artifactID, version, filename, nil
}

// artifactPath 计算制品在磁盘上的存储路径
func (s *Service) artifactPath(groupID, artifactID, version, filename string) string {
	// 将 groupId 中的 . 替换为 /
	groupPath := strings.ReplaceAll(groupID, ".", "/")
	return filepath.Join(s.dataDir, groupPath, artifactID, version, filename)
}

// metadataPath 计算 maven-metadata.xml 的存储路径
func (s *Service) metadataPath(groupID, artifactID string) string {
	groupPath := strings.ReplaceAll(groupID, ".", "/")
	return filepath.Join(s.dataDir, groupPath, artifactID, "maven-metadata.xml")
}

// calculateSHA1 计算文件的 SHA1
func calculateSHA1(r io.Reader) (string, error) {
	h := sha1.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetArtifactFile 获取制品文件，优先从本地缓存获取，不存在则从上游代理
func (s *Service) GetArtifactFile(ctx context.Context, path string) (io.ReadCloser, int64, string, error) {
	groupID, artifactID, version, filename, err := parseGAV(path)
	if err != nil {
		return nil, 0, "", err
	}

	localPath := s.artifactPath(groupID, artifactID, version, filename)

	// 1. 检查本地缓存
	if fileExists(localPath) {
		file, err := os.Open(localPath)
		if err != nil {
			return nil, 0, "", err
		}
		stat, _ := file.Stat()
		return file, stat.Size(), localPath, nil
	}

	// 2. 从上游代理
	return s.proxyFromUpstream(ctx, groupID, artifactID, version, filename, localPath)
}

// proxyFromUpstream 从上游仓库获取文件并缓存
func (s *Service) proxyFromUpstream(ctx context.Context, groupID, artifactID, version, filename, localPath string) (io.ReadCloser, int64, string, error) {
	upstream := s.upstream()

	// 构建上游 URL
	groupPath := strings.ReplaceAll(groupID, ".", "/")
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
	if err := ensureDir(filepath.Dir(localPath)); err != nil {
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

	// 同时写入文件和计算 SHA1
	teeReader := io.TeeReader(resp.Body, tmpFile)
	sha1Hash := sha1.New()
	size, err := io.Copy(sha1Hash, teeReader)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		resp.Body.Close()
		return nil, 0, "", err
	}

	tmpFile.Close()
	resp.Body.Close()

	// 重命名为最终文件
	if err := os.Rename(tmpPath, localPath); err != nil {
		os.Remove(tmpPath)
		return nil, 0, "", err
	}

	// 保存 SHA1 校验文件
	sha1Value := hex.EncodeToString(sha1Hash.Sum(nil))
	sha1Path := localPath + ".sha1"
	os.WriteFile(sha1Path, []byte(sha1Value), 0644)

	// 记录到数据库（异步）
	go s.recordArtifact(groupID, artifactID, version, filename, size, sha1Value)

	// 重新打开文件供读取
	file, err := os.Open(localPath)
	if err != nil {
		return nil, 0, "", err
	}

	return file, size, localPath, nil
}

// recordArtifact 记录制品信息到数据库
func (s *Service) recordArtifact(groupID, artifactID, version, filename string, size int64, checksum string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 解析文件扩展名和 classifier
	ext := filepath.Ext(filename)
	if ext != "" {
		ext = ext[1:] // 移除前导点
	}

	classifier := ""
	// 文件名格式: artifactId-version[-classifier].extension
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	prefix := artifactID + "-" + version
	if strings.HasPrefix(baseName, prefix+"-") {
		classifier = strings.TrimPrefix(baseName, prefix+"-")
	}

	// 查找或创建 artifact
	var artifact model.MavenArtifact
	err := s.db.WithContext(ctx).Where(
		"group_id = ? AND artifact_id = ? AND version = ?",
		groupID, artifactID, version,
	).First(&artifact).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			artifact = model.MavenArtifact{
				GroupID:    groupID,
				ArtifactID: artifactID,
				Version:    version,
				IsUploaded: false,
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			file = model.MavenArtifactFile{
				ArtifactID: artifact.ID,
				Filename:   filename,
				Path:       s.artifactPath(groupID, artifactID, version, filename),
				Size:       size,
				Checksum:   checksum,
				Classifier: classifier,
				Extension:  ext,
				Cached:     true,
				IsUploaded: false,
			}
			s.db.WithContext(ctx).Create(&file)
		}
	}
}

// GetMetadata 获取 maven-metadata.xml
func (s *Service) GetMetadata(ctx context.Context, groupID, artifactID string) (io.ReadCloser, int64, error) {
	localPath := s.metadataPath(groupID, artifactID)

	// 优先使用本地缓存
	if fileExists(localPath) {
		file, err := os.Open(localPath)
		if err != nil {
			return nil, 0, err
		}
		stat, _ := file.Stat()
		return file, stat.Size(), nil
	}

	// 从上游获取
	upstream := s.upstream()
	groupPath := strings.ReplaceAll(groupID, ".", "/")
	upstreamURL := fmt.Sprintf("%s/%s/%s/maven-metadata.xml", upstream, groupPath, artifactID)

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

	// 缓存到本地
	if err := ensureDir(filepath.Dir(localPath)); err != nil {
		resp.Body.Close()
		return nil, 0, err
	}

	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, 0, err
	}

	if err := os.WriteFile(localPath, data, 0644); err != nil {
		return nil, 0, err
	}

	// 解析并保存元数据到数据库
	go s.parseAndSaveMetadata(groupID, artifactID, data)

	file, err := os.Open(localPath)
	if err != nil {
		return nil, 0, err
	}

	return file, int64(len(data)), nil
}

// parseAndSaveMetadata 解析 maven-metadata.xml 并保存到数据库
func (s *Service) parseAndSaveMetadata(groupID, artifactID string, data []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 简单解析 XML 提取版本信息
	// 注意：这里使用简单的字符串解析，实际生产环境建议使用 xml.Unmarshal
	content := string(data)

	// 提取最新版本
	latest := extractXMLValue(content, "<latest>", "</latest>")
	release := extractXMLValue(content, "<release>", "</release>")
	lastUpdated := extractXMLValue(content, "<lastUpdated>", "</lastUpdated>")

	// 提取所有版本
	versions := extractAllVersions(content)
	versionsJSON := "[]"
	if len(versions) > 0 {
		// 简单拼接 JSON 数组
		versionsJSON = "["
		for i, v := range versions {
			if i > 0 {
				versionsJSON += ","
			}
			versionsJSON += "\"" + v + "\""
		}
		versionsJSON += "]"
	}

	// 保存到数据库
	var metadata model.MavenMetadata
	err := s.db.WithContext(ctx).Where(
		"group_id = ? AND artifact_id = ?",
		groupID, artifactID,
	).First(&metadata).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			metadata = model.MavenMetadata{
				GroupID:        groupID,
				ArtifactID:     artifactID,
				LatestVersion:  latest,
				ReleaseVersion: release,
				VersionsJSON:   versionsJSON,
				LastUpdated:    lastUpdated,
			}
			s.db.WithContext(ctx).Create(&metadata)
		}
	} else {
		s.db.WithContext(ctx).Model(&metadata).Updates(map[string]interface{}{
			"latest_version":  latest,
			"release_version": release,
			"versions":        versionsJSON,
			"last_updated":    lastUpdated,
		})
	}
}

// extractXMLValue 从 XML 中提取标签值
func extractXMLValue(content, startTag, endTag string) string {
	start := strings.Index(content, startTag)
	if start == -1 {
		return ""
	}
	start += len(startTag)
	end := strings.Index(content[start:], endTag)
	if end == -1 {
		return ""
	}
	return content[start : start+end]
}

// extractAllVersions 从 XML 中提取所有版本
func extractAllVersions(content string) []string {
	var versions []string
	startTag := "<version>"
	endTag := "</version>"

	for {
		start := strings.Index(content, startTag)
		if start == -1 {
			break
		}
		start += len(startTag)
		end := strings.Index(content[start:], endTag)
		if end == -1 {
			break
		}
		version := content[start : start+end]
		versions = append(versions, version)
		content = content[start+end+len(endTag):]
	}

	return versions
}

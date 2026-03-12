package maven

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
)

// RepositoryConfig 仓库配置
type RepositoryConfig struct {
	Name        string `json:"name" xml:"name"`
	URL         string `json:"url" xml:"url"`
	Enabled     bool   `json:"enabled" xml:"enabled,attr"`
	Priority    int    `json:"priority" xml:"priority,attr"` // 优先级，数字越小优先级越高
	IsPrivate   bool   `json:"is_private" xml:"isPrivate,attr"`
	Username    string `json:"username" xml:"username,omitempty"`
	Password    string `json:"password" xml:"password,omitempty"`
}

// DefaultRepositories 默认仓库配置
var DefaultRepositories = []RepositoryConfig{
	{
		Name:     "central",
		URL:      "https://repo.maven.apache.org/maven2",
		Enabled:  true,
		Priority: 100,
	},
	{
		Name:     "aliyun",
		URL:      "https://maven.aliyun.com/repository/public",
		Enabled:  true,
		Priority: 50,
	},
}

// GetRepositories 获取所有启用的仓库配置
func (s *Service) GetRepositories(ctx context.Context) ([]RepositoryConfig, error) {
	var repos []model.MavenRepository
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Order("id").Find(&repos).Error; err != nil {
		return nil, err
	}

	if len(repos) == 0 {
		// 返回默认配置
		return DefaultRepositories, nil
	}

	configs := make([]RepositoryConfig, 0, len(repos))
	for _, repo := range repos {
		configs = append(configs, RepositoryConfig{
			Name:    repo.Name,
			URL:     repo.Upstream,
			Enabled: repo.Enabled,
		})
	}

	return configs, nil
}

// AddRepository 添加仓库
func (s *Service) AddRepository(ctx context.Context, config RepositoryConfig) (*model.MavenRepository, error) {
	repo := model.MavenRepository{
		Name:        config.Name,
		Upstream:    config.URL,
		Enabled:     config.Enabled,
		Description: "",
	}

	if err := s.db.WithContext(ctx).Create(&repo).Error; err != nil {
		return nil, err
	}

	return &repo, nil
}

// UpdateRepository 更新仓库
func (s *Service) UpdateRepository(ctx context.Context, id uint, config RepositoryConfig) error {
	return s.db.WithContext(ctx).Model(&model.MavenRepository{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name":     config.Name,
		"upstream": config.URL,
		"enabled":  config.Enabled,
	}).Error
}

// DeleteRepository 删除仓库
func (s *Service) DeleteRepository(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.MavenRepository{}, id).Error
}

// GetArtifactFileWithFallback 多仓库回退获取制品文件
func (s *Service) GetArtifactFileWithFallback(ctx context.Context, path string) (io.ReadCloser, int64, string, error) {
	groupID, artifactID, version, filename, err := parseGAV(path)
	if err != nil {
		return nil, 0, "", err
	}

	// 1. 检查本地缓存
	localPath := s.artifactPath(groupID, artifactID, version, filename)
	if fileExists(localPath) {
		file, err := os.Open(localPath)
		if err == nil {
			stat, _ := file.Stat()
			return file, stat.Size(), localPath, nil
		}
	}

	// 2. 从配置的仓库列表中依次尝试
	repos, err := s.GetRepositories(ctx)
	if err != nil {
		// 使用默认仓库
		repos = DefaultRepositories
	}

	var lastErr error
	for _, repo := range repos {
		if !repo.Enabled {
			continue
		}

		reader, size, actualPath, err := s.proxyFromRepository(ctx, repo, groupID, artifactID, version, filename, localPath)
		if err == nil {
			return reader, size, actualPath, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, 0, "", lastErr
	}
	return nil, 0, "", ErrFileNotFound
}

// proxyFromRepository 从指定仓库代理文件
func (s *Service) proxyFromRepository(ctx context.Context, repo RepositoryConfig, groupID, artifactID, version, filename, localPath string) (io.ReadCloser, int64, string, error) {
	groupPath := strings.ReplaceAll(groupID, ".", "/")
	upstreamURL := fmt.Sprintf("%s/%s/%s/%s/%s", repo.URL, groupPath, artifactID, version, filename)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return nil, 0, "", err
	}

	// 添加认证信息
	if repo.IsPrivate && repo.Username != "" && repo.Password != "" {
		req.SetBasicAuth(repo.Username, repo.Password)
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
		return nil, 0, "", fmt.Errorf("repository %s returned status %d", repo.Name, resp.StatusCode)
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
	go s.recordArtifact(groupID, artifactID, version, filename, size, "")

	// 重新打开文件
	file, err := os.Open(localPath)
	if err != nil {
		return nil, 0, "", err
	}

	return file, size, localPath, nil
}

// GetMetadataWithFallback 多仓库回退获取 metadata
func (s *Service) GetMetadataWithFallback(ctx context.Context, groupID, artifactID string) (io.ReadCloser, int64, error) {
	localPath := s.metadataPath(groupID, artifactID)

	// 优先使用本地缓存
	if fileExists(localPath) {
		file, err := os.Open(localPath)
		if err == nil {
			stat, _ := file.Stat()
			return file, stat.Size(), nil
		}
	}

	// 从配置的仓库列表中依次尝试
	repos, err := s.GetRepositories(ctx)
	if err != nil {
		repos = DefaultRepositories
	}

	var lastErr error
	for _, repo := range repos {
		if !repo.Enabled {
			continue
		}

		reader, size, err := s.getMetadataFromRepository(ctx, repo, groupID, artifactID, localPath)
		if err == nil {
			return reader, size, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, 0, lastErr
	}
	return nil, 0, ErrFileNotFound
}

// getMetadataFromRepository 从指定仓库获取 metadata
func (s *Service) getMetadataFromRepository(ctx context.Context, repo RepositoryConfig, groupID, artifactID, localPath string) (io.ReadCloser, int64, error) {
	groupPath := strings.ReplaceAll(groupID, ".", "/")
	upstreamURL := fmt.Sprintf("%s/%s/%s/maven-metadata.xml", repo.URL, groupPath, artifactID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return nil, 0, err
	}

	if repo.IsPrivate && repo.Username != "" && repo.Password != "" {
		req.SetBasicAuth(repo.Username, repo.Password)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("repository %s returned status %d", repo.Name, resp.StatusCode)
	}

	// 读取内容
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, 0, err
	}

	// 缓存到本地
	if err := ensureDir(filepath.Dir(localPath)); err == nil {
		os.WriteFile(localPath, data, 0644)
	}

	// 解析并保存
	go s.parseAndSaveMetadata(groupID, artifactID, data)

	return io.NopCloser(strings.NewReader(string(data))), int64(len(data)), nil
}

// SearchArtifacts 搜索制品
func (s *Service) SearchArtifacts(ctx context.Context, query string, page, pageSize int) ([]model.MavenArtifact, int64, error) {
	var artifacts []model.MavenArtifact
	var total int64

	dbQuery := s.db.WithContext(ctx).Model(&model.MavenArtifact{})

	// 支持按 groupId 或 artifactId 搜索
	if query != "" {
		dbQuery = dbQuery.Where(
			"group_id LIKE ? OR artifact_id LIKE ?",
			"%"+query+"%", "%"+query+"%",
		)
	}

	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := dbQuery.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&artifacts).Error; err != nil {
		return nil, 0, err
	}

	return artifacts, total, nil
}

// GetVersions 获取制品的所有版本
func (s *Service) GetVersions(ctx context.Context, groupID, artifactID string) ([]string, error) {
	var artifacts []model.MavenArtifact
	if err := s.db.WithContext(ctx).
		Where("group_id = ? AND artifact_id = ?", groupID, artifactID).
		Order("created_at DESC").
		Find(&artifacts).Error; err != nil {
		return nil, err
	}

	versions := make([]string, 0, len(artifacts))
	for _, a := range artifacts {
		versions = append(versions, a.Version)
	}

	return versions, nil
}

// ParsePOM 解析 POM 文件
type POM struct {
	XMLName      xml.Name `xml:"project"`
	GroupID      string   `xml:"groupId"`
	ArtifactID   string   `xml:"artifactId"`
	Version      string   `xml:"version"`
	Packaging    string   `xml:"packaging"`
	Name         string   `xml:"name"`
	Description  string   `xml:"description"`
	URL          string   `xml:"url"`
	Dependencies struct {
		Dependency []struct {
			GroupID    string `xml:"groupId"`
			ArtifactID string `xml:"artifactId"`
			Version    string `xml:"version"`
			Scope      string `xml:"scope"`
			Optional   bool   `xml:"optional"`
			Exclusions []struct {
				Exclusion struct {
					GroupID    string `xml:"groupId"`
					ArtifactID string `xml:"artifactId"`
				} `xml:"exclusion"`
			} `xml:"exclusions>exclusion"`
		} `xml:"dependency"`
	} `xml:"dependencies>dependency"`
	Parent struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Version    string `xml:"version"`
	} `xml:"parent"`
	DependencyManagement struct {
		Dependencies struct {
			Dependency []struct {
				GroupID    string `xml:"groupId"`
				ArtifactID string `xml:"artifactId"`
				Version    string `xml:"version"`
			} `xml:"dependency"`
		} `xml:"dependencies"`
	} `xml:"dependencyManagement"`
	Modules struct {
		Module []string `xml:"module"`
	} `xml:"modules>module"`
	Properties struct {
		Property []struct {
			Name  string `xml:"name"`
			Value string `xml:"value"`
		} `xml:"property"`
	} `xml:"properties>property"`
}

// ParsePOMFile 解析 POM 文件内容
func ParsePOMFile(data []byte) (*POM, error) {
	var pom POM
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil, err
	}

	// 处理继承的 groupId 和 version
	if pom.GroupID == "" && pom.Parent.GroupID != "" {
		pom.GroupID = pom.Parent.GroupID
	}
	if pom.Version == "" && pom.Parent.Version != "" {
		pom.Version = pom.Parent.Version
	}

	// 默认 packaging 为 jar
	if pom.Packaging == "" {
		pom.Packaging = "jar"
	}

	return &pom, nil
}

// ResolveDependencies 解析依赖
func (s *Service) ResolveDependencies(ctx context.Context, pom *POM) ([]Dependency, error) {
	deps := make([]Dependency, 0)

	for _, d := range pom.Dependencies.Dependency {
		// 跳过 optional 依赖
		if d.Optional {
			continue
		}

		// 跳过 test scope（除非特别指定）
		if d.Scope == "test" {
			continue
		}

		dep := Dependency{
			GroupID:    d.GroupID,
			ArtifactID: d.ArtifactID,
			Version:    d.Version,
			Scope:      d.Scope,
		}

		// 处理版本号中的属性引用
		if strings.HasPrefix(dep.Version, "${") && strings.HasSuffix(dep.Version, "}") {
			propName := dep.Version[2 : len(dep.Version)-1]
			for _, prop := range pom.Properties.Property {
				if prop.Name == propName {
					dep.Version = prop.Value
					break
				}
			}
		}

		// 如果版本号为空，尝试从 dependencyManagement 中查找
		if dep.Version == "" {
			for _, dm := range pom.DependencyManagement.Dependencies.Dependency {
				if dm.GroupID == d.GroupID && dm.ArtifactID == d.ArtifactID {
					dep.Version = dm.Version
					break
				}
			}
		}

		deps = append(deps, dep)
	}

	return deps, nil
}

// Dependency 依赖信息
type Dependency struct {
	GroupID    string `json:"group_id"`
	ArtifactID string `json:"artifact_id"`
	Version    string `json:"version"`
	Scope      string `json:"scope"`
}

// DownloadDependency 下载依赖
func (s *Service) DownloadDependency(ctx context.Context, dep Dependency) (string, error) {
	// 构建路径
	groupPath := strings.ReplaceAll(dep.GroupID, ".", "/")
	filename := fmt.Sprintf("%s-%s.jar", dep.ArtifactID, dep.Version)
	path := fmt.Sprintf("/%s/%s/%s/%s", groupPath, dep.ArtifactID, dep.Version, filename)

	_, _, actualPath, err := s.GetArtifactFileWithFallback(ctx, path)
	if err != nil {
		// 尝试下载 pom 文件获取 packaging 类型
		pomFilename := fmt.Sprintf("%s-%s.pom", dep.ArtifactID, dep.Version)
		pomPath := fmt.Sprintf("/%s/%s/%s/%s", groupPath, dep.ArtifactID, dep.Version, pomFilename)
		_, _, _, err = s.GetArtifactFileWithFallback(ctx, pomPath)
		if err != nil {
			return "", err
		}

		// 解析 pom 获取实际的 packaging 类型
		pomData, err := os.ReadFile(actualPath[:len(actualPath)-3] + "pom")
		if err == nil {
			if pom, err := ParsePOMFile(pomData); err == nil {
				if pom.Packaging != "jar" && pom.Packaging != "" {
					// 重新构建文件名
					filename = fmt.Sprintf("%s-%s.%s", dep.ArtifactID, dep.Version, pom.Packaging)
					path = fmt.Sprintf("/%s/%s/%s/%s", groupPath, dep.ArtifactID, dep.Version, filename)
					_, _, actualPath, err = s.GetArtifactFileWithFallback(ctx, path)
					if err != nil {
						return "", err
					}
				}
			}
		}
	}

	return actualPath, nil
}

package npm

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

var (
	ErrPackageNotFound = errors.New("package not found")
	ErrVersionNotFound = errors.New("version not found")
	ErrTarballNotFound = errors.New("tarball not found")
	ErrVersionExists   = errors.New("version already exists")
)

// Service npm 仓库服务，负责本地发布、代理缓存、元数据管理
type Service struct {
	db          *gorm.DB
	dataDir     string           // {data}/npm
	settingSvc  *service.SettingService
	httpClient  *http.Client
	sfGroup     singleflight.Group // 防止并发重复代理同一包
}

func New(db *gorm.DB, dataDir string, settingSvc *service.SettingService) *Service {
	return &Service{
		db:         db,
		dataDir:    filepath.Join(dataDir, "npm"),
		settingSvc: settingSvc,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// upstream 动态读取上游地址
func (s *Service) upstream() string {
	return s.settingSvc.GetNpmUpstream()
}

// abbreviatedFields 是 abbreviated packument 格式（application/vnd.npm.install-v1+json）
// 中每个 version 对象需要保留的字段，其余字段（readme、scripts、homepage 等）一律丢弃。
var abbreviatedFields = map[string]bool{
	"name": true, "version": true, "dist": true,
	"dependencies": true, "optionalDependencies": true,
	"peerDependencies": true, "peerDependenciesMeta": true,
	"devDependencies": true, "bundleDependencies": true,
	"engines": true, "deprecated": true, "_hasShrinkwrap": true,
}

// abbreviateVersionMeta 将完整的 version 元数据 JSON 精简为仅含安装必需字段的格式。
func abbreviateVersionMeta(raw json.RawMessage) json.RawMessage {
	var full map[string]json.RawMessage
	if err := json.Unmarshal(raw, &full); err != nil {
		return raw
	}
	abbr := make(map[string]json.RawMessage, len(abbreviatedFields))
	for k, v := range full {
		if abbreviatedFields[k] {
			abbr[k] = v
		}
	}
	result, err := json.Marshal(abbr)
	if err != nil {
		return raw
	}
	return result
}

// ── 磁盘路径辅助 ─────────────────────────────────────────────────────────────

// tarballDir 返回存放某个包 tarball 的目录：{dataDir}/{name}/-/
func (s *Service) tarballDir(pkgName string) string {
	// 对于 scoped 包（@scope/name）将 / 替换为 %2F 避免嵌套目录歧义
	safe := strings.ReplaceAll(pkgName, "/", "%2F")
	return filepath.Join(s.dataDir, safe, "-")
}

func (s *Service) tarballPath(pkgName, filename string) string {
	return filepath.Join(s.tarballDir(pkgName), filename)
}

// ── 公共辅助：改写版本元数据中的 tarball URL ─────────────────────────────────

// rewriteTarballURL 将版本元数据 JSON 中的 dist.tarball 改写为本地地址
func (s *Service) rewriteTarballURL(metaRaw json.RawMessage, pkgName, tarballName, baseURL string) (json.RawMessage, error) {
	var meta map[string]interface{}
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		return metaRaw, err
	}

	localURL := fmt.Sprintf("%s/npm/%s/-/%s", baseURL, pkgName, tarballName)
	if dist, ok := meta["dist"].(map[string]interface{}); ok {
		dist["tarball"] = localURL
	}

	return json.Marshal(meta)
}

// ── 公共辅助：从版本元数据提取 dist 信息 ────────────────────────────────────

type distInfo struct {
	tarballName string
	shasum      string
	integrity   string
	upstreamURL string
}

func extractDistInfo(metaRaw json.RawMessage, pkgName, version string) distInfo {
	info := distInfo{
		tarballName: fmt.Sprintf("%s-%s.tgz", sanitizePkgName(pkgName), version),
	}

	var meta struct {
		Dist struct {
			Tarball   string `json:"tarball"`
			Shasum    string `json:"shasum"`
			Integrity string `json:"integrity"`
		} `json:"dist"`
	}
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		return info
	}

	info.shasum = meta.Dist.Shasum
	info.integrity = meta.Dist.Integrity
	info.upstreamURL = meta.Dist.Tarball

	// 从 URL 提取 tarball 文件名（更准确）
	if meta.Dist.Tarball != "" {
		parts := strings.Split(meta.Dist.Tarball, "/")
		if len(parts) > 0 {
			info.tarballName = parts[len(parts)-1]
		}
	}

	return info
}

// sanitizePkgName 将 @scope/name 转换为 name（npm tarball 约定只用短名）
func sanitizePkgName(name string) string {
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

// ── 从 DB 构建 Packument ──────────────────────────────────────────────────────

func (s *Service) buildPackumentFromDB(name, baseURL string, abbreviated bool) (*Packument, error) {
	var pkg model.NpmPackage
	if err := s.db.Where("name = ?", name).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPackageNotFound
		}
		return nil, err
	}

	var versions []model.NpmVersion
	if err := s.db.Where("package_id = ?", pkg.ID).Find(&versions).Error; err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, ErrPackageNotFound
	}

	var distTags map[string]string
	_ = json.Unmarshal([]byte(pkg.DistTags), &distTags)
	if distTags == nil {
		distTags = map[string]string{}
	}

	versionMap := make(map[string]json.RawMessage, len(versions))
	for _, v := range versions {
		rewritten, err := s.rewriteTarballURL(json.RawMessage(v.MetaJSON), name, v.TarballName, baseURL)
		if err != nil {
			rewritten = json.RawMessage(v.MetaJSON)
		}
		if abbreviated {
			rewritten = abbreviateVersionMeta(rewritten)
		}
		versionMap[v.Version] = rewritten
	}

	return &Packument{
		ID:          name,
		Name:        name,
		Description: pkg.Description,
		Readme:      pkg.Readme,
		DistTags:    distTags,
		Versions:    versionMap,
	}, nil
}

// upsertPackage 创建或更新 NpmPackage 记录，返回最新的 pkg
func (s *Service) upsertPackage(name, description, readme string, distTags map[string]string) (model.NpmPackage, error) {
	return s.upsertPackageWith(s.db, name, description, readme, distTags)
}

func (s *Service) upsertPackageWith(db *gorm.DB, name, description, readme string, distTags map[string]string) (model.NpmPackage, error) {
	distTagsJSON, _ := json.Marshal(distTags)

	var pkg model.NpmPackage
	err := db.Where("name = ?", name).First(&pkg).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		pkg = model.NpmPackage{
			Name:        name,
			Description: description,
			Readme:      readme,
			DistTags:    string(distTagsJSON),
		}
		return pkg, db.Create(&pkg).Error
	}
	if err != nil {
		return pkg, err
	}

	updates := map[string]interface{}{"dist_tags": string(distTagsJSON)}
	if description != "" {
		updates["description"] = description
	}
	if readme != "" {
		updates["readme"] = readme
	}
	return pkg, db.Model(&pkg).Updates(updates).Error
}

// ensureDir 确保目录存在
func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

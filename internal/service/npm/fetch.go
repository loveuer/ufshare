package npm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"gorm.io/gorm"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
)

// PackageSummary 用于前端列表展示
type PackageSummary struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	DistTags     map[string]string `json:"dist_tags"`
	VersionCount int               `json:"version_count"`
	CachedCount  int               `json:"cached_count"`
}

// VersionSummary 用于版本列表展示
type VersionSummary struct {
	Version     string `json:"version"`
	TarballName string `json:"tarball_name"`
	Size        int64  `json:"size"`
	Shasum      string `json:"shasum"`
	Cached      bool   `json:"cached"`
	Uploader    string `json:"uploader"`
	CreatedAt   string `json:"created_at"`
}

// versionCount 用于 GROUP BY 聚合查询
type versionCount struct {
	PackageID   uint
	Total       int
	CachedCount int
}

// ListPackages 返回分页后的包摘要列表，同时返回总数。
// search 为空时不过滤；单次查询聚合版本计数，无 N+1 问题。
func (s *Service) ListPackages(ctx context.Context, page, pageSize int, search string) ([]PackageSummary, int64, error) {
	query := s.db.WithContext(ctx).Model(&model.NpmPackage{})
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var pkgs []model.NpmPackage
	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&pkgs).Error; err != nil {
		return nil, 0, err
	}
	if len(pkgs) == 0 {
		return []PackageSummary{}, total, nil
	}

	// 收集本页包的 ID，一次查出所有版本计数（消除 N+1）
	pkgIDs := make([]uint, len(pkgs))
	for i, p := range pkgs {
		pkgIDs[i] = p.ID
	}
	var counts []versionCount
	s.db.WithContext(ctx).Model(&model.NpmVersion{}).
		Select("package_id, COUNT(*) as total, SUM(CASE WHEN cached = 1 THEN 1 ELSE 0 END) as cached_count").
		Where("package_id IN ?", pkgIDs).
		Group("package_id").
		Scan(&counts)
	countMap := make(map[uint]versionCount, len(counts))
	for _, c := range counts {
		countMap[c.PackageID] = c
	}

	result := make([]PackageSummary, 0, len(pkgs))
	for _, pkg := range pkgs {
		var distTags map[string]string
		_ = json.Unmarshal([]byte(pkg.DistTags), &distTags)
		c := countMap[pkg.ID]
		result = append(result, PackageSummary{
			Name:         pkg.Name,
			Description:  pkg.Description,
			DistTags:     distTags,
			VersionCount: c.Total,
			CachedCount:  c.CachedCount,
		})
	}
	return result, total, nil
}

// ListVersions 返回某个包的版本列表
func (s *Service) ListVersions(ctx context.Context, name string) ([]VersionSummary, error) {
	var pkg model.NpmPackage
	if err := s.db.WithContext(ctx).Where("name = ?", name).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPackageNotFound
		}
		return nil, err
	}

	var versions []model.NpmVersion
	if err := s.db.WithContext(ctx).Where("package_id = ?", pkg.ID).Order("created_at desc").Find(&versions).Error; err != nil {
		return nil, err
	}

	result := make([]VersionSummary, 0, len(versions))
	for _, v := range versions {
		result = append(result, VersionSummary{
			Version:     v.Version,
			TarballName: v.TarballName,
			Size:        v.Size,
			Shasum:      v.Shasum,
			Cached:      v.Cached,
			Uploader:    v.Uploader,
			CreatedAt:   v.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return result, nil
}

// GetPackument 返回包的完整 packument。
// 优先从本地 DB 构建；本地没有则代理上游并缓存。
// abbreviated=true 时返回精简格式（仅含安装必需字段），响应体更小。
func (s *Service) GetPackument(ctx context.Context, name, baseURL string, abbreviated bool) (*Packument, error) {
	pack, err := s.buildPackumentFromDB(ctx, name, baseURL, abbreviated)
	if err == nil {
		return pack, nil
	}
	if !errors.Is(err, ErrPackageNotFound) {
		return nil, err
	}
	// 上游拉取后始终存完整元数据，再按需精简返回
	if _, e := s.proxyAndCachePackument(ctx, name, ""); e != nil {
		return nil, e
	}
	return s.buildPackumentFromDB(ctx, name, baseURL, abbreviated)
}

// GetVersion 返回指定版本的元数据（version-level packument）
func (s *Service) GetVersion(ctx context.Context, name, version string) (json.RawMessage, error) {
	var pkg model.NpmPackage
	if err := s.db.WithContext(ctx).Where("name = ?", name).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPackageNotFound
		}
		return nil, err
	}

	var ver model.NpmVersion
	if err := s.db.WithContext(ctx).Where("package_id = ? AND version = ?", pkg.ID, version).First(&ver).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVersionNotFound
		}
		return nil, err
	}

	return json.RawMessage(ver.MetaJSON), nil
}

// ServeTarball 将 tarball 内容写入 w。
// 已缓存时直接从磁盘读取；未缓存时流式从上游拉取并同步写入磁盘缓存，
// 使 npm 客户端无需等待完整下载即可开始接收字节。
func (s *Service) ServeTarball(ctx context.Context, pkgName, filename string, w io.Writer) error {
	diskPath := s.tarballPath(pkgName, filename)

	// 命中缓存：直接读磁盘
	if f, err := os.Open(diskPath); err == nil {
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	}

	// 未缓存：先解析上游 URL，再流式传输
	upstreamURL, ver, err := s.resolveTarballURL(ctx, pkgName, filename)
	if err != nil {
		return err
	}
	return s.streamAndCacheTarball(ctx, pkgName, ver, upstreamURL, diskPath, w)
}

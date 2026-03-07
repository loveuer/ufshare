package npm

import (
	"encoding/json"
	"errors"
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

// ListPackages 返回所有已缓存/发布的包摘要列表
func (s *Service) ListPackages() ([]PackageSummary, error) {
	var pkgs []model.NpmPackage
	if err := s.db.Find(&pkgs).Error; err != nil {
		return nil, err
	}

	result := make([]PackageSummary, 0, len(pkgs))
	for _, pkg := range pkgs {
		var total, cached int64
		s.db.Model(&model.NpmVersion{}).Where("package_id = ?", pkg.ID).Count(&total)
		s.db.Model(&model.NpmVersion{}).Where("package_id = ? AND cached = ?", pkg.ID, true).Count(&cached)

		var distTags map[string]string
		_ = json.Unmarshal([]byte(pkg.DistTags), &distTags)

		result = append(result, PackageSummary{
			Name:         pkg.Name,
			Description:  pkg.Description,
			DistTags:     distTags,
			VersionCount: int(total),
			CachedCount:  int(cached),
		})
	}
	return result, nil
}

// ListVersions 返回某个包的版本列表
func (s *Service) ListVersions(name string) ([]VersionSummary, error) {
	var pkg model.NpmPackage
	if err := s.db.Where("name = ?", name).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPackageNotFound
		}
		return nil, err
	}

	var versions []model.NpmVersion
	if err := s.db.Where("package_id = ?", pkg.ID).Order("created_at desc").Find(&versions).Error; err != nil {
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
func (s *Service) GetPackument(name, baseURL string) (*Packument, error) {
	pack, err := s.buildPackumentFromDB(name, baseURL)
	if err == nil {
		return pack, nil
	}
	if !errors.Is(err, ErrPackageNotFound) {
		return nil, err
	}

	// 本地未找到 → 代理上游
	return s.proxyAndCachePackument(name, baseURL)
}

// GetVersion 返回指定版本的元数据（version-level packument）
func (s *Service) GetVersion(name, version string) (json.RawMessage, error) {
	var pkg model.NpmPackage
	if err := s.db.Where("name = ?", name).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPackageNotFound
		}
		return nil, err
	}

	var ver model.NpmVersion
	if err := s.db.Where("package_id = ? AND version = ?", pkg.ID, version).First(&ver).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVersionNotFound
		}
		return nil, err
	}

	return json.RawMessage(ver.MetaJSON), nil
}

// GetTarball 返回 tarball 的本地磁盘路径。
// 若未缓存则先从上游下载并缓存。
func (s *Service) GetTarball(name, filename string) (string, error) {
	diskPath := s.tarballPath(name, filename)

	// 已缓存，直接返回
	if _, err := os.Stat(diskPath); err == nil {
		return diskPath, nil
	}

	// 未缓存 → 代理下载
	if err := s.proxyAndCacheTarball(name, filename, diskPath); err != nil {
		return "", err
	}

	return diskPath, nil
}

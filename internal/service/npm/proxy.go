package npm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gorm.io/gorm"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
)

// proxyAndCachePackument 从上游拉取 packument，持久化到 DB，返回带本地 URL 的 packument
func (s *Service) proxyAndCachePackument(name, baseURL string) (*Packument, error) {
	upstreamURL := fmt.Sprintf("%s/%s", s.upstream, name)
	resp, err := s.httpClient.Get(upstreamURL)
	if err != nil {
		return nil, fmt.Errorf("upstream unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, ErrPackageNotFound
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upstream response: %w", err)
	}

	// 解析上游 packument
	var upstream struct {
		Name        string                     `json:"name"`
		Description string                     `json:"description"`
		Readme      string                     `json:"readme"`
		DistTags    map[string]string          `json:"dist-tags"`
		Versions    map[string]json.RawMessage `json:"versions"`
	}
	if err := json.Unmarshal(body, &upstream); err != nil {
		return nil, fmt.Errorf("parse upstream packument: %w", err)
	}

	// 写入 DB（create or update）
	pkg, err := s.upsertPackage(upstream.Name, upstream.Description, upstream.Readme, upstream.DistTags)
	if err != nil {
		return nil, fmt.Errorf("cache package metadata: %w", err)
	}

	// 写入版本元数据（跳过已存在的版本）
	for version, metaRaw := range upstream.Versions {
		var existing model.NpmVersion
		if s.db.Where("package_id = ? AND version = ?", pkg.ID, version).First(&existing).Error == nil {
			continue
		}

		di := extractDistInfo(metaRaw, name, version)

		npmVer := model.NpmVersion{
			PackageID:   pkg.ID,
			Version:     version,
			MetaJSON:    string(metaRaw), // 保存原始上游 URL，对外输出时改写
			TarballName: di.tarballName,
			Shasum:      di.shasum,
			Integrity:   di.integrity,
			Cached:      false, // tarball 尚未缓存
		}
		s.db.Create(&npmVer) //nolint:errcheck // 单个版本失败不影响整体
	}

	// 从 DB 构建带本地 URL 的 packument 返回
	return s.buildPackumentFromDB(name, baseURL)
}

// proxyAndCacheTarball 从上游下载 tarball 并保存到本地磁盘
func (s *Service) proxyAndCacheTarball(pkgName, filename, diskPath string) error {
	// 从 DB 查 version 记录，以获取上游 tarball URL
	var ver model.NpmVersion
	err := s.db.
		Joins("JOIN npm_packages ON npm_packages.id = npm_versions.package_id").
		Where("npm_packages.name = ? AND npm_versions.tarball_name = ?", pkgName, filename).
		First(&ver).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// DB 中没有记录，尝试先缓存 packument（会填充版本信息）
		if _, proxyErr := s.proxyAndCachePackument(pkgName, ""); proxyErr != nil {
			return ErrTarballNotFound
		}
		// 重新查询
		if err2 := s.db.
			Joins("JOIN npm_packages ON npm_packages.id = npm_versions.package_id").
			Where("npm_packages.name = ? AND npm_versions.tarball_name = ?", pkgName, filename).
			First(&ver).Error; err2 != nil {
			return ErrTarballNotFound
		}
	} else if err != nil {
		return err
	}

	// 从元数据中提取上游 URL
	di := extractDistInfo(json.RawMessage(ver.MetaJSON), pkgName, ver.Version)
	upstreamURL := di.upstreamURL
	if upstreamURL == "" {
		upstreamURL = fmt.Sprintf("%s/%s/-/%s", s.upstream, pkgName, filename)
	}

	resp, err := s.httpClient.Get(upstreamURL)
	if err != nil {
		return fmt.Errorf("upstream unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("upstream returned HTTP %d for tarball", resp.StatusCode)
	}

	// 原子写：先写临时文件，成功后 rename
	if err := ensureDir(filepath.Dir(diskPath)); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	tmp := diskPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("download tarball: %w", err)
	}
	f.Close()

	if err := os.Rename(tmp, diskPath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("save tarball: %w", err)
	}

	// 标记为已缓存
	s.db.Model(&ver).Update("cached", true) //nolint:errcheck

	return nil
}

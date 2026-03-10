package npm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
)

// proxyAndCachePackument 从上游拉取 packument，持久化到 DB，返回带本地 URL 的 packument。
// 内部通过 singleflight 合并同名包的并发请求，避免 SQLite 并发写冲突。
func (s *Service) proxyAndCachePackument(ctx context.Context, name, baseURL string) (*Packument, error) {
	// singleflight：只有一个 goroutine 真正去上游拉取并写 DB
	type sfResult struct {
		err error
	}
	v, _, _ := s.sfGroup.Do("packument:"+name, func() (interface{}, error) {
		return &sfResult{err: s.fetchAndSavePackument(ctx, name)}, nil
	})
	if res := v.(*sfResult); res.err != nil {
		// 如果是 404，返回 ErrPackageNotFound
		if errors.Is(res.err, ErrPackageNotFound) {
			return nil, ErrPackageNotFound
		}
		return nil, res.err
	}

	// 从 DB 构建带本地 URL 的 packument（每个调用者用各自的 baseURL）
	return s.buildPackumentFromDB(ctx, name, baseURL, false)
}

// fetchAndSavePackument 从上游拉取 packument 并写入 DB（不含 baseURL 改写）
func (s *Service) fetchAndSavePackument(ctx context.Context, name string) error {
	upstreamURL := fmt.Sprintf("%s/%s", s.upstream(), name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upstream unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return ErrPackageNotFound
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read upstream response: %w", err)
	}

	var upstream struct {
		Name        string                     `json:"name"`
		Description string                     `json:"description"`
		Readme      string                     `json:"readme"`
		DistTags    map[string]string          `json:"dist-tags"`
		Versions    map[string]json.RawMessage `json:"versions"`
	}
	if err := json.Unmarshal(body, &upstream); err != nil {
		return fmt.Errorf("parse upstream packument: %w", err)
	}

	pkg, err := s.upsertPackage(ctx, upstream.Name, upstream.Description, upstream.Readme, upstream.DistTags)
	if err != nil {
		return fmt.Errorf("cache package metadata: %w", err)
	}

	// 一次查出已存在的版本号，避免逐条 SELECT
	var existingVersions []string
	s.db.WithContext(ctx).Model(&model.NpmVersion{}).
		Where("package_id = ?", pkg.ID).
		Pluck("version", &existingVersions)
	existingSet := make(map[string]struct{}, len(existingVersions))
	for _, v := range existingVersions {
		existingSet[v] = struct{}{}
	}

	// 收集需要新建的版本，批量插入
	var newVersions []model.NpmVersion
	for version, metaRaw := range upstream.Versions {
		if _, exists := existingSet[version]; exists {
			continue
		}
		di := extractDistInfo(metaRaw, upstream.Name, version)
		newVersions = append(newVersions, model.NpmVersion{
			PackageID:   pkg.ID,
			Version:     version,
			MetaJSON:    string(metaRaw),
			TarballName: di.tarballName,
			Shasum:      di.shasum,
			Integrity:   di.integrity,
			Cached:      false,
		})
	}
	if len(newVersions) > 0 {
		s.db.WithContext(ctx).CreateInBatches(newVersions, 100) //nolint:errcheck
	}
	return nil
}

// resolveTarballURL 从 DB 查找 tarball 的上游 URL。
// 若 DB 中无记录，则先拉取 packument 填充，再重查。
func (s *Service) resolveTarballURL(ctx context.Context, pkgName, filename string) (string, model.NpmVersion, error) {
	var ver model.NpmVersion
	query := func() error {
		return s.db.WithContext(ctx).
			Joins("JOIN npm_packages ON npm_packages.id = npm_versions.package_id").
			Where("npm_packages.name = ? AND npm_versions.tarball_name = ?", pkgName, filename).
			First(&ver).Error
	}

	if err := query(); errors.Is(err, gorm.ErrRecordNotFound) {
		// DB 中无记录 → 先缓存 packument
		if proxyErr := s.fetchAndSavePackument(ctx, pkgName); proxyErr != nil {
			return "", ver, ErrTarballNotFound
		}
		if err2 := query(); err2 != nil {
			return "", ver, ErrTarballNotFound
		}
	} else if err != nil {
		return "", ver, err
	}

	di := extractDistInfo(json.RawMessage(ver.MetaJSON), pkgName, ver.Version)
	upstreamURL := di.upstreamURL
	if upstreamURL == "" {
		upstreamURL = fmt.Sprintf("%s/%s/-/%s", s.upstream(), pkgName, filename)
	}
	return upstreamURL, ver, nil
}

// streamAndCacheTarball 从 upstreamURL 流式拉取 tarball，同时：
//   - 即时转发字节流给 w（npm 客户端无需等待完整下载）
//   - 用 TeeReader 并行写入临时磁盘文件，下载完成后原子 rename 为最终缓存路径
//
// 多个并发请求同一 tarball 各自独立流式下载（使用唯一 tmp 文件名），
// 最后一个 rename 覆盖前一个（内容相同，幂等）。
func (s *Service) streamAndCacheTarball(ctx context.Context, pkgName string, ver model.NpmVersion, upstreamURL, diskPath string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upstream unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("upstream returned HTTP %d for tarball", resp.StatusCode)
	}

	// 准备缓存目录和唯一临时文件（避免并发写冲突）
	var cacheFile *os.File
	tmp := fmt.Sprintf("%s.tmp.%d", diskPath, time.Now().UnixNano())
	if err2 := ensureDir(filepath.Dir(diskPath)); err2 == nil {
		cacheFile, _ = os.Create(tmp)
	}

	var src io.Reader = resp.Body
	if cacheFile != nil {
		src = io.TeeReader(resp.Body, cacheFile) // 边读边写磁盘
	}

	_, copyErr := io.Copy(w, src) // 立即推送给客户端

	if cacheFile != nil {
		cacheFile.Close()
		if copyErr == nil {
			if renameErr := os.Rename(tmp, diskPath); renameErr == nil {
				// 标记为已缓存（忽略 DB 错误，不影响已完成的传输）
				s.db.WithContext(ctx).Model(&ver).Update("cached", true) //nolint:errcheck
			}
		} else {
			os.Remove(tmp)
		}
	}

	return copyErr
}

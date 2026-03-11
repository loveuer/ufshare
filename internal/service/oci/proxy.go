package oci

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
)

// upstreamRegistryHost 将上游地址转换为实际的 registry API host
// Docker Hub 特殊处理：docker.io → registry-1.docker.io
func (s *Service) upstreamRegistryHost() string {
	upstream := s.upstream()
	upstream = strings.TrimRight(upstream, "/")

	// Docker Hub 特殊处理
	if upstream == "https://docker.io" || upstream == "https://index.docker.io" {
		return "https://registry-1.docker.io"
	}
	return upstream
}

// authScope 根据 repo name 生成 auth scope
func authScope(name string) string {
	return fmt.Sprintf("repository:%s:pull", name)
}

// normalizeImageName 标准化镜像名称
// 对 Docker Hub：短名如 "nginx" → "library/nginx"
func (s *Service) normalizeImageName(name string) string {
	upstream := s.upstream()
	// Docker Hub 的镜像如果没有 / 则补 library/ 前缀
	if (strings.Contains(upstream, "docker.io") || strings.Contains(upstream, "docker.com")) && !strings.Contains(name, "/") {
		return "library/" + name
	}
	return name
}

// ProxyManifest 从上游拉取 manifest 并缓存到 DB
// reference 可以是 tag 名或 digest
func (s *Service) ProxyManifest(ctx context.Context, name, reference string) (content []byte, mediaType, digest string, err error) {
	name = s.normalizeImageName(name)
	scope := authScope(name)

	type sfResult struct {
		content   []byte
		mediaType string
		digest    string
		err       error
	}

	key := fmt.Sprintf("manifest:%s:%s", name, reference)
	v, _, _ := s.sfManifest.Do(key, func() (interface{}, error) {
		c, mt, d, e := s.fetchManifestFromUpstream(ctx, name, reference, scope)
		return &sfResult{content: c, mediaType: mt, digest: d, err: e}, nil
	})

	res := v.(*sfResult)
	return res.content, res.mediaType, res.digest, res.err
}

// fetchManifestFromUpstream 从上游拉取 manifest
func (s *Service) fetchManifestFromUpstream(ctx context.Context, name, reference, scope string) ([]byte, string, string, error) {
	registryHost := s.upstreamRegistryHost()
	requestURL := fmt.Sprintf("%s/v2/%s/manifests/%s", registryHost, name, reference)

	auth := newUpstreamAuth(s.client)
	resp, err := auth.doWithAuth(ctx, http.MethodGet, requestURL, scope)
	if err != nil {
		return nil, "", "", fmt.Errorf("upstream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, "", "", ErrManifestNotFound
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", "", fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", fmt.Errorf("read manifest: %w", err)
	}

	mediaType := resp.Header.Get("Content-Type")
	digest := resp.Header.Get("Docker-Content-Digest")

	// 如果上游没返回 digest，自己算
	if digest == "" {
		digest = computeDigest(body)
	}

	// 持久化到 DB
	if saveErr := s.saveManifest(ctx, name, reference, digest, mediaType, body); saveErr != nil {
		log.Printf("[oci] save manifest %s:%s: %v", name, reference, saveErr)
	}

	return body, mediaType, digest, nil
}

// saveManifest 将 manifest 保存到 DB
func (s *Service) saveManifest(ctx context.Context, name, reference, digest, mediaType string, content []byte) error {
	// 确保 repository 存在
	repo, err := s.ensureRepository(ctx, name)
	if err != nil {
		return err
	}

	// upsert manifest
	var existing model.OciManifest
	err = s.db.WithContext(ctx).Where("digest = ?", digest).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		manifest := model.OciManifest{
			RepositoryID: repo.ID,
			Digest:       digest,
			MediaType:    mediaType,
			Content:      string(content),
			Size:         int64(len(content)),
		}
		if err := s.db.WithContext(ctx).Create(&manifest).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// 如果 reference 是 tag（非 digest），更新 tag 映射
	if !strings.HasPrefix(reference, "sha256:") {
		return s.upsertTag(ctx, repo.ID, reference, digest)
	}

	return nil
}

// ensureRepository 确保 repository 记录存在
func (s *Service) ensureRepository(ctx context.Context, name string) (model.OciRepository, error) {
	var repo model.OciRepository
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&repo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		upstream := s.upstream()
		repo = model.OciRepository{
			Name:     name,
			Upstream: upstream,
		}
		return repo, s.db.WithContext(ctx).Create(&repo).Error
	}
	return repo, err
}

// upsertTag 创建或更新 tag
func (s *Service) upsertTag(ctx context.Context, repoID uint, tag, digest string) error {
	var existing model.OciTag
	err := s.db.WithContext(ctx).
		Where("repository_id = ? AND tag = ?", repoID, tag).
		First(&existing).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.WithContext(ctx).Create(&model.OciTag{
			RepositoryID:   repoID,
			Tag:            tag,
			ManifestDigest: digest,
		}).Error
	}
	if err != nil {
		return err
	}

	if existing.ManifestDigest != digest {
		return s.db.WithContext(ctx).Model(&existing).Update("manifest_digest", digest).Error
	}
	return nil
}

// ProxyBlob 从上游拉取 blob 并流式返回+缓存
func (s *Service) ProxyBlob(ctx context.Context, name, digest string, w io.Writer) (int64, error) {
	name = s.normalizeImageName(name)
	scope := authScope(name)

	diskPath := s.blobPath(digest)

	// 检查是否已缓存
	if fi, err := os.Stat(diskPath); err == nil {
		f, err := os.Open(diskPath)
		if err == nil {
			defer f.Close()
			io.Copy(w, f)
			return fi.Size(), nil
		}
	}

	registryHost := s.upstreamRegistryHost()
	requestURL := fmt.Sprintf("%s/v2/%s/blobs/%s", registryHost, name, digest)

	auth := newUpstreamAuth(s.client)
	resp, err := auth.doWithAuth(ctx, http.MethodGet, requestURL, scope)
	if err != nil {
		return 0, fmt.Errorf("upstream request: %w", err)
	}

	if resp.StatusCode == 404 {
		resp.Body.Close()
		return 0, ErrBlobNotFound
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return 0, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(body))
	}

	return s.streamAndCacheBlob(ctx, name, digest, diskPath, resp, w)
}

// streamAndCacheBlob 流式下载 blob，边传给客户端边写磁盘
func (s *Service) streamAndCacheBlob(ctx context.Context, name, digest, diskPath string, resp *http.Response, w io.Writer) (int64, error) {
	defer resp.Body.Close()

	size := resp.ContentLength

	// 准备缓存文件
	var cacheFile *os.File
	tmp := fmt.Sprintf("%s.tmp.%d", diskPath, time.Now().UnixNano())
	if err := ensureDir(filepath.Dir(diskPath)); err == nil {
		cacheFile, _ = os.Create(tmp)
	}

	var src io.Reader = resp.Body
	if cacheFile != nil {
		src = io.TeeReader(resp.Body, cacheFile)
	}

	n, copyErr := io.Copy(w, src)

	if cacheFile != nil {
		cacheFile.Close()
		if copyErr == nil {
			if renameErr := os.Rename(tmp, diskPath); renameErr == nil {
				// 记录 blob 到 DB
				s.ensureBlobRecord(ctx, name, digest, n)
			}
		} else {
			os.Remove(tmp)
		}
	}

	if size <= 0 {
		size = n
	}
	return size, copyErr
}

// ensureBlobRecord 确保 blob 在 DB 中有记录
func (s *Service) ensureBlobRecord(ctx context.Context, name, digest string, size int64) {
	repo, err := s.ensureRepository(ctx, name)
	if err != nil {
		return
	}

	var existing model.OciBlob
	err = s.db.WithContext(ctx).Where("digest = ?", digest).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s.db.WithContext(ctx).Create(&model.OciBlob{
			RepositoryID: repo.ID,
			Digest:       digest,
			Size:         size,
			Cached:       true,
		})
	} else if err == nil && !existing.Cached {
		s.db.WithContext(ctx).Model(&existing).Update("cached", true)
	}
}

// computeDigest 计算 content 的 sha256 digest
func computeDigest(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

// manifestMediaTypes 判断是否为 manifest list / image index
func isManifestList(mediaType string) bool {
	return mediaType == "application/vnd.docker.distribution.manifest.list.v2+json" ||
		mediaType == "application/vnd.oci.image.index.v1+json"
}

// ManifestPlatform 从 manifest list 中提取的平台信息
type ManifestPlatform struct {
	Digest    string `json:"digest"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Platform  struct {
		Architecture string `json:"architecture"`
		OS           string `json:"os"`
		Variant      string `json:"variant,omitempty"`
	} `json:"platform"`
}

// ParseManifestList 解析 manifest list/image index
func ParseManifestList(content []byte) ([]ManifestPlatform, error) {
	var ml struct {
		Manifests []ManifestPlatform `json:"manifests"`
	}
	if err := json.Unmarshal(content, &ml); err != nil {
		return nil, err
	}
	return ml.Manifests, nil
}

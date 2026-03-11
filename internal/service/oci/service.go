package oci

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

var (
	ErrManifestNotFound = errors.New("manifest not found")
	ErrBlobNotFound     = errors.New("blob not found")
	ErrRepoNotFound     = errors.New("repository not found")
)

// Service OCI 镜像代理服务
type Service struct {
	db         *gorm.DB
	dataDir    string // {data}/oci
	settingSvc *service.SettingService

	mu         sync.RWMutex
	httpClient *http.Client

	sfManifest singleflight.Group
	sfBlob     singleflight.Group
}

// New 创建 OCI 服务实例
func New(db *gorm.DB, dataDir string, settingSvc *service.SettingService) *Service {
	s := &Service{
		db:         db,
		dataDir:    filepath.Join(dataDir, "oci"),
		settingSvc: settingSvc,
	}
	s.rebuildClient()

	// 代理设置变更时重建 httpClient
	settingSvc.OnChange(service.SettingOciHttpProxy, func(_ string) { s.rebuildClient() })
	settingSvc.OnChange(service.SettingOciHttpsProxy, func(_ string) { s.rebuildClient() })

	return s
}

// upstream 获取上游 registry 地址
func (s *Service) upstream() string {
	return s.settingSvc.GetOciUpstream()
}

// rebuildClient 根据当前 proxy 设置重建 httpClient
func (s *Service) rebuildClient() {
	httpProxy := s.settingSvc.Get(service.SettingOciHttpProxy)
	httpsProxy := s.settingSvc.Get(service.SettingOciHttpsProxy)

	transport := &http.Transport{
		MaxIdleConns:        50,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	if httpProxy != "" || httpsProxy != "" {
		transport.Proxy = func(req *http.Request) (*url.URL, error) {
			if req.URL.Scheme == "https" && httpsProxy != "" {
				return url.Parse(httpsProxy)
			}
			if httpProxy != "" {
				return url.Parse(httpProxy)
			}
			return nil, nil
		}
	}

	s.mu.Lock()
	s.httpClient = &http.Client{
		Timeout:   300 * time.Second, // 大镜像下载需要更长超时
		Transport: transport,
	}
	s.mu.Unlock()
}

// client 获取 httpClient（线程安全）
func (s *Service) client() *http.Client {
	s.mu.RLock()
	c := s.httpClient
	s.mu.RUnlock()
	return c
}

// blobDir 返回 blob 存储目录
func (s *Service) blobDir() string {
	return filepath.Join(s.dataDir, "blobs", "sha256")
}

// blobPath 返回 blob 文件路径
func (s *Service) blobPath(digest string) string {
	// digest 格式: sha256:abc123...
	hash := digest
	if len(digest) > 7 && digest[:7] == "sha256:" {
		hash = digest[7:]
	}
	return filepath.Join(s.blobDir(), hash)
}

// ensureDir 确保目录存在
func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

package goproxy

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/goproxy/goproxy"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

// Service 封装 goproxy，提供 Go 模块代理服务
type Service struct {
	dataDir    string
	settingSvc *service.SettingService

	mu        sync.RWMutex    // 保护 proxy、upstream、goprivate 的并发读写
	proxy     *goproxy.Goproxy
	upstream  string
	goprivate string
}

// New 创建 Go 模块代理服务
func New(dataDir string, settingSvc *service.SettingService) *Service {
	s := &Service{
		dataDir:    dataDir,
		settingSvc: settingSvc,
		upstream:   settingSvc.GetGoUpstream(),
		goprivate:  settingSvc.GetGoPrivate(),
	}

	s.initProxy()

	// 监听配置变更，持锁更新字段后重新初始化 proxy
	settingSvc.OnChange(service.SettingGoUpstream, func(v string) {
		s.mu.Lock()
		s.upstream = v
		s.initProxyLocked()
		s.mu.Unlock()
		log.Printf("[go] upstream changed to: %s", v)
	})

	settingSvc.OnChange(service.SettingGoPrivate, func(v string) {
		s.mu.Lock()
		s.goprivate = v
		s.initProxyLocked()
		s.mu.Unlock()
		log.Printf("[go] goprivate changed to: %s", v)
	})

	return s
}

// initProxy 首次初始化（调用前无需加锁，New 尚未返回故无并发）
func (s *Service) initProxy() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initProxyLocked()
}

// initProxyLocked 重新构建 goproxy 实例，调用方必须持有 s.mu 写锁
func (s *Service) initProxyLocked() {
	cacheDir := filepath.Join(s.dataDir, "go-cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("[go] failed to create cache dir: %v", err)
	}

	upstream := s.upstream
	if upstream == "" {
		upstream = service.DefaultGoUpstream
	}

	env := append(
		os.Environ(),
		"GOPROXY="+upstream+",direct",
	)
	if s.goprivate != "" {
		env = append(env, "GOPRIVATE="+s.goprivate)
	}

	s.proxy = &goproxy.Goproxy{
		Fetcher: &goproxy.GoFetcher{
			Env: env,
		},
		Cacher: goproxy.DirCacher(cacheDir),
		ProxiedSumDBs: []string{
			"sum.golang.org https://goproxy.cn/sumdb/sum.golang.org",
		},
	}
}

// ServeHTTP 实现 http.Handler 接口，持读锁读取 proxy 指针后转发请求
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	p := s.proxy
	s.mu.RUnlock()
	p.ServeHTTP(w, r)
}

// GetCacheStats 返回缓存统计信息
func (s *Service) GetCacheStats() (map[string]interface{}, error) {
	cacheDir := filepath.Join(s.dataDir, "go-cache")

	var size int64
	var fileCount int

	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
			fileCount++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	upstream := s.upstream
	goprivate := s.goprivate
	s.mu.RUnlock()

	return map[string]interface{}{
		"cache_dir":  cacheDir,
		"size_bytes": size,
		"file_count": fileCount,
		"upstream":   upstream,
		"goprivate":  goprivate,
	}, nil
}

// CleanCache 清理缓存目录
func (s *Service) CleanCache() error {
	cacheDir := filepath.Join(s.dataDir, "go-cache")
	return os.RemoveAll(cacheDir)
}

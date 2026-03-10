package goproxy

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/goproxy/goproxy"
	"gorm.io/gorm"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/service"
)

// Service 封装 goproxy，提供 Go 模块代理服务
type Service struct {
	dataDir       string
	settingSvc    *service.SettingService
	proxy         *goproxy.Goproxy
	upstream      string
	goprivate     string
}

// New 创建 Go 模块代理服务
func New(db *gorm.DB, dataDir string, settingSvc *service.SettingService) *Service {
	s := &Service{
		dataDir:    dataDir,
		settingSvc: settingSvc,
		upstream:   settingSvc.GetGoUpstream(),
		goprivate:  settingSvc.GetGoPrivate(),
	}

	// 初始化 goproxy
	s.initProxy()

	// 监听配置变更
	settingSvc.OnChange(service.SettingGoUpstream, func(v string) {
		s.upstream = v
		s.initProxy()
		log.Printf("[go] upstream changed to: %s", v)
	})

	settingSvc.OnChange(service.SettingGoPrivate, func(v string) {
		s.goprivate = v
		s.initProxy()
		log.Printf("[go] goprivate changed to: %s", v)
	})

	return s
}

// initProxy 初始化或重新初始化 goproxy 实例
func (s *Service) initProxy() {
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

// Handler 返回 http.Handler 用于处理 Go 模块代理请求
func (s *Service) Handler() http.Handler {
	return s.proxy
}

// ServeHTTP 实现 http.Handler 接口
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.proxy.ServeHTTP(w, r)
}

// GetCacheStats 返回缓存统计信息
func (s *Service) GetCacheStats() (map[string]interface{}, error) {
	cacheDir := filepath.Join(s.dataDir, "go-cache")
	
	var size int64
	var fileCount int
	
	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误文件
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

	return map[string]interface{}{
		"cache_dir":   cacheDir,
		"size_bytes":  size,
		"file_count":  fileCount,
		"upstream":    s.upstream,
		"goprivate":   s.goprivate,
	}, nil
}

// CleanCache 清理缓存目录
func (s *Service) CleanCache() error {
	cacheDir := filepath.Join(s.dataDir, "go-cache")
	return os.RemoveAll(cacheDir)
}

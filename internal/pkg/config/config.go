package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBodySize    int64 = 1 << 30 // 1 GB
	defaultJWTSecret         = "ufshare-secret-key-change-in-production"
)

type Config struct {
	Debug    bool
	Address  string        // 监听地址，如 0.0.0.0:9817
	Data     string        // 数据目录，存放上传文件和数据库
	DBPath   string        // 数据库文件路径（可选），不指定时默认为 <data>/ufshare.db
	NpmAddr  string        // npm 专用端口，如 0.0.0.0:4873（可选）
	FileAddr string        // file-store 专用端口，如 0.0.0.0:8001（可选）
	GoAddr   string        // go 模块代理专用端口，如 0.0.0.0:8081（可选）
	OciAddr  string        // OCI/Docker 镜像代理专用端口，如 0.0.0.0:5000（可选）
	BodySize int64         // 请求体大小限制（字节），-1 表示不限制
	Database DatabaseConfig
	JWT      JWTConfig
}

type DatabaseConfig struct {
	Driver string // sqlite, mysql, postgres
	DSN    string
}

type JWTConfig struct {
	Secret string
	Expire time.Duration
}

func Load() *Config {
	return &Config{
		Address: getEnv("UFSHARE_ADDRESS", "0.0.0.0:9817"),
		Data:    getEnv("UFSHARE_DATA", "."),
		Database: DatabaseConfig{
			Driver: getEnv("DB_DRIVER", "sqlite"),
			DSN:    getEnv("DB_DSN", ""),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", ""),
			Expire: 24 * time.Hour,
		},
		BodySize: parseBodySize(getEnv("BODY_SIZE", "1GB")),
	}
}

// Finalize 在命令行 flag 解析完成后调用，补全运行时依赖 Data 才能确定的默认值
func (c *Config) Finalize() {
	if c.Database.Driver == "sqlite" && c.Database.DSN == "" {
		if c.DBPath != "" {
			c.Database.DSN = c.DBPath
		} else {
			c.Database.DSN = filepath.Join(c.Data, "ufshare.db")
		}
	}
}

func (c *Config) Validate() error {
	if c.JWT.Secret == "" {
		return fmt.Errorf(
			"JWT_SECRET environment variable is not set.\n" +
				"Please set a strong random secret before starting UFShare, e.g.:\n\n" +
				"  export JWT_SECRET=$(openssl rand -hex 32)\n\n" +
				"This secret is used to sign authentication tokens and must be kept private.",
		)
	}
	if c.JWT.Secret == defaultJWTSecret {
		return fmt.Errorf(
			"JWT_SECRET is set to the default insecure value.\n" +
				"Please replace it with a strong random secret, e.g.:\n\n" +
				"  export JWT_SECRET=$(openssl rand -hex 32)",
		)
	}
	return nil
}

// parseBodySize 解析人类可读的大小字符串，如 "1GB"、"500MB"、"10737418240"。
// 无法解析时返回默认值 DefaultBodySize。
func parseBodySize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return DefaultBodySize
	}
	if s == "-1" {
		return -1
	}

	upper := strings.ToUpper(s)
	// 按后缀长度从长到短匹配，避免 "GB" 被 "B" 先匹配
	units := []struct {
		suffix string
		mult   int64
	}{
		{"TIB", 1 << 40}, {"GIB", 1 << 30}, {"MIB", 1 << 20}, {"KIB", 1 << 10},
		{"TB", 1_000_000_000_000}, {"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1024}, {"B", 1},
	}
	for _, u := range units {
		if strings.HasSuffix(upper, u.suffix) {
			numStr := strings.TrimSpace(upper[:len(upper)-len(u.suffix)])
			n, err := strconv.ParseFloat(numStr, 64)
			if err != nil || n < 0 {
				return DefaultBodySize
			}
			return int64(n * float64(u.mult))
		}
	}

	// 纯数字（字节）
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return DefaultBodySize
	}
	return n
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

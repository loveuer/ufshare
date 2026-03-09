package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Debug    bool
	Address  string        // 监听地址，如 0.0.0.0:8000
	Data     string        // 数据目录，存放上传文件和数据库
	NpmAddr  string        // npm 专用端口，如 0.0.0.0:4873（可选）
	FileAddr string        // file-store 专用端口，如 0.0.0.0:8001（可选）
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

const defaultJWTSecret = "ufshare-secret-key-change-in-production"

func Load() *Config {
	return &Config{
		Address: getEnv("UFSHARE_ADDRESS", "0.0.0.0:8000"),
		Data:    getEnv("UFSHARE_DATA", "."),
		Database: DatabaseConfig{
			Driver: getEnv("DB_DRIVER", "sqlite"),
			DSN:    getEnv("DB_DSN", ""),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", ""),
			Expire: 24 * time.Hour,
		},
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

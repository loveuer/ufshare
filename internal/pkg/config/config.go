package config

import (
	"os"
	"time"
)

type Config struct {
	Address  string        // 监听地址，如 0.0.0.0:8000
	Data     string        // 数据目录，存放上传文件和数据库
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
		Address: getEnv("UFSHARE_ADDRESS", "0.0.0.0:8000"),
		Data:    getEnv("UFSHARE_DATA", "."),
		Database: DatabaseConfig{
			Driver: getEnv("DB_DRIVER", "sqlite"),
			DSN:    getEnv("DB_DSN", ""),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "ufshare-secret-key-change-in-production"),
			Expire: 24 * time.Hour,
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

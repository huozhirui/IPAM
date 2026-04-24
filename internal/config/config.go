// Package config 提供应用配置管理，支持命令行参数和环境变量两种方式。
package config

import (
	"flag"
	"os"
)

// Config 应用配置项
type Config struct {
	DSN       string // MySQL 连接串，格式: user:pass@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True
	Port      string // HTTP 监听端口，默认 8080
	JWTSecret string // JWT 签名密钥
}

// RegisterFlags 注册命令行参数（不调用 Parse）
func (cfg *Config) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&cfg.DSN, "dsn", "", "MySQL DSN (e.g. user:pass@tcp(127.0.0.1:3306)/ipam?charset=utf8mb4&parseTime=True)")
	fs.StringVar(&cfg.Port, "port", "8080", "HTTP listen port")
	fs.StringVar(&cfg.JWTSecret, "jwt-secret", "change-me-in-production", "JWT signing secret")
}

// ApplyEnv 环境变量覆盖默认值
func (cfg *Config) ApplyEnv() {
	if env := os.Getenv("IPAM_DSN"); env != "" && cfg.DSN == "" {
		cfg.DSN = env
	}
	if env := os.Getenv("IPAM_PORT"); env != "" && cfg.Port == "8080" {
		cfg.Port = env
	}
	if env := os.Getenv("IPAM_JWT_SECRET"); env != "" && cfg.JWTSecret == "change-me-in-production" {
		cfg.JWTSecret = env
	}
}

// Load 加载配置，优先级：命令行参数 > 环境变量 > 默认值
func Load() *Config {
	cfg := &Config{}
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()
	cfg.ApplyEnv()
	return cfg
}

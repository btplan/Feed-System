package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 保存 API 启动时需要读取的服务地址和后续数据库配置。
type Config struct {
	Server struct {
		Addr string `yaml:"addr"`
	} `yaml:"server"`
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
	Auth struct {
		JWTSecret string `yaml:"-"`
	} `yaml:"auth"`
}

// Load 读取公开 YAML 配置和环境中的签名密钥，缺少有效配置时拒绝启动。
func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	if strings.TrimSpace(cfg.Server.Addr) == "" {
		return cfg, fmt.Errorf("config %s: server.addr is required", path)
	}
	secret, err := base64.StdEncoding.DecodeString(os.Getenv("CLIPFLOW_JWT_SECRET"))
	if err != nil || len(secret) < 32 {
		return cfg, fmt.Errorf("CLIPFLOW_JWT_SECRET must be base64 encoding of at least 32 random bytes")
	}
	cfg.Auth.JWTSecret = string(secret)
	return cfg, nil
}

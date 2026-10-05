package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoad 验证配置读取函数能取得 YAML 中的服务地址和数据库路径。
func TestLoad(t *testing.T) {
	secret := strings.Repeat("k", 32)
	t.Setenv("CLIPFLOW_JWT_SECRET", base64.StdEncoding.EncodeToString([]byte(secret)))
	path := filepath.Join(t.TempDir(), "local.yaml")
	data := []byte("server:\n  addr: 127.0.0.1:18081\ndatabase:\n  path: .run/clipflow.db\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Addr != "127.0.0.1:18081" || cfg.Database.Path != ".run/clipflow.db" || cfg.Auth.JWTSecret != secret {
		t.Fatal("unexpected configuration")
	}
}

// TestLoadRejectsInvalidSecret 验证缺失、格式错误和长度不足的密钥均不能启动服务。
func TestLoadRejectsInvalidSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "local.yaml")
	if err := os.WriteFile(path, []byte("server:\n  addr: 127.0.0.1:18080\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"", "not-base64!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		t.Setenv("CLIPFLOW_JWT_SECRET", secret)
		if _, err := Load(path); err == nil {
			t.Fatal("invalid secret accepted")
		}
	}
}

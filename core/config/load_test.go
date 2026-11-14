package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadWithBOM 带 UTF-8 BOM 的配置也应能读取，兼容记事本等编辑器的保存结果
func TestLoadWithBOM(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SESHAT_HOME", dir)

	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte(DefaultConfigTOML)...)
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("带 BOM 的配置应能解析: %v", err)
	}
	if cfg.Server.Port != 12500 || cfg.Server.BindAddr != "127.0.0.1" {
		t.Fatalf("解析结果异常: %+v", cfg.Server)
	}
}

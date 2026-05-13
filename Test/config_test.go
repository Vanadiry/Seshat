package Test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vanadiry/seshat/Core/config"
)

func TestDefaults(t *testing.T) {
	cfg := config.Defaults
	if cfg.Server.Port != 4000 {
		t.Errorf("expected port 4000, got %d", cfg.Server.Port)
	}
	if cfg.Server.BindAddr != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %s", cfg.Server.BindAddr)
	}
	if cfg.Server.Concurrency != 32 {
		t.Errorf("expected concurrency 32, got %d", cfg.Server.Concurrency)
	}
	if cfg.Upstream.BaseURL != "https://api.bgm.tv" {
		t.Errorf("expected api.bgm.tv, got %s", cfg.Upstream.BaseURL)
	}
}

func TestValidate(t *testing.T) {
	cfg := config.Defaults
	if err := config.Validate(&cfg); err != nil {
		t.Errorf("valid config should not error: %v", err)
	}

	cfg.Server.Port = 0
	if err := config.Validate(&cfg); err == nil {
		t.Error("port 0 should error")
	}

	cfg = config.Defaults
	cfg.Server.Concurrency = 0
	if err := config.Validate(&cfg); err == nil {
		t.Error("concurrency 0 should error")
	}
}

func TestDir(t *testing.T) {
	// SESHAT_HOME env var
	os.Setenv("SESHAT_HOME", "/tmp/seshat_test")
	defer os.Unsetenv("SESHAT_HOME")

	dir := config.Dir()
	if dir != "/tmp/seshat_test" {
		t.Errorf("expected /tmp/seshat_test, got %s", dir)
	}
}

func TestDataDir(t *testing.T) {
	os.Setenv("SESHAT_HOME", "/tmp/seshat_test")
	defer os.Unsetenv("SESHAT_HOME")

	cfg := config.Defaults
	dd := cfg.DataDir()
	expected := filepath.Join("/tmp/seshat_test", "data")
	if dd != expected {
		t.Errorf("expected %s, got %s", expected, dd)
	}

	cfg.Server.DataHome = "/custom/data"
	if dd := cfg.DataDir(); dd != "/custom/data" {
		t.Errorf("expected /custom/data, got %s", dd)
	}
}

func TestTrackerDir(t *testing.T) {
	os.Setenv("SESHAT_HOME", "/tmp/seshat_test")
	defer os.Unsetenv("SESHAT_HOME")

	cfg := config.Defaults
	td := cfg.TrackerDir()
	expected := filepath.Join("/tmp/seshat_test", "tracker")
	if td != expected {
		t.Errorf("expected %s, got %s", expected, td)
	}
}

func TestLoadSave(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("SESHAT_HOME", tmp)
	defer os.Unsetenv("SESHAT_HOME")

	// Fresh load should use defaults
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Port != 4000 {
		t.Errorf("expected 4000, got %d", cfg.Server.Port)
	}

	// Modify and save
	cfg.Server.Port = 9000
	cfg.User.Username = "testuser"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Reload
	cfg2, err := config.Load()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if cfg2.Server.Port != 9000 {
		t.Errorf("expected 9000, got %d", cfg2.Server.Port)
	}
	if cfg2.User.Username != "testuser" {
		t.Errorf("expected testuser, got %s", cfg2.User.Username)
	}
}

func TestDefaultConfigTOML(t *testing.T) {
	if config.DefaultConfigTOML == "" {
		t.Error("DefaultConfigTOML should not be empty")
	}
}

func TestTrackerTemplate(t *testing.T) {
	tmpl := config.TrackerTemplate
	if tmpl == "" {
		t.Error("TrackerTemplate should not be empty")
	}
}

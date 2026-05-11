package Test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vanadiry/seshat/Core/config"
)

func TestDefaults(t *testing.T) {
	cfg := config.Defaults
	if cfg.Port != 4000 {
		t.Errorf("expected port 4000, got %d", cfg.Port)
	}
	if cfg.BindAddr != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %s", cfg.BindAddr)
	}
	if cfg.Concurrency != 32 {
		t.Errorf("expected concurrency 32, got %d", cfg.Concurrency)
	}
	if cfg.BaseURL != "https://api.bgm.tv" {
		t.Errorf("expected api.bgm.tv, got %s", cfg.BaseURL)
	}
}

func TestValidate(t *testing.T) {
	cfg := config.Defaults
	if err := config.Validate(&cfg); err != nil {
		t.Errorf("valid config should not error: %v", err)
	}

	cfg.Port = 0
	if err := config.Validate(&cfg); err == nil {
		t.Error("port 0 should error")
	}

	cfg = config.Defaults
	cfg.Concurrency = 999
	if err := config.Validate(&cfg); err == nil {
		t.Error("concurrency 999 should error")
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

	cfg.DataHome = "/custom/data"
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
	if cfg.Port != 4000 {
		t.Errorf("expected 4000, got %d", cfg.Port)
	}

	// Modify and save
	cfg.Port = 9000
	cfg.Username = "testuser"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Reload
	cfg2, err := config.Load()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if cfg2.Port != 9000 {
		t.Errorf("expected 9000, got %d", cfg2.Port)
	}
	if cfg2.Username != "testuser" {
		t.Errorf("expected testuser, got %s", cfg2.Username)
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

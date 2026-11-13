package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validBase() Config {
	cfg := Config{}
	cfg.Server.BindAddr = "127.0.0.1"
	cfg.Server.Port = 12500
	cfg.Server.ConcurrencyInfo = 4
	cfg.Server.ConcurrencyImage = 4
	cfg.Server.APIToken = "token"
	cfg.Upstream.BaseURL = "https://api.bgm.tv"
	return cfg
}

func hasError(errs []string, keyword string) bool {
	for _, e := range errs {
		if strings.Contains(e, keyword) {
			return true
		}
	}
	return false
}

func TestValidateBaseOK(t *testing.T) {
	cfg := validBase()
	if errs := cfg.Validate(); len(errs) != 0 {
		t.Fatalf("默认配置不应报错: %v", errs)
	}
}

func TestValidateBindAddr(t *testing.T) {
	cases := []struct {
		addr string
		bad  bool
	}{
		{"127.0.0.1", false},
		{"0.0.0.0", false},
		{"localhost", false},
		{"[::1]", false},
		{"http://127.0.0.1", true},
		{"127.0.0.1/path", true},
		{"127.0.0.1:12500", true},
		{"::1", true},
	}
	for _, c := range cases {
		cfg := validBase()
		cfg.Server.BindAddr = c.addr
		errs := cfg.Validate()
		if c.bad && !hasError(errs, "bind_addr") {
			t.Errorf("bind_addr=%q 应当报错: %v", c.addr, errs)
		}
		if !c.bad && hasError(errs, "bind_addr") {
			t.Errorf("bind_addr=%q 不应报错: %v", c.addr, errs)
		}
	}
}

func TestValidateTokenNotRequiredOnNonLoopback(t *testing.T) {
	cfg := validBase()
	cfg.Server.BindAddr = "0.0.0.0"
	cfg.Server.APIToken = ""
	if errs := cfg.Validate(); len(errs) != 0 {
		t.Fatalf("非回环地址不要求 api_token: %v", errs)
	}
}

func TestValidateURLs(t *testing.T) {
	cfg := validBase()
	cfg.Upstream.BaseURL = "ftp://api.bgm.tv"
	cfg.Frontend.BackendURL = "not-a-url"
	cfg.Frontend.FallbackURL = "https://example.com"
	errs := cfg.Validate()
	if !hasError(errs, "upstream.base_url") {
		t.Fatalf("base_url 应当报错: %v", errs)
	}
	if !hasError(errs, "frontend.backend_url") {
		t.Fatalf("backend_url 应当报错: %v", errs)
	}
	if hasError(errs, "frontend.fallback_url") {
		t.Fatalf("合法 fallback_url 不应报错: %v", errs)
	}
}

func TestValidateDataHome(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := validBase()
	cfg.Server.DataHome = file
	if errs := cfg.Validate(); !hasError(errs, "data_home") {
		t.Fatalf("data_home 指向文件时应报错: %v", errs)
	}

	cfg.Server.DataHome = t.TempDir()
	if errs := cfg.Validate(); hasError(errs, "data_home") {
		t.Fatalf("data_home 指向目录时不应报错: %v", errs)
	}
}

func TestIsLoopbackBind(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":       true,
		"127.0.0.1:12500": true,
		"[::1]:12500":     true,
		"localhost":       true,
		"0.0.0.0":         false,
		"192.168.1.10":    false,
		"":                false,
	}
	for addr, want := range cases {
		if got := isLoopbackBind(addr); got != want {
			t.Errorf("isLoopbackBind(%q) = %v，期望 %v", addr, got, want)
		}
	}
}

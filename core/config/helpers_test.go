package config

import (
	"strings"
	"testing"
)

func TestProxyURL(t *testing.T) {
	cfg := &Config{}
	if cfg.ProxyURL() != nil {
		t.Fatal("未配置代理时应返回 nil")
	}
	cfg.Proxy.Host = "127.0.0.1"
	cfg.Proxy.Port = 7890
	u := cfg.ProxyURL()
	if u == nil || u.String() != "http://127.0.0.1:7890" {
		t.Fatalf("代理地址 = %v", u)
	}
	cfg.Proxy.Port = 0
	if cfg.ProxyURL() != nil {
		t.Fatal("端口不合法时应返回 nil")
	}
}

func TestDataAndTrackerDir(t *testing.T) {
	t.Setenv("SESHAT_HOME", t.TempDir())
	cfg := &Config{}
	if got, want := cfg.DataDir(), Dir()+"/data"; got != want {
		t.Fatalf("DataDir = %q，期望 %q", got, want)
	}
	cfg.Server.DataHome = "/tmp/custom-data"
	if cfg.DataDir() != "/tmp/custom-data" {
		t.Fatalf("DataHome 未生效: %q", cfg.DataDir())
	}
	if cfg.TrackerDir() != Dir()+"/tracker" {
		t.Fatalf("TrackerDir = %q", cfg.TrackerDir())
	}
}

func TestWarnings(t *testing.T) {
	cfg := &Config{}
	cfg.Server.BindAddr = "127.0.0.1"
	cfg.Server.LogLevel = "bogus"
	w := cfg.Warnings()
	joined := strings.Join(w, "|")
	for _, want := range []string{"base_url", "user_agent", "api_token", "log_level"} {
		if !strings.Contains(joined, want) {
			t.Errorf("缺少 %q 警告: %v", want, w)
		}
	}

	cfg.Server.BindAddr = "0.0.0.0"
	w = cfg.Warnings()
	if !strings.Contains(strings.Join(w, "|"), "局域网") {
		t.Errorf("非回环地址时 api_token 警告应提到局域网: %v", w)
	}
}

func TestBuildConfigKVMasksToken(t *testing.T) {
	cfg := &Config{}
	cfg.Access.Token = "abcdefgh"
	kv := cfg.BuildConfigKV()
	if kv["bangumi_access_token"] != "abc***fgh" {
		t.Fatalf("长令牌掩码 = %v", kv["bangumi_access_token"])
	}
	cfg.Access.Token = "abc"
	if got := cfg.BuildConfigKV()["bangumi_access_token"]; got != "***" {
		t.Fatalf("短令牌掩码 = %v", got)
	}
	cfg.Access.Token = ""
	if got := cfg.BuildConfigKV()["bangumi_access_token"]; got != "" {
		t.Fatalf("空令牌 = %v", got)
	}
}

func TestTrackerFileContent(t *testing.T) {
	s := TrackerFileContent("my id", `名"字`)
	if !strings.Contains(s, `name = "名\"字"`) {
		t.Fatalf("名称转义不正确: %s", s)
	}
	if !strings.Contains(s, "ids = []") || !strings.Contains(s, "# Tracker: my id") {
		t.Fatalf("Tracker 内容不完整: %s", s)
	}
}

func TestQuoteTOMLString(t *testing.T) {
	got := quoteTOMLString("a\"b\\c\n")
	want := `"a\"b\\c\n"`
	if got != want {
		t.Fatalf("quote = %s，期望 %s", got, want)
	}
}

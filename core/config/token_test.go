package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadGeneratesTokenOnFirstCreate(t *testing.T) {
	t.Setenv("SESHAT_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Server.APIToken) != 64 {
		t.Fatalf("首次创建应生成 64 位令牌，实际长度 %d", len(cfg.Server.APIToken))
	}

	before, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), `api_token = "`+cfg.Server.APIToken+`"`) {
		t.Fatal("配置文件里没有写入令牌")
	}

	again, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("再次加载不应修改配置文件")
	}
	if again.Server.APIToken != cfg.Server.APIToken {
		t.Fatal("重启后令牌应保持不变")
	}
}

func TestLoadLeavesMissingTokenUntouched(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SESHAT_HOME", dir)

	body := "[server]\nport = 12500\n"
	if err := os.WriteFile(Path(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.APIToken != "" {
		t.Fatalf("缺少 api_token 时应留空，实际为 %q", cfg.Server.APIToken)
	}
	after, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != body {
		t.Fatal("已有配置缺少 api_token 时不应被修改")
	}
}

func TestLoadReadsAPIToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SESHAT_HOME", dir)

	body := "[server]\napi_token = \"abc\"\nport = 12500\n"
	if err := os.WriteFile(Path(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.APIToken != "abc" {
		t.Fatalf("令牌 = %q", cfg.Server.APIToken)
	}
}

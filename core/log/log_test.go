package log

import (
	"os"
	"path/filepath"
	"testing"
)

func TestToAttrs(t *testing.T) {
	attrs := toAttrs([]any{"a", 1, "b", 2})
	if len(attrs) != 2 || attrs[0].Key != "a" || attrs[1].Key != "b" {
		t.Fatalf("attrs = %+v", attrs)
	}

	attrs = toAttrs([]any{"a", 1, "odd"})
	if len(attrs) != 2 || attrs[1].Key != "odd" {
		t.Fatalf("奇数参数 = %+v", attrs)
	}

	attrs = toAttrs([]any{1, "v"})
	if len(attrs) != 1 || attrs[0].Key != "!BADKEY" {
		t.Fatalf("非字符串键 = %+v", attrs)
	}
}

func TestInitAndLog(t *testing.T) {
	t.Setenv("SESHAT_HOME", t.TempDir())
	Init("bogus")
	Debug("debug")
	Info("info")
	Warn("warn")
	Error("error")
	HTTP("GET", "/x", 200, 0, "id")

	logDir := filepath.Join(os.Getenv("SESHAT_HOME"), "logs")
	entries, err := os.ReadDir(logDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("日志目录没有文件: %v", err)
	}
}

func TestRotateLogs(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 12; i++ {
		name := filepath.Join(dir, "seshat_20260101_0000"+string(rune('0'+i%10))+".log")
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 用确定的递增文件名，避免取模重复
	for i := 0; i < 12; i++ {
		os.Remove(filepath.Join(dir, "seshat_20260101_0000"+string(rune('0'+i%10))+".log"))
	}
	for i := 0; i < 12; i++ {
		name := filepath.Join(dir, "seshat_20260101_00"+string(rune('0'+i/10))+string(rune('0'+i%10))+".log")
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rotateLogs(dir, 10)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 10 {
		t.Fatalf("轮转后文件数 = %d，期望 10", len(entries))
	}
}

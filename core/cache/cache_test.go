package cache

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteAtomicCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c.json")

	if err := WriteAtomic(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("内容为 %q，期望 %q", got, "hello")
	}
}

func TestWriteAtomicOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.json")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteAtomic(path, []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("内容为 %q，期望 %q", got, "new")
	}
}

func TestWriteAtomicLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	if err := WriteAtomic(filepath.Join(dir, "c.json"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("残留临时文件 %s", e.Name())
		}
	}
}

func TestWriteAtomicPermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 权限模型不同")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "c.json")
	if err := WriteAtomic(path, []byte("x"), 0o640); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Errorf("权限为 %o，期望 640", got)
	}
}

func TestWriteAtomicReportsError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := WriteAtomic(filepath.Join(blocker, "c.json"), []byte("y"), 0o644)
	if err == nil {
		t.Fatal("父路径为文件时应返回错误")
	}
	got, readErr := os.ReadFile(blocker)
	if readErr != nil {
		t.Fatalf("读取原文件: %v", readErr)
	}
	if string(got) != "x" {
		t.Errorf("原文件被改动，内容为 %q", got)
	}
}

func TestPutRoundTrip(t *testing.T) {
	dir := t.TempDir()
	key := Key("subjects", 51, "info.json")
	want := []byte(`{"id":51}`)

	if err := Put(dir, key, want); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !Has(dir, key) {
		t.Fatal("Has 返回 false")
	}
	got, err := Get(dir, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("内容为 %q，期望 %q", got, want)
	}
}

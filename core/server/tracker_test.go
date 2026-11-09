package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClearContentDataKeepsUser(t *testing.T) {
	dd := t.TempDir()
	for _, name := range []string{"api/a", "images/b", "index/c", "user/info.json"} {
		path := filepath.Join(dd, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	clearContentData(dd)

	for _, name := range []string{"api", "images", "index"} {
		if _, err := os.Stat(filepath.Join(dd, name)); !os.IsNotExist(err) {
			t.Errorf("%s 仍然存在", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dd, "user", "info.json")); err != nil {
		t.Fatalf("用户数据被删除: %v", err)
	}
}

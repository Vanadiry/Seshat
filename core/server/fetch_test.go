package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vanadiry/seshat/core/cache"
)

func TestPutCacheSuccess(t *testing.T) {
	dd := t.TempDir()
	key := cache.Key("subjects", 51, "info.json")
	if !putCache(dd, key, []byte(`{"id":51}`), nil) {
		t.Fatal("putCache 返回失败")
	}
	if _, err := cache.Get(dd, key); err != nil {
		t.Fatalf("读取缓存: %v", err)
	}
}

func TestPutCacheFailure(t *testing.T) {
	dd := t.TempDir()
	// 在 api 目录的位置放一个文件，让 MkdirAll 失败
	if err := os.WriteFile(filepath.Join(dd, "api"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if putCache(dd, cache.Key("subjects", 51, "info.json"), []byte(`{"id":51}`), nil) {
		t.Fatal("putCache 应当返回失败")
	}
}

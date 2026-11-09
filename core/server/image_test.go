package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/cache"
)

func TestMissingImageSizes(t *testing.T) {
	imgBase := t.TempDir()
	if got := missingImageSizes(imgBase, "subject", 51); len(got) != 3 {
		t.Fatalf("缺少全部尺寸时返回 %v", got)
	}
	for _, size := range []string{"large", "small"} {
		path := filepath.Join(imgBase, "subjects_"+size, "1", "51.jpg")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := missingImageSizes(imgBase, "subject", 51)
	if len(got) != 1 || got[0] != "grid" {
		t.Fatalf("只缺 grid 时返回 %v", got)
	}
}

func imageTestServer(t *testing.T, requests *[]string, mu *sync.Mutex) *bangumi.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		*requests = append(*requests, r.URL.Query().Get("type"))
		mu.Unlock()
		w.Write([]byte("image"))
	}))
	t.Cleanup(srv.Close)
	return bangumi.NewClient("test", srv.URL, nil, nil)
}

func TestDLImageOnlyRequestsMissingSizes(t *testing.T) {
	imgBase := t.TempDir()
	for _, size := range []string{"large", "small"} {
		path := filepath.Join(imgBase, "subjects_"+size, "1", "51.jpg")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var requests []string
	var mu sync.Mutex
	c := imageTestServer(t, &requests, &mu)
	dlImage(c, "subject", 51, nil, imgBase, &mu, nil)

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 1 || requests[0] != "grid" {
		t.Fatalf("请求了 %v，期望只有 grid", requests)
	}
	if len(missingImageSizes(imgBase, "subject", 51)) != 0 {
		t.Fatal("grid 没有被写入")
	}
}

func TestDLImageOnlyRequestsMissingSizesFromIndex(t *testing.T) {
	imgBase := t.TempDir()
	var requests []string
	var mu sync.Mutex
	c := imageTestServer(t, &requests, &mu)
	imgMap := map[int]cache.ImageEntry{
		51: {Large: "large.jpg", Grid: "grid.jpg"},
	}
	dlImage(c, "subject", 51, imgMap, imgBase, &mu, nil)

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 1 || requests[0] != "small" {
		t.Fatalf("请求了 %v，期望只有 small", requests)
	}
	if imgMap[51].Small == "" {
		t.Fatal("small 索引没有更新")
	}
}

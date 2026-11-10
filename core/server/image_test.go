package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/store"
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
	dlImage(c, "subject", 51, nil, imgBase, &mu, nil, false)

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
	imgMap := map[int]store.ImageEntry{
		51: {Large: "large.jpg", Grid: "grid.jpg"},
	}
	dlImage(c, "subject", 51, imgMap, imgBase, &mu, nil, false)

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 1 || requests[0] != "small" {
		t.Fatalf("请求了 %v，期望只有 small", requests)
	}
	if imgMap[51].Small == "" {
		t.Fatal("small 索引没有更新")
	}
}

func TestPickImageSize(t *testing.T) {
	all := store.ImageEntry{Large: "l", Grid: "g", Small: "s"}
	cases := []struct {
		entry     store.ImageEntry
		requested string
		want      string
	}{
		{all, "large", "large"},
		{all, "small", "small"},
		{all, "grid", "grid"},
		{all, "medium", "large"},
		{all, "common", "large"},
		{store.ImageEntry{Large: "l"}, "small", "large"},
		{store.ImageEntry{Large: "l"}, "grid", "large"},
		{store.ImageEntry{Grid: "g"}, "large", "grid"},
		{store.ImageEntry{Grid: "g"}, "small", "grid"},
		{store.ImageEntry{}, "large", ""},
	}
	for _, c := range cases {
		if got := pickImageSize(c.entry, c.requested); got != c.want {
			t.Errorf("pickImageSize(%v, %q) = %q, 期望 %q", c.entry, c.requested, got, c.want)
		}
	}
}

func TestServeImageFallback(t *testing.T) {
	dd := t.TempDir()
	imgPath := filepath.Join(dd, "images", "subjects_large", "1", "51.jpg")
	if err := os.MkdirAll(filepath.Dir(imgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(imgPath, []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
	index := map[int]store.ImageEntry{51: {Large: "subjects_large/1/51.jpg"}}
	data, _ := json.Marshal(index)
	indexFile := store.IndexFile(dd, "subjects_image.json")
	if err := os.MkdirAll(filepath.Dir(indexFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/subjects/{id}/image", func(w http.ResponseWriter, r *http.Request) {
		serveImage(w, r, dd, "subject", r.URL.Query().Get("type"))
	})
	for _, typ := range []string{"large", "small", "grid", "medium", "common"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v0/subjects/51/image?type="+typ, nil))
		if w.Code != http.StatusOK || w.Body.String() != "img" {
			t.Fatalf("type=%s 状态码 %d，内容 %q", typ, w.Code, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v0/subjects/999/image?type=large", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("缺失图片状态码 = %d", w.Code)
	}
}

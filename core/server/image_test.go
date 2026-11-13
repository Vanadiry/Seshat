package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/vanadiry/seshat/core/store"
)

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

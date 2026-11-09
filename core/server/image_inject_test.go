package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanadiry/seshat/core/cache"
)

func putJSON(t *testing.T, dd, key string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Put(dd, key, data); err != nil {
		t.Fatal(err)
	}
}

func putImageIndex(t *testing.T, dd, name string, index map[int]cache.ImageEntry) {
	t.Helper()
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	path := cache.IndexFile(dd, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func serveData(t *testing.T, dd, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	handleDataReader(dd)(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func decodeObject(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &obj); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	return obj
}

func imagesOf(t *testing.T, obj map[string]any) map[string]any {
	t.Helper()
	images, ok := obj["images"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 images: %v", obj)
	}
	return images
}

func TestInjectSubjectImages(t *testing.T) {
	dd := t.TempDir()
	putJSON(t, dd, cache.Key("subjects", 51, "info.json"), map[string]any{"id": 51})
	putImageIndex(t, dd, "subjects_image.json", map[int]cache.ImageEntry{
		51: {Large: "l.jpg", Grid: "g.jpg", Small: "s.jpg"},
	})

	w := serveData(t, dd, "/api/v0/subjects/51")
	images := imagesOf(t, decodeObject(t, w))
	for _, key := range []string{"large", "common", "medium", "small", "grid"} {
		if _, ok := images[key]; !ok {
			t.Fatalf("缺少 %s", key)
		}
	}
	base := "http://example.com/api/v0/subjects/51/image?type="
	if images["large"] != base+"large" || images["small"] != base+"small" || images["grid"] != base+"grid" {
		t.Fatalf("images = %v", images)
	}
	if images["medium"] != base+"large" || images["common"] != base+"large" {
		t.Fatalf("medium/common 应映射到 large: %v", images)
	}

	raw, err := cache.Get(dd, cache.Key("subjects", 51, "info.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"images"`) {
		t.Fatal("数据文件被写入 images")
	}
}

func TestInjectCharacterImages(t *testing.T) {
	dd := t.TempDir()
	putJSON(t, dd, cache.Key("characters", 319, "info.json"), map[string]any{"id": 319})
	putImageIndex(t, dd, "characters_image.json", map[int]cache.ImageEntry{
		319: {Large: "l.jpg"},
	})

	w := serveData(t, dd, "/api/v0/characters/319")
	images := imagesOf(t, decodeObject(t, w))
	if len(images) != 4 {
		t.Fatalf("character images 应有 4 个键: %v", images)
	}
	if _, ok := images["common"]; ok {
		t.Fatal("character 不应有 common")
	}
	base := "http://example.com/api/v0/characters/319/image?type=large"
	for _, key := range []string{"large", "medium", "small", "grid"} {
		if images[key] != base {
			t.Fatalf("%s = %v，期望 %s", key, images[key], base)
		}
	}
}

func TestInjectRelatedArray(t *testing.T) {
	dd := t.TempDir()
	putJSON(t, dd, cache.Key("subjects", 51, "characters.json"), []map[string]any{
		{"id": 1},
		{"id": 2},
	})
	putImageIndex(t, dd, "characters_image.json", map[int]cache.ImageEntry{
		1: {Large: "l.jpg"},
		2: {Small: "s.jpg"},
	})

	w := serveData(t, dd, "/api/v0/subjects/51/characters")
	var items []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("条数 = %d", len(items))
	}
	first := imagesOf(t, items[0])
	second := imagesOf(t, items[1])
	if first["large"] != "http://example.com/api/v0/characters/1/image?type=large" {
		t.Fatalf("第一条 images = %v", first)
	}
	if second["large"] != "http://example.com/api/v0/characters/2/image?type=small" {
		t.Fatalf("第二条 images = %v", second)
	}
}

func TestInjectSkipsEpisodes(t *testing.T) {
	dd := t.TempDir()
	putJSON(t, dd, cache.Key("subjects", 51, "episodes.json"), []map[string]any{{"id": 1}})

	w := serveData(t, dd, "/api/v0/subjects/51/episodes")
	if strings.Contains(w.Body.String(), `"images"`) {
		t.Fatalf("剧集不应注入 images: %s", w.Body.String())
	}
}

func TestInjectPlaceholderAndEmpty(t *testing.T) {
	dd := t.TempDir()
	putJSON(t, dd, cache.Key("subjects", 51, "info.json"), map[string]any{"id": 51})
	putJSON(t, dd, cache.Key("characters", 319, "info.json"), map[string]any{"id": 319})

	subject := imagesOf(t, decodeObject(t, serveData(t, dd, "/api/v0/subjects/51")))
	if subject["large"] != "http://example.com/assets/no-image.png" {
		t.Fatalf("Subject 无图应使用占位图: %v", subject)
	}

	character := imagesOf(t, decodeObject(t, serveData(t, dd, "/api/v0/characters/319")))
	for key, value := range character {
		if value != "" {
			t.Fatalf("Character 无图应为空字符串，%s = %v", key, value)
		}
	}
}

func TestRequestBaseURLForwarded(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "peer:1234, proxy")
	if got := requestBaseURL(r); got != "https://peer:1234" {
		t.Fatalf("base = %q", got)
	}
}

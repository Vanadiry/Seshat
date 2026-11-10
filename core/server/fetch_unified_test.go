package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/store"
)

func TestFetchRequestValidate(t *testing.T) {
	cases := []struct {
		name string
		req  fetchRequest
		ok   bool
	}{
		{"tracker new", fetchRequest{Source: fetchSourceTrackerNew, JSON: fetchJSONFill, Images: fetchImagesFill}, true},
		{"tracker all", fetchRequest{Source: fetchSourceTrackerAll, JSON: fetchJSONOverwrite, Images: fetchImagesNone}, true},
		{"names", fetchRequest{Source: fetchSourceTrackerNames, Names: []string{"user"}, JSON: fetchJSONFill, Images: fetchImagesFill}, true},
		{"ids", fetchRequest{Source: fetchSourceIDs, IDs: []int{1}, JSON: fetchJSONOverwrite, Images: fetchImagesOverwrite}, true},
		{"local", fetchRequest{Source: fetchSourceLocal, JSON: fetchJSONFill, Images: fetchImagesFill}, true},
		{"bad source", fetchRequest{Source: "nope", JSON: fetchJSONFill, Images: fetchImagesFill}, false},
		{"empty names", fetchRequest{Source: fetchSourceTrackerNames, JSON: fetchJSONFill, Images: fetchImagesFill}, false},
		{"bad name", fetchRequest{Source: fetchSourceTrackerNames, Names: []string{"bad name"}, JSON: fetchJSONFill, Images: fetchImagesFill}, false},
		{"empty ids", fetchRequest{Source: fetchSourceIDs, JSON: fetchJSONFill, Images: fetchImagesFill}, false},
		{"bad id", fetchRequest{Source: fetchSourceIDs, IDs: []int{0}, JSON: fetchJSONFill, Images: fetchImagesFill}, false},
		{"bad json", fetchRequest{Source: fetchSourceLocal, JSON: "nope", Images: fetchImagesFill}, false},
		{"bad images", fetchRequest{Source: fetchSourceLocal, JSON: fetchJSONFill, Images: "nope"}, false},
	}
	for _, c := range cases {
		err := c.req.validate()
		if c.ok && err != nil {
			t.Errorf("%s: 不应报错: %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: 应当报错", c.name)
		}
	}
}

func TestResolveFetchIDs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	dd := filepath.Join(home, "data")

	if err := os.MkdirAll(filepath.Join(home, "tracker"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "tracker", "user.toml"), []byte("name = \"user\"\nids = [1, 2, 3]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "tracker", "other.toml"), []byte("name = \"other\"\nids = [3, 4]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeSubjectCore(t, dd, 1, `{"id":1,"total_episodes":0,"eps":0}`)
	if err := store.Put(dd, store.Key("subjects", 2, "info.json"), []byte(`{"id":2}`)); err != nil {
		t.Fatal(err)
	}
	index, _ := json.Marshal([]store.SubjectSummary{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}})
	if err := store.WriteAtomic(store.IndexFile(dd, "subjects.json"), index, 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		req  fetchRequest
		want map[int]bool
	}{
		{"new", fetchRequest{Source: fetchSourceTrackerNew}, map[int]bool{2: true, 3: true, 4: true}},
		{"all", fetchRequest{Source: fetchSourceTrackerAll}, map[int]bool{1: true, 2: true, 3: true, 4: true}},
		{"names", fetchRequest{Source: fetchSourceTrackerNames, Names: []string{"other"}}, map[int]bool{3: true, 4: true}},
		{"ids", fetchRequest{Source: fetchSourceIDs, IDs: []int{9}}, map[int]bool{9: true}},
		{"local", fetchRequest{Source: fetchSourceLocal}, map[int]bool{1: true, 2: true}},
	}
	for _, c := range cases {
		got, err := resolveFetchIDs(cfg, dd, c.req)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(got) != len(c.want) {
			t.Fatalf("%s: got %v, want %v", c.name, got, c.want)
		}
		for _, id := range got {
			if !c.want[id] {
				t.Fatalf("%s: got %v, want %v", c.name, got, c.want)
			}
		}
	}

	if _, err := resolveFetchIDs(cfg, dd, fetchRequest{
		Source: fetchSourceTrackerNames,
		Names:  []string{"missing"},
	}); err == nil {
		t.Fatal("不存在的 Tracker 应当报错")
	}
}

func TestFetchUnifiedValidatesBeforeTask(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	cfg.Upstream.BaseURL = "http://127.0.0.1:1"
	dd := filepath.Join(home, "data")
	if err := os.MkdirAll(filepath.Join(home, "tracker"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "tracker", "user.toml"), []byte("ids = [1]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSubjectCore(t, dd, 1, `{"id":1,"total_episodes":0,"eps":0}`)
	index, _ := json.Marshal([]store.SubjectSummary{{ID: 1, Name: "a"}})
	if err := store.WriteAtomic(store.IndexFile(dd, "subjects.json"), index, 0o644); err != nil {
		t.Fatal(err)
	}

	h := handleFetchUnified(cfg, nil, dd, filepath.Join(dd, "images"))
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodPost, "/api/v0/fetch", strings.NewReader(`{"source":"nope","json":"fill","images":"fill"}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 source 状态码 = %d", w.Code)
	}

	w = httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodPost, "/api/v0/fetch", strings.NewReader(`{"source":"tracker_new","json":"fill","images":"fill"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("无新增条目状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "done" || body["count"] != float64(0) {
		t.Fatalf("无新增条目响应 = %v", body)
	}
}

func TestFetchDeepRequiresConfirm(t *testing.T) {
	cfg := &config.Config{}
	cfg.Upstream.BaseURL = "http://127.0.0.1:1"
	dd := t.TempDir()
	h := handleFetchDeep(cfg, nil, dd, filepath.Join(dd, "images"))
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodPost, "/api/v0/fetch/deep", strings.NewReader(`{}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未确认时状态码 = %d", w.Code)
	}
}

func TestLegacyFetchWrappers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	cfg.Upstream.BaseURL = "http://127.0.0.1:1"
	dd := filepath.Join(home, "data")
	imgDir := filepath.Join(dd, "images")
	if err := os.MkdirAll(filepath.Join(home, "tracker"), 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		h    http.Handler
		body string
		code int
	}{
		{"all", handleFetchAll(cfg, nil, dd, imgDir), `{}`, http.StatusOK},
		{"update", handleFetchUpdate(cfg, nil, dd, imgDir), `{}`, http.StatusOK},
		{"gap", handleFetchGap(cfg, nil, dd, imgDir), `{}`, http.StatusOK},
		{"meta", handleFetchMeta(cfg, nil, dd), `{}`, http.StatusOK},
		{"tracker 不存在", handleFetchTracker(cfg, nil, dd, imgDir), `{"names":["missing"]}`, http.StatusBadRequest},
		{"subject 为空", handleFetchSubject(cfg, nil, dd, imgDir), `{}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v0/fetch", strings.NewReader(c.body)))
		if w.Code != c.code {
			t.Fatalf("%s 状态码 = %d, body=%s", c.name, w.Code, w.Body.String())
		}
		if c.code != http.StatusOK {
			continue
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["status"] != "done" || body["count"] != float64(0) {
			t.Fatalf("%s 响应 = %v", c.name, body)
		}
	}
}

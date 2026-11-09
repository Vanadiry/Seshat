package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/store"
)

func writeIndexJSON(t *testing.T, dd, name string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	path := store.IndexFile(dd, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func decodeListBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	return body
}

func TestListSubjectsShape(t *testing.T) {
	dd := t.TempDir()
	writeIndexJSON(t, dd, "subjects.json", []store.SubjectSummary{{ID: 1, Name: "a"}})
	w := httptest.NewRecorder()
	handleListSubjects(dd)(w, httptest.NewRequest(http.MethodGet, "/api/v0/subjects/list", nil))
	body := decodeListBody(t, w)
	if body["total"] != float64(1) || body["limit"] != float64(0) || body["offset"] != float64(0) {
		t.Fatalf("body = %v", body)
	}
	data, ok := body["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("data = %v", body["data"])
	}
}

func TestTagsShape(t *testing.T) {
	dd := t.TempDir()
	writeIndexJSON(t, dd, "tags.json", map[string]tagInfo{
		"喜剧": {Count: 2, Subjects: []int{1, 2}},
	})
	w := httptest.NewRecorder()
	handleTags(dd)(w, httptest.NewRequest(http.MethodGet, "/api/v0/tags", nil))
	body := decodeListBody(t, w)
	if body["total"] != float64(1) {
		t.Fatalf("body = %v", body)
	}
	data, ok := body["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("data = %v", body["data"])
	}
}

func TestTagSubjectsShape(t *testing.T) {
	dd := t.TempDir()
	writeIndexJSON(t, dd, "tags.json", map[string]tagInfo{
		"喜剧": {Count: 1, Subjects: []int{1}},
	})
	putJSON(t, dd, store.Key("subjects", 1, "info.json"), map[string]any{"id": 1, "name": "a"})
	r := httptest.NewRequest(http.MethodGet, "/api/v0/tags/喜剧/subjects", nil)
	r.SetPathValue("name", "喜剧")
	w := httptest.NewRecorder()
	handleTagSubjects(dd)(w, r)
	body := decodeListBody(t, w)
	if body["total"] != float64(1) {
		t.Fatalf("body = %v", body)
	}
	if _, ok := body["data"].([]any); !ok {
		t.Fatalf("data = %v", body["data"])
	}
}

func TestActiveTasksShape(t *testing.T) {
	w := httptest.NewRecorder()
	handleActiveTasks(w, httptest.NewRequest(http.MethodGet, "/api/v0/tasks", nil))
	body := decodeListBody(t, w)
	if _, ok := body["data"].([]any); !ok {
		t.Fatalf("data = %v", body["data"])
	}
	for _, key := range []string{"total", "limit", "offset"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("缺少 %s: %v", key, body)
		}
	}
}

func TestTrackerListShape(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "tracker"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "tracker", "user.toml"), []byte("name = \"user\"\nids = [1, 2]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	w := httptest.NewRecorder()
	handleTrackerList(cfg)(w, httptest.NewRequest(http.MethodGet, "/api/v0/tracker", nil))
	body := decodeListBody(t, w)
	if body["total"] != float64(1) {
		t.Fatalf("body = %v", body)
	}
	data, ok := body["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("data = %v", body["data"])
	}
	item := data[0].(map[string]any)
	if item["name"] != "user" || item["count"] != float64(2) {
		t.Fatalf("item = %v", item)
	}
}

func TestELOHistoryShape(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	w := httptest.NewRecorder()
	handleELOHistory("")(w, httptest.NewRequest(http.MethodGet, "/api/v0/elo/history", nil))
	body := decodeListBody(t, w)
	if body["total"] != float64(0) {
		t.Fatalf("body = %v", body)
	}
	if data, ok := body["data"].([]any); !ok || len(data) != 0 {
		t.Fatalf("data = %v", body["data"])
	}
}

func TestELORankingShape(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	dd := t.TempDir()
	w := httptest.NewRecorder()
	handleELORanking(dd)(w, httptest.NewRequest(http.MethodGet, "/api/v0/elo/ranking", nil))
	body := decodeListBody(t, w)
	if body["total"] != float64(0) || body["limit"] != float64(0) || body["offset"] != float64(0) {
		t.Fatalf("body = %v", body)
	}
	if data, ok := body["data"].([]any); !ok || len(data) != 0 {
		t.Fatalf("data = %v", body["data"])
	}
}

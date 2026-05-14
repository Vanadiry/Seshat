package Test

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/config"
)

func TestPreferencesLoadSave(t *testing.T) {
	os.Setenv("SESHAT_HOME", t.TempDir())
	defer os.Unsetenv("SESHAT_HOME")

	p, err := config.LoadPreferences()
	if err != nil {
		t.Fatalf("LoadPreferences: %v", err)
	}
	if p.PreferLang != "original" {
		t.Errorf("expected original, got %s", p.PreferLang)
	}

	// Verify file was created
	if _, err := os.Stat(config.PrefPath()); err != nil {
		t.Errorf("preferences.json not created: %v", err)
	}
}

func TestSettingsAPI(t *testing.T) {
	srv, _ := testServer(t)

	// GET
	resp, err := http.Get(srv.URL + "/api/v0/settings")
	if err != nil {
		t.Fatalf("GET settings: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var data map[string]any
	json.NewDecoder(resp.Body).Decode(&data)
	resp.Body.Close()
	if data["prefer_lang"] == nil {
		t.Error("missing prefer_lang")
	}

	// POST — valid update
	resp, err = http.Post(srv.URL+"/api/v0/settings", "application/json",
		strings.NewReader(`{"prefer_lang":"chinese"}`))
	if err != nil {
		t.Fatalf("POST settings: %v", err)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	if result["success"] == nil {
		t.Errorf("expected success, got %v", result)
	}

	// POST — invalid key
	resp, _ = http.Post(srv.URL+"/api/v0/settings", "application/json",
		strings.NewReader(`{"nonexistent":"foo"}`))
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	if result["error"] == nil {
		t.Errorf("expected error for invalid key, got %v", result)
	}
}

func TestUserCollectionsEmpty(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Get(srv.URL + "/api/v0/users/testuser/collections")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var data map[string]any
	json.NewDecoder(resp.Body).Decode(&data)
	resp.Body.Close()
	if _, ok := data["data"]; !ok {
		t.Error("missing data field")
	}
}

func TestAvatarNoCache(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Get(srv.URL + "/api/v0/users/testuser/avatar?type=large")
	// No cached avatar file → 404
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestStatsAPI(t *testing.T) {
	srv, _ := testServer(t)
	resp, err := http.Get(srv.URL + "/api/v0/stats")
	if err != nil {
		t.Fatalf("GET stats: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var data map[string]any
	json.NewDecoder(resp.Body).Decode(&data)
	resp.Body.Close()
	for _, key := range []string{"images", "cache_entries", "collections", "elo_scores"} {
		if data[key] == nil {
			t.Errorf("missing key: %s", key)
		}
	}
}

func TestSearchSubjectsPost(t *testing.T) {
	srv, _ := testServer(t)
	resp, err := http.Post(srv.URL+"/api/v0/search/subjects", "application/json",
		strings.NewReader(`{"keyword":"test","filter":{"type":[2]}}`))
	if err != nil {
		t.Fatalf("POST search: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var data map[string]any
	json.NewDecoder(resp.Body).Decode(&data)
	resp.Body.Close()
	if _, ok := data["data"]; !ok {
		t.Error("missing data field")
	}
}

func TestSearchCharactersPost(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Post(srv.URL+"/api/v0/search/characters", "application/json",
		strings.NewReader(`{"keyword":"test"}`))
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestSearchPersonsPost(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Post(srv.URL+"/api/v0/search/persons", "application/json",
		strings.NewReader(`{"keyword":"test"}`))
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestSearchTagsPost(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Post(srv.URL+"/api/v0/search/tags", "application/json",
		strings.NewReader(`{"keyword":"test"}`))
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestELOHistory(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Get(srv.URL + "/api/v0/elo/history")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestELORebuild(t *testing.T) {
	srv, _ := testServer(t)
	resp, err := http.Post(srv.URL+"/api/v0/elo/rebuild", "application/json", nil)
	if err != nil {
		t.Fatalf("POST elo/rebuild: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestNameEndpoints(t *testing.T) {
	srv, _ := testServer(t)
	for _, ep := range []string{"subjects/name", "characters/name", "persons/name"} {
		resp, _ := http.Get(srv.URL + "/api/v0/" + ep)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", ep, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestImportCollectionsEmpty(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Post(srv.URL+"/api/v0/tracker/import-collections", "application/json", nil)
	// No collections file → 400
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHTMLRedirect(t *testing.T) {
	srv, _ := testServer(t)
	// Request with .html should redirect
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, _ := client.Get(srv.URL + "/subject.html")
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Errorf("expected 301, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/subject" {
		t.Errorf("expected /subject, got %s", loc)
	}
	resp.Body.Close()
}

func TestHTMLEndpoint(t *testing.T) {
	srv, _ := testServer(t)
	// Clean URL should return 503 (no embed FS in test) or 200
	resp, _ := http.Get(srv.URL + "/subject")
	t.Logf("/subject status: %d", resp.StatusCode)
	resp.Body.Close()
}

func TestListEndpoints(t *testing.T) {
	srv, _ := testServer(t)
	for _, ep := range []string{"subjects/list", "characters/list", "persons/list"} {
		resp, _ := http.Get(srv.URL + "/api/v0/" + ep)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", ep, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestTasksEndpoint(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Get(srv.URL + "/api/v0/tasks")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestStripImagesHandlesSingleImage(t *testing.T) {
	// Test that cache.StripImages removes "image" (singular) field
	data := []byte(`{"id":51,"image":"https://x.com/img.jpg","name":"test"}`)
	result := cache.StripImages(data)
	if strings.Contains(string(result), `"image"`) {
		t.Error("singular 'image' field should be stripped")
	}
}

func TestCacheKeyFormat(t *testing.T) {
	key := cache.Key("subjects", 51, "info.json")
	expected := "subjects/1/51/info.json"
	if key != expected {
		t.Errorf("expected %s, got %s", expected, key)
	}
}

func TestELOResponseStructure(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Get(srv.URL + "/api/v0/elo/ranking")
	var data map[string]any
	json.NewDecoder(resp.Body).Decode(&data)
	resp.Body.Close()
	for _, key := range []string{"entries", "no_rating", "orphans"} {
		if _, ok := data[key]; !ok {
			t.Errorf("elo/ranking missing field: %s", key)
		}
	}
}

func TestTrackerCreateInvalidName(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Post(srv.URL+"/api/v0/tracker/create", "application/json",
		strings.NewReader(`{"name":"invalid name!"}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid name, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

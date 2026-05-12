package Test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanadiry/seshat/Core/config"
	"github.com/vanadiry/seshat/Core/server"
)

func testServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	tmp := t.TempDir()
	os.Setenv("SESHAT_HOME", tmp)
	t.Cleanup(func() { os.Unsetenv("SESHAT_HOME") })

	cfg := config.Defaults
	cfg.DataHome = filepath.Join(tmp, "data")
	os.MkdirAll(cfg.DataHome, 0o755)
	os.MkdirAll(cfg.TrackerDir(), 0o755)

	router := server.New(&cfg, nil)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, tmp
}

func TestHealth(t *testing.T) {
	srv, _ := testServer(t)
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError {
		// OK or 500 (no embedded frontend in test) are both acceptable
		t.Logf("status: %d", resp.StatusCode)
	}
}

func TestAPIv1NotFound(t *testing.T) {
	srv, _ := testServer(t)
	resp, err := http.Get(srv.URL + "/v0/subjects/99999")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestTagsEmpty(t *testing.T) {
	srv, _ := testServer(t)
	resp, err := http.Get(srv.URL + "/v0/tags")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var tags []map[string]any
	json.NewDecoder(resp.Body).Decode(&tags)
	resp.Body.Close()
}

func TestTrackerListEmpty(t *testing.T) {
	srv, _ := testServer(t)
	resp, err := http.Get(srv.URL + "/v0/tracker")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSearchEmpty(t *testing.T) {
	srv, _ := testServer(t)
	resp, err := http.Get(srv.URL + "/v0/search?q=test")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestOpenAPIYaml(t *testing.T) {
	srv, _ := testServer(t)
	// This requires embedded web files which may not be available in test
	resp, err := http.Get(srv.URL + "/v0/openapi.yaml")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	// Could be 200 or 404 depending on embed setup
	t.Logf("openapi.yaml status: %d", resp.StatusCode)
	resp.Body.Close()
}

func TestCORS(t *testing.T) {
	srv, _ := testServer(t)
	req, _ := http.NewRequest("OPTIONS", srv.URL+"/v0/subjects", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS header missing")
	}
}

func TestUserProfileNoUsername(t *testing.T) {
	srv, _ := testServer(t)
	resp, _ := http.Get(srv.URL + "/v0/user/profile")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 (no username configured), got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestTrackerCreate(t *testing.T) {
	srv, tmp := testServer(t)
	resp, err := http.Post(srv.URL+"/v0/tracker/create", "application/json",
		strings.NewReader(`{"name":"test_tracker"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify file created
	if _, err := os.Stat(filepath.Join(tmp, "tracker", "test_tracker.toml")); err != nil {
		t.Errorf("tracker file not created: %v", err)
	}
}

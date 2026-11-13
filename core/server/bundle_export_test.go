package server

import (
	"archive/tar"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/store"
)

func writeBundleFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readBundleTar(t *testing.T, path string) map[string][]byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	entries := map[string][]byte{}
	tr := tar.NewReader(f)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		entries[h.Name] = data
	}
	return entries
}

func TestBundleExportIDs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	dd := cfg.DataDir()

	for _, id := range []int{51, 52} {
		if err := store.Put(dd, store.Key("subjects", id, "info.json"), []byte(`{"id":`+strconv.Itoa(id)+`}`)); err != nil {
			t.Fatal(err)
		}
		if err := store.Put(dd, store.Key("subjects", id, "characters.json"), []byte(`[{"id":4,"actors":[{"id":100}]}]`)); err != nil {
			t.Fatal(err)
		}
		if err := store.Put(dd, store.Key("subjects", id, "persons.json"), []byte(`[{"id":100}]`)); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Put(dd, store.Key("characters", 4, "info.json"), []byte(`{"id":4}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(dd, store.Key("persons", 100, "info.json"), []byte(`{"id":100}`)); err != nil {
		t.Fatal(err)
	}
	writeBundleFile(t, filepath.Join(dd, "images", "subjects_large", "1", "51.jpg"), "large")
	writeBundleFile(t, filepath.Join(dd, "images", "subjects_grid", "1", "51.jpg"), "grid")
	writeBundleFile(t, filepath.Join(dd, "images", "characters_small", "4", "4.jpg"), "small")
	writeBundleFile(t, filepath.Join(dd, "images", "persons_large", "0", "100.jpg"), "large")
	writeBundleFile(t, filepath.Join(dd, "user", "info.json"), "user")

	h := handleBundleExport(cfg)
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodPost, "/api/v0/bundle/export",
		strings.NewReader(`{"source":"ids","ids":[51,52,999],"description":"测试数据包"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Status   string          `json:"status"`
		Path     string          `json:"path"`
		Filename string          `json:"filename"`
		Size     int64           `json:"size"`
		Counts   bundleCounts    `json:"counts"`
		Missing  []bundleMissing `json:"missing"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" || resp.Size <= 0 || !strings.HasSuffix(resp.Filename, ".tar") {
		t.Fatalf("响应不对: %+v", resp)
	}
	if resp.Counts.Subjects != 2 || resp.Counts.Characters != 1 || resp.Counts.Persons != 1 || resp.Counts.Images != 4 {
		t.Fatalf("统计不对: %+v", resp.Counts)
	}
	hasMissingRoot := false
	for _, m := range resp.Missing {
		if m.Kind == "subjects" && m.ID == 999 {
			hasMissingRoot = true
		}
	}
	if !hasMissingRoot {
		t.Fatalf("未记录缺失根条目: %+v", resp.Missing)
	}
	if _, err := os.Stat(resp.Path); err != nil {
		t.Fatalf("导出文件不存在: %v", err)
	}

	entries := readBundleTar(t, resp.Path)
	manifestData, ok := entries["manifest.json"]
	if !ok {
		t.Fatal("缺少 manifest.json")
	}
	var manifest bundleManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Format != bundleFormat || manifest.Description != "测试数据包" {
		t.Fatalf("manifest 头不对: %+v", manifest)
	}
	if len(manifest.Subjects) != 2 || manifest.Subjects[0] != 51 || manifest.Subjects[1] != 52 {
		t.Fatalf("manifest subjects 不对: %v", manifest.Subjects)
	}
	if len(manifest.Characters) != 1 || manifest.Characters[0] != 4 {
		t.Fatalf("manifest characters 不对: %v", manifest.Characters)
	}
	if len(manifest.Persons) != 1 || manifest.Persons[0] != 100 {
		t.Fatalf("manifest persons 不对: %v", manifest.Persons)
	}
	for _, path := range []string{
		"api/subjects/1/51/info.json",
		"api/subjects/2/52/info.json",
		"api/characters/4/4/info.json",
		"api/persons/0/100/info.json",
		"images/subjects_large/1/51.jpg",
		"images/characters_small/4/4.jpg",
	} {
		if _, ok := entries[path]; !ok {
			t.Fatalf("tar 缺少 %s", path)
		}
	}
	for path := range entries {
		if strings.HasPrefix(path, "user/") {
			t.Fatalf("tar 包含用户数据: %s", path)
		}
	}
}

func TestBundleExportTracker(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	dd := cfg.DataDir()
	if err := store.Put(dd, store.Key("subjects", 51, "info.json"), []byte(`{"id":51}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.TrackerDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.TrackerDir(), "user.toml"), []byte("name = \"user\"\nids = [51]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := handleBundleExport(cfg)
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodPost, "/api/v0/bundle/export",
		strings.NewReader(`{"source":"trackers","trackers":["user"]}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	entries := readBundleTar(t, mustBundlePath(t, w.Body.Bytes()))
	var manifest bundleManifest
	if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Subjects) != 1 || manifest.Subjects[0] != 51 {
		t.Fatalf("Tracker 导出 subjects 不对: %v", manifest.Subjects)
	}
}

func TestBundleExportRejectsMissingTracker(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	h := handleBundleExport(cfg)
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodPost, "/api/v0/bundle/export",
		strings.NewReader(`{"source":"trackers","trackers":["missing"]}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
}

func mustBundlePath(t *testing.T, body []byte) string {
	t.Helper()
	var resp struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	return resp.Path
}

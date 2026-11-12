package server

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/store"
)

func waitBundleTask(t *testing.T, id string) {
	t.Helper()
	for i := 0; i < 500; i++ {
		p := getProgress(id)
		if p == nil || p.Closed() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("导入任务没有结束")
}

func TestBundleImportRoundTrip(t *testing.T) {
	homeA := t.TempDir()
	t.Setenv("SESHAT_HOME", homeA)
	cfgA := &config.Config{}
	ddA := cfgA.DataDir()
	for _, id := range []int{51} {
		if err := store.Put(ddA, store.Key("subjects", id, "info.json"), []byte(`{"id":51}`)); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Put(ddA, store.Key("subjects", 51, "characters.json"), []byte(`[{"id":4,"actors":[{"id":100}]}]`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ddA, store.Key("subjects", 51, "persons.json"), []byte(`[{"id":100}]`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ddA, store.Key("characters", 4, "info.json"), []byte(`{"id":4}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ddA, store.Key("persons", 100, "info.json"), []byte(`{"id":100}`)); err != nil {
		t.Fatal(err)
	}
	writeBundleFile(t, filepath.Join(ddA, "images", "subjects_large", "1", "51.jpg"), "image")

	export := handleBundleExport(cfgA)
	w := httptest.NewRecorder()
	export(w, httptest.NewRequest(http.MethodPost, "/api/v0/bundle/export",
		strings.NewReader(`{"source":"ids","ids":[51],"description":"roundtrip"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("导出状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	tarPath := mustBundlePath(t, w.Body.Bytes())

	homeB := t.TempDir()
	t.Setenv("SESHAT_HOME", homeB)
	cfgB := &config.Config{}
	ddB := cfgB.DataDir()

	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v0/bundle/import", f)
	req.ContentLength = stat.Size()
	w = httptest.NewRecorder()
	handleBundleImport(cfgB)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("导入状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Status string `json:"status"`
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "importing" || resp.TaskID == "" {
		t.Fatalf("导入响应不对: %+v", resp)
	}
	waitBundleTask(t, resp.TaskID)

	for _, key := range []string{
		store.Key("subjects", 51, "info.json"),
		store.Key("characters", 4, "info.json"),
		store.Key("persons", 100, "info.json"),
	} {
		if !store.Has(ddB, key) {
			t.Fatalf("导入后缺少 %s", key)
		}
	}
	if _, err := os.Stat(filepath.Join(ddA, "images", "subjects_large", "1", "51.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ddB, "images", "subjects_large", "1", "51.jpg")); err != nil {
		t.Fatalf("导入后缺少图片: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ddB, "manifest.json")); !os.IsNotExist(err) {
		t.Fatal("manifest.json 不应写入数据目录")
	}
	index, err := os.ReadFile(store.IndexFile(ddB, "subjects.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(index, []byte(`"id": 51`)) && !bytes.Contains(index, []byte(`"id":51`)) {
		t.Fatalf("索引没有包含导入条目: %s", index)
	}
}

func writeTestTar(t *testing.T, path string, entries []testTarEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tw := tar.NewWriter(f)
	for _, e := range entries {
		typ := e.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		h := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.data)), Typeflag: typ}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if typ == tar.TypeReg {
			if _, err := tw.Write([]byte(e.data)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
}

type testTarEntry struct {
	name string
	data string
	typ  byte
}

func TestExtractBundleTarRejectsUnsafeEntries(t *testing.T) {
	validManifest := `{"format":"seshat-bundle"}`
	cases := []struct {
		name    string
		entries []testTarEntry
	}{
		{"parent", []testTarEntry{{name: "../evil.json", data: "x"}}},
		{"absolute", []testTarEntry{{name: "/tmp/evil.json", data: "x"}}},
		{"symlink", []testTarEntry{{name: "api/subjects/1/51/info.json", typ: tar.TypeSymlink}}},
		{"duplicate", []testTarEntry{
			{name: "manifest.json", data: validManifest},
			{name: "api/subjects/1/51/info.json", data: "{}"},
			{name: "api/subjects/1/51/info.json", data: "{}"},
		}},
		{"missing manifest", []testTarEntry{{name: "api/subjects/1/51/info.json", data: "{}"}}},
		{"bad manifest", []testTarEntry{
			{name: "manifest.json", data: `{"format":"other"}`},
			{name: "api/subjects/1/51/info.json", data: "{}"},
		}},
		{"unknown file", []testTarEntry{
			{name: "manifest.json", data: validManifest},
			{name: "api/subjects/1/51/evil.txt", data: "x"},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tarPath := filepath.Join(t.TempDir(), "bundle.tar")
			writeTestTar(t, tarPath, c.entries)
			if err := extractBundleTar(tarPath, t.TempDir()); err == nil {
				t.Fatal("应当拒绝不安全的数据包")
			}
		})
	}
}

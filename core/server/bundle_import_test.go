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
	if err := store.Put(ddB, store.Key("subjects", 51, "info.json"), []byte(`{"id":51,"name":"old"}`)); err != nil {
		t.Fatal(err)
	}

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
		t.Fatalf("导入响应应为 importing 且带 task_id，实际为 %+v", resp)
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
	info, err := store.Get(ddB, store.Key("subjects", 51, "info.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(info, []byte("old")) {
		t.Fatalf("导入没有覆盖已有文件: %s", info)
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

func TestBundleImportPhasePlan(t *testing.T) {
	specs := bundleImportPhasePlan()
	if len(specs) != 3 {
		t.Fatalf("阶段数应为 3，实际为 %d", len(specs))
	}
	if specs[2].Name != "重建索引" || specs[2].Start != 95 || specs[2].End != 100 {
		t.Fatalf("索引阶段应占 5%%: %v", specs[2])
	}
}

func TestBundleImportProgress(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	tarPath := filepath.Join(t.TempDir(), "bundle.tar")
	writeTestTar(t, tarPath, []testTarEntry{
		{name: "manifest.json", data: `{"format":"seshat-bundle"}`},
		{name: "api/subjects/1/51/info.json", data: `{"id":51,"name":"a"}`},
		{name: "api/subjects/1/511/info.json", data: `{"id":511,"name":"b"}`},
		{name: "api/subjects/1/521/info.json", data: `{"id":521,"name":"c"}`},
	})
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
	w := httptest.NewRecorder()
	handleBundleImport(cfg)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("导入状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	p := getProgress(resp.TaskID)
	if p == nil {
		t.Fatal("未找到导入任务")
	}

	var percents []int
	phases := map[int]bool{}
	sawComplete := false
	for msg := range p.Channel {
		var ev struct {
			Step    string `json:"step"`
			Percent *int   `json:"percent"`
			Phase   int    `json:"phase"`
		}
		if err := json.Unmarshal([]byte(msg), &ev); err != nil {
			t.Fatalf("进度事件 JSON 不合法: %s", msg)
		}
		if ev.Step == "progress" && ev.Percent != nil {
			percents = append(percents, *ev.Percent)
			phases[ev.Phase] = true
		}
		if ev.Step == "complete" {
			sawComplete = true
		}
	}
	if !sawComplete {
		t.Fatal("缺少完成事件")
	}
	if len(percents) < 2 {
		t.Fatalf("进度事件至少 2 条，实际为 %d 条", len(percents))
	}
	for i := 1; i < len(percents); i++ {
		if percents[i] < percents[i-1] {
			t.Fatalf("进度百分比应单调不减，实际为 %v", percents)
		}
	}
	if !phases[1] || !phases[2] || !phases[3] {
		t.Fatalf("缺少阶段事件: %v", phases)
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
			if err := extractBundleTar(tarPath, t.TempDir(), nil, nil); err == nil {
				t.Fatal("应当拒绝不安全的数据包")
			}
		})
	}
}

func TestExtractBundleTarRejectsOversizedFile(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	h := &tar.Header{Name: "api/subjects/1/51/info.json", Mode: 0o644, Size: bundleImportMax + 1}
	if err := tw.WriteHeader(h); err != nil {
		t.Fatal(err)
	}
	tarPath := filepath.Join(t.TempDir(), "large.tar")
	if err := os.WriteFile(tarPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := extractBundleTar(tarPath, t.TempDir(), nil, nil); err == nil || !strings.Contains(err.Error(), "文件过大") {
		t.Fatalf("超限文件应被拒绝: %v", err)
	}
}

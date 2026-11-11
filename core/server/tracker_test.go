package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/store"
)

func TestClearContentDataKeepsUser(t *testing.T) {
	dd := t.TempDir()
	for _, name := range []string{"api/a", "images/b", "index/c", "user/info.json"} {
		path := filepath.Join(dd, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	clearContentData(dd)

	for _, name := range []string{"api", "images", "index"} {
		if _, err := os.Stat(filepath.Join(dd, name)); !os.IsNotExist(err) {
			t.Errorf("%s 仍然存在", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dd, "user", "info.json")); err != nil {
		t.Fatalf("用户数据被删除: %v", err)
	}
}

func writeTrackerTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadTrackerInfosMergesFormats(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	td := cfg.TrackerDir()
	writeTrackerTestFile(t, td, "user.toml", "name = \"我的追番\"\nids = [1, 2]\n")
	writeTrackerTestFile(t, td, "user.json", `{"name":"忽略此名","subjects":[2,3]}`)
	writeTrackerTestFile(t, td, "_seshat.json", `{"name":"seshat","subjects":[9]}`)
	writeTrackerTestFile(t, td, "empty.toml", "name = \"\"\nids = []\n")
	writeTrackerTestFile(t, td, "plain.json", `{"name":"普通 JSON","subjects":[5]}`)

	infos := loadTrackerInfos(cfg)
	if len(infos) != 4 {
		t.Fatalf("合并后应有 4 个 Tracker，实际 %d", len(infos))
	}
	if infos[0].ID != "_seshat" || infos[1].ID != "empty" || infos[2].ID != "plain" || infos[3].ID != "user" {
		t.Fatalf("未按 id 排序: %+v", infos)
	}
	if infos[0].Name != "seshat" || infos[0].Type != trackerTypeAuto || infos[0].Count != 1 {
		t.Fatalf("自动 Tracker 结果不对: %+v", infos[0])
	}
	if infos[1].Name != "empty" || infos[1].Type != trackerTypeManual || infos[1].Count != 0 {
		t.Fatalf("空名称回退不对: %+v", infos[1])
	}
	if infos[2].Name != "普通 JSON" || infos[2].Type != trackerTypeManual || infos[2].Count != 1 {
		t.Fatalf("普通 JSON 应为手动 Tracker: %+v", infos[2])
	}
	if infos[3].Name != "我的追番" || infos[3].Type != trackerTypeManual || infos[3].Count != 3 {
		t.Fatalf("合并 Tracker 结果不对: %+v", infos[3])
	}
}

func TestUserJSONIsAutoTracker(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	writeTrackerTestFile(t, cfg.TrackerDir(), "user.json", `{"name":"user","subjects":[1]}`)
	infos := loadTrackerInfos(cfg)
	if len(infos) != 1 || infos[0].Type != trackerTypeAuto {
		t.Fatalf("user.json 应为自动 Tracker: %+v", infos)
	}
}

func TestTrackerQueriesMergeFormats(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	td := cfg.TrackerDir()
	writeTrackerTestFile(t, td, "user.toml", "name = \"user\"\nids = [1, 2]\n")
	writeTrackerTestFile(t, td, "user.json", `{"name":"user","subjects":[2,3]}`)

	ids := trackerIDsByNames(cfg, []string{"user"})
	sort.Ints(ids)
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Fatalf("合并后的 ID 不对: %v", ids)
	}
	if got := countTrackerNames(cfg, []string{"user"}); got != 3 {
		t.Fatalf("合并后的条目数 = %d", got)
	}
	allIDs := allTrackerIDs(cfg)
	sort.Ints(allIDs)
	if len(allIDs) != 3 || allIDs[0] != 1 || allIDs[1] != 2 || allIDs[2] != 3 {
		t.Fatalf("全部 Tracker 的 ID 不对: %v", allIDs)
	}
	if got := countTrackerTotal(cfg); got != 3 {
		t.Fatalf("全部 Tracker 的条目数 = %d", got)
	}

	dd := t.TempDir()
	index, err := json.Marshal([]store.SubjectSummary{{ID: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteAtomic(store.IndexFile(dd, "subjects.json"), index, 0o644); err != nil {
		t.Fatal(err)
	}
	newIDs := diffTrackerIDs(cfg, dd)
	sort.Ints(newIDs)
	if len(newIDs) != 2 || newIDs[0] != 2 || newIDs[1] != 3 {
		t.Fatalf("增量 ID 不对: %v", newIDs)
	}
}

func TestReservedTrackerID(t *testing.T) {
	cases := map[string]bool{
		"_seshat":   true,
		"_internal": true,
		"user":      false,
		"empty":     false,
	}
	for id, want := range cases {
		if got := reservedTrackerID(id); got != want {
			t.Errorf("%s: got %v, want %v", id, got, want)
		}
	}
}

func TestTrackerCreateRestrictions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	td := cfg.TrackerDir()
	h := handleTrackerCreate(cfg)
	post := func(body string) int {
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest(http.MethodPost, "/api/v0/tracker/create", strings.NewReader(body)))
		return w.Code
	}

	if code := post(`{"id":"anime","name":"我的追番"}`); code != http.StatusOK {
		t.Fatalf("创建 Tracker 状态码 = %d", code)
	}
	if name := loadTrackerName(filepath.Join(td, "anime.toml")); name != "我的追番" {
		t.Fatalf("创建后的名称 = %q", name)
	}
	if code := post(`{"id":"anime"}`); code != http.StatusConflict {
		t.Fatalf("重复创建状态码 = %d", code)
	}
	writeTrackerTestFile(t, td, "auto.json", `{"name":"auto","subjects":[]}`)
	if code := post(`{"id":"auto"}`); code != http.StatusConflict {
		t.Fatalf("JSON 已存在时状态码 = %d", code)
	}
	if code := post(`{"id":"_seshat"}`); code != http.StatusBadRequest {
		t.Fatalf("保留名状态码 = %d", code)
	}
	for _, body := range []string{
		`{"id":"bad name"}`,
		`{"id":"中文"}`,
		`{"id":"../evil"}`,
		`{"id":"a/b"}`,
		`{"id":"a.b"}`,
	} {
		if code := post(body); code != http.StatusBadRequest {
			t.Fatalf("不合法 id 状态码 = %d: %s", code, body)
		}
	}
	if code := post(`{"id":"special","name":"我的 \"追番\" \\ 2026"}`); code != http.StatusOK {
		t.Fatalf("特殊名称创建状态码 = %d", code)
	}
	if name := loadTrackerName(filepath.Join(td, "special.toml")); name != "我的 \"追番\" \\ 2026" {
		t.Fatalf("特殊名称 = %q", name)
	}
	if code := post(`{"name":"legacy"}`); code != http.StatusOK {
		t.Fatalf("只传 name 状态码 = %d", code)
	}
	if name := loadTrackerName(filepath.Join(td, "legacy.toml")); name != "legacy" {
		t.Fatalf("只传 name 创建后的名称 = %q", name)
	}
}

func TestTrackerIDValidation(t *testing.T) {
	valid := []string{"a", "Z", "0", "a-b_c", "A0_-"}
	for _, id := range valid {
		if !validTrackerName(id) {
			t.Errorf("%q 应当合法", id)
		}
	}
	invalid := []string{"", "中文", "a b", "a.b", "a/b", "../a", "a\\b", "a\tb"}
	for _, id := range invalid {
		if validTrackerName(id) {
			t.Errorf("%q 应当不合法", id)
		}
	}
	if paths := trackerFilePaths(t.TempDir(), "../evil"); len(paths) != 0 {
		t.Fatalf("不合法 id 不应拼接路径: %v", paths)
	}
}

func TestTrackerListIgnoresInvalidIDs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	td := cfg.TrackerDir()
	writeTrackerTestFile(t, td, "good.toml", "name = \"good\"\nids = [1]\n")
	writeTrackerTestFile(t, td, "中文.toml", "name = \"中文\"\nids = [2]\n")
	writeTrackerTestFile(t, td, "bad name.toml", "name = \"bad\"\nids = [3]\n")
	infos := loadTrackerInfos(cfg)
	if len(infos) != 1 || infos[0].ID != "good" {
		t.Fatalf("不合法 id 不应出现在列表: %+v", infos)
	}
}

func importCollections(t *testing.T, dd string) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	handleImportCollections(dd)(w, httptest.NewRequest(http.MethodPost, "/api/v0/tracker/import-collections", nil))
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return w.Code, body
}

func TestImportCollectionsKeepsUserTOML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	td := cfg.TrackerDir()
	original := "# Tracker: user\nname = \"我的追番\"\nids = [1]\n"
	writeTrackerTestFile(t, td, "user.toml", original)

	dd := filepath.Join(home, "data")
	writeTrackerTestFile(t, userDir(dd), "collections.json", `{"data":[
		{"subject_id":2,"subject_type":2},
		{"subject_id":3,"subject_type":2},
		{"subject_id":2,"subject_type":2},
		{"subject_id":4,"subject_type":1},
		{"subject_id":0,"subject_type":2}
	]}`)

	code, body := importCollections(t, dd)
	if code != http.StatusOK || body["count"] != float64(2) {
		t.Fatalf("导入结果 = %d %v", code, body)
	}
	data, err := os.ReadFile(filepath.Join(td, "user.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("user.toml 被修改: %s", data)
	}
	var imported struct {
		Name     string `json:"name"`
		Subjects []int  `json:"subjects"`
	}
	raw, err := os.ReadFile(filepath.Join(td, "user.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &imported); err != nil {
		t.Fatal(err)
	}
	if imported.Name != "user" || len(imported.Subjects) != 2 || imported.Subjects[0] != 2 || imported.Subjects[1] != 3 {
		t.Fatalf("user.json 内容不对: %+v", imported)
	}

	infos := loadTrackerInfos(cfg)
	if len(infos) != 1 {
		t.Fatalf("合并后应只有一项: %+v", infos)
	}
	got := infos[0]
	if got.ID != "user" || got.Name != "我的追番" || got.Type != trackerTypeManual || got.Count != 3 {
		t.Fatalf("合并结果不对: %+v", got)
	}
}

func TestImportCollectionsCreatesAutoUserJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	cfg := &config.Config{}
	dd := filepath.Join(home, "data")
	writeTrackerTestFile(t, userDir(dd), "collections.json", `{"data":[{"subject_id":9,"subject_type":2}]}`)

	code, body := importCollections(t, dd)
	if code != http.StatusOK || body["count"] != float64(1) {
		t.Fatalf("导入结果 = %d %v", code, body)
	}
	infos := loadTrackerInfos(cfg)
	if len(infos) != 1 || infos[0].ID != "user" || infos[0].Type != trackerTypeAuto || infos[0].Count != 1 {
		t.Fatalf("只有 user.json 时应为自动 Tracker: %+v", infos)
	}
}

func TestImportCollectionsRequiresData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	code, _ := importCollections(t, filepath.Join(home, "data"))
	if code != http.StatusBadRequest {
		t.Fatalf("缺少收藏数据时状态码 = %d", code)
	}
}

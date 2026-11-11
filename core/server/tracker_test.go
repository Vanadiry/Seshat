package server

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/vanadiry/seshat/core/config"
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

	infos := loadTrackerInfos(cfg)
	if len(infos) != 3 {
		t.Fatalf("合并后应有 3 个 Tracker，实际 %d", len(infos))
	}
	if infos[0].ID != "_seshat" || infos[1].ID != "empty" || infos[2].ID != "user" {
		t.Fatalf("未按 id 排序: %+v", infos)
	}
	if infos[0].Name != "seshat" || infos[0].Type != trackerTypeAuto || infos[0].Count != 1 {
		t.Fatalf("自动 Tracker 结果不对: %+v", infos[0])
	}
	if infos[1].Name != "empty" || infos[1].Type != trackerTypeManual || infos[1].Count != 0 {
		t.Fatalf("空名称回退不对: %+v", infos[1])
	}
	if infos[2].Name != "我的追番" || infos[2].Type != trackerTypeManual || infos[2].Count != 3 {
		t.Fatalf("合并 Tracker 结果不对: %+v", infos[2])
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

package config

import (
	"encoding/json"
	"os"
	"testing"
)

func writePrefs(t *testing.T, m map[string]string) {
	t.Helper()
	if err := os.MkdirAll(PrefDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(PrefPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPreferencesDefaults(t *testing.T) {
	t.Setenv("SESHAT_HOME", t.TempDir())

	pref, err := LoadPreferences()
	if err != nil {
		t.Fatal(err)
	}
	if pref.PreferLang != "original" || pref.SubjectSort != "elo" || pref.AutoLinkNames != "true" || pref.Username != "" {
		t.Fatalf("默认偏好 = %+v", pref)
	}
}

func TestPreferencesMerge(t *testing.T) {
	t.Setenv("SESHAT_HOME", t.TempDir())
	writePrefs(t, map[string]string{
		"prefer_lang":  "chinese",
		"subject_sort": "score",
		"unknown":      "ignored",
	})

	pref, err := LoadPreferences()
	if err != nil {
		t.Fatal(err)
	}
	if pref.PreferLang != "chinese" || pref.SubjectSort != "score" {
		t.Fatalf("合并后 = %+v", pref)
	}
	if pref.AutoLinkNames != "true" {
		t.Fatalf("未覆盖项应使用默认值: %+v", pref)
	}
}

func TestApplyOverrides(t *testing.T) {
	t.Setenv("SESHAT_HOME", t.TempDir())

	ok, fail := ApplyOverrides(map[string]any{
		"prefer_lang":  "chinese",
		"theme":        "auto",
		"username":     "",
		"bad_key":      "x",
		"subject_sort": 1,
	})
	has := func(list []string, key string) bool {
		for _, v := range list {
			if v == key {
				return true
			}
		}
		return false
	}
	if !has(ok, "prefer_lang") || !has(ok, "theme") || !has(ok, "username") {
		t.Fatalf("okList = %v", ok)
	}
	if !has(fail, "bad_key") || !has(fail, "subject_sort") {
		t.Fatalf("failList = %v", fail)
	}

	m, err := loadOverrides()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m["prefer_lang"] != "chinese" {
		t.Fatalf("覆盖文件 = %v", m)
	}

	ApplyOverrides(map[string]any{
		"prefer_lang":  "original",
		"theme":        "auto",
		"username":     "",
		"subject_sort": "elo",
	})
	if _, err := os.Stat(PrefPath()); !os.IsNotExist(err) {
		t.Fatal("全部恢复默认后应删除覆盖文件")
	}
}

func TestBuildPrefsKV(t *testing.T) {
	t.Setenv("SESHAT_HOME", t.TempDir())
	writePrefs(t, map[string]string{"theme": "light", "prefer_lang": ""})

	kv := BuildPrefsKV()
	if kv["theme"] != "light" {
		t.Fatalf("theme = %q", kv["theme"])
	}
	if kv["prefer_lang"] != "original" {
		t.Fatalf("空值应回落到默认值: %q", kv["prefer_lang"])
	}
}

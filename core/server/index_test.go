package server

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/vanadiry/seshat/core/store"
)

func putTestJSON(t *testing.T, dd, key string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(dd, key, data); err != nil {
		t.Fatal(err)
	}
}

func readTestIndex[T any](t *testing.T, dd, name string) T {
	t.Helper()
	data, err := os.ReadFile(store.IndexFile(dd, name))
	if err != nil {
		t.Fatal(err)
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestBuildIndexesScansData(t *testing.T) {
	dd := t.TempDir()

	putTestJSON(t, dd, store.Key("subjects", 11, "info.json"), map[string]any{
		"id":       11,
		"name":     "Alpha",
		"name_cn":  "甲",
		"platform": "TV",
		"date":     "2020-01-01",
		"rating":   map[string]any{"score": 8.5},
		"tags":     []map[string]any{{"name": "科幻", "count": 2}},
	})
	putTestJSON(t, dd, store.Key("subjects", 12, "info.json"), map[string]any{
		"id":      12,
		"name":    "Beta",
		"name_cn": "乙",
		"rating":  map[string]any{"score": 7.0},
		"tags": []map[string]any{
			{"name": "科幻", "count": 1},
			{"name": "动画", "count": 1},
		},
	})
	putTestJSON(t, dd, store.Key("characters", 22, "info.json"), map[string]any{
		"id":   22,
		"name": "Char",
		"infobox": []map[string]any{
			{"key": "简体中文名", "value": "角色"},
		},
	})
	putTestJSON(t, dd, store.Key("persons", 33, "info.json"), map[string]any{
		"id":   33,
		"name": "Person",
		"infobox": []map[string]any{
			{"key": "简体中文名", "value": "人物"},
		},
	})

	buildIndexes(dd, nil)

	subjects := readTestIndex[[]store.SubjectSummary](t, dd, "subjects.json")
	if len(subjects) != 2 {
		t.Fatalf("条目数 = %d", len(subjects))
	}
	if subjects[0].ID != 11 || subjects[1].ID != 12 {
		t.Fatalf("条目应为 11,12，实际 %+v", subjects)
	}
	if subjects[0].Score != 8.5 || subjects[0].Platform != "TV" || subjects[0].Date != "2020-01-01" {
		t.Fatalf("条目字段 = %+v", subjects[0])
	}

	tags := readTestIndex[map[string]tagInfo](t, dd, "tags.json")
	if tags["科幻"].Count != 2 || len(tags["科幻"].Subjects) != 2 {
		t.Fatalf("标签统计 = %+v", tags["科幻"])
	}
	if tags["动画"].Count != 1 || tags["动画"].Subjects[0] != 12 {
		t.Fatalf("标签统计 = %+v", tags["动画"])
	}

	subjectNames := readTestIndex[map[int][]string](t, dd, "subjects_name.json")
	if !hasName(subjectNames[11], "Alpha") || !hasName(subjectNames[11], "甲") {
		t.Fatalf("条目名称索引 = %v", subjectNames[11])
	}

	charNames := readTestIndex[map[int][]string](t, dd, "characters_name.json")
	if !hasName(charNames[22], "Char") || !hasName(charNames[22], "角色") {
		t.Fatalf("角色名称索引 = %v", charNames[22])
	}

	personNames := readTestIndex[map[int][]string](t, dd, "persons_name.json")
	if !hasName(personNames[33], "Person") || !hasName(personNames[33], "人物") {
		t.Fatalf("人物名称索引 = %v", personNames[33])
	}
}

func hasName(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

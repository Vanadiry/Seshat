package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanadiry/seshat/core/store"
)

func TestSearchAllJSONCapsAndCounts(t *testing.T) {
	dd := t.TempDir()
	const matches = searchLimit + 5
	for i := 1; i <= matches; i++ {
		body := fmt.Sprintf(`{"id":%d,"name":"测试条目"}`, i)
		path := filepath.Join(store.Dir(dd), store.Key("subjects", i, "info.json"))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("写入条目 %d: %v", i, err)
		}
	}

	results, total := searchAllJSON(dd, "subjects", "测试")
	if len(results) != searchLimit {
		t.Fatalf("结果数 = %d，期望 %d", len(results), searchLimit)
	}
	if total != matches {
		t.Fatalf("命中数 = %d，期望 %d", total, matches)
	}
}

func TestSearchSubjectsResponse(t *testing.T) {
	dd := t.TempDir()
	for i := 1; i <= 2; i++ {
		body := fmt.Sprintf(`{"id":%d,"name":"测试条目"}`, i)
		path := filepath.Join(store.Dir(dd), store.Key("subjects", i, "info.json"))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	w := httptest.NewRecorder()
	handleSearchSubjects(dd)(w, httptest.NewRequest(
		http.MethodPost,
		"/api/v0/search/subjects",
		strings.NewReader(`{"keyword":"测试"}`),
	))

	var body struct {
		Total  int              `json:"total"`
		Limit  int              `json:"limit"`
		Offset int              `json:"offset"`
		Data   []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if body.Total != 2 || body.Limit != searchLimit || body.Offset != 0 {
		t.Fatalf("分页字段 = %+v", body)
	}
	if len(body.Data) != 2 {
		t.Fatalf("data 长度 = %d", len(body.Data))
	}
}

func TestSearchTagsCapsAndCounts(t *testing.T) {
	dd := t.TempDir()
	tags := map[string]tagInfo{}
	const matches = searchLimit + 3
	for i := 0; i < matches; i++ {
		tags[fmt.Sprintf("标签%03d", i)] = tagInfo{Count: i + 1}
	}
	data, err := json.Marshal(tags)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteAtomic(store.IndexFile(dd, "tags.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	handleSearchTags(dd)(w, httptest.NewRequest(
		http.MethodPost,
		"/api/v0/search/tags",
		strings.NewReader(`{"keyword":"标签"}`),
	))

	var body struct {
		Total int              `json:"total"`
		Data  []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if body.Total != matches {
		t.Fatalf("命中数 = %d，期望 %d", body.Total, matches)
	}
	if len(body.Data) != searchLimit {
		t.Fatalf("data 长度 = %d，期望 %d", len(body.Data), searchLimit)
	}
}

func TestSearchEmptyKeyword(t *testing.T) {
	dd := t.TempDir()
	w := httptest.NewRecorder()
	handleSearchSubjects(dd)(w, httptest.NewRequest(
		http.MethodPost,
		"/api/v0/search/subjects",
		strings.NewReader(`{}`),
	))

	var body struct {
		Total  int              `json:"total"`
		Limit  int              `json:"limit"`
		Offset int              `json:"offset"`
		Data   []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if body.Total != 0 || body.Limit != searchLimit || body.Offset != 0 || len(body.Data) != 0 {
		t.Fatalf("空关键词响应 = %+v", body)
	}
}

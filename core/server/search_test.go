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

	"github.com/vanadiry/seshat/core/index"
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
	tags := map[string]index.TagInfo{}
	const matches = searchLimit + 3
	for i := 0; i < matches; i++ {
		tags[fmt.Sprintf("标签%03d", i)] = index.TagInfo{Count: i + 1}
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

func writeSearchEntry(t *testing.T, dd string, id int, body string) {
	t.Helper()
	path := filepath.Join(store.Dir(dd), store.Key("subjects", id, "info.json"))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("写入条目 %d: %v", id, err)
	}
}

func TestSearchMatchFields(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		query    string
		field    string
		key      string
		wantText bool
	}{
		{
			name:  "名称",
			body:  `{"id":1,"name":"测试条目"}`,
			query: "测试",
			field: "name",
		},
		{
			name:  "顶层中文名",
			body:  `{"id":1,"name":"a","name_cn":"测试中文"}`,
			query: "测试",
			field: "name_cn",
		},
		{
			name:  "信息栏中文名",
			body:  `{"id":1,"name":"a","infobox":[{"key":"简体中文名","value":"测试中文"}]}`,
			query: "测试",
			field: "name_cn",
		},
		{
			name:     "信息栏字符串",
			body:     `{"id":1,"name":"a","infobox":[{"key":"别名","value":"测试别名"}]}`,
			query:    "测试",
			field:    "infobox",
			key:      "别名",
			wantText: true,
		},
		{
			name:     "信息栏字符串数组",
			body:     `{"id":1,"name":"a","infobox":[{"key":"别名","value":["别名一","测试别名二"]}]}`,
			query:    "测试",
			field:    "infobox",
			key:      "别名",
			wantText: true,
		},
		{
			name:     "信息栏键值对",
			body:     `{"id":1,"name":"a","infobox":[{"key":"名字","value":[{"k":"纯假名","v":"测试假名"}]}]}`,
			query:    "测试",
			field:    "infobox",
			key:      "纯假名",
			wantText: true,
		},
		{
			name:     "简介",
			body:     `{"id":1,"name":"a","summary":"这是一段简介，包含测试关键词"}`,
			query:    "测试",
			field:    "summary",
			wantText: true,
		},
		{
			name:  "大小写不敏感",
			body:  `{"id":1,"name":"Clannad"}`,
			query: "clannad",
			field: "name",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dd := t.TempDir()
			writeSearchEntry(t, dd, 1, tc.body)
			results, total := searchAllJSON(dd, "subjects", tc.query)
			if total != 1 || len(results) != 1 {
				t.Fatalf("结果 = %d，命中 = %d", len(results), total)
			}
			match, ok := results[0]["match"].(*searchMatch)
			if !ok || match == nil {
				t.Fatalf("缺少 match: %+v", results[0])
			}
			if match.Field != tc.field {
				t.Fatalf("field = %q，期望 %q", match.Field, tc.field)
			}
			if match.Key != tc.key {
				t.Fatalf("key = %q，期望 %q", match.Key, tc.key)
			}
			if tc.wantText {
				if match.Text == "" {
					t.Fatal("text 为空")
				}
				if !strings.Contains(match.Text, tc.query) {
					t.Fatalf("text = %q，缺失关键词", match.Text)
				}
			} else if match.Text != "" {
				t.Fatalf("text = %q，期望为空", match.Text)
			}
		})
	}
}

func TestSearchSkipsKeysAndURLs(t *testing.T) {
	dd := t.TempDir()
	writeSearchEntry(t, dd, 1, `{"id":1,"name":"a","infobox":[{"key":"测试键","value":"https://example.com/测试"}],"images":{"large":"https://example.com/测试.jpg"}}`)
	results, total := searchAllJSON(dd, "subjects", "测试")
	if total != 0 || len(results) != 0 {
		t.Fatalf("结果 = %d，命中 = %d，期望 0", len(results), total)
	}
}

func TestSearchMatchPriority(t *testing.T) {
	dd := t.TempDir()
	writeSearchEntry(t, dd, 1, `{"id":1,"name":"测试条目","name_cn":"测试中文","infobox":[{"key":"别名","value":"测试别名"}],"summary":"测试简介"}`)
	results, _ := searchAllJSON(dd, "subjects", "测试")
	match, ok := results[0]["match"].(*searchMatch)
	if !ok || match.Field != "name" {
		t.Fatalf("match = %+v，期望名称优先", results[0]["match"])
	}
}

func TestSnippetTruncatesAtRuneBoundary(t *testing.T) {
	dd := t.TempDir()
	summary := strings.Repeat("前", 50) + "测试" + strings.Repeat("后", 50)
	writeSearchEntry(t, dd, 1, fmt.Sprintf(`{"id":1,"name":"a","summary":%q}`, summary))
	results, _ := searchAllJSON(dd, "subjects", "测试")
	match, ok := results[0]["match"].(*searchMatch)
	if !ok || match.Text == "" {
		t.Fatal("缺少片段")
	}
	if !strings.HasPrefix(match.Text, "…") || !strings.HasSuffix(match.Text, "…") {
		t.Fatalf("片段未截断: %q", match.Text)
	}
	if !strings.Contains(match.Text, "测试") {
		t.Fatalf("片段缺失关键词: %q", match.Text)
	}
	if strings.ContainsRune(match.Text, '\uFFFD') {
		t.Fatalf("片段含不合法字符: %q", match.Text)
	}
}

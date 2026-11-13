package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"unicode"

	"github.com/vanadiry/seshat/core/index"
)

const searchLimit = 500
const searchSnippetRadius = 30

// searchMatch 记录命中字段，供前端高亮
type searchMatch struct {
	Field string `json:"field"`
	Key   string `json:"key,omitempty"`
	Text  string `json:"text,omitempty"`
}

// infoboxEntry 为 info.json 的信息栏字段
type infoboxEntry struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// searchEntry 为搜索需要读取的 info.json 字段
type searchEntry struct {
	ID      int            `json:"id"`
	Name    string         `json:"name"`
	NameCN  string         `json:"name_cn"`
	Infobox []infoboxEntry `json:"infobox"`
	Summary string         `json:"summary"`
}

// matchRunes 返回 q 在 s 中首次出现的位置，忽略大小写
func matchRunes(s, q string) int {
	rs := []rune(s)
	rq := []rune(q)
	if len(rq) == 0 || len(rq) > len(rs) {
		return -1
	}
	for i := 0; i+len(rq) <= len(rs); i++ {
		matched := true
		for j := 0; j < len(rq); j++ {
			if unicode.ToLower(rs[i+j]) != unicode.ToLower(rq[j]) {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}

// containsFold 判断 s 是否包含 q，忽略大小写
func containsFold(s, q string) bool {
	return matchRunes(s, q) >= 0
}

// snippet 截取命中位置前后的内容
func snippet(text, q string) string {
	rs := []rune(text)
	pos := matchRunes(text, q)
	if pos < 0 {
		return ""
	}
	start := pos - searchSnippetRadius
	if start < 0 {
		start = 0
	}
	end := pos + len([]rune(q)) + searchSnippetRadius
	if end > len(rs) {
		end = len(rs)
	}
	out := strings.Join(strings.Fields(string(rs[start:end])), " ")
	if start > 0 {
		out = "…" + out
	}
	if end < len(rs) {
		out += "…"
	}
	return out
}

// infoboxValue 为展开后的信息栏值
type infoboxValue struct {
	Key   string
	Value string
}

// infoboxValues 展开信息栏的字符串、字符串数组和键值对数组
func infoboxValues(raw json.RawMessage) []infoboxValue {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return []infoboxValue{{Value: text}}
	}
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}
	out := make([]infoboxValue, 0, len(items))
	for _, item := range items {
		if json.Unmarshal(item, &text) == nil {
			out = append(out, infoboxValue{Value: text})
			continue
		}
		var kv struct {
			K string `json:"k"`
			V string `json:"v"`
		}
		if json.Unmarshal(item, &kv) == nil && (kv.K != "" || kv.V != "") {
			out = append(out, infoboxValue{Key: kv.K, Value: kv.V})
		}
	}
	return out
}

// looksLikeURL 判断信息栏值是否为链接
func looksLikeURL(s string) bool {
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://")
}

// entryNameCN 提取顶层中文名，缺失时读信息栏的简体中文名
func entryNameCN(entry searchEntry) string {
	if entry.NameCN != "" {
		return entry.NameCN
	}
	for _, ib := range entry.Infobox {
		if ib.Key != "简体中文名" {
			continue
		}
		for _, value := range infoboxValues(ib.Value) {
			if value.Value != "" {
				return value.Value
			}
		}
	}
	return ""
}

// findSearchMatch 按名称、中文名、信息栏、简介的顺序找命中
func findSearchMatch(entry searchEntry, nameCN, q string, withText bool) *searchMatch {
	if containsFold(entry.Name, q) {
		return &searchMatch{Field: "name"}
	}
	if nameCN != "" && containsFold(nameCN, q) {
		return &searchMatch{Field: "name_cn"}
	}
	for _, ib := range entry.Infobox {
		for _, value := range infoboxValues(ib.Value) {
			if value.Value == "" || looksLikeURL(value.Value) {
				continue
			}
			if !containsFold(value.Value, q) {
				continue
			}
			if ib.Key == "简体中文名" {
				return &searchMatch{Field: "name_cn"}
			}
			key := value.Key
			if key == "" {
				key = ib.Key
			}
			text := ""
			if withText {
				text = snippet(value.Value, q)
			}
			return &searchMatch{
				Field: "infobox",
				Key:   key,
				Text:  text,
			}
		}
	}
	if containsFold(entry.Summary, q) {
		text := ""
		if withText {
			text = snippet(entry.Summary, q)
		}
		return &searchMatch{Field: "summary", Text: text}
	}
	return nil
}

// searchAllJSON 扫描 data/api/{domain} 下的 info.json，按关键词匹配
func searchAllJSON(dd, domain, q string) ([]map[string]any, int) {
	results := []map[string]any{}
	seen := map[int]bool{}
	total := 0
	if q == "" {
		return results, total
	}
	qLower := strings.ToLower(q)

	index.ScanAPIDir(dd, domain, func(path string) {
		if !strings.HasSuffix(path, "info.json") {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) < 4 {
			return
		}
		if !strings.Contains(strings.ToLower(string(data)), qLower) {
			return
		}
		var entry searchEntry
		if json.Unmarshal(data, &entry) != nil || entry.ID == 0 || seen[entry.ID] {
			return
		}
		nameCN := entryNameCN(entry)
		match := findSearchMatch(entry, nameCN, q, len(results) < searchLimit)
		if match == nil {
			return
		}
		seen[entry.ID] = true
		total++
		if len(results) >= searchLimit {
			return
		}
		item := map[string]any{
			"id":    entry.ID,
			"name":  entry.Name,
			"match": match,
		}
		if nameCN != "" {
			item["name_cn"] = nameCN
		}
		results = append(results, item)
	})
	return results, total
}

// writeSearchList 输出搜索结果分页，data 最多 500 条
func writeSearchList(w http.ResponseWriter, data []map[string]any, total int) {
	if data == nil {
		data = []map[string]any{}
	}
	writeJSON(w, map[string]any{
		"total":  total,
		"limit":  searchLimit,
		"offset": 0,
		"data":   data,
	})
}

func keywordFromRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	limitBody(w, r)
	var req struct {
		Keyword string `json:"keyword"`
	}
	if !decodeJSON(w, r, &req) {
		return "", false
	}
	if req.Keyword == "" {
		writeSearchList(w, nil, 0)
		return "", false
	}
	return req.Keyword, true
}

func handleSearchSubjects(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyword, ok := keywordFromRequest(w, r)
		if !ok {
			return
		}
		results, total := searchAllJSON(dd, "subjects", keyword)
		writeSearchList(w, results, total)
	}
}

func handleSearchCharacters(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyword, ok := keywordFromRequest(w, r)
		if !ok {
			return
		}
		results, total := searchAllJSON(dd, "characters", keyword)
		writeSearchList(w, results, total)
	}
}

func handleSearchPersons(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyword, ok := keywordFromRequest(w, r)
		if !ok {
			return
		}
		results, total := searchAllJSON(dd, "persons", keyword)
		writeSearchList(w, results, total)
	}
}

func handleSearchTags(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyword, ok := keywordFromRequest(w, r)
		if !ok {
			return
		}
		tags := index.LoadTags(dd)
		results := []map[string]any{}
		ql := strings.ToLower(keyword)
		total := 0
		for name, info := range tags {
			if !strings.Contains(strings.ToLower(name), ql) {
				continue
			}
			total++
			if len(results) >= searchLimit {
				continue
			}
			results = append(results, map[string]any{"name": name, "count": info.Count})
		}
		writeSearchList(w, results, total)
	}
}

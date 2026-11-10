package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

const searchLimit = 500

// searchAllJSON 扫描 data/api/{domain} 下的 info.json，按关键词匹配
func searchAllJSON(dd, domain, q string) ([]map[string]any, int) {
	q = strings.ToLower(q)
	results := []map[string]any{}
	seen := map[int]bool{}
	total := 0

	scanAPIDir(dd, domain, func(path string) {
		if !strings.HasSuffix(path, "info.json") {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) < 4 {
			return
		}
		if !strings.Contains(strings.ToLower(string(data)), q) {
			return
		}
		var entry struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			NameCN  string `json:"name_cn"`
			Infobox []struct {
				Key   string          `json:"key"`
				Value json.RawMessage `json:"value"`
			} `json:"infobox"`
		}
		if json.Unmarshal(data, &entry) != nil || entry.ID == 0 || seen[entry.ID] {
			return
		}
		seen[entry.ID] = true
		total++
		if len(results) >= searchLimit {
			return
		}
		item := map[string]any{"id": entry.ID, "name": entry.Name}
		if domain == "subjects" {
			if entry.NameCN != "" {
				item["name_cn"] = entry.NameCN
			} else {
				for _, ib := range entry.Infobox {
					if ib.Key == "简体中文名" {
						var s string
						if json.Unmarshal(ib.Value, &s) == nil && s != "" {
							item["name_cn"] = s
						}
						break
					}
				}
			}
		} else {
			for _, ib := range entry.Infobox {
				if ib.Key == "简体中文名" {
					var s string
					if json.Unmarshal(ib.Value, &s) == nil && s != "" {
						item["name_cn"] = s
					}
					break
				}
			}
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
		tags := loadTags(dd)
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

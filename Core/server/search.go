package server

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/vanadiry/seshat/Core/cache"
	"path/filepath"
)

func quickSearch(dd, q string, types []string) map[string]any {
	result := map[string]any{}
	q = strings.ToLower(q)
	for _, t := range types {
		switch t {
		case "subjects":
			result["subjects"] = searchList(dd, "subjects.json", q)
		case "characters":
			result["characters"] = searchList(dd, "characters.json", q)
		case "persons":
			result["persons"] = searchList(dd, "persons.json", q)
		case "tags":
			result["tags"] = searchTags(dd, q)
		}
	}
	return result
}

func searchList(dd, file, q string) []map[string]any {
	data, err := os.ReadFile(cache.IndexFile(dd, file))
	if err != nil {
		return []map[string]any{}
	}
	var list []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		NameCN string `json:"name_cn"`
	}
	json.Unmarshal(data, &list)
	var results []map[string]any
	for _, entry := range list {
		if strings.Contains(strings.ToLower(entry.Name), q) || (entry.NameCN != "" && strings.Contains(strings.ToLower(entry.NameCN), q)) {
			results = append(results, map[string]any{
				"id":      entry.ID,
				"name":    entry.Name,
				"name_cn": entry.NameCN,
			})
		}
	}
	return results
}

func searchTags(dd, q string) []map[string]any {
	tags := loadTags(dd)
	var results []map[string]any
	for name, info := range tags {
		if strings.Contains(strings.ToLower(name), q) {
			results = append(results, map[string]any{
				"name":  name,
				"count": info.Count,
			})
		}
	}
	return results
}

// deepSearchStream 扫描 JSON 文件，每找到一个结果就回调 send，返回总数。
func deepSearchStream(dd, q string, types []string, send func(map[string]any)) int {
	q = strings.ToLower(q)
	apiDir := cache.Dir(dd)
	count := 0

	for _, t := range types {
		dir := filepath.Join(apiDir, t)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") || strings.Contains(e.Name(), "/") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			if strings.Contains(strings.ToLower(string(data)), q) {
				var s struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				}
				json.Unmarshal(data, &s)
				send(map[string]any{"type": t, "id": s.ID, "name": s.Name})
				count++
			}
		}
	}
	return count
}

// deepSearch 扫描所有 JSON 文件进行全文搜索（非流式，保留兼容）。
func deepSearch(dd, q string, types []string) []map[string]any {
	var results []map[string]any
	ql := strings.ToLower(q)
	apiDir := cache.Dir(dd)

	for _, t := range types {
		dir := filepath.Join(apiDir, t)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") || strings.Contains(e.Name(), "/") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			if strings.Contains(strings.ToLower(string(data)), ql) {
				// Extract name + id
				var s struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				}
				json.Unmarshal(data, &s)
				results = append(results, map[string]any{
					"type": t,
					"id":   s.ID,
					"name": s.Name,
				})
			}
		}
	}
	return results
}

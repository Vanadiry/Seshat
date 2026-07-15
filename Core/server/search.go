package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

// searchAllJSON scans all info.json files under data/api/{domain} for keyword match.
func searchAllJSON(dd, domain, q string) []map[string]any {
	q = strings.ToLower(q)
	var results []map[string]any
	seen := map[int]bool{}

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
	return results
}

func handleSearchSubjects(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Keyword string `json:"keyword"`
		}
		if limitBody(w, r); json.NewDecoder(r.Body).Decode(&req) != nil || req.Keyword == "" {
			writeJSON(w, map[string]any{"data": []any{}})
			return
		}
		writeJSON(w, map[string]any{"data": searchAllJSON(dd, "subjects", req.Keyword)})
	}
}

func handleSearchCharacters(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Keyword string `json:"keyword"`
		}
		if limitBody(w, r); json.NewDecoder(r.Body).Decode(&req) != nil || req.Keyword == "" {
			writeJSON(w, map[string]any{"data": []any{}})
			return
		}
		writeJSON(w, map[string]any{"data": searchAllJSON(dd, "characters", req.Keyword)})
	}
}

func handleSearchPersons(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Keyword string `json:"keyword"`
		}
		if limitBody(w, r); json.NewDecoder(r.Body).Decode(&req) != nil || req.Keyword == "" {
			writeJSON(w, map[string]any{"data": []any{}})
			return
		}
		writeJSON(w, map[string]any{"data": searchAllJSON(dd, "persons", req.Keyword)})
	}
}

func handleSearchTags(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Keyword string `json:"keyword"`
		}
		if limitBody(w, r); json.NewDecoder(r.Body).Decode(&req) != nil || req.Keyword == "" {
			writeJSON(w, map[string]any{"data": []any{}})
			return
		}
		tags := loadTags(dd)
		var results []map[string]any
		ql := strings.ToLower(req.Keyword)
		for name, info := range tags {
			if strings.Contains(strings.ToLower(name), ql) {
				results = append(results, map[string]any{"name": name, "count": info.Count})
			}
		}
		writeJSON(w, map[string]any{"data": results})
	}
}

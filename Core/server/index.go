package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/log"
)

func buildIndexes(dd string, p *Progress) {
	log.Info("Building indexes...")
	os.MkdirAll(cache.IndexDir(dd), 0o755)
	os.MkdirAll(filepath.Join(dd, "images"), 0o755)
	if len(noImageData) > 0 && noImagePath != "" {
		os.WriteFile(noImagePath, noImageData, 0o644)
	}

	tags := map[string]tagInfo{}

	apiDir := cache.Dir(dd)

	// Scan subjects for tags
	subjDir := filepath.Join(apiDir, "subjects")
	if entries, _ := os.ReadDir(subjDir); len(entries) > 0 {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") || strings.Contains(e.Name(), "/") {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(subjDir, e.Name()))
			var s struct {
				ID   int `json:"id"`
				Tags []struct{
					Name  string `json:"name"`
					Count int    `json:"count"`
				} `json:"tags"`
			}
			if json.Unmarshal(data, &s) == nil && s.ID > 0 {
				for _, t := range s.Tags {
					info := tags[t.Name]
					info.Count++
					info.Subjects = append(info.Subjects, s.ID)
					tags[t.Name] = info
				}
			}
		}
	}

	saveJSON(cache.IndexFile(dd, "tags.json"), tags)

	log.Info("Indexes built: %d tags", len(tags))
}

// Phase 3: Download images via official endpoints.
func tagsPath(dd string) string {
	return cache.IndexFile(dd, "tags.json")
}

func loadTags(dd string) map[string]tagInfo {
	data, err := os.ReadFile(tagsPath(dd))
	if err != nil {
		return map[string]tagInfo{}
	}
	var tags map[string]tagInfo
	json.Unmarshal(data, &tags)
	if tags == nil {
		tags = map[string]tagInfo{}
	}
	return tags
}

// rebuildTags 扫描所有已缓存的 subject JSON，重建 tags.json。
func rebuildTags(dd string) {
	dir := filepath.Join(cache.Dir(dd), "subjects")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	tags := map[string]tagInfo{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || strings.Contains(e.Name(), "/") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var s struct {
			ID   int `json:"id"`
			Tags []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"tags"`
		}
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		for _, t := range s.Tags {
			info := tags[t.Name]
			info.Count++
			info.Subjects = append(info.Subjects, s.ID)
			tags[t.Name] = info
		}
	}

	result, _ := json.Marshal(tags)
	os.WriteFile(tagsPath(dd), result, 0o644)
	log.Info("Tags index rebuilt: %d tags", len(tags))
}

// fetchUserCollections 拉取用户收藏列表（只拉动画，type=2）

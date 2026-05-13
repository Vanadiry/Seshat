package server

import (
	"sort"
	"net/http"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/log"
)

// scanAPIDir walks the new cache layout {domain}/{id%10}/{id}/*.json and calls fn for each file.
func scanAPIDir(dd, domain string, fn func(path string)) {
	base := filepath.Join(cache.Dir(dd), domain)
	digits, _ := os.ReadDir(base)
	for _, d := range digits {
		if !d.IsDir() { continue }
		ids, _ := os.ReadDir(filepath.Join(base, d.Name()))
		for _, id := range ids {
			if !id.IsDir() { continue }
			idDir := filepath.Join(base, d.Name(), id.Name())
			files, _ := os.ReadDir(idDir)
			for _, f := range files {
				if strings.HasSuffix(f.Name(), ".json") && !f.IsDir() {
					fn(filepath.Join(idDir, f.Name()))
				}
			}
		}
	}
}

func buildIndexes(dd string, p *Progress) {
	log.Info("Building indexes...")
	os.MkdirAll(cache.IndexDir(dd), 0o755)
	os.MkdirAll(filepath.Join(dd, "images"), 0o755)
	if len(noImageData) > 0 && noImagePath != "" {
		os.WriteFile(noImagePath, noImageData, 0o644)
	}

	tags := map[string]tagInfo{}

	// Scan subjects for tags (new dir layout: subjects/{id%10}/{id}/info.json)
	scanAPIDir(dd, "subjects", func(path string) {
		if !strings.HasSuffix(path, "info.json") { return }
		data, _ := os.ReadFile(path)
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
	})

	saveJSON(cache.IndexFile(dd, "tags.json"), tags)
	buildPersonNames(dd)

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
	tags := map[string]tagInfo{}
	scanAPIDir(dd, "subjects", func(path string) {
		if !strings.HasSuffix(path, "info.json") { return }
		data, err := os.ReadFile(path)
		if err != nil { return }
		var s struct {
			ID   int `json:"id"`
			Tags []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"tags"`
		}
		if err := json.Unmarshal(data, &s); err != nil { return }
		for _, t := range s.Tags {
			info := tags[t.Name]
			info.Count++
			info.Subjects = append(info.Subjects, s.ID)
			tags[t.Name] = info
		}
	})
	result, _ := json.Marshal(tags)
	os.WriteFile(tagsPath(dd), result, 0o644)
	buildPersonNames(dd)
	log.Info("Tags index rebuilt: %d tags", len(tags))
}

// buildPersonNames generates a name→id lookup from persons.json.
func buildPersonNames(dd string) {
	data, err := os.ReadFile(cache.IndexFile(dd, "persons.json"))
	if err != nil { log.Warn("persons_name: persons.json not found"); return }
	var list []cache.NameEntry
	json.Unmarshal(data, &list)
	m := map[string]int{}
	for _, p := range list { m[p.Name] = p.ID }
	result, _ := json.Marshal(m)
	os.WriteFile(cache.IndexFile(dd, "persons_name.json"), result, 0o644)
	log.Info("persons_name.json built: %d names", len(m))
}

// rebuildFromScan scans all cached API JSON files and rebuilds list files + tags.
func rebuildFromScan(dd string, p *Progress) {
	log.Info("Rebuilding indexes from scan...")
	os.MkdirAll(cache.IndexDir(dd), 0o755)

	var subjects []cache.SubjectSummary
	var chars []cache.NameEntry
	var persons []cache.NameEntry
	tags := map[string]tagInfo{}

	// Scan subjects
	if p != nil { p.Send("phase", 0, 4, "scanning subjects") }
	scanAPIDir(dd, "subjects", func(path string) {
		if !strings.HasSuffix(path, "info.json") { return }
		data, _ := os.ReadFile(path)
		var s struct {
			ID       int     `json:"id"`
			Name     string  `json:"name"`
			NameCN   string  `json:"name_cn"`
			Rating   struct{ Score float64 `json:"score"` } `json:"rating"`
			Platform string  `json:"platform"`
			Date     string  `json:"date"`
			Tags     []struct{ Name string `json:"name"`; Count int `json:"count"` } `json:"tags"`
		}
		if json.Unmarshal(data, &s) == nil && s.ID > 0 {
			subjects = append(subjects, cache.SubjectSummary{ID: s.ID, Name: s.Name, NameCN: s.NameCN, Score: s.Rating.Score, Platform: s.Platform, Date: s.Date})
			for _, t := range s.Tags { info := tags[t.Name]; info.Count++; info.Subjects = append(info.Subjects, s.ID); tags[t.Name] = info }
		}
	})
	saveJSON(cache.IndexFile(dd, "subjects.json"), subjects)

	// Scan characters
	if p != nil { p.Send("phase", 1, 4, "scanning characters") }
	scanAPIDir(dd, "characters", func(path string) {
		if !strings.HasSuffix(path, "info.json") { return }
		data, _ := os.ReadFile(path)
		var c struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Infobox []struct{ Key string `json:"key"`; Value json.RawMessage `json:"value"` } `json:"infobox"`
		}
		if json.Unmarshal(data, &c) == nil && c.ID > 0 {
			nameCN := ""
			for _, ib := range c.Infobox { if ib.Key == "简体中文名" { var v string; if json.Unmarshal(ib.Value, &v) == nil { nameCN = v }; break } }
			chars = append(chars, cache.NameEntry{ID: c.ID, Name: c.Name, NameCN: nameCN})
		}
	})
	saveJSON(cache.IndexFile(dd, "characters.json"), chars)

	// Scan persons
	if p != nil { p.Send("phase", 2, 4, "scanning persons") }
	scanAPIDir(dd, "persons", func(path string) {
		if !strings.HasSuffix(path, "info.json") { return }
		data, _ := os.ReadFile(path)
		var p struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Infobox []struct{ Key string `json:"key"`; Value json.RawMessage `json:"value"` } `json:"infobox"`
		}
		if json.Unmarshal(data, &p) == nil && p.ID > 0 {
			nameCN := ""
			for _, ib := range p.Infobox { if ib.Key == "简体中文名" { var v string; if json.Unmarshal(ib.Value, &v) == nil { nameCN = v }; break } }
			persons = append(persons, cache.NameEntry{ID: p.ID, Name: p.Name, NameCN: nameCN})
		}
	})
saveJSON(cache.IndexFile(dd, "persons.json"), persons)

	// Save tags
	if p != nil { p.Send("phase", 3, 4, "saving tags") }
	saveJSON(cache.IndexFile(dd, "tags.json"), tags)
	buildPersonNames(dd)

	log.Info("Rebuild complete: %d subjects, %d chars, %d persons, %d tags",
		len(subjects), len(chars), len(persons), len(tags))
}

func handleTags(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tags := loadTags(dd)
		type tagItem struct{ Name string `json:"name"`; Count int `json:"count"` }
		var list []tagItem
		for name, info := range tags { list = append(list, tagItem{Name: name, Count: info.Count}) }
		sort.Slice(list, func(i, j int) bool { return list[i].Count > list[j].Count })
		writeJSON(w, list)
	}
}

func handleTagSubjects(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		tags := loadTags(dd)
		info, ok := tags[name]
		if !ok { writeJSON(w, []int{}); return }
		var subjects []any
		for _, sid := range info.Subjects {
			data, err := cache.Get(dd, cache.Key("subjects", sid, "info.json"))
			if err != nil { continue }
			var s struct {
				ID int `json:"id"`; Name string `json:"name"`; NameCN string `json:"name_cn"`
				Rating struct{ Score float64 `json:"score"` } `json:"rating"`
				Platform string `json:"platform"`
			}
			json.Unmarshal(data, &s)
			subjects = append(subjects, s)
		}
		writeJSON(w, subjects)
	}
}

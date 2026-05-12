package server

import (
	"net/http"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vanadiry/seshat/Core/bangumi"
	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/config"
	"github.com/vanadiry/seshat/Core/log"
)

func countTrackerTotal(cfg *config.Config) int {
	files, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.json"))
	files2, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.toml"))
	files = append(files, files2...)
	seen := map[int]bool{}
	for _, f := range files {
		for _, sid := range loadTrackerIDs(f) {
			seen[sid] = true
		}
	}
	return len(seen)
}

// countTrackerNames 统计指定 tracker 列表中的 subject 总数。
func countTrackerNames(cfg *config.Config, names []string) int {
	td := cfg.TrackerDir()
	seen := map[int]bool{}
	for _, name := range names {
		var path string
		if _, err := os.Stat(filepath.Join(td, name+".json")); err == nil {
			path = filepath.Join(td, name+".json")
		} else if _, err := os.Stat(filepath.Join(td, name+".toml")); err == nil {
			path = filepath.Join(td, name+".toml")
		} else {
			continue
		}
		for _, sid := range loadTrackerIDs(path) {
			seen[sid] = true
		}
	}
	return len(seen)
}

// forceRefresh 删除所有缓存数据后完整重建。
func forceRefresh(cfg *config.Config, bg *bangumi.Client, dd, imgDir string, p *Progress) {
	log.Info("Force refresh: clearing all cached data...")
	os.RemoveAll(dd)
	log.Info("Force refresh: cache cleared, starting rebuild")
	refreshAllTrackers(cfg, bg, dd, imgDir, p)
}

// refreshAllTrackers 刷新 tracker 文件夹中的所有列表。
func refreshAllTrackers(cfg *config.Config, bg *bangumi.Client, dd, imgDir string, p *Progress) {
	files, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.json"))
	files2, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.toml"))
	files = append(files, files2...)
	seen := map[int]bool{}
	var allIDs []int
	for _, f := range files {
		for _, sid := range loadTrackerIDs(f) {
			if !seen[sid] {
				seen[sid] = true
				allIDs = append(allIDs, sid)
			}
		}
	}
	fetchSubjectList(allIDs, bg, dd, imgDir, nil)
	log.Info("All trackers refreshed: %d subjects", len(seen))
	buildIndexes(dd, p)
	downloadImages(dd, bg, p)
}

// refreshTrackers 刷新指定的 tracker 列表。
func refreshTrackers(cfg *config.Config, bg *bangumi.Client, dd, imgDir string, names []string, p *Progress) {
	td := cfg.TrackerDir()
	seen := map[int]bool{}
	var allIDs []int
	for _, name := range names {
		var path string
		if _, err := os.Stat(filepath.Join(td, name+".json")); err == nil {
			path = filepath.Join(td, name+".json")
		} else if _, err := os.Stat(filepath.Join(td, name+".toml")); err == nil {
			path = filepath.Join(td, name+".toml")
		} else {
			continue
		}
		for _, sid := range loadTrackerIDs(path) {
			if !seen[sid] {
				seen[sid] = true
				allIDs = append(allIDs, sid)
			}
		}
	}
	fetchSubjectList(allIDs, bg, dd, imgDir, p)
	log.Info("Trackers %v refreshed: %d subjects", names, len(seen))
	buildIndexes(dd, p)
	downloadImages(dd, bg, p)
}

// addToSeshatTracker 将用户手动添加的 subject ID 记录到 _seshat.json。
func addToSeshatTracker(cfg *config.Config, sid int) {
	path := filepath.Join(cfg.TrackerDir(), "_seshat.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	var data struct {
		Name     string `json:"name"`
		Subjects []int  `json:"subjects"`
	}
	raw, _ := os.ReadFile(path)
	json.Unmarshal(raw, &data)
	if data.Name == "" {
		data.Name = "seshat"
	}
	for _, existing := range data.Subjects {
		if existing == sid {
			return
		}
	}
	data.Subjects = append(data.Subjects, sid)
	result, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile(path, result, 0o644)
}

// loadTrackerIDs 从 tracker 文件（JSON 或 TOML）读取 subject ID 列表。
func loadTrackerIDs(path string) []int {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	// JSON: {"subjects": [1,2,3]} 或 {"subjects":[{"subject_id":1}]}
	if strings.HasSuffix(path, ".json") {
		var list struct{ Subjects []int `json:"subjects"` }
		if json.Unmarshal(data, &list) == nil && len(list.Subjects) > 0 {
			return list.Subjects
		}
		var userList struct {
			Subjects []struct {
				SubjectID int `json:"subject_id"`
			} `json:"subjects"`
		}
		if json.Unmarshal(data, &userList) == nil {
			var ids []int
			for _, s := range userList.Subjects {
				ids = append(ids, s.SubjectID)
			}
			return ids
		}
		return nil
	}
	// TOML: ids = [1, 2, 3]
	var ids []int
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ids = [") {
			content := strings.TrimSuffix(strings.TrimPrefix(line, "ids = ["), "]")
			for _, s := range strings.Split(content, ",") {
				var id int
				fmt.Sscanf(strings.TrimSpace(s), "%d", &id)
				if id > 0 {
					ids = append(ids, id)
				}
			}
		}
	}
	return ids
}


// diffTrackerIDs returns IDs in trackers but not yet in subjects.json.
func diffTrackerIDs(cfg *config.Config, dd string) []int {
	seen := map[int]bool{}
	var allIDs []int
	files, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.json"))
	files2, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.toml"))
	for _, f := range append(files, files2...) {
		for _, sid := range loadTrackerIDs(f) {
			if !seen[sid] { seen[sid] = true; allIDs = append(allIDs, sid) }
		}
	}
	// Load existing subject IDs
	existing := map[int]bool{}
	if data, err := os.ReadFile(cache.IndexFile(dd, "subjects.json")); err == nil {
		var list []struct{ ID int `json:"id"` }
		if json.Unmarshal(data, &list) == nil {
			for _, s := range list { existing[s.ID] = true }
		}
	}
	var newIDs []int
	for _, sid := range allIDs {
		if !existing[sid] { newIDs = append(newIDs, sid) }
	}
	return newIDs
}
// fetchConcurrent 并发拉取，使用 semaphore 控制并发数。

func handleTrackerCreate(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Name string `json:"name"` }
		json.NewDecoder(r.Body).Decode(&req)
		if req.Name == "" { http.Error(w, `{"error":"name required"}`, 400); return }
		path := filepath.Join(cfg.TrackerDir(), req.Name+".toml")
		if _, err := os.Stat(path); err == nil { http.Error(w, `{"error":"tracker already exists"}`, 409); return }
		tmpl := fmt.Sprintf(config.TrackerTemplate, req.Name, req.Name)
		os.MkdirAll(cfg.TrackerDir(), 0o755)
		os.WriteFile(path, []byte(tmpl), 0o644)
		writeJSON(w, map[string]string{"status": "created", "name": req.Name})
	}
}

func handleTrackerList(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.json"))
		files2, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.toml"))
		files = append(files, files2...)
		type tinfo struct{ Name string `json:"name"`; Count int `json:"count"` }
		var list []tinfo
		for _, f := range files {
			name := strings.TrimSuffix(filepath.Base(f), ".json")
			name = strings.TrimSuffix(name, ".toml")
			list = append(list, tinfo{Name: name, Count: len(loadTrackerIDs(f))})
		}
		writeJSON(w, list)
	}
}

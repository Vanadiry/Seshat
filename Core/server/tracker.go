package server

import (
	"net/http"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	p.SetPhase(1, 5, "拉取动画数据")
	fetchSubjectList(allIDs, bg, dd, imgDir, p)
	log.Info("All trackers refreshed: %d subjects", len(seen))
	downloadImages(dd, bg, p, 2, 5)
	p.SetPhase(5, 5, "建立索引")
	buildIndexes(dd, p)
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
	if len(allIDs) == 0 {
		log.Warn("refreshTrackers: no IDs found for %v", names)
		return
	}
	p.SetPhase(1, 5, "拉取动画数据")
	fetchSubjectList(allIDs, bg, dd, imgDir, p)
	log.Info("Trackers %v refreshed: %d subjects", names, len(seen))
	downloadImages(dd, bg, p, 2, 5)
	p.SetPhase(5, 5, "建立索引")
	buildIndexes(dd, p)
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
		var userMap struct {
			Subjects map[string]int `json:"subjects"`
		}
		if json.Unmarshal(data, &userMap) == nil && len(userMap.Subjects) > 0 {
			var ids []int
			for k := range userMap.Subjects {
				id, _ := strconv.Atoi(k)
				if id > 0 { ids = append(ids, id) }
			}
			return ids
		}
		return nil
	}
	// TOML: ids = [1, 2, 3] 或 ids = [\n1,\n2\n]
	return parseTOMLIDArray(data)
}

// parseTOMLIDArray 解析 TOML 格式的 ids = [...] 数组，支持多行逗号分隔。
func parseTOMLIDArray(data []byte) []int {
	var ids []int
	inArray := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ids = [") {
			inArray = true
			rest := strings.TrimPrefix(trimmed, "ids = [")
			if idx := strings.Index(rest, "]"); idx >= 0 {
				rest = rest[:idx]
				inArray = false
			}
			parseIDList(rest, &ids)
			continue
		}
		if inArray {
			if idx := strings.Index(trimmed, "]"); idx >= 0 {
				rest := trimmed[:idx]
				inArray = false
				parseIDList(rest, &ids)
			} else {
				parseIDList(trimmed, &ids)
			}
		}
	}
	return ids
}

func parseIDList(s string, ids *[]int) {
	for _, part := range strings.Split(s, ",") {
		var id int
		fmt.Sscanf(strings.TrimSpace(part), "%d", &id)
		if id > 0 {
			*ids = append(*ids, id)
		}
	}
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


// validTrackerName checks if a tracker name is valid (alphanumeric, dash, underscore).
func validTrackerName(name string) bool {
	if name == "" { return false }
	for _, c := range name {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' { return false }
	}
	return true
}

func handleTrackerCreate(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Name string `json:"name"` }
		json.NewDecoder(r.Body).Decode(&req)
		if req.Name == "" || !validTrackerName(req.Name) {
			http.Error(w, `{"error":"tracker name must be alphanumeric, dash, or underscore"}`, 400)
			return
		}
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

func handleImportCollections(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join(config.Dir(), "user", "info", "collections.json"))
		if err != nil { http.Error(w, `{"error":"collections not found"}`, 400); return }
		var coll struct {
			Subjects map[string]int `json:"subjects"`
		}
		if json.Unmarshal(data, &coll) != nil { http.Error(w, `{"error":"invalid collections data"}`, 400); return }
		var ids []int
		for sid := range coll.Subjects {
			id, _ := strconv.Atoi(sid)
			if id > 0 { ids = append(ids, id) }
		}
		td := filepath.Join(config.Dir(), "tracker")
		os.MkdirAll(td, 0o755)
		trackerData, _ := json.Marshal(map[string]any{"name": "user", "subjects": ids})
		os.WriteFile(filepath.Join(td, "user.json"), trackerData, 0o644)
		writeJSON(w, map[string]any{"status": "ok", "count": len(ids)})
	}
}

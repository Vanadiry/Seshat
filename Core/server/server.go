// Package server 提供 HTTP 路由和 API 处理。
package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vanadiry/seshat/Core/bangumi"
	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/config"
	"github.com/vanadiry/seshat/Core/log"
)

func New(cfg *config.Config, embedFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	dd := cfg.DataDir()
	id := filepath.Join(dd, "images")
	bg := bangumi.NewClient("Seshat/Test", cfg.BaseURL)

	// ── Frontend ──
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, embedFS, "web/index.html")
	})

	// ── API 文档 ──
	mux.HandleFunc("GET /doc/api", serveFile(embedFS, "web/api/index.html", "text/html"))
	mux.HandleFunc("GET /api/v1/openapi.yaml", serveFile(embedFS, "web/api/openapi.yaml", "application/yaml"))

	// ── SSE progress ──
	mux.HandleFunc("GET /api/v1/progress/{id}", func(w http.ResponseWriter, r *http.Request) {
		p := getProgress(r.PathValue("id"))
		if p == nil {
			writeJSON(w, map[string]string{"error": "task not found"})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		for event := range p.Channel {
			fmt.Fprintf(w, "data: %s\n\n", event)
			flusher.Flush()
		}
	})

	// ── Cache API ──
	mux.HandleFunc("GET /api/v1/subjects", func(w http.ResponseWriter, r *http.Request) {
		keys, _ := cache.List(dd, "subjects")
		type item struct {
			ID       int     `json:"id"`
			Name     string  `json:"name"`
			NameCN   string  `json:"name_cn"`
			Score    float64 `json:"score"`
			Platform string  `json:"platform"`
			Date     string  `json:"date"`
			Image    string  `json:"image"`
		}
		var list []item
		for _, k := range keys {
			data, err := cache.Get(dd, "subjects/"+k+".json")
			if err != nil {
				continue
			}
			var s struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				NameCN   string `json:"name_cn"`
				Rating   struct {
					Score float64 `json:"score"`
				} `json:"rating"`
				Platform string `json:"platform"`
				Date     string `json:"date"`
				Images   struct {
					Grid string `json:"grid"`
				} `json:"images"`
			}
			json.Unmarshal(data, &s)
			list = append(list, item{
				ID: s.ID, Name: s.Name, NameCN: s.NameCN,
				Score: s.Rating.Score, Platform: s.Platform, Date: s.Date,
				Image: s.Images.Grid,
			})
		}
		writeJSON(w, list)
	})

	// Generic cache reader for /api/v1/SUBJECTS|CHARACTERS|PERSONS|EPISODES
	mux.HandleFunc("GET /api/v1/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/api/v1/") + ".json"
		data, err := cache.Get(dd, key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	// ── Fetch: 刷新全部 tracker（覆盖更新）──
	mux.HandleFunc("POST /api/v1/fetch", func(w http.ResponseWriter, r *http.Request) {
		p := newProgress(0)
		go func() {
			refreshAllTrackers(cfg, bg, dd, id)
			p.Send("complete", 1, 1, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "task_id": p.ID})
	})

	// ── Fetch: 深度重建（删除全部缓存后重新拉取）──
	mux.HandleFunc("POST /api/v1/fetch/deep", func(w http.ResponseWriter, r *http.Request) {
		p := newProgress(0)
		go func() {
			forceRefresh(cfg, bg, dd, id)
			p.Send("complete", 1, 1, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "deep refresh", "task_id": p.ID})
	})

	// ── Fetch: 按名称刷新指定 tracker ──
	mux.HandleFunc("POST /api/v1/fetch/tracker", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Names []string `json:"names"` }
		json.NewDecoder(r.Body).Decode(&req)
		p := newProgress(0)
		go func() {
			refreshTrackers(cfg, bg, dd, id, req.Names)
			p.Send("complete", 1, 1, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "names": req.Names, "task_id": p.ID})
	})

	// ── Fetch: 刷新用户收藏 ──
	mux.HandleFunc("POST /api/v1/fetch/user", func(w http.ResponseWriter, r *http.Request) {
		uname := cfg.Username
		if uname == "" {
			http.Error(w, `{"error":"username not configured"}`, 400)
			return
		}
		p := newProgress(0)
		go func() {
			fetchUserCollections(uname, bg, dd)
			refreshTrackers(cfg, bg, dd, id, []string{"user"})
			p.Send("complete", 1, 1, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "username": uname, "task_id": p.ID})
	})

	// ── Fetch: 接受单个或数组 ──
	mux.HandleFunc("POST /api/v1/fetch/subject", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID  int   `json:"id"`
			IDs []int `json:"ids"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		ids := req.IDs
		if req.ID != 0 {
			ids = []int{req.ID}
		}
		if len(ids) == 0 {
			http.Error(w, `{"error":"id or ids required"}`, 400)
			return
		}
		p := newProgress(len(ids))
		go func() {
			for _, sid := range ids {
				fetchAll(sid, bg, dd, id, p)
				addToSeshatTracker(cfg, sid)
			}
			rebuildTags(dd)
			rebuildIndexes(dd)
			p.Send("complete", len(ids), len(ids), "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "count": len(ids), "task_id": p.ID})
	})

	// ── Tracker 创建 ──
	mux.HandleFunc("POST /api/v1/tracker/create", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Name string `json:"name"` }
		json.NewDecoder(r.Body).Decode(&req)
		if req.Name == "" {
			http.Error(w, `{"error":"name required"}`, 400)
			return
		}
		path := filepath.Join(cfg.TrackerDir(), req.Name+".toml")
		if _, err := os.Stat(path); err == nil {
			http.Error(w, `{"error":"tracker already exists"}`, 409)
			return
		}
		tmpl := fmt.Sprintf(config.TrackerTemplate, req.Name, req.Name)
		os.MkdirAll(cfg.TrackerDir(), 0o755)
		os.WriteFile(path, []byte(tmpl), 0o644)
		writeJSON(w, map[string]string{"status": "created", "name": req.Name})
	})

	// ── Tracker 列表 ──
	mux.HandleFunc("GET /api/v1/tracker", func(w http.ResponseWriter, r *http.Request) {
		files, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.json"))
		files2, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.toml"))
		files = append(files, files2...)
		type tinfo struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}
		var list []tinfo
		for _, f := range files {
			name := strings.TrimSuffix(filepath.Base(f), ".json")
			name = strings.TrimSuffix(name, ".toml")
			ids := loadTrackerIDs(f)
			list = append(list, tinfo{Name: name, Count: len(ids)})
		}
		writeJSON(w, list)
	})

	// ── User profile ──
	mux.HandleFunc("GET /api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		uname := cfg.Username
		if uname == "" {
			http.Error(w, `{"error":"no username"}`, 404)
			return
		}
		data, err := cache.Get(dd, fmt.Sprintf("users/%s.json", uname))
		if err != nil {
			raw, err := bg.GetRaw(fmt.Sprintf("v0/users/%s", uname))
			if err != nil {
				http.Error(w, `{"error":"user not found"}`, 404)
				return
			}
			cache.Put(dd, fmt.Sprintf("users/%s.json", uname), raw)
			w.Header().Set("Content-Type", "application/json")
			w.Write(raw)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	// ── Tags ──
	mux.HandleFunc("GET /api/v1/tags", func(w http.ResponseWriter, r *http.Request) {
		tags := loadTags(dd)
		type tagItem struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}
		var list []tagItem
		for name, info := range tags {
			list = append(list, tagItem{Name: name, Count: info.Count})
		}
		// Sort by count desc
		sort.Slice(list, func(i, j int) bool { return list[i].Count > list[j].Count })
		writeJSON(w, list)
	})

	mux.HandleFunc("GET /api/v1/tags/{name}/subjects", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		tags := loadTags(dd)
		info, ok := tags[name]
		if !ok {
			writeJSON(w, []int{})
			return
		}
		// Return subject list with basic info
		var subjects []any
		for _, sid := range info.Subjects {
			data, err := cache.Get(dd, fmt.Sprintf("subjects/%d.json", sid))
			if err != nil {
				continue
			}
			var s struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				NameCN   string `json:"name_cn"`
				Rating   struct {
					Score float64 `json:"score"`
				} `json:"rating"`
				Platform string `json:"platform"`
				Date     string `json:"date"`
				Images   struct {
					Grid string `json:"grid"`
				} `json:"images"`
			}
			json.Unmarshal(data, &s)
			subjects = append(subjects, s)
		}
		writeJSON(w, subjects)
	})

	// ── Search ──
	mux.HandleFunc("GET /api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		q := strings.ToLower(r.URL.Query().Get("q"))
		types := r.URL.Query().Get("type") // subjects,characters,persons,tags (comma-separated)
		if types == "" {
			types = "subjects,characters,persons,tags"
		}
		if q == "" {
			writeJSON(w, map[string]any{"subjects": []any{}, "characters": []any{}, "persons": []any{}, "tags": []any{}})
			return
		}
		result := quickSearch(dd, q, strings.Split(types, ","))
		writeJSON(w, result)
	})

	mux.HandleFunc("POST /api/v1/search/deep", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query string `json:"q"`
			Type  string `json:"type"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Query == "" {
			writeJSON(w, map[string]any{"results": []any{}})
			return
		}
		if req.Type == "" {
			req.Type = "subjects,characters,persons"
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSON(w, map[string]string{"error": "streaming not supported"})
			return
		}

		log.Info("Deep search: %q (types: %s)", req.Query, req.Type)
		count := deepSearchStream(dd, req.Query, strings.Split(req.Type, ","), func(result map[string]any) {
			data, _ := json.Marshal(result)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		})
		fmt.Fprintf(w, "data: {\"step\":\"complete\",\"total\":%d}\n\n", count)
		flusher.Flush()
		log.Info("Deep search done: %d results", count)
	})

	// ── Images ──
	mux.Handle("GET /images/", http.StripPrefix("/images/", http.FileServer(http.Dir(id))))

	return withLogging(withCORS(mux))
}

// fetchAll 拉取动画的所有数据及关联角色/人物/图片。
func fetchAll(sid int, bg *bangumi.Client, dd, imgDir string, p *Progress) {
	log.Info("Fetching subject #%d", sid)

	// Estimate: 1 subject + chars list + persons list + each char detail + each person detail + eps + rels
	stage := "subject"
	p.Send(stage, 0, 1, "fetching")

	// Subject
	if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d", sid)); err == nil {
		cache.Put(dd, "subjects/"+fmt.Sprintf("%d", sid)+".json", cache.ReplaceImageURLs(data))
		cache.ProcessImages(data, imgDir)
	}
	p.Send(stage, 1, 1, "done")

	// Subject
	if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d", sid)); err == nil {
		cache.Put(dd, "subjects/"+fmt.Sprintf("%d", sid)+".json", cache.ReplaceImageURLs(data))
		cache.ProcessImages(data, imgDir)
	}

	// Characters list + individual details
	if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/characters", sid)); err == nil {
		cache.Put(dd, fmt.Sprintf("subjects/%d/characters.json", sid), cache.ReplaceImageURLs(data))
		cache.ProcessImages(data, imgDir)
		// Fetch each character's detail page
		var chars []struct{ ID int `json:"id"` }
		if json.Unmarshal(data, &chars) == nil {
			if p != nil { p.Send("characters", 0, len(chars), "fetching") }
			for i, c := range chars {
				if cdata, err := bg.GetRaw(fmt.Sprintf("v0/characters/%d", c.ID)); err == nil {
					cache.Put(dd, fmt.Sprintf("characters/%d.json", c.ID), cache.ReplaceImageURLs(cdata))
					cache.ProcessImages(cdata, imgDir)
				}
				if p != nil { p.Send("characters", i+1, len(chars), "") }
			}
		}
	}

	// Persons list + individual details
	if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/persons", sid)); err == nil {
		cache.Put(dd, fmt.Sprintf("subjects/%d/persons.json", sid), cache.ReplaceImageURLs(data))
		cache.ProcessImages(data, imgDir)
		var persons []struct{ ID int `json:"id"` }
		if json.Unmarshal(data, &persons) == nil {
			if p != nil { p.Send("persons", 0, len(persons), "fetching") }
			for i, pp := range persons {
				if pdata, err := bg.GetRaw(fmt.Sprintf("v0/persons/%d", pp.ID)); err == nil {
					cache.Put(dd, fmt.Sprintf("persons/%d.json", p.ID), cache.ReplaceImageURLs(pdata))
					cache.ProcessImages(pdata, imgDir)
				}
				if p != nil { p.Send("persons", i+1, len(persons), "") }
			}
		}
	}

	// Episodes
	if data, err := bg.GetRaw(fmt.Sprintf("v0/episodes?subject_id=%d&limit=200", sid)); err == nil {
		cache.Put(dd, fmt.Sprintf("subjects/%d/episodes.json", sid), cache.ReplaceImageURLs(data))
	}

	// Relations
	if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/subjects", sid)); err == nil {
		cache.Put(dd, fmt.Sprintf("subjects/%d/relations.json", sid), cache.ReplaceImageURLs(data))
		cache.ProcessImages(data, imgDir)
	}

	log.Info("Subject #%d fetch done", sid)
}

// ── Search ──

// quickSearch 使用索引文件快速搜索。
func quickSearch(dd, q string, types []string) map[string]any {
	result := map[string]any{}
	for _, t := range types {
		switch t {
		case "subjects":
			result["subjects"] = searchIndex(dd, "subjects_index.json", q)
		case "characters":
			result["characters"] = searchIndex(dd, "characters_index.json", q)
		case "persons":
			result["persons"] = searchIndex(dd, "persons_index.json", q)
		case "tags":
			result["tags"] = searchTags(dd, q)
		}
	}
	return result
}

func searchIndex(dd, file, q string) []map[string]any {
	data, err := os.ReadFile(filepath.Join(dd, file))
	if err != nil {
		return []map[string]any{}
	}
	var index map[string][]any // name → [id, displayName]
	json.Unmarshal(data, &index)

	var results []map[string]any
	for name, entry := range index {
		if strings.Contains(strings.ToLower(name), q) {
			results = append(results, map[string]any{
				"name": name,
				"id":   entry[0],
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

// rebuildIndexes 重建所有搜索索引。
func rebuildIndexes(dd string) {
	apiDir := cache.Dir(dd)
	subjects := map[string][]any{}
	characters := map[string][]any{}
	persons := map[string][]any{}

	// Scan subjects
	dir := filepath.Join(apiDir, "subjects")
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") || strings.Contains(e.Name(), "/") {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
			var s struct {
				ID     int    `json:"id"`
				Name   string `json:"name"`
				NameCN string `json:"name_cn"`
			}
			if json.Unmarshal(data, &s) == nil && s.ID > 0 {
				display := s.NameCN
				if display == "" {
					display = s.Name
				}
				subjects[s.Name] = []any{s.ID, display}
				if s.NameCN != "" && s.NameCN != s.Name {
					subjects[s.NameCN] = []any{s.ID, display}
				}
			}
		}
	}

	// Scan characters
	dir = filepath.Join(apiDir, "characters")
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
			var c struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}
			if json.Unmarshal(data, &c) == nil && c.ID > 0 {
				characters[c.Name] = []any{c.ID, c.Name}
			}
		}
	}

	// Scan persons
	dir = filepath.Join(apiDir, "persons")
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
			var p struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}
			if json.Unmarshal(data, &p) == nil && p.ID > 0 {
				persons[p.Name] = []any{p.ID, p.Name}
			}
		}
	}

	saveJSON(filepath.Join(dd, "subjects_index.json"), subjects)
	saveJSON(filepath.Join(dd, "characters_index.json"), characters)
	saveJSON(filepath.Join(dd, "persons_index.json"), persons)
	log.Info("Search indexes rebuilt: %d subjects, %d chars, %d persons",
		len(subjects), len(characters), len(persons))
}

func saveJSON(path string, v any) {
	data, _ := json.Marshal(v)
	os.WriteFile(path, data, 0o644)
}

// ── Tags index ──

type tagInfo struct {
	Count    int   `json:"count"`
	Subjects []int `json:"subjects"`
}

func tagsPath(dd string) string {
	return filepath.Join(dd, "tags.json")
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
func fetchUserCollections(username string, bg *bangumi.Client, dd string) {
	log.Info("Fetching collections for %s", username)

	type item struct {
		SubjectID int `json:"subject_id"`
		Type      int `json:"type"`
	}
	var all []item

	offset := 0
	for {
		data, err := bg.GetRawPaged(fmt.Sprintf("v0/users/%s/collections", username), offset, 50)
		if err != nil {
			log.Warn("user collections offset %d: %v", offset, err)
			break
		}
		// Parse just the data array
		var resp struct {
			Data []struct {
				SubjectID   int `json:"subject_id"`
				SubjectType int `json:"subject_type"`
				Type        int `json:"type"`
			} `json:"data"`
			Total int `json:"total"`
		}
		json.Unmarshal(data, &resp)
		for _, d := range resp.Data {
			if d.SubjectType == 2 {
				all = append(all, item{SubjectID: d.SubjectID, Type: d.Type})
			}
		}
		if offset+50 >= resp.Total {
			break
		}
		offset += 50
	}

	// Save to tracker with underscore prefix
	path := filepath.Join(config.Dir(), "tracker", fmt.Sprintf("_user-%s.json", username))
	os.MkdirAll(filepath.Dir(path), 0o755)
	result, _ := json.MarshalIndent(map[string]any{
		"auto":       true,
		"username":   username,
		"updated_at": time.Now().Format(time.RFC3339),
		"subjects":   all,
	}, "", "  ")
	os.WriteFile(path, result, 0o644)
	log.Info("User collections for %s saved (%d items)", username, len(all))
}

// forceRefresh 删除所有缓存数据后完整重建。
func forceRefresh(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) {
	log.Info("Force refresh: clearing all cached data...")
	os.RemoveAll(filepath.Join(dd, "api"))
	os.RemoveAll(filepath.Join(dd, "images"))
	os.Remove(filepath.Join(dd, "tags.json"))
	log.Info("Force refresh: cache cleared, starting rebuild")
	refreshAllTrackers(cfg, bg, dd, imgDir)
}

// refreshAllTrackers 刷新 tracker 文件夹中的所有列表。
func refreshAllTrackers(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) {
	files, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.json"))
	files2, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.toml"))
	files = append(files, files2...)
	seen := map[int]bool{}
	for _, f := range files {
		for _, sid := range loadTrackerIDs(f) {
			if !seen[sid] {
				seen[sid] = true
				fetchAll(sid, bg, dd, imgDir, nil)
			}
		}
	}
	log.Info("All trackers refreshed: %d subjects", len(seen))
	rebuildTags(dd)
	rebuildIndexes(dd)
}

// refreshTrackers 刷新指定的 tracker 列表。
func refreshTrackers(cfg *config.Config, bg *bangumi.Client, dd, imgDir string, names []string) {
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
			if !seen[sid] {
				seen[sid] = true
				fetchAll(sid, bg, dd, imgDir, nil)
			}
		}
	}
	log.Info("Trackers %v refreshed: %d subjects", names, len(seen))
	rebuildTags(dd)
	rebuildIndexes(dd)
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

// ── Helpers ──

func serveFile(fsys fs.FS, path, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) { r.status = code; r.ResponseWriter.WriteHeader(code) }
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(sr, r)
		log.HTTP(r.Method, r.URL.Path, sr.status, time.Since(start))
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

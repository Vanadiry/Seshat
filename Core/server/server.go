// Package server 提供 HTTP 路由和 API 处理。
package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"strconv"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vanadiry/seshat/Core/bangumi"
	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/config"
	"github.com/vanadiry/seshat/Core/log"
)

var maxConcurrency = 32

func New(cfg *config.Config, embedFS fs.FS) http.Handler {
	maxConcurrency = cfg.Concurrency
	mux := http.NewServeMux()
	dd := cfg.DataDir()
	id := filepath.Join(dd, "images")
	bg := bangumi.NewClient("HyperGraph/APIRRRRRR", cfg.BaseURL)

	// ── Frontend ──
	mux.HandleFunc("GET /subject.html", serveFile(embedFS, "web/subject.html", "text/html"))
	mux.HandleFunc("GET /character.html", serveFile(embedFS, "web/character.html", "text/html"))
	mux.HandleFunc("GET /person.html", serveFile(embedFS, "web/person.html", "text/html"))
	mux.HandleFunc("GET /character-list.html", serveFile(embedFS, "web/character-list.html", "text/html"))
	mux.HandleFunc("GET /person-list.html", serveFile(embedFS, "web/person-list.html", "text/html"))
	mux.HandleFunc("GET /tags.html", serveFile(embedFS, "web/tags.html", "text/html"))
	mux.HandleFunc("GET /tags-subject.html", serveFile(embedFS, "web/tags-subject.html", "text/html"))
	mux.HandleFunc("GET /search.html", serveFile(embedFS, "web/search.html", "text/html"))
	mux.HandleFunc("GET /assets/app.js", serveFile(embedFS, "web/assets/app.js", "application/javascript"))
	mux.HandleFunc("GET /assets/app.css", serveFile(embedFS, "web/assets/app.css", "text/css"))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if embedFS == nil {
			http.Error(w, "frontend not embedded", http.StatusInternalServerError)
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
		data, err := os.ReadFile(cache.IndexFile(dd, "subjects_list.json"))
		if err != nil {
			writeJSON(w, []any{})
			return
		}
		var list []cache.SubjectSummary
		json.Unmarshal(data, &list)
		writeJSON(w, list)
	})

	mux.HandleFunc("GET /api/v1/characters", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(cache.IndexFile(dd, "characters_list.json"))
		if err != nil {
			writeJSON(w, []any{})
			return
		}
		var list []cache.NameEntry
		json.Unmarshal(data, &list)
		writeJSON(w, list)
	})

	mux.HandleFunc("GET /api/v1/persons", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(cache.IndexFile(dd, "persons_list.json"))
		if err != nil {
			writeJSON(w, []any{})
			return
		}
		var list []cache.NameEntry
		json.Unmarshal(data, &list)
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
		p := newProgress(countTrackerTotal(cfg))
		go func() {
			refreshAllTrackers(cfg, bg, dd, id, p)
			p.Send("complete", p.Total, p.Total, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "task_id": p.ID})
	})

	// ── Fetch: 深度重建（删除全部缓存后重新拉取）──
	mux.HandleFunc("POST /api/v1/fetch/deep", func(w http.ResponseWriter, r *http.Request) {
		p := newProgress(countTrackerTotal(cfg))
		go func() {
			forceRefresh(cfg, bg, dd, id, p)
			p.Send("complete", p.Total, p.Total, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "deep refresh", "task_id": p.ID})
	})

	// ── Fetch: 按名称刷新指定 tracker ──
	mux.HandleFunc("POST /api/v1/fetch/tracker", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Names []string `json:"names"` }
		json.NewDecoder(r.Body).Decode(&req)
		p := newProgress(countTrackerNames(cfg, req.Names))
		go func() {
			refreshTrackers(cfg, bg, dd, id, req.Names, p)
			p.Send("complete", p.Total, p.Total, "")
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
			refreshTrackers(cfg, bg, dd, id, []string{"user"}, p)
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
		for _, sid := range ids {
			addToSeshatTracker(cfg, sid)
		}
		go func() {
			fetchSubjectList(ids, bg, dd, id, p)
			p.Send("phase", 2, 3, "building indexes")
			buildIndexes(dd, p)
			p.Send("phase", 3, 3, "downloading images")
			downloadImages(dd, bg, p)
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

	// ── ELO ──
	mux.HandleFunc("GET /api/v1/elo/pair", func(w http.ResponseWriter, r *http.Request) {
		pair := getELOPair(dd)
		if pair == nil {
			writeJSON(w, map[string]string{"error": "need at least 2 cached subjects"})
			return
		}
		writeJSON(w, pair)
	})

	mux.HandleFunc("POST /api/v1/elo/compare", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Winner int `json:"winner"`
			Loser  int `json:"loser"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Winner == 0 || req.Loser == 0 {
			http.Error(w, `{"error":"winner and loser required"}`, 400)
			return
		}
		updateELO(dd, req.Winner, req.Loser)
		writeJSON(w, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/v1/elo/ranking", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, getELORanking(dd))
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
	// ── Image API (official Bangumi-style endpoints) ──
	mux.HandleFunc("GET /images/subject/{id}", func(w http.ResponseWriter, r *http.Request) {
		serveImage(w, r, dd, "subject", r.URL.Query().Get("type"))
	})
	mux.HandleFunc("GET /images/character/{id}", func(w http.ResponseWriter, r *http.Request) {
		serveImage(w, r, dd, "character", r.URL.Query().Get("type"))
	})
	mux.HandleFunc("GET /images/person/{id}", func(w http.ResponseWriter, r *http.Request) {
		serveImage(w, r, dd, "person", r.URL.Query().Get("type"))
	})
	// Legacy image path
	mux.Handle("GET /images/", http.StripPrefix("/images/", http.FileServer(http.Dir(id))))

	return withLogging(withCORS(mux))
}

// fetchSubjectList 并发拉取多个动画的所有数据。
func fetchSubjectList(ids []int, bg *bangumi.Client, dd, imgDir string, p *Progress) {
	if len(ids) == 0 {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency)
	var done int
	var mu sync.Mutex
	for _, sid := range ids {
		wg.Add(1)
		go func(sid int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			fetchAll(sid, bg, dd, imgDir, p)
			if p != nil {
				mu.Lock()
				done++
				p.Send("subjects", done, len(ids), "")
				mu.Unlock()
			}
		}(sid)
	}
	wg.Wait()
}

// fetchAll 拉取动画的所有数据及关联角色/人物/图片。
// Phase 1: Fetch API JSON data (strip images).
func fetchAll(sid int, bg *bangumi.Client, dd, imgDir string, p *Progress) {
	log.Info("Fetching subject #%d", sid)

	// Subject (404 is reported for user-specified subjects)
	if p != nil { p.Send("subject", 0, 1, "fetching") }
	data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d", sid))
	if err != nil {
		log.Warn("Subject #%d: %v", sid, err)
		if p != nil { p.Send("subject", 1, 1, "error") }
		return
	}
	cache.Put(dd, fmt.Sprintf("subjects/%d.json", sid), cache.StripImages(data))
	if p != nil { p.Send("subject", 1, 1, "done") }

	// Characters list + details
	type charRef struct{ ID int `json:"id"` }
	type personRef struct{ ID int `json:"id"` }
	var chars []charRef
	var persons []personRef
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/characters", sid)); err == nil {
			cache.Put(dd, fmt.Sprintf("subjects/%d/characters.json", sid), cache.StripImages(data))
			json.Unmarshal(data, &chars)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/persons", sid)); err == nil {
			cache.Put(dd, fmt.Sprintf("subjects/%d/persons.json", sid), cache.StripImages(data))
			json.Unmarshal(data, &persons)
		}
	}()
	wg.Wait()

	var wg2 sync.WaitGroup
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		fetchConcurrent(chars, func(c charRef) {
			if cdata, err := bg.GetRaw(fmt.Sprintf("v0/characters/%d", c.ID)); err == nil {
				cache.Put(dd, fmt.Sprintf("characters/%d.json", c.ID), cache.StripImages(cdata))
			}
		}, p, "characters", maxConcurrency)
	}()
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		fetchConcurrent(persons, func(pp personRef) {
			if pdata, err := bg.GetRaw(fmt.Sprintf("v0/persons/%d", pp.ID)); err == nil {
				cache.Put(dd, fmt.Sprintf("persons/%d.json", pp.ID), cache.StripImages(pdata))
			}
		}, p, "persons", maxConcurrency)
	}()
	wg2.Wait()

	// Episodes
	if data, err := bg.GetRaw(fmt.Sprintf("v0/episodes?subject_id=%d&limit=200", sid)); err == nil {
		cache.Put(dd, fmt.Sprintf("subjects/%d/episodes.json", sid), cache.StripImages(data))
	}

	// Relations
	if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/subjects", sid)); err == nil {
		cache.Put(dd, fmt.Sprintf("subjects/%d/relations.json", sid), cache.StripImages(data))
	}

	log.Info("Subject #%d API fetch done", sid)
}

// Phase 2: Build index files from all cached API data.
func buildIndexes(dd string, p *Progress) {
	log.Info("Building indexes...")
	os.MkdirAll(cache.IndexDir(dd), 0o755)

	var subjects []cache.SubjectSummary
	var chars []cache.NameEntry
	var persons []cache.NameEntry
	subjIndex := map[string][]any{}
	charIndex := map[string][]any{}
	persIndex := map[string][]any{}
	tags := map[string]tagInfo{}

	apiDir := cache.Dir(dd)

	// Scan subjects
	subjDir := filepath.Join(apiDir, "subjects")
	if entries, _ := os.ReadDir(subjDir); len(entries) > 0 {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") || strings.Contains(e.Name(), "/") {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(subjDir, e.Name()))
			var s struct {
				ID       int     `json:"id"`
				Name     string  `json:"name"`
				NameCN   string  `json:"name_cn"`
				Rating   struct{ Score float64 `json:"score"` } `json:"rating"`
				Platform string  `json:"platform"`
				Date     string  `json:"date"`
				Tags     []struct{
					Name  string `json:"name"`
					Count int    `json:"count"`
				} `json:"tags"`
			}
			if json.Unmarshal(data, &s) == nil && s.ID > 0 {
				subjects = append(subjects, cache.SubjectSummary{
					ID: s.ID, Name: s.Name, NameCN: s.NameCN,
					Score: s.Rating.Score, Platform: s.Platform, Date: s.Date,
				})
				subjIndex[s.Name] = []any{s.ID, s.NameCN}
				if s.NameCN != "" && s.NameCN != s.Name {
					subjIndex[s.NameCN] = []any{s.ID, s.Name}
				}
				for _, t := range s.Tags {
					info := tags[t.Name]
					info.Count++
					info.Subjects = append(info.Subjects, s.ID)
					tags[t.Name] = info
				}
			}
		}
	}

	// Scan characters
	charDir := filepath.Join(apiDir, "characters")
	if entries, _ := os.ReadDir(charDir); len(entries) > 0 {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") { continue }
			data, _ := os.ReadFile(filepath.Join(charDir, e.Name()))
			var c struct{ ID int `json:"id"`; Name string `json:"name"` }
			if json.Unmarshal(data, &c) == nil && c.ID > 0 {
				chars = append(chars, cache.NameEntry{ID: c.ID, Name: c.Name})
				charIndex[c.Name] = []any{c.ID, c.Name}
			}
		}
	}

	// Scan persons
	persDir := filepath.Join(apiDir, "persons")
	if entries, _ := os.ReadDir(persDir); len(entries) > 0 {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") { continue }
			data, _ := os.ReadFile(filepath.Join(persDir, e.Name()))
			var p struct{ ID int `json:"id"`; Name string `json:"name"` }
			if json.Unmarshal(data, &p) == nil && p.ID > 0 {
				persons = append(persons, cache.NameEntry{ID: p.ID, Name: p.Name})
				persIndex[p.Name] = []any{p.ID, p.Name}
			}
		}
	}

	saveJSON(cache.IndexFile(dd, "subjects_list.json"), subjects)
	saveJSON(cache.IndexFile(dd, "characters_list.json"), chars)
	saveJSON(cache.IndexFile(dd, "persons_list.json"), persons)
	saveJSON(cache.IndexFile(dd, "subjects_index.json"), subjIndex)
	saveJSON(cache.IndexFile(dd, "characters_index.json"), charIndex)
	saveJSON(cache.IndexFile(dd, "persons_index.json"), persIndex)
	saveJSON(filepath.Join(dd, "tags.json"), tags)

	log.Info("Indexes built: %d subjects, %d chars, %d persons, %d tags",
		len(subjects), len(chars), len(persons), len(tags))
}

// Phase 3: Download images via official endpoints.
func downloadImages(dd string, bg *bangumi.Client, p *Progress) {
	log.Info("Downloading images...")
	os.MkdirAll(cache.IndexDir(dd), 0o755)

	subjImg := loadImageIndex(dd, "subjects_image.json")
	charImg := loadImageIndex(dd, "characters_image.json")
	persImg := loadImageIndex(dd, "persons_image.json")
	imgBase := filepath.Join(dd, "images")

	// Subjects
	subjList := loadNameList(cache.IndexFile(dd, "subjects_list.json"))
	if p != nil { p.Send("images_subjects", 0, len(subjList), "downloading") }
	dlImageList(subjList, "subject", subjImg, imgBase, bg, dd, p, "images_subjects")
	saveJSON(cache.IndexFile(dd, "subjects_image.json"), subjImg)

	// Characters
	charList := loadNameList(cache.IndexFile(dd, "characters_list.json"))
	if p != nil { p.Send("images_characters", 0, len(charList), "downloading") }
	dlImageList(charList, "character", charImg, imgBase, bg, dd, p, "images_characters")
	saveJSON(cache.IndexFile(dd, "characters_image.json"), charImg)

	// Persons
	persList := loadNameList(cache.IndexFile(dd, "persons_list.json"))
	if p != nil { p.Send("images_persons", 0, len(persList), "downloading") }
	dlImageList(persList, "person", persImg, imgBase, bg, dd, p, "images_persons")
	saveJSON(cache.IndexFile(dd, "persons_image.json"), persImg)

	log.Info("Images download complete")
}

func dlImageList(list []cache.NameEntry, kind string, imgMap map[int]cache.ImageEntry, imgBase string, bg *bangumi.Client, dd string, p *Progress, stage string) {
	if len(list) == 0 {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency)
	var done int
	var mu sync.Mutex
	for _, entry := range list {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			dlImage(bg, kind, id, imgMap, imgBase, &mu)
			if p != nil {
				mu.Lock()
				done++
				if done%10 == 0 || done == len(list) {
					p.Send(stage, done, len(list), "")
				}
				mu.Unlock()
			}
		}(entry.ID)
	}
	wg.Wait()
}

func dlImage(bg *bangumi.Client, kind string, id int, imgMap map[int]cache.ImageEntry, imgBase string, mu *sync.Mutex) {
	var entry cache.ImageEntry
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		data, err := bg.GetImage(fmt.Sprintf("v0/%ss/%d/image?type=large", kind, id))
		if err != nil {
			return
		}
		relPath := fmt.Sprintf("%s_large/%d/%d.jpg", kind, id%10, id)
		fullPath := filepath.Join(imgBase, relPath)
		os.MkdirAll(filepath.Dir(fullPath), 0o755)
		os.WriteFile(fullPath, data, 0o644)
		entry.Large = relPath
	}()
	go func() {
		defer wg.Done()
		data, err := bg.GetImage(fmt.Sprintf("v0/%ss/%d/image?type=grid", kind, id))
		if err != nil {
			return
		}
		relPath := fmt.Sprintf("%s_grid/%d/%d.jpg", kind, id%10, id)
		fullPath := filepath.Join(imgBase, relPath)
		os.MkdirAll(filepath.Dir(fullPath), 0o755)
		os.WriteFile(fullPath, data, 0o644)
		entry.Grid = relPath
	}()
	wg.Wait()
	if entry.Large != "" || entry.Grid != "" {
		mu.Lock()
		imgMap[id] = entry
		mu.Unlock()
	}
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
	data, err := os.ReadFile(cache.IndexFile(dd, file))
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

// countTrackerTotal 统计所有 tracker 中的 subject 总数。
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
	os.RemoveAll(filepath.Join(dd, "api"))
	os.RemoveAll(filepath.Join(dd, "images"))
	os.Remove(filepath.Join(dd, "tags.json"))
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
	buildIndexes(dd, nil)
	downloadImages(dd, bg, nil)
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
	buildIndexes(dd, nil)
	downloadImages(dd, bg, nil)
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

// fetchConcurrent 并发拉取，使用 semaphore 控制并发数。
func fetchConcurrent[T any](items []T, fn func(T), p *Progress, stage string, concurrency int) {
	if len(items) == 0 {
		return
	}
	if concurrency < 1 {
		concurrency = 32
	}
	if p != nil {
		p.Send(stage, 0, len(items), "fetching")
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var done int
	var mu sync.Mutex
	for _, item := range items {
		wg.Add(1)
		go func(item T) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			fn(item)
			if p != nil {
				mu.Lock()
				done++
				p.Send(stage, done, len(items), "")
				mu.Unlock()
			}
		}(item)
	}
	wg.Wait()
}

// ── Helpers ──

func serveFile(fsys fs.FS, path, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if fsys == nil {
			http.Error(w, "not available", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}
}


func loadImageIndex(dd, name string) map[int]cache.ImageEntry {
	data, err := os.ReadFile(cache.IndexFile(dd, name))
	if err != nil {
		return map[int]cache.ImageEntry{}
	}
	var m map[int]cache.ImageEntry
	json.Unmarshal(data, &m)
	if m == nil { m = map[int]cache.ImageEntry{} }
	return m
}

func loadNameList(path string) []cache.NameEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var list []cache.NameEntry
	json.Unmarshal(data, &list)
	return list
}

// (saveJSON already declared above)

func serveImage(w http.ResponseWriter, r *http.Request, dd, kind, size string) {
	if size == "" { size = "grid" }
	idStr := r.PathValue("id")
	imgFile := cache.IndexFile(dd, kind+"s_image.json")
	data, err := os.ReadFile(imgFile)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var images map[int]cache.ImageEntry
	json.Unmarshal(data, &images)
	id, _ := strconv.Atoi(idStr)
	entry, ok := images[id]
	if !ok {
		http.NotFound(w, r)
		return
	}
	path := entry.Large
	if size == "grid" {
		path = entry.Grid
	}
	if path == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(dd, "images", path))
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

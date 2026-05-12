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
	"sync"
	"time"

	"github.com/vanadiry/seshat/Core/bangumi"
	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/config"
	"github.com/vanadiry/seshat/Core/log"
)

var maxConcurrency = 32

// listMutex 保护 list 文件的并发读写。
var listMutex sync.Mutex

// noImageData 存放 no-image.png 的内容，用于在下载时识别占位图。
var noImageData []byte
var noImagePath string

// mergeListEntry 将一个条目合并到 list 文件中（按 ID 去重，若已存在则更新 name）。
func New(cfg *config.Config, embedFS fs.FS) http.Handler {
	maxConcurrency = cfg.Server.Concurrency
	mux := http.NewServeMux()
	dd := cfg.DataDir()
	os.MkdirAll(cache.IndexDir(dd), 0o755)
	imgDir := filepath.Join(dd, "images")
	os.MkdirAll(imgDir, 0o755)
	noImagePath = filepath.Join(imgDir, "no-image.png")

	// 从 embed 加载 no-image.png，写入 images 目录，并缓存内容用于后续比对
	if embedFS != nil {
		if b, err := fs.ReadFile(embedFS, "web/assets/no-image.png"); err == nil {
			noImageData = b
			os.WriteFile(noImagePath, b, 0o644)
		}
	}

	id := imgDir
	bg := bangumi.NewClient(cfg.Upstream.UserAgent, cfg.Upstream.BaseURL)

	// ── Frontend ──
	mux.HandleFunc("GET /subject.html", serveFile(embedFS, "web/subject.html", "text/html"))
	mux.HandleFunc("GET /character.html", serveFile(embedFS, "web/character.html", "text/html"))
	mux.HandleFunc("GET /person.html", serveFile(embedFS, "web/person.html", "text/html"))
	mux.HandleFunc("GET /character-list.html", serveFile(embedFS, "web/character-list.html", "text/html"))
	mux.HandleFunc("GET /person-list.html", serveFile(embedFS, "web/person-list.html", "text/html"))
	mux.HandleFunc("GET /tags.html", serveFile(embedFS, "web/tags.html", "text/html"))
	mux.HandleFunc("GET /tags-subject.html", serveFile(embedFS, "web/tags-subject.html", "text/html"))
	mux.HandleFunc("GET /search.html", serveFile(embedFS, "web/search.html", "text/html"))
	mux.HandleFunc("GET /assets/app.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprintf(w, "window.BACKEND_URL=%q;\n", cfg.Frontend.BackendURL)
		data, _ := fs.ReadFile(embedFS, "web/assets/app.js")
		w.Write(data)
	})
	mux.HandleFunc("GET /assets/app.css", serveFile(embedFS, "web/assets/app.css", "text/css"))
	mux.HandleFunc("GET /assets/tailwindcss-3.4.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFileFS(w, r, embedFS, "web/assets/tailwindcss-3.4.js")
	})
	mux.HandleFunc("GET /assets/scalar-api-reference.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFileFS(w, r, embedFS, "web/assets/scalar-api-reference.js")
	})
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
	mux.HandleFunc("GET /api/v0/openapi.yaml", serveFile(embedFS, "web/api/openapi.yaml", "application/yaml"))

	// ── SSE progress ──
	mux.HandleFunc("GET /api/v0/progress/{id}", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("GET /api/v0/subjects", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(cache.IndexFile(dd, "subjects.json"))
		if err != nil {
			writeJSON(w, []any{})
			return
		}
		var list []cache.SubjectSummary
		json.Unmarshal(data, &list)
		writeJSON(w, list)
	})

	mux.HandleFunc("GET /api/v0/characters", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(cache.IndexFile(dd, "characters.json"))
		if err != nil {
			writeJSON(w, []any{})
			return
		}
		var list []cache.NameEntry
		json.Unmarshal(data, &list)
		writeJSON(w, list)
	})

	mux.HandleFunc("GET /api/v0/persons", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(cache.IndexFile(dd, "persons.json"))
		if err != nil {
			writeJSON(w, []any{})
			return
		}
		var list []cache.NameEntry
		json.Unmarshal(data, &list)
		writeJSON(w, list)
	})

	// Generic cache reader for /v0/SUBJECTS|CHARACTERS|PERSONS|EPISODES
	mux.HandleFunc("GET /api/v0/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/api/v0/") + ".json"
		data, err := cache.Get(dd, key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	// ── Fetch: 刷新全部 tracker（覆盖更新）──
	mux.HandleFunc("POST /api/v0/fetch", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
		p := newProgress(countTrackerTotal(cfg))
		go func() {
			refreshAllTrackers(cfg, bg, dd, id, p)
			p.Send("complete", p.Total, p.Total, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "task_id": p.ID})
	})

	// ── Fetch: 深度重建（删除全部缓存后重新拉取）──
	mux.HandleFunc("POST /api/v0/fetch/deep", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
		p := newProgress(countTrackerTotal(cfg))
		go func() {
			forceRefresh(cfg, bg, dd, id, p)
			p.Send("complete", p.Total, p.Total, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "deep refresh", "task_id": p.ID})
	})

	// ── Fetch: 按名称刷新指定 tracker ──
	mux.HandleFunc("POST /api/v0/fetch/tracker", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
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
	mux.HandleFunc("POST /api/v0/fetch/user", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
		uname := cfg.User.Username
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
	mux.HandleFunc("POST /api/v0/fetch/subject", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
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
	mux.HandleFunc("POST /api/v0/tracker/create", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("GET /api/v0/tracker", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("GET /api/v0/user/profile", func(w http.ResponseWriter, r *http.Request) {
		uname := cfg.User.Username
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
	mux.HandleFunc("GET /api/v0/tags", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("GET /api/v0/tags/{name}/subjects", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("GET /api/v0/elo/pair", func(w http.ResponseWriter, r *http.Request) {
		pair := getELOPair(dd)
		if pair == nil {
			writeJSON(w, map[string]string{"error": "need at least 2 cached subjects"})
			return
		}
		writeJSON(w, pair)
	})

	mux.HandleFunc("POST /api/v0/elo/compare", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("GET /api/v0/elo/ranking", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, getELORanking(dd))
	})

	// ── Search ──
	mux.HandleFunc("GET /api/v0/search", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("POST /api/v0/search/deep", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
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

	// ── Image API (official-style: /v0/subjects/{id}/image?type=large|grid) ──
	imgHandler := func(kind string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			serveImage(w, r, dd, kind, r.URL.Query().Get("type"))
		}
	}
	mux.HandleFunc("GET /api/v0/subjects/{id}/image", imgHandler("subject"))
	mux.HandleFunc("GET /api/v0/characters/{id}/image", imgHandler("character"))
	mux.HandleFunc("GET /api/v0/persons/{id}/image", imgHandler("person"))
	// no-image placeholder
	mux.HandleFunc("GET /images/no-image.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(noImageData)
	})

	return withLogging(withCORS(mux))
}

// fetchSubjectList 并发拉取多个动画的所有数据。
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

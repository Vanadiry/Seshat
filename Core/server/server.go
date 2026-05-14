// Package server 提供 HTTP 路由和 API 处理。
package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

// mergeListEntry 将一个条目合并到 list 文件中（按 ID 去重，若已存在则更新 name）。
func New(cfg *config.Config, embedFS fs.FS) http.Handler {
	maxConcurrency = cfg.Server.Concurrency
	mux := http.NewServeMux()
	dd := cfg.DataDir()
	os.MkdirAll(cache.IndexDir(dd), 0o755)
	imgDir := filepath.Join(dd, "images")
	os.MkdirAll(imgDir, 0o755)
	config.LoadPreferences() // ensure settings dir and preferences.json exist at startup

	id := imgDir
	bg := bangumi.NewClient(cfg.Upstream.UserAgent, cfg.Upstream.BaseURL)

	// ── Frontend ──
	mux.HandleFunc("GET /{page}", func(w http.ResponseWriter, r *http.Request) {
		page := r.PathValue("page")
		if strings.HasSuffix(page, ".html") {
			p := strings.TrimSuffix(page, ".html")
			if p == "index" { http.Redirect(w, r, "/", http.StatusMovedPermanently); return }
			http.Redirect(w, r, "/"+p, http.StatusMovedPermanently)
			return
		}
		if embedFS == nil { http.NotFound(w, r); return }
			http.ServeFileFS(w, r, embedFS, "web/"+page+".html")
	})
	// app.js with config injection
	mux.HandleFunc("GET /assets/app.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-cache")
		pref, _ := config.LoadPreferences()
		if pref == nil { pref = &config.DefaultPreferences }
		fmt.Fprintf(w, "window.BACKEND_URL=%q;\nwindow.PREFER_LANG=%q;\nwindow.USERNAME=%q;\nwindow.FALLBACK_URL=%q;\nwindow.SESHAT_HOME=%q;\n", cfg.Frontend.BackendURL, pref.PreferLang, pref.Username, cfg.Frontend.FallbackURL, config.Dir())
		data, _ := fs.ReadFile(embedFS, "web/assets/app.js")
		w.Write(data)
	})
	// All other assets
	mux.HandleFunc("GET /assets/{path...}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, embedFS, "web/assets/"+r.PathValue("path"))
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
	mux.HandleFunc("GET /api/v0/task/{id}", handleProgress)
	mux.HandleFunc("GET /api/v0/tasks", handleActiveTasks)

	// ── Cache API ──
	mux.HandleFunc("GET /api/v0/subjects/list", handleListSubjects(dd))
	mux.HandleFunc("GET /api/v0/characters/list", handleListCharacters(dd))
	mux.HandleFunc("GET /api/v0/persons/list", handleListPersons(dd))
		nameHandler := func(domain string) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				data, err := os.ReadFile(cache.IndexFile(dd, domain+"_name.json"))
				if err != nil { writeJSON(w, map[string]int{}); return }
				w.Header().Set("Content-Type", "application/json")
				w.Write(data)
			}
		}
		mux.HandleFunc("GET /api/v0/subjects/name", nameHandler("subjects"))
		mux.HandleFunc("GET /api/v0/characters/name", nameHandler("characters"))
		mux.HandleFunc("GET /api/v0/persons/name", nameHandler("persons"))
	mux.HandleFunc("GET /api/v0/episodes", func(w http.ResponseWriter, r *http.Request) {
		sid := r.URL.Query().Get("subject_id")
		if sid == "" { http.Error(w, `{"error":"subject_id required"}`, 400); return }
		id, err := strconv.Atoi(sid)
		if err != nil { writeJSON(w, []any{}); return }
		data, err := cache.Get(dd, cache.Key("subjects", id, "episodes.json"))
		if err != nil { writeJSON(w, []any{}); return }
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})
	mux.HandleFunc("GET /api/v0/", handleCacheReader(dd))

	// ── Fetch ──
	mux.HandleFunc("POST /api/v0/fetch/all", handleFetchAll(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/deep", handleFetchDeep(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/tracker", handleFetchTracker(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/user", handleFetchUser(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/subject", handleFetchSubject(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/update", handleFetchUpdate(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/index", handleFetchIndex(dd))

	// ── Tracker ──
	mux.HandleFunc("POST /api/v0/tracker/create", handleTrackerCreate(cfg))
	mux.HandleFunc("GET /api/v0/tracker", handleTrackerList(cfg))
	mux.HandleFunc("POST /api/v0/tracker/import-collections", handleImportCollections(dd))

	// ── Settings ──
	mux.HandleFunc("GET /api/v0/settings", handleSettingsGet)
	mux.HandleFunc("POST /api/v0/settings", handleSettingsPost)

	// ── User ──
	mux.HandleFunc("GET /api/v0/users/{username}", handleUser(dd))
	mux.HandleFunc("GET /api/v0/users/{username}/avatar", serveUserAvatar(dd))
	mux.HandleFunc("GET /api/v0/users/{username}/collections", handleUserCollections(dd))

	// ── Tags ──
	mux.HandleFunc("GET /api/v0/tags", handleTags(dd))
	mux.HandleFunc("GET /api/v0/tags/{name}/subjects", handleTagSubjects(dd))

	// ── Stats ──
	mux.HandleFunc("GET /api/v0/stats", handleStats(dd))

	// ── ELO ──
	mux.HandleFunc("GET /api/v0/elo/pair", handleELOPair(dd))
	mux.HandleFunc("POST /api/v0/elo/compare", handleELOCompare(dd))
	mux.HandleFunc("GET /api/v0/elo/ranking", handleELORanking(dd))
	mux.HandleFunc("GET /api/v0/elo/history", handleELOHistory(dd))
	mux.HandleFunc("POST /api/v0/elo/rebuild", handleELORebuild(dd))

	// ── Search ──
	mux.HandleFunc("POST /api/v0/search/subjects", handleSearchSubjects(dd))
	mux.HandleFunc("POST /api/v0/search/characters", handleSearchCharacters(dd))
	mux.HandleFunc("POST /api/v0/search/persons", handleSearchPersons(dd))
	mux.HandleFunc("POST /api/v0/search/tags", handleSearchTags(dd))

	// ── Image API (official-style: /v0/subjects/{id}/image?type=large|grid) ──
	imgHandler := func(kind string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			serveImage(w, r, dd, kind, r.URL.Query().Get("type"))
		}
	}
	mux.HandleFunc("GET /api/v0/subjects/{id}/image", imgHandler("subject"))
	mux.HandleFunc("GET /api/v0/characters/{id}/image", imgHandler("character"))
	mux.HandleFunc("GET /api/v0/persons/{id}/image", imgHandler("person"))
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

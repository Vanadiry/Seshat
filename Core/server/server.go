// Package server 提供 HTTP 路由和 API 处理。
package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vanadiry/seshat/Core/bangumi"
	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/config"
	"github.com/vanadiry/seshat/Core/events"
	"github.com/vanadiry/seshat/Core/log"
)

var maxInfoConcurrency int
var maxImageConcurrency int

func New(cfg *config.Config, embedFS fs.FS) http.Handler {
	maxInfoConcurrency = cfg.Server.ConcurrencyInfo
	maxImageConcurrency = cfg.Server.ConcurrencyImage
	mux := http.NewServeMux()
	dd := cfg.DataDir()
	os.MkdirAll(cache.IndexDir(dd), 0o755)
	imgDir := filepath.Join(dd, "images")
	os.MkdirAll(imgDir, 0o755)
	config.LoadPreferences() // ensure settings dir and preferences.json exist at startup

	id := imgDir
	bg := bangumi.NewClient(cfg.Upstream.UserAgent, cfg.Upstream.BaseURL, func() string {
		return cfg.Access.Token
	})

	// Frontend
	mux.HandleFunc("GET /{page}", func(w http.ResponseWriter, r *http.Request) {
		page := r.PathValue("page")
		if strings.HasSuffix(page, ".html") {
			p := strings.TrimSuffix(page, ".html")
			if p == "index" {
				http.Redirect(w, r, "/", http.StatusMovedPermanently)
				return
			}
			http.Redirect(w, r, "/"+p, http.StatusMovedPermanently)
			return
		}
		if embedFS == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, embedFS, "web/"+page+".html")
	})
	// app.min.js with config injection
	mux.HandleFunc("GET /assets/app.min.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-cache")
		pref, err := config.LoadPreferences()
		if err != nil || pref == nil {
			pref = &config.DefaultPreferences
		}
		fmt.Fprintf(w, "window.BACKEND_URL=%q;\nwindow.PREFER_LANG=%q;\nwindow.USERNAME=%q;\nwindow.SUBJECT_SORT=%q;\nwindow.AUTO_LINK_NAMES=%q;\nwindow.FALLBACK_URL=%q;\nwindow.SESHAT_HOME=%q;\nwindow.ACCESS_TOKEN=%q;\n", cfg.Frontend.BackendURL, pref.PreferLang, pref.Username, pref.SubjectSort, pref.AutoLinkNames, cfg.Frontend.FallbackURL, config.Dir(), cfg.Access.Token)
		data, err := fs.ReadFile(embedFS, "web/assets/app.min.js")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "app bundle not found")
			return
		}
		w.Write(data)
	})
	// All other assets
	mux.HandleFunc("GET /assets/{path...}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, embedFS, "web/assets/"+r.PathValue("path"))
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if embedFS == nil {
			writeError(w, http.StatusInternalServerError, "frontend not embedded")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, embedFS, "web/index.html")
	})

	// SSE
	mux.HandleFunc("GET /api/v0/events", events.HandleSSE)
	mux.HandleFunc("GET /api/v0/task/{id}", handleProgress)
	mux.HandleFunc("POST /api/v0/task/cancel", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "cancelled"})
		go func() { time.Sleep(100 * time.Millisecond); os.Exit(0) }()
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
	})
	mux.HandleFunc("GET /api/v0/tasks", handleActiveTasks)

	// Cache API
	mux.HandleFunc("GET /api/v0/subjects/list", handleListSubjects(dd))
	mux.HandleFunc("GET /api/v0/characters/list", handleListCharacters(dd))
	mux.HandleFunc("GET /api/v0/persons/list", handleListPersons(dd))
	nameHandler := func(domain string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			path := cache.IndexFile(dd, domain+"_name.json")
			stat, err := os.Stat(path)
			if err != nil {
				writeJSON(w, map[string]int{})
				return
			}
			etag := fmt.Sprintf(`"%d"`, stat.ModTime().Unix())
			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", "max-age=0, must-revalidate")
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			m, _ := loadCachedIndex[map[int][]string](path)
			writeJSON(w, m)
		}
	}
	mux.HandleFunc("GET /api/v0/subjects/name", nameHandler("subjects"))
	mux.HandleFunc("GET /api/v0/characters/name", nameHandler("characters"))
	mux.HandleFunc("GET /api/v0/persons/name", nameHandler("persons"))
	mux.HandleFunc("GET /api/v0/episodes", func(w http.ResponseWriter, r *http.Request) {
		sid := r.URL.Query().Get("subject_id")
		if sid == "" {
			writeError(w, 400, "subject_id required")
			return
		}
		id, err := strconv.Atoi(sid)
		if err != nil {
			writeJSON(w, []any{})
			return
		}
		data, err := cache.Get(dd, cache.Key("subjects", id, "episodes.json"))
		if err != nil {
			writeJSON(w, []any{})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})
	mux.HandleFunc("GET /api/v0/", handleCacheReader(dd))

	// Fetch
	mux.HandleFunc("POST /api/v0/fetch/all", handleFetchAll(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/deep", handleFetchDeep(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/tracker", handleFetchTracker(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/user", handleFetchUser(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/subject", handleFetchSubject(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/update", handleFetchUpdate(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/gap", handleFetchGap(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/meta", handleFetchMeta(cfg, bg, dd))
	mux.HandleFunc("POST /api/v0/fetch/index", handleFetchIndex(dd))

	// Tracker
	mux.HandleFunc("POST /api/v0/tracker/create", handleTrackerCreate(cfg))
	mux.HandleFunc("GET /api/v0/tracker", handleTrackerList(cfg))
	mux.HandleFunc("POST /api/v0/tracker/import-collections", handleImportCollections(dd))

	// Settings
	mux.HandleFunc("GET /api/v0/settings", handleSettingsGet(cfg))
	mux.HandleFunc("POST /api/v0/settings", handleSettingsPost)

	// User
	mux.HandleFunc("GET /api/v0/users/{username}", handleUser(dd))
	mux.HandleFunc("GET /api/v0/users/{username}/avatar", serveUserAvatar(dd))
	mux.HandleFunc("GET /api/v0/users/{username}/collections", handleUserCollections(dd))

	// Tags
	mux.HandleFunc("GET /api/v0/tags", handleTags(dd))
	mux.HandleFunc("GET /api/v0/tags/{name}/subjects", handleTagSubjects(dd))

	// Stats
	mux.HandleFunc("GET /api/v0/stats", handleStats(dd))

	// ELO
	mux.HandleFunc("GET /api/v0/elo/pair", handleELOPair(dd))
	mux.HandleFunc("POST /api/v0/elo/compare", handleELOCompare(dd))
	mux.HandleFunc("GET /api/v0/elo/ranking", handleELORanking(dd))
	mux.HandleFunc("GET /api/v0/elo/history", handleELOHistory(dd))
	mux.HandleFunc("POST /api/v0/elo/rebuild", handleELORebuild(dd))

	// Search
	mux.HandleFunc("POST /api/v0/search/subjects", handleSearchSubjects(dd))
	mux.HandleFunc("POST /api/v0/search/characters", handleSearchCharacters(dd))
	mux.HandleFunc("POST /api/v0/search/persons", handleSearchPersons(dd))
	mux.HandleFunc("POST /api/v0/search/tags", handleSearchTags(dd))

	// Image API (official-style: /v0/subjects/{id}/image?type=large|grid)
	imgHandler := func(kind string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			serveImage(w, r, dd, kind, r.URL.Query().Get("type"))
		}
	}
	mux.HandleFunc("GET /api/v0/subjects/{id}/image", imgHandler("subject"))
	mux.HandleFunc("GET /api/v0/characters/{id}/image", imgHandler("character"))
	mux.HandleFunc("GET /api/v0/persons/{id}/image", imgHandler("person"))
	return withLogging(withCORS(cfg)(mux))
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

func withCORS(cfg *config.Config) func(http.Handler) http.Handler {
	origin := fmt.Sprintf("http://%s:%d", cfg.Server.BindAddr, cfg.Server.Port)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

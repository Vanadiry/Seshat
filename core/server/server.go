// Package server 提供 HTTP 路由和 API 处理
package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

func New(cfg *config.Config, embedFS fs.FS) http.Handler {
	r := buildRouter(cfg, embedFS)
	return withRequestID(withLogging(withCORS(cfg)(r.mux)))
}

func buildRouter(cfg *config.Config, embedFS fs.FS) *router {
	r := newRouter()
	dd := cfg.DataDir()
	if err := os.MkdirAll(store.IndexDir(dd), 0o755); err != nil {
		log.Error("create index dir failed", "err", err)
	}
	imgDir := filepath.Join(dd, "images")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		log.Error("create images dir failed", "err", err)
	}
	config.LoadPreferences() // ensure settings dir and preferences.json exist at startup

	bg := bangumi.NewClient(cfg.Upstream.UserAgent, cfg.Upstream.BaseURL, func() string {
		return cfg.Access.Token
	}, cfg.ProxyURL())

	// 前端
	r.page("GET /{page}", func(w http.ResponseWriter, r *http.Request) {
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
			writeError(w, r, http.StatusInternalServerError, "frontend not embedded")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, embedFS, "web/"+page+".html")
	})
	// 注入配置的 app.min.js
	r.page("GET /assets/app.min.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-cache")
		pref, err := config.LoadPreferences()
		if err != nil || pref == nil {
			pref = &config.DefaultPreferences
		}
		token := ""
		if cfg.Frontend.BackendURL != "" || cfg.Frontend.FallbackURL != "" {
			token = cfg.Access.Token
		}
		fmt.Fprintf(w, "window.BACKEND_URL=%q;\nwindow.PREFER_LANG=%q;\nwindow.USERNAME=%q;\nwindow.SUBJECT_SORT=%q;\nwindow.AUTO_LINK_NAMES=%q;\nwindow.FALLBACK_URL=%q;\nwindow.SESHAT_HOME=%q;\nwindow.ACCESS_TOKEN=%q;\n", cfg.Frontend.BackendURL, pref.PreferLang, pref.Username, pref.SubjectSort, pref.AutoLinkNames, cfg.Frontend.FallbackURL, config.Dir(), token)
		data, err := fs.ReadFile(embedFS, "web/assets/app.min.js")
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "app bundle not found")
			return
		}
		w.Write(data)
	})
	// 其他静态资源
	r.page("GET /assets/{path...}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, embedFS, "web/assets/"+r.PathValue("path"))
	})
	r.page("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			writeNotFound(w, r, "页面不存在")
			return
		}
		if embedFS == nil {
			writeError(w, r, http.StatusInternalServerError, "frontend not embedded")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, embedFS, "web/index.html")
	})

	// SSE 事件
	r.feature("events", "GET /api/v0/events", events.HandleSSE)
	r.local("POST /api/v0/open-url", handleOpenURL)
	r.feature("task", "GET /api/v0/task/{id}", handleProgress)
	r.feature("task", "POST /api/v0/task/cancel", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "cancelled"})
		go func() { time.Sleep(100 * time.Millisecond); os.Exit(0) }()
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
	})
	r.feature("task", "GET /api/v0/tasks", handleActiveTasks)
	r.core("GET /api/v0/info", handleInfo(r.features))

	// 数据 API
	r.feature("lists", "GET /api/v0/subjects/list", handleListSubjects(dd))
	r.feature("lists", "GET /api/v0/characters/list", handleListCharacters(dd))
	r.feature("lists", "GET /api/v0/persons/list", handleListPersons(dd))
	nameHandler := func(domain string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			path := store.IndexFile(dd, domain+"_name.json")
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
	r.feature("names", "GET /api/v0/subjects/name", nameHandler("subjects"))
	r.feature("names", "GET /api/v0/characters/name", nameHandler("characters"))
	r.feature("names", "GET /api/v0/persons/name", nameHandler("persons"))
	r.core("GET /api/v0/episodes", handleEpisodes(dd))
	r.core("GET /api/v0/", handleDataReader(dd))

	// 拉取
	r.feature("fetch", "POST /api/v0/fetch/all", handleFetchAll(cfg, bg, dd, imgDir))
	r.feature("fetch", "POST /api/v0/fetch/deep", handleFetchDeep(cfg, bg, dd, imgDir))
	r.feature("fetch", "POST /api/v0/fetch/tracker", handleFetchTracker(cfg, bg, dd, imgDir))
	r.feature("fetch", "POST /api/v0/fetch/user", handleFetchUser(cfg, bg, dd, imgDir))
	r.feature("fetch", "POST /api/v0/fetch/subject", handleFetchSubject(cfg, bg, dd, imgDir))
	r.feature("fetch", "POST /api/v0/fetch/update", handleFetchUpdate(cfg, bg, dd, imgDir))
	r.feature("fetch", "POST /api/v0/fetch/gap", handleFetchGap(cfg, bg, dd, imgDir))
	r.feature("fetch", "POST /api/v0/fetch/meta", handleFetchMeta(cfg, bg, dd))
	r.feature("fetch", "POST /api/v0/fetch/index", handleFetchIndex(dd))

	// Tracker 管理
	r.feature("tracker", "POST /api/v0/tracker/create", handleTrackerCreate(cfg))
	r.feature("tracker", "GET /api/v0/tracker", handleTrackerList(cfg))
	r.feature("tracker", "POST /api/v0/tracker/import-collections", handleImportCollections(dd))

	// 设置
	r.local("GET /api/v0/settings", handleSettingsGet(cfg))
	r.local("POST /api/v0/settings", handleSettingsPost)

	// 用户
	r.core("GET /api/v0/users/{username}", handleUser(dd))
	r.core("GET /api/v0/users/{username}/avatar", serveUserAvatar(dd))
	r.core("GET /api/v0/users/{username}/collections", handleUserCollections(dd))

	// 标签
	r.feature("tags", "GET /api/v0/tags", handleTags(dd))
	r.feature("tags", "GET /api/v0/tags/{name}/subjects", handleTagSubjects(dd))

	// 统计
	r.feature("stats", "GET /api/v0/stats", handleStats(dd))

	// ELO 评分
	r.feature("elo", "GET /api/v0/elo/pair", handleELOPair(dd))
	r.feature("elo", "POST /api/v0/elo/compare", handleELOCompare(dd))
	r.feature("elo", "GET /api/v0/elo/ranking", handleELORanking(dd))
	r.feature("elo", "GET /api/v0/elo/history", handleELOHistory(dd))
	r.feature("elo", "POST /api/v0/elo/rebuild", handleELORebuild(dd))

	// 搜索
	r.feature("search.subjects", "POST /api/v0/search/subjects", handleSearchSubjects(dd))
	r.feature("search.characters", "POST /api/v0/search/characters", handleSearchCharacters(dd))
	r.feature("search.persons", "POST /api/v0/search/persons", handleSearchPersons(dd))
	r.feature("tags", "POST /api/v0/search/tags", handleSearchTags(dd))

	// 图片 API，兼容官方路径 /v0/subjects/{id}/image?type=large|grid
	imgHandler := func(kind string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			serveImage(w, r, dd, kind, r.URL.Query().Get("type"))
		}
	}
	r.core("GET /api/v0/subjects/{id}/image", imgHandler("subject"))
	r.core("GET /api/v0/characters/{id}/image", imgHandler("character"))
	r.core("GET /api/v0/persons/{id}/image", imgHandler("person"))
	return r
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
		log.HTTP(r.Method, r.URL.Path, sr.status, time.Since(start), requestIDFrom(r.Context()))
	})
}

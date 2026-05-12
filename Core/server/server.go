// Package server 提供 HTTP 路由和 API 处理。
package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
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
	mux.HandleFunc("GET /api/v0/progress/{id}", handleProgress)

	// ── Cache API ──
	mux.HandleFunc("GET /api/v0/subjects", handleListSubjects(dd))
	mux.HandleFunc("GET /api/v0/characters", handleListCharacters(dd))
	mux.HandleFunc("GET /api/v0/persons", handleListPersons(dd))
	mux.HandleFunc("GET /api/v0/", handleCacheReader(dd))

	// ── Fetch ──
	mux.HandleFunc("POST /api/v0/fetch/all", handleFetchAll(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/deep", handleFetchDeep(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/tracker", handleFetchTracker(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/user", handleFetchUser(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/subject", handleFetchSubject(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/update", handleFetchUpdate(cfg, bg, dd, id))
	mux.HandleFunc("POST /api/v0/fetch/images", handleFetchImages(cfg, bg, dd))
	mux.HandleFunc("POST /api/v0/fetch/index", handleFetchIndex(dd))

	// ── Tracker ──
	mux.HandleFunc("POST /api/v0/tracker/create", handleTrackerCreate(cfg))
	mux.HandleFunc("GET /api/v0/tracker", handleTrackerList(cfg))

	// ── User ──
	mux.HandleFunc("GET /api/v0/user/profile", handleUserProfile(cfg, bg, dd))

	// ── Tags ──
	mux.HandleFunc("GET /api/v0/tags", handleTags(dd))
	mux.HandleFunc("GET /api/v0/tags/{name}/subjects", handleTagSubjects(dd))

	// ── ELO ──
	mux.HandleFunc("GET /api/v0/elo/pair", handleELOPair(dd))
	mux.HandleFunc("POST /api/v0/elo/compare", handleELOCompare(dd))
	mux.HandleFunc("GET /api/v0/elo/ranking", handleELORanking(dd))

	// ── Search ──
	mux.HandleFunc("GET /api/v0/search", handleSearch(cfg, dd))
	mux.HandleFunc("POST /api/v0/search/deep", handleSearchDeep(cfg, dd))

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

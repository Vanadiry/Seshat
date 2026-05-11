// Package server 配置 HTTP 路由和中间件（CORS、请求日志）。
package server

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/vanadiry/seshat/internal/handler"
	"github.com/vanadiry/seshat/internal/log"
)

// New 创建并配置完整的 HTTP 路由。
// embedFS 是打包进二进制的 web/ 目录。
func New(h *handler.Handler, embedFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	// 静态页面（从 embed.FS 中读取，打包进二进制）
	mux.HandleFunc("GET /doc/api", serveFile(embedFS, "web/doc_api.html", "text/html"))
	mux.HandleFunc("GET /api/v1/openapi.yaml", serveFile(embedFS, "web/openapi.yaml", "application/yaml"))

	// 公开静态资源
	mux.Handle("GET /public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))

	// 文档文件（从磁盘 doc/ 目录）
	mux.Handle("GET /doc/", http.StripPrefix("/doc/", http.FileServer(http.Dir("doc"))))

	// 用户界面占位（后续替换为 Pico.css SPA）
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, embedFS, "web/index.html")
	})

	// API
	mux.HandleFunc("GET /api/v1/health", h.Health)
	mux.HandleFunc("GET /api/v1/settings", h.GetSettings)
	mux.HandleFunc("PUT /api/v1/settings", h.PutSettings)
	mux.HandleFunc("GET /api/v1/subjects", h.ListSubjects)
	mux.HandleFunc("GET /api/v1/subjects/{id}", h.GetSubject)
	mux.HandleFunc("POST /api/v1/subjects/fetch", h.FetchSubject)
	mux.HandleFunc("POST /api/v1/subjects/fetch/batch", h.FetchSubjectsBatch)
	mux.HandleFunc("GET /api/v1/subjects/{id}/characters", h.GetSubjectCharacters)
	mux.HandleFunc("GET /api/v1/subjects/{id}/persons", h.GetSubjectPersons)
	mux.HandleFunc("GET /api/v1/subjects/{id}/tags", h.GetSubjectTags)
	mux.HandleFunc("GET /api/v1/subjects/{id}/episodes", h.GetSubjectEpisodes)
	mux.HandleFunc("GET /api/v1/subjects/{id}/relations", h.GetSubjectRelations)
	mux.HandleFunc("GET /api/v1/characters/{id}", h.GetCharacter)
	mux.HandleFunc("GET /api/v1/characters/{id}/subjects", h.GetCharacterSubjects)
	mux.HandleFunc("GET /api/v1/characters/{id}/persons", h.GetCharacterPersons)
	mux.HandleFunc("GET /api/v1/persons/{id}", h.GetPerson)
	mux.HandleFunc("GET /api/v1/persons/{id}/subjects", h.GetPersonSubjects)
	mux.HandleFunc("GET /api/v1/persons/{id}/characters", h.GetPersonCharacters)
	mux.HandleFunc("GET /api/v1/episodes/{id}", h.GetEpisode)
	mux.HandleFunc("GET /api/v1/tags", h.ListTags)
	mux.HandleFunc("GET /api/v1/tags/{name}/subjects", h.GetTagSubjects)
	mux.HandleFunc("GET /api/v1/tasks/{id}/events", h.TaskEvents)

	return withLogging(withCORS(mux))
}

// serveFile 从 embed.FS 读取文件并返回，设置 Content-Type。
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

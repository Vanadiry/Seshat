package server

import (
	"log"
	"net/http"
	"time"

	"github.com/vanadiry/seshat/internal/handler"
)

func New(h *handler.Handler) http.Handler {
	mux := http.NewServeMux()
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
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sr.status, time.Since(start))
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

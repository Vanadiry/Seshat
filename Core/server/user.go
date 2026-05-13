package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

func handleUser(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join(dd, "user", "info.json"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		var m map[string]any
		if json.Unmarshal(data, &m) != nil {
			http.NotFound(w, r)
			return
		}
		delete(m, "avatar")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m)
	}
}

func serveUserAvatar(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		size := r.URL.Query().Get("type")
		if size == "" {
			size = "large"
		}
		data, err := os.ReadFile(filepath.Join(dd, "user", "image.json"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		var m map[string]string
		if json.Unmarshal(data, &m) != nil {
			http.NotFound(w, r)
			return
		}
		path, ok := m[size]
		if !ok || path == "" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dd, path))
	}
}

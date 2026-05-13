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
		for _, ext := range []string{".jpg", ".png"} {
			path := filepath.Join(dd, "user", "large"+ext)
			if _, err := os.Stat(path); err == nil {
				http.ServeFile(w, r, path)
				return
			}
		}
		http.NotFound(w, r)
	}
}

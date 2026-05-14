package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

func handleUserCollections(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join(dd, "user", "collections.json"))
		if err != nil {
			writeJSON(w, map[string]any{"data": []any{}})
			return
		}
		var coll struct {
			Subjects map[string]int `json:"subjects"`
		}
		if json.Unmarshal(data, &coll) != nil {
			writeJSON(w, map[string]any{"data": []any{}})
			return
		}
		var list []map[string]any
		for sid, t := range coll.Subjects {
			id, _ := strconv.Atoi(sid)
			if id > 0 { list = append(list, map[string]any{"subject_id": id, "type": t}) }
		}
		writeJSON(w, map[string]any{"data": list})
	}
}

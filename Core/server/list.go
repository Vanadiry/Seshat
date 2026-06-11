package server

import (
	"strconv"
	"strings"
	"net/http"
	"encoding/json"
	"os"

	"github.com/vanadiry/seshat/Core/cache"
)

func saveJSON(path string, v any) {
	data, _ := json.Marshal(v)
	os.WriteFile(path, data, 0o644)
}

// ── Tags index ──

type tagInfo struct {
	Count    int   `json:"count"`
	Subjects []int `json:"subjects"`
}

func loadNameList(path string) []cache.NameEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var list []cache.NameEntry
	json.Unmarshal(data, &list)
	return list
}

// (saveJSON already declared above)

func handleListSubjects(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(cache.IndexFile(dd, "subjects.json"))
		if err != nil { writeJSON(w, []any{}); return }
		var list []cache.SubjectSummary
		json.Unmarshal(data, &list)
		writeJSON(w, list)
	}
}

func handleListCharacters(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(cache.IndexFile(dd, "characters.json"))
		if err != nil { writeJSON(w, []any{}); return }
		var list []cache.NameEntry
		json.Unmarshal(data, &list)
		writeJSON(w, list)
	}
}

func handleListPersons(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(cache.IndexFile(dd, "persons.json"))
		if err != nil { writeJSON(w, []any{}); return }
		var list []cache.NameEntry
		json.Unmarshal(data, &list)
		writeJSON(w, list)
	}
}

func handleCacheReader(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v0/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 2 { http.NotFound(w, r); return }
		domain := parts[0]
		id, err := strconv.Atoi(parts[1])
		if err != nil { http.NotFound(w, r); return }
		file := "info.json"
		if len(parts) >= 3 { file = parts[2] + ".json" }
		data, err := cache.Get(dd, cache.Key(domain, id, file))
		if err != nil { http.NotFound(w, r); return }
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

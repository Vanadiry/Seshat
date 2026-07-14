package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/log"
)

func saveJSON(path string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Error("saveJSON marshal", "path", path, "err", err)
		return
	}
	os.WriteFile(path, data, 0o644)
	clearIndexCache(path)
}

// 标签索引

type tagInfo struct {
	Count    int   `json:"count"`
	Subjects []int `json:"subjects"`
}

func loadNameList(path string) []cache.NameEntry {
	list, _ := loadCachedIndex[[]cache.NameEntry](path)
	return list
}

// (saveJSON already declared above)

func handleListSubjects(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, _ := loadCachedIndex[[]cache.SubjectSummary](cache.IndexFile(dd, "subjects.json"))
		writeJSON(w, list)
	}
}

func handleListCharacters(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, _ := loadCachedIndex[[]cache.NameEntry](cache.IndexFile(dd, "characters.json"))
		writeJSON(w, list)
	}
}

func handleListPersons(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, _ := loadCachedIndex[[]cache.NameEntry](cache.IndexFile(dd, "persons.json"))
		writeJSON(w, list)
	}
}

func handleCacheReader(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v0/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		domain := parts[0]
		id, err := strconv.Atoi(parts[1])
		if err != nil {
			http.NotFound(w, r)
			return
		}
		file := "info.json"
		if len(parts) >= 3 {
			file = parts[2] + ".json"
		}
		data, err := cache.Get(dd, cache.Key(domain, id, file))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

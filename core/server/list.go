package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/vanadiry/seshat/core/index"
	"github.com/vanadiry/seshat/core/store"
)

func loadNameList(path string) []store.NameEntry {
	list, _ := index.LoadCached[[]store.NameEntry](path)
	return list
}

func handleListSubjects(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, _ := index.LoadCached[[]store.SubjectSummary](store.IndexFile(dd, "subjects.json"))
		writeList(w, list)
	}
}

func handleListCharacters(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, _ := index.LoadCached[[]store.NameEntry](store.IndexFile(dd, "characters.json"))
		writeList(w, list)
	}
}

func handleListPersons(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, _ := index.LoadCached[[]store.NameEntry](store.IndexFile(dd, "persons.json"))
		writeList(w, list)
	}
}

// dataRelatedFiles 列出兜底读取允许访问的关联文件
var dataRelatedFiles = map[string]map[string]bool{
	"subjects": {
		"characters.json": true,
		"persons.json":    true,
		"subjects.json":   true,
	},
	"characters": {
		"persons.json":  true,
		"subjects.json": true,
	},
	"persons": {
		"characters.json": true,
		"subjects.json":   true,
	},
}

func handleDataReader(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v0/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 2 || len(parts) > 3 {
			writeNotFound(w, r, "数据路径不合法")
			return
		}
		domain := parts[0]
		files, ok := dataRelatedFiles[domain]
		if !ok {
			writeNotFound(w, r, "数据路径不合法")
			return
		}
		id, err := strconv.Atoi(parts[1])
		if err != nil || id <= 0 {
			writeNotFound(w, r, "条目 ID 不合法")
			return
		}
		file := "info.json"
		if len(parts) == 3 {
			file = parts[2] + ".json"
			if !files[file] {
				writeNotFound(w, r, "数据路径不合法")
				return
			}
		}
		data, err := store.Get(dd, store.Key(domain, id, file))
		if err != nil {
			writeNotFound(w, r, "数据不存在")
			return
		}
		data = injectImages(data, domain, file, id, r, dd)
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

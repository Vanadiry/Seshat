package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

func saveJSON(path string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Error("saveJSON marshal", "path", path, "err", err)
		events.Bus.Error("索引文件保存失败")
		return
	}
	if err := store.WriteAtomic(path, data, 0o644); err != nil {
		log.Error("saveJSON write", "path", path, "err", err)
		events.Bus.Error("索引文件保存失败")
		return
	}
	clearIndexCache(path)
}

// 标签索引

type tagInfo struct {
	Count    int   `json:"count"`
	Subjects []int `json:"subjects"`
}

func loadNameList(path string) []store.NameEntry {
	list, _ := loadCachedIndex[[]store.NameEntry](path)
	return list
}

func handleListSubjects(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, _ := loadCachedIndex[[]store.SubjectSummary](store.IndexFile(dd, "subjects.json"))
		writeList(w, list)
	}
}

func handleListCharacters(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, _ := loadCachedIndex[[]store.NameEntry](store.IndexFile(dd, "characters.json"))
		writeList(w, list)
	}
}

func handleListPersons(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, _ := loadCachedIndex[[]store.NameEntry](store.IndexFile(dd, "persons.json"))
		writeList(w, list)
	}
}

func handleDataReader(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v0/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 2 {
			writeNotFound(w, r, "数据路径不合法")
			return
		}
		domain := parts[0]
		id, err := strconv.Atoi(parts[1])
		if err != nil {
			writeNotFound(w, r, "条目 ID 不合法")
			return
		}
		file := "info.json"
		if len(parts) >= 3 {
			file = parts[2] + ".json"
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

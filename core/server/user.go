package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
)

func userDir() string { return filepath.Join(config.Dir(), "user", "info") }

func handleUser(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join(userDir(), "info.json"))
		if err != nil {
			writeNotFound(w, r, "用户信息不存在")
			return
		}
		var m map[string]any
		if json.Unmarshal(data, &m) != nil {
			writeError(w, r, http.StatusInternalServerError, "用户信息解析失败")
			return
		}
		delete(m, "avatar")
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(m); err != nil {
			log.Error("user json encode", "err", err)
			events.Bus.Error("用户数据响应失败")
		}
	}
}

func serveUserAvatar(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, ext := range []string{".jpg", ".png"} {
			path := filepath.Join(userDir(), "large"+ext)
			if _, err := os.Stat(path); err == nil {
				http.ServeFile(w, r, path)
				return
			}
		}
		writeNotFound(w, r, "用户头像不存在")
	}
}

func handleUserCollections(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join(userDir(), "collections.json"))
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
			if id > 0 {
				list = append(list, map[string]any{"subject_id": id, "type": t})
			}
		}
		writeJSON(w, map[string]any{"data": list})
	}
}

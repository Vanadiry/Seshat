package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/index"
	"github.com/vanadiry/seshat/core/log"
)

func userDir(dd string) string { return filepath.Join(dd, "user") }

func configuredUsername() (string, bool) {
	pref, err := config.LoadPreferences()
	if err != nil || pref == nil {
		return "", false
	}
	return pref.Username, true
}

// requirePathUsername 校验路径 username 与配置用户一致
func requirePathUsername(w http.ResponseWriter, r *http.Request) (string, bool) {
	want, ok := configuredUsername()
	if !ok {
		writeError(w, r, http.StatusInternalServerError, "读取用户配置失败")
		return "", false
	}
	got := r.PathValue("username")
	if want == "" || got != want {
		writeNotFound(w, r, "用户不存在")
		return "", false
	}
	return got, true
}

func handleUser(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, ok := requirePathUsername(w, r)
		if !ok {
			return
		}
		data, err := os.ReadFile(filepath.Join(userDir(dd), "info.json"))
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
		base := requestBaseURL(r)
		avatar := base + "/api/v0/users/" + url.PathEscape(name) + "/avatar?type=large"
		m["avatar"] = map[string]string{
			"large":  avatar,
			"medium": avatar,
			"small":  avatar,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(m); err != nil {
			log.Error("user json encode", "err", err)
			events.Bus.Error("用户数据响应失败")
		}
	}
}

func serveUserAvatar(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requirePathUsername(w, r); !ok {
			return
		}
		for _, ext := range []string{".jpg", ".png"} {
			path := filepath.Join(userDir(dd), "large"+ext)
			if _, err := os.Stat(path); err == nil {
				http.ServeFile(w, r, path)
				return
			}
		}
		writeNotFound(w, r, "用户头像不存在")
	}
}

type collectionFile struct {
	UpdatedAt string           `json:"updated_at"`
	Data      []map[string]any `json:"data"`
}

func queryInt(r *http.Request, name string, def, min, max int) (int, bool) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return def, true
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < min || (max >= 0 && v > max) {
		return 0, false
	}
	return v, true
}

func injectCollectionImages(items []map[string]any, r *http.Request, dd string) {
	base := requestBaseURL(r)
	index := index.LoadImageIndex(dd, "subjects_image.json")
	for _, item := range items {
		subject, ok := item["subject"].(map[string]any)
		if !ok {
			continue
		}
		id := jsonInt(subject["id"])
		if id <= 0 {
			continue
		}
		subject["images"] = buildImages("subject", id, base, index)
	}
}

func handleCollectionSummary(w http.ResponseWriter, r *http.Request, dd string) {
	if data, err := os.ReadFile(filepath.Join(userDir(dd), "collections_summary.json")); err == nil {
		var summary struct {
			Data []map[string]any `json:"data"`
		}
		if json.Unmarshal(data, &summary) == nil {
			writeJSON(w, map[string]any{"total": len(summary.Data), "data": summary.Data})
			return
		}
	}

	data, err := os.ReadFile(filepath.Join(userDir(dd), "collections.json"))
	if err != nil {
		writeJSON(w, map[string]any{"total": 0, "data": []any{}})
		return
	}
	var coll collectionFile
	if json.Unmarshal(data, &coll) != nil {
		writeError(w, r, http.StatusInternalServerError, "收藏数据解析失败")
		return
	}
	list := make([]map[string]any, 0, len(coll.Data))
	for _, item := range coll.Data {
		list = append(list, map[string]any{
			"subject_id":   item["subject_id"],
			"subject_type": item["subject_type"],
			"type":         item["type"],
			"rate":         item["rate"],
		})
	}
	writeJSON(w, map[string]any{"total": len(list), "data": list})
}

func handleUserCollections(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requirePathUsername(w, r); !ok {
			return
		}
		view := r.URL.Query().Get("view")
		if view != "" && view != "summary" {
			writeError(w, r, http.StatusBadRequest, "view 不合法")
			return
		}
		if view == "summary" {
			handleCollectionSummary(w, r, dd)
			return
		}

		data, err := os.ReadFile(filepath.Join(userDir(dd), "collections.json"))
		if err != nil {
			writeJSON(w, map[string]any{"total": 0, "limit": 0, "offset": 0, "data": []any{}})
			return
		}
		var coll collectionFile
		if json.Unmarshal(data, &coll) != nil {
			writeError(w, r, http.StatusInternalServerError, "收藏数据解析失败")
			return
		}

		subjectType, ok := queryInt(r, "subject_type", -1, 0, -1)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "subject_type 不合法")
			return
		}
		collType, ok := queryInt(r, "type", -1, 0, -1)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "type 不合法")
			return
		}
		limit, ok := queryInt(r, "limit", -1, 1, 100)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "limit 不合法")
			return
		}
		offset, ok := queryInt(r, "offset", 0, 0, -1)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "offset 不合法")
			return
		}

		filtered := make([]map[string]any, 0, len(coll.Data))
		for _, item := range coll.Data {
			if subjectType >= 0 && jsonInt(item["subject_type"]) != subjectType {
				continue
			}
			if collType >= 0 && jsonInt(item["type"]) != collType {
				continue
			}
			filtered = append(filtered, item)
		}

		total := len(filtered)
		if limit < 0 {
			limit = total - offset
			if limit < 0 {
				limit = 0
			}
		}
		if offset > total {
			offset = total
		}
		end := offset + limit
		if end > total {
			end = total
		}
		page := filtered[offset:end]
		injectCollectionImages(page, r, dd)

		writeJSON(w, map[string]any{
			"total":  total,
			"limit":  limit,
			"offset": offset,
			"data":   page,
		})
	}
}

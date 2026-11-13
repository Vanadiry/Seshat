package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/vanadiry/seshat/core/store"
)

const maxEpisodesLimit = 200

// handleEpisodes 返回 Paged_Episode，limit 不传时返回全部
func handleEpisodes(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sid := r.URL.Query().Get("subject_id")
		if sid == "" {
			writeError(w, r, http.StatusBadRequest, "subject_id 不能为空")
			return
		}
		id, err := strconv.Atoi(sid)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "subject_id 不合法")
			return
		}

		typeFilter := -1
		if t := r.URL.Query().Get("type"); t != "" {
			v, err := strconv.Atoi(t)
			if err != nil || v < 0 {
				writeError(w, r, http.StatusBadRequest, "type 不合法")
				return
			}
			typeFilter = v
		}

		limit := -1
		if l := r.URL.Query().Get("limit"); l != "" {
			v, err := strconv.Atoi(l)
			if err != nil || v < 1 || v > maxEpisodesLimit {
				writeError(w, r, http.StatusBadRequest, "limit 不合法")
				return
			}
			limit = v
		}

		offset := 0
		if o := r.URL.Query().Get("offset"); o != "" {
			v, err := strconv.Atoi(o)
			if err != nil || v < 0 {
				writeError(w, r, http.StatusBadRequest, "offset 不合法")
				return
			}
			offset = v
		}

		episodes := []json.RawMessage{}
		if data, err := store.Get(dd, store.Key("subjects", id, "episodes.json")); err == nil {
			if json.Unmarshal(data, &episodes) != nil {
				writeError(w, r, http.StatusInternalServerError, "剧集数据解析失败")
				return
			}
		}

		filtered := make([]json.RawMessage, 0, len(episodes))
		for _, raw := range episodes {
			if typeFilter >= 0 {
				var meta struct {
					Type int `json:"type"`
				}
				if json.Unmarshal(raw, &meta) != nil || meta.Type != typeFilter {
					continue
				}
			}
			filtered = append(filtered, raw)
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

		writeJSON(w, map[string]any{
			"total":  total,
			"limit":  limit,
			"offset": offset,
			"data":   filtered[offset:end],
		})
	}
}

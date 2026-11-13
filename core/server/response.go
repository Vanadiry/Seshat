package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vanadiry/seshat/core/log"
)

const maxBodySize = 1 << 20

// errorBody 为对外错误响应，error 为过渡字段
type errorBody struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Details     any    `json:"details,omitempty"`
	RequestID   string `json:"request_id,omitempty"`
}

func limitBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
}

func statusTitle(code int) string {
	switch code {
	case http.StatusBadRequest:
		return "Bad Request"
	case http.StatusUnauthorized:
		return "Unauthorized"
	case http.StatusForbidden:
		return "Forbidden"
	case http.StatusNotFound:
		return "Not Found"
	case http.StatusMethodNotAllowed:
		return "Method Not Allowed"
	case http.StatusConflict:
		return "Conflict"
	case http.StatusRequestEntityTooLarge:
		return "Payload Too Large"
	case http.StatusUnprocessableEntity:
		return "Unprocessable Entity"
	case http.StatusInternalServerError:
		return "Internal Server Error"
	case http.StatusServiceUnavailable:
		return "Service Unavailable"
	default:
		if title := http.StatusText(code); title != "" {
			return title
		}
		return "Error"
	}
}

func writeError(w http.ResponseWriter, r *http.Request, code int, description string, details ...any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	var detail any
	if len(details) > 0 {
		detail = details[0]
	}
	body := errorBody{
		Title:       statusTitle(code),
		Description: description,
		Details:     detail,
		RequestID:   requestIDFrom(r.Context()),
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Error("writeError encode", "err", err)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error("writeJSON encode", "err", err)
	}
}

// writeList 输出统一的分页列表结构，不分页时 limit 和 offset 为 0
func writeList[T any](w http.ResponseWriter, list []T) {
	if list == nil {
		list = []T{}
	}
	writeJSON(w, map[string]any{
		"total":  len(list),
		"limit":  0,
		"offset": 0,
		"data":   list,
	})
}

// writeNotFound 输出带 path 和 method 的 404
func writeNotFound(w http.ResponseWriter, r *http.Request, description string) {
	writeError(w, r, http.StatusNotFound, description, map[string]any{
		"path":   r.URL.Path,
		"method": r.Method,
	})
}

// decodeJSON 解码请求体，超限返回 413，其余解析失败返回 400
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "请求体过大", map[string]any{
				"max_bytes": maxErr.Limit,
			})
			return false
		}
		writeError(w, r, http.StatusBadRequest, "请求体不合法")
		return false
	}
	return true
}

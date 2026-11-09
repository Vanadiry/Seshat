package server

import (
	"encoding/json"
	"net/http"

	"github.com/vanadiry/seshat/core/log"
)

const maxBodySize = 1 << 20

// errorBody 是对外错误响应，error 为过渡字段，前端切换后删除
type errorBody struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Details     any    `json:"details,omitempty"`
	RequestID   string `json:"request_id,omitempty"`
	Error       string `json:"error,omitempty"`
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
		Error:       description,
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

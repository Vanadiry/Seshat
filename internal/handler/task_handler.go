// 任务进度端点：SSE 流式推送。
package handler

import (
	"fmt"
	"net/http"

	"github.com/vanadiry/seshat/internal/task"
)

func (h *Handler) TaskEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t := h.Fetch.Tasks.Get(id)
	if t == nil {
		writeJSON(w, http.StatusNotFound, errorResp("NOT_FOUND", "task not found"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, errorResp("SSE_ERROR", "streaming not supported"))
		return
	}

	// If task already finished (e.g. instant failure), send last event and close.
	if t.Status != task.StatusRunning {
		if t.LastEvent != "" {
			fmt.Fprintf(w, "data: %s\n\n", t.LastEvent)
			flusher.Flush()
		}
		return
	}

	events, err := h.Fetch.Tasks.Events(id)
	if err != nil {
		return
	}
	for event := range events {
		fmt.Fprintf(w, "data: %s\n\n", event)
		flusher.Flush()
	}
}

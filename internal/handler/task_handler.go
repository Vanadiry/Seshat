// 任务进度端点：SSE 流式推送。
package handler

import (
	"fmt"
	"net/http"
)

func (h *Handler) TaskEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.Fetch.Tasks.Events(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorResp("NOT_FOUND", err.Error()))
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
	for event := range events {
		fmt.Fprintf(w, "data: %s\n\n", event)
		flusher.Flush()
	}
}

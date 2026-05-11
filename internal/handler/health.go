// 健康检查端点（GET /api/v1/health）。
package handler

import (
	"net/http"

	"github.com/vanadiry/seshat/internal/model"
)

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, model.APIResponse{Data: map[string]string{"status": "ok"}})
}

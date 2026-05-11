// 剧集端点：单集详情。
package handler

import (
	"net/http"
	"strconv"

	"github.com/vanadiry/seshat/internal/model"
)

func (h *Handler) GetEpisode(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	ep, err := h.Queries.GetEpisode(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorResp("NOT_FOUND", "episode not found"))
		return
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: ep})
}

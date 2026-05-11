// 角色端点：详情、出演动画、声优列表。
package handler

import (
	"net/http"
	"strconv"

	"github.com/vanadiry/seshat/internal/model"
)

func (h *Handler) GetCharacter(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	c, err := h.Queries.GetCharacter(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorResp("NOT_FOUND", "character not found"))
		return
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: c})
}

func (h *Handler) GetCharacterSubjects(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	data, err := h.Queries.GetCharacterSubjects(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResp("QUERY_ERROR", err.Error()))
		return
	}
	if data == nil {
		data = []model.SubjectCharacter{}
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: data})
}

func (h *Handler) GetCharacterPersons(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	data, err := h.Queries.GetCharacterPersons(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResp("QUERY_ERROR", err.Error()))
		return
	}
	if data == nil {
		data = []model.SubjectPerson{}
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: data})
}

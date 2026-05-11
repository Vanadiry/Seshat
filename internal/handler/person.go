package handler

import (
	"net/http"
	"strconv"

	"github.com/vanadiry/seshat/internal/model"
)

func (h *Handler) GetPerson(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	p, err := h.Queries.GetPerson(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorResp("NOT_FOUND", "person not found"))
		return
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: p})
}

func (h *Handler) GetPersonSubjects(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	data, err := h.Queries.GetPersonSubjects(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResp("QUERY_ERROR", err.Error()))
		return
	}
	if data == nil {
		data = []model.SubjectPerson{}
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: data})
}

func (h *Handler) GetPersonCharacters(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	data, err := h.Queries.GetPersonCharacters(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResp("QUERY_ERROR", err.Error()))
		return
	}
	if data == nil {
		data = []model.SubjectCharacter{}
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: data})
}

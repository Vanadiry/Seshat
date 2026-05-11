package handler

import (
	"net/http"
	"strconv"

	"github.com/vanadiry/seshat/internal/model"
)

func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	data, err := h.Queries.ListTags()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResp("QUERY_ERROR", err.Error()))
		return
	}
	if data == nil {
		data = []model.Tag{}
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: data})
}

func (h *Handler) GetTagSubjects(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 30
	}
	subjects, total, err := h.Queries.GetTagSubjects(name, page, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResp("QUERY_ERROR", err.Error()))
		return
	}
	if subjects == nil {
		subjects = []model.Subject{}
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: subjects, Total: &total, Page: &page, Limit: &limit})
}

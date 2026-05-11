// 动画相关端点：列表、详情、关联数据、数据拉取。
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/vanadiry/seshat/internal/model"
)

func (h *Handler) ListSubjects(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 30
	}
	subjects, total, err := h.Queries.ListSubjects(q.Get("q"), q.Get("tag"), q.Get("platform"), q.Get("year"), page, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResp("QUERY_ERROR", err.Error()))
		return
	}
	if subjects == nil {
		subjects = []model.Subject{}
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: subjects, Total: &total, Page: &page, Limit: &limit})
}

func (h *Handler) GetSubject(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	s, err := h.Queries.GetSubject(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorResp("NOT_FOUND", "subject not found"))
		return
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: s})
}

func (h *Handler) GetSubjectCharacters(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	data, err := h.Queries.GetSubjectCharacters(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResp("QUERY_ERROR", err.Error()))
		return
	}
	for i := range data {
		if data[i].Character != nil {
			data[i].Character.Summary = ""
			data[i].Character.Infobox = ""
		}
	}
	if data == nil {
		data = []model.SubjectCharacter{}
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: data})
}

func (h *Handler) GetSubjectPersons(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	data, err := h.Queries.GetSubjectPersons(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResp("QUERY_ERROR", err.Error()))
		return
	}
	for i := range data {
		if data[i].Person != nil {
			data[i].Person.Summary = ""
			data[i].Person.Infobox = ""
		}
	}
	if data == nil {
		data = []model.SubjectPerson{}
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: data})
}

func (h *Handler) GetSubjectTags(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	data, err := h.Queries.GetSubjectTags(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResp("QUERY_ERROR", err.Error()))
		return
	}
	if data == nil {
		data = []model.Tag{}
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: data})
}

func (h *Handler) GetSubjectEpisodes(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	data, err := h.Queries.GetSubjectEpisodes(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResp("QUERY_ERROR", err.Error()))
		return
	}
	for i := range data {
		data[i].Duration = ""
		data[i].Desc = ""
	}
	if data == nil {
		data = []model.Episode{}
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: data})
}

func (h *Handler) GetSubjectRelations(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	data, err := h.Queries.GetSubjectRelations(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResp("QUERY_ERROR", err.Error()))
		return
	}
	if data == nil {
		data = []model.SubjectRelation{}
	}
	writeJSON(w, http.StatusOK, model.APIResponse{Data: data})
}

func (h *Handler) FetchSubject(w http.ResponseWriter, r *http.Request) {
	var req struct{ ID int `json:"id"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == 0 {
		writeJSON(w, http.StatusBadRequest, errorResp("BAD_REQUEST", "JSON body must include {id: N}"))
		return
	}
	t := h.Fetch.Tasks.Create(req.ID)
	go h.Fetch.FetchSubject(t)
	writeJSON(w, http.StatusAccepted, model.APIResponse{Data: map[string]string{"task_id": t.ID, "status": "accepted"}})
}

func (h *Handler) FetchSubjectsBatch(w http.ResponseWriter, r *http.Request) {
	var req struct{ IDs []int `json:"ids"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, errorResp("BAD_REQUEST", "JSON body must include {ids: [1,2,...]}"))
		return
	}
	taskIDs := make([]string, 0, len(req.IDs))
	for _, id := range req.IDs {
		t := h.Fetch.Tasks.Create(id)
		go h.Fetch.FetchSubject(t)
		taskIDs = append(taskIDs, t.ID)
	}
	writeJSON(w, http.StatusAccepted, model.APIResponse{Data: map[string]any{"task_ids": taskIDs, "status": "accepted"}})
}

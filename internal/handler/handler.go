package handler

import (
	"encoding/json"
	"net/http"

	"github.com/vanadiry/seshat/internal/config"
	"github.com/vanadiry/seshat/internal/fetch"
	"github.com/vanadiry/seshat/internal/model"
	"github.com/vanadiry/seshat/internal/query"
)

type Handler struct {
	Queries *query.Queries
	Config  *config.Config
	Fetch   *fetch.Service
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errorResp(code, message string) model.APIResponse {
	return model.APIResponse{Error: &model.APIError{Code: code, Message: message}}
}

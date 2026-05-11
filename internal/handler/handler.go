// Package handler 实现所有 HTTP API 端点的处理逻辑。
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
	DataDir string // 数据目录（用于图片服务）
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errorResp(code, message string) model.APIResponse {
	return model.APIResponse{Error: &model.APIError{Code: code, Message: message}}
}

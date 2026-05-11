// 设置端点（GET/PUT /api/v1/settings），读写 TOML 配置文件。
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/vanadiry/seshat/internal/config"
	"github.com/vanadiry/seshat/internal/model"
)

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	c := h.Config
	writeJSON(w, http.StatusOK, model.APIResponse{Data: map[string]string{
		"bind_addr":   c.BindAddr,
		"port":        strconv.Itoa(c.Port),
		"data_home":   c.DataHome,
		"username":    c.Username,
		"concurrency": strconv.Itoa(c.Concurrency),
	}})
}

func (h *Handler) PutSettings(w http.ResponseWriter, r *http.Request) {
	var in map[string]string
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResp("BAD_REQUEST", "invalid JSON"))
		return
	}
	c := *h.Config
	if v, ok := in["bind_addr"]; ok {
		c.BindAddr = v
	}
	if v, ok := in["port"]; ok {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 {
			writeJSON(w, http.StatusBadRequest, errorResp("SETTINGS_WRITE_ERROR", "port must be 1-65535"))
			return
		}
		c.Port = p
	}
	if v, ok := in["data_home"]; ok {
		c.DataHome = v
	}
	if v, ok := in["username"]; ok {
		c.Username = v
	}
	if v, ok := in["concurrency"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 128 {
			writeJSON(w, http.StatusBadRequest, errorResp("SETTINGS_WRITE_ERROR", "concurrency must be 1-128"))
			return
		}
		c.Concurrency = n
	}
	if err := config.Validate(&c); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResp("SETTINGS_WRITE_ERROR", err.Error()))
		return
	}
	if err := config.Save(&c); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResp("SETTINGS_WRITE_ERROR", err.Error()))
		return
	}
	*h.Config = c
	writeJSON(w, http.StatusOK, model.APIResponse{Data: map[string]string{"saved": "ok"}})
}

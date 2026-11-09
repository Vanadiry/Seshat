package server

import (
	"net/http"
	"strings"

	"github.com/vanadiry/seshat/core/config"
)

func handleSettingsGet(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"prefs":  config.BuildPrefsKV(),
			"config": cfg.BuildConfigKV(),
		})
	}
}

func handleSettingsPost(w http.ResponseWriter, r *http.Request) {
	var updates map[string]any
	limitBody(w, r)
	if !decodeJSON(w, r, &updates) {
		return
	}
	okList, failList := config.ApplyOverrides(updates)

	resp := map[string]any{}
	if len(okList) > 0 {
		resp["success"] = strings.Join(okList, ", ")
	}
	if len(failList) > 0 {
		resp["error"] = strings.Join(failList, ", ")
	}
	writeJSON(w, resp)
}

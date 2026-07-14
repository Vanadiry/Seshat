package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/vanadiry/seshat/Core/config"
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
	if json.NewDecoder(r.Body).Decode(&updates) != nil {
		writeError(w, 400, "invalid JSON")
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

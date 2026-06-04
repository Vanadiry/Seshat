package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/vanadiry/seshat/Core/config"
)

func handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(config.PrefPath())
	if err != nil {
		writeJSON(w, map[string]any{})
		return
	}
	var v any
	json.Unmarshal(data, &v)
	writeJSON(w, v)
}

func handleSettingsPost(w http.ResponseWriter, r *http.Request) {
	existing := map[string]any{}
	if data, err := os.ReadFile(config.PrefPath()); err == nil {
		json.Unmarshal(data, &existing)
	}

	var updates map[string]any
	if json.NewDecoder(r.Body).Decode(&updates) != nil {
		http.Error(w, `{"error":"invalid JSON"}`, 400)
		return
	}
	var okList, failList []string
	for k, v := range updates {
		obj, ok := existing[k].(map[string]any)
		if !ok || obj["value"] == nil {
			failList = append(failList, k)
			continue
		}
		obj["value"] = v
		okList = append(okList, k)
	}

	os.MkdirAll(config.PrefDir(), 0o755)
	result, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(config.PrefPath(), result, 0o644)
	loadCachedToken() // refresh access token cache

	resp := map[string]any{}
	if len(okList) > 0 { resp["success"] = strings.Join(okList, ", ") }
	if len(failList) > 0 { resp["error"] = strings.Join(failList, ", ") }
	writeJSON(w, resp)
}

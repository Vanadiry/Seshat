package server

import (
	"net/http"

	"github.com/vanadiry/seshat/core/config"
)

const (
	instanceName       = "Seshat"
	instanceLicense    = "AGPL-3.0"
	instanceRepository = "https://github.com/Vanadiry/Seshat"
	instanceCopyright  = "Copyright © 2026 Vanadiry"
)

// handleInfo 返回实例信息，供关于页展示
func handleInfo(features func() []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"name":       instanceName,
			"version":    config.Version,
			"license":    instanceLicense,
			"repository": instanceRepository,
			"copyright":  instanceCopyright,
			"features":   features(),
		})
	}
}

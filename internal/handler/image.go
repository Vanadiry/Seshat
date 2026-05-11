package handler

import (
	"net/http"
	"path/filepath"
)

func ImageFS(dataDir string) http.Handler {
	return http.StripPrefix("/api/v1/images/",
		http.FileServer(http.Dir(filepath.Join(dataDir, "images"))))
}

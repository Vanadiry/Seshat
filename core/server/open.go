package server

import (
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

func OpenBrowser(url string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	_ = c.Start()
}

func handleOpenURL(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r)
	var body struct {
		URL string `json:"url"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.URL == "" {
		writeError(w, r, http.StatusBadRequest, "missing url")
		return
	}
	u, ok := parseHTTPURL(body.URL)
	if !ok {
		writeError(w, r, http.StatusBadRequest, "only http/https URLs are allowed")
		return
	}
	OpenBrowser(u.String())
	writeJSON(w, map[string]string{"status": "ok"})
}

// parseHTTPURL 解析并限制为 http/https 地址
func parseHTTPURL(raw string) (*url.URL, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, false
	}
	if u.Host == "" {
		return nil, false
	}
	return u, true
}

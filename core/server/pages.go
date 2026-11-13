package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/vanadiry/seshat/core/config"
)

// servePage 把干净路径映射到 web-dist 下的 HTML 文件
func servePage(embedFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := r.PathValue("page")
		if strings.HasSuffix(page, ".html") {
			p := strings.TrimSuffix(page, ".html")
			if p == "index" {
				http.Redirect(w, r, "/", http.StatusMovedPermanently)
				return
			}
			http.Redirect(w, r, "/"+p, http.StatusMovedPermanently)
			return
		}
		if embedFS == nil {
			writeError(w, r, http.StatusInternalServerError, "frontend not embedded")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, embedFS, "web-dist/"+page+".html")
	}
}

// serveIndex 返回首页
func serveIndex(embedFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			writeNotFound(w, r, "页面不存在")
			return
		}
		if embedFS == nil {
			writeError(w, r, http.StatusInternalServerError, "frontend not embedded")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, embedFS, "web-dist/index.html")
	}
}

// serveAsset 返回静态资源
func serveAsset(embedFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, embedFS, "web-dist/assets/"+r.PathValue("path"))
	}
}

// serveAppJS 注入运行时配置后再返回共享脚本
func serveAppJS(cfg *config.Config, embedFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-cache")
		pref, err := config.LoadPreferences()
		if err != nil || pref == nil {
			pref = &config.DefaultPreferences
		}
		token := ""
		if cfg.Frontend.BackendURL != "" || cfg.Frontend.FallbackURL != "" {
			token = cfg.Access.Token
		}
		fmt.Fprintf(w, "window.BACKEND_URL=%q;\nwindow.PREFER_LANG=%q;\nwindow.USERNAME=%q;\nwindow.SUBJECT_SORT=%q;\nwindow.AUTO_LINK_NAMES=%q;\nwindow.FALLBACK_URL=%q;\nwindow.SESHAT_HOME=%q;\nwindow.ACCESS_TOKEN=%q;\nwindow.API_TOKEN=%q;\nwindow.UPSTREAM_ENABLED=%t;\n", cfg.Frontend.BackendURL, pref.PreferLang, pref.Username, pref.SubjectSort, pref.AutoLinkNames, cfg.Frontend.FallbackURL, config.Dir(), token, cfg.Server.APIToken, cfg.Upstream.BaseURL != "")
		data, err := fs.ReadFile(embedFS, "web-dist/assets/app.min.js")
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "app bundle not found")
			return
		}
		w.Write(data)
	}
}

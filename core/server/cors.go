package server

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/vanadiry/seshat/core/config"
)

// withCORS 按监听地址决定跨源策略
func withCORS(cfg *config.Config) func(http.Handler) http.Handler {
	allowAny := bindAllowsAnyOrigin(cfg.Server.BindAddr)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := origin == "" || (validOrigin(origin) && (allowAny || isLoopbackOrigin(origin)))
			if origin != "" {
				w.Header().Add("Vary", "Origin")
			}
			if allowed && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if !allowed {
				writeError(w, r, http.StatusForbidden, "来源不被允许")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// bindAllowsAnyOrigin 判断监听地址是否视为对外公开
func bindAllowsAnyOrigin(addr string) bool {
	addr = strings.TrimSpace(addr)
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}
	addr = strings.Trim(addr, "[]")
	if addr == "" {
		return true
	}
	if strings.EqualFold(addr, "localhost") {
		return false
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		return true
	}
	return !ip.IsLoopback()
}

// validOrigin 校验 Origin 是否为合法的 http 或 https 地址
func validOrigin(origin string) bool {
	_, ok := parseOrigin(origin)
	return ok
}

func parseOrigin(origin string) (*url.URL, bool) {
	u, err := url.Parse(origin)
	if err != nil {
		return nil, false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, false
	}
	if u.Host == "" || u.User != nil {
		return nil, false
	}
	if u.Path != "" && u.Path != "/" {
		return nil, false
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, false
	}
	return u, true
}

// isLoopbackOrigin 判断 Origin 是否指向本机
func isLoopbackOrigin(origin string) bool {
	u, ok := parseOrigin(origin)
	if !ok {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

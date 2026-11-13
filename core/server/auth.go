package server

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/vanadiry/seshat/core/config"
)

// withAPIToken 校验本地 API 的 POST 请求令牌。
// 只保护写入类请求；GET 和页面资源不校验。
func withAPIToken(cfg *config.Config) func(http.Handler) http.Handler {
	token := cfg.Server.APIToken
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" || r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/api/") {
				next.ServeHTTP(w, r)
				return
			}
			got := r.Header.Get("X-Seshat-Token")
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				writeError(w, r, http.StatusForbidden, "缺少或错误的 API 令牌")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vanadiry/seshat/core/config"
)

func TestAPIToken(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.APIToken = "secret"
	called := false
	h := withAPIToken(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v0/fetch", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden || called {
		t.Fatalf("缺少令牌的 POST 应被拒绝，状态码 = %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v0/fetch", nil)
	req.Header.Set("X-Seshat-Token", "wrong")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden || called {
		t.Fatalf("错误令牌应被拒绝，状态码 = %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v0/fetch", nil)
	req.Header.Set("X-Seshat-Token", "secret")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !called {
		t.Fatalf("正确令牌应放行，状态码 = %d", w.Code)
	}

	called = false
	req = httptest.NewRequest(http.MethodGet, "/api/v0/info", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !called {
		t.Fatalf("GET 不应校验令牌，状态码 = %d", w.Code)
	}
}

func TestAPITokenSkipsWhenEmpty(t *testing.T) {
	cfg := &config.Config{}
	called := false
	h := withAPIToken(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v0/fetch", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !called {
		t.Fatalf("未配置令牌时不应拦截，状态码 = %d", w.Code)
	}
}

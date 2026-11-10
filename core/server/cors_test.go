package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vanadiry/seshat/core/config"
)

func corsFor(bind string, next http.Handler) http.Handler {
	cfg := &config.Config{}
	cfg.Server.BindAddr = bind
	return withCORS(cfg)(next)
}

func TestCORSOriginPolicy(t *testing.T) {
	cases := []struct {
		bind   string
		origin string
		want   int
	}{
		{"127.0.0.1", "", http.StatusOK},
		{"127.0.0.1", "http://127.0.0.1:12500", http.StatusOK},
		{"127.0.0.1", "http://localhost:8000", http.StatusOK},
		{"127.0.0.1", "http://[::1]:12500", http.StatusOK},
		{"127.0.0.1", "http://192.168.1.10:12500", http.StatusForbidden},
		{"127.0.0.1", "https://example.com", http.StatusForbidden},
		{"127.0.0.1", "null", http.StatusForbidden},
		{"127.0.0.1", "http://example.com/path", http.StatusForbidden},
		{"::1", "https://example.com", http.StatusForbidden},
		{"0.0.0.0", "https://example.com", http.StatusOK},
		{"0.0.0.0", "http://192.168.1.10:12500", http.StatusOK},
		{"", "https://example.com", http.StatusOK},
		{"192.168.1.10", "https://example.com", http.StatusOK},
		{"0.0.0.0", "null", http.StatusForbidden},
	}

	for _, c := range cases {
		called := false
		h := corsFor(c.bind, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != c.want {
			t.Errorf("bind=%q origin=%q 状态码 = %d，期望 %d", c.bind, c.origin, w.Code, c.want)
		}
		if c.want == http.StatusOK && !called {
			t.Errorf("bind=%q origin=%q 没有进入 handler", c.bind, c.origin)
		}
		if c.want == http.StatusForbidden && called {
			t.Errorf("bind=%q origin=%q 被拒绝却进入了 handler", c.bind, c.origin)
		}
	}
}

func TestCORSHeaders(t *testing.T) {
	h := corsFor("0.0.0.0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("Allow-Origin = %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Fatalf("Allow-Methods = %q", got)
	}
	allowHeaders := w.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(allowHeaders, "Authorization") || !strings.Contains(allowHeaders, "X-Request-ID") {
		t.Fatalf("Allow-Headers = %q", allowHeaders)
	}
	if got := w.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(got, "X-Request-ID") {
		t.Fatalf("Expose-Headers = %q", got)
	}
	if got := w.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Fatalf("Vary = %q", got)
	}
}

func TestCORSNoOrigin(t *testing.T) {
	h := corsFor("0.0.0.0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("无 Origin 时 Allow-Origin = %q", got)
	}
	if got := w.Header().Get("Vary"); got != "" {
		t.Fatalf("无 Origin 时 Vary = %q", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	called := false
	h := corsFor("127.0.0.1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://127.0.0.1:12500")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("允许的预检状态码 = %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:12500" {
		t.Fatalf("允许的预检 Allow-Origin = %q", got)
	}
	if called {
		t.Fatal("预检进入了 handler")
	}

	req = httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("拒绝的预检状态码 = %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("拒绝的预检 Allow-Origin = %q", got)
	}
	if called {
		t.Fatal("拒绝的预检进入了 handler")
	}
}

func TestBindAllowsAnyOrigin(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1", false},
		{"127.0.0.1:12500", false},
		{"::1", false},
		{"[::1]:12500", false},
		{"localhost", false},
		{"localhost:12500", false},
		{"0.0.0.0", true},
		{"0.0.0.0:12500", true},
		{"::", true},
		{"", true},
		{":12500", true},
		{"192.168.1.10", true},
		{"example.com", true},
	}

	for _, c := range cases {
		if got := bindAllowsAnyOrigin(c.addr); got != c.want {
			t.Errorf("bindAllowsAnyOrigin(%q) = %v，期望 %v", c.addr, got, c.want)
		}
	}
}

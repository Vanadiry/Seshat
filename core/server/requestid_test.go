package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithRequestIDGenerates(t *testing.T) {
	var got string
	h := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = requestIDFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	header := w.Header().Get("X-Request-ID")
	if header == "" {
		t.Fatal("响应头缺少 X-Request-ID")
	}
	if got != header {
		t.Fatalf("上下文 ID %q 与响应头 %q 不一致", got, header)
	}
	if len(header) != 16 {
		t.Fatalf("生成的 ID 长度 = %d", len(header))
	}
}

func TestWithRequestIDReuses(t *testing.T) {
	const want = "client-request-123"
	var got string
	h := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = requestIDFrom(r.Context())
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", want)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if got != want {
		t.Fatalf("上下文 ID = %q，期望 %q", got, want)
	}
	if w.Header().Get("X-Request-ID") != want {
		t.Fatalf("响应头 = %q，期望 %q", w.Header().Get("X-Request-ID"), want)
	}
}

func TestWithRequestIDRejectsInvalid(t *testing.T) {
	for _, invalid := range []string{"bad id", strings.Repeat("a", 65)} {
		h := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Request-ID", invalid)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		got := w.Header().Get("X-Request-ID")
		if got == invalid || !validRequestID(got) {
			t.Fatalf("非法输入 %q 没有被替换，得到 %q", invalid, got)
		}
	}
}

func TestWriteErrorIncludesRequestID(t *testing.T) {
	h := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "未找到")
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v0/x", nil))

	var body struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	header := w.Header().Get("X-Request-ID")
	if body.RequestID == "" || body.RequestID != header {
		t.Fatalf("错误体 request_id = %q，响应头 = %q", body.RequestID, header)
	}
}

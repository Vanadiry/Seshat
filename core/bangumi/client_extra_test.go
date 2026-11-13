package bangumi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGetRawPaged(t *testing.T) {
	var gotPath, gotLimit, gotOffset string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotLimit = r.URL.Query().Get("limit")
		gotOffset = r.URL.Query().Get("offset")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewClient("test", srv.URL, nil, nil)
	if _, err := c.GetRawPaged("v0/x", 20, 40); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v0/x" || gotLimit != "40" || gotOffset != "20" {
		t.Fatalf("请求 = %s limit=%s offset=%s", gotPath, gotLimit, gotOffset)
	}
}

func TestGetImage404NoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := NewClient("test", srv.URL, nil, nil)
	if _, err := c.GetImage("v0/subjects/1/image?type=large"); err == nil {
		t.Fatal("应当返回错误")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("404 不应重试，实际请求 %d 次", got)
	}
}

func TestGetImagePlaceholderNoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Redirect(w, r, "/no_icon", http.StatusFound)
	}))
	defer srv.Close()

	c := NewClient("test", srv.URL, nil, nil)
	_, err := c.GetImage("v0/subjects/1/image")
	if err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("错误 = %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("占位图不应重试，实际请求 %d 次", got)
	}
}

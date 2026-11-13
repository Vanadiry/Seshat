package bangumi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestPostRaw(t *testing.T) {
	var gotMethod, gotPath, gotLimit, gotOffset string
	var gotContentType, gotUserAgent, gotAuth string
	var gotKeyword string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotLimit = r.URL.Query().Get("limit")
		gotOffset = r.URL.Query().Get("offset")
		gotContentType = r.Header.Get("Content-Type")
		gotUserAgent = r.Header.Get("User-Agent")
		gotAuth = r.Header.Get("Authorization")
		var body struct {
			Keyword string `json:"keyword"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("解析请求体失败: %v", err)
		}
		gotKeyword = body.Keyword
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewClient("test-agent", srv.URL, func() string { return "token" }, nil)
	data, err := c.PostRaw("v0/search/subjects?limit=2&offset=1", map[string]any{"keyword": "CLANNAD"})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"status":"ok"}` {
		t.Fatalf("响应 = %s", data)
	}
	if gotMethod != http.MethodPost || gotPath != "/v0/search/subjects" {
		t.Fatalf("请求 = %s %s", gotMethod, gotPath)
	}
	if gotLimit != "2" || gotOffset != "1" || gotKeyword != "CLANNAD" {
		t.Fatalf("参数不对: limit=%s offset=%s keyword=%s", gotLimit, gotOffset, gotKeyword)
	}
	if gotContentType != "application/json" || gotUserAgent != "test-agent" || gotAuth != "Bearer token" {
		t.Fatalf("请求头不对: content-type=%s ua=%s auth=%s", gotContentType, gotUserAgent, gotAuth)
	}
}

func TestPostRawStopsOnBadRequest(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewClient("test", srv.URL, nil, nil)
	if _, err := c.PostRaw("v0/search/subjects", map[string]any{"keyword": "x"}); err == nil {
		t.Fatal("应当返回错误")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("400 不应重试，实际请求 %d 次", got)
	}
}

func TestCancelAbortsRequest(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c := NewClient("test", srv.URL, nil, nil).WithContext(ctx)
	done := make(chan error, 1)
	go func() {
		_, err := c.GetRaw("v0/slow")
		done <- err
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("取消后应返回 context.Canceled，实际为 %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("取消后请求没有及时返回")
	}
}

func TestCancelStopsRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c := NewClient("test", srv.URL, nil, nil).WithContext(ctx)
	cancel()
	if _, err := c.GetRaw("v0/fail"); !errors.Is(err, context.Canceled) {
		t.Fatalf("已取消的上下文应立即返回 context.Canceled，实际为 %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("取消后不应发起请求，实际 %d 次", got)
	}
}

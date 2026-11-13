package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vanadiry/seshat/core/config"
)

func TestCancelEndpoint(t *testing.T) {
	t.Setenv("SESHAT_HOME", t.TempDir())
	r := buildRouter(&config.Config{}, nil)
	p := newProgress(10, "test", "测试")
	defer func() {
		progressMu.Lock()
		delete(progressMap, p.ID)
		progressMu.Unlock()
	}()

	w := httptest.NewRecorder()
	r.mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v0/task/cancel/"+p.ID, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，body = %s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应解析失败: %v", err)
	}
	if body["status"] != "cancelled" {
		t.Fatalf("status = %q", body["status"])
	}
	if !p.Cancelled() {
		t.Fatal("任务应当被标记为已取消")
	}
	select {
	case <-p.Context().Done():
	default:
		t.Fatal("任务上下文应当已取消")
	}
	p.Close()

	w = httptest.NewRecorder()
	r.mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v0/task/cancel/"+p.ID, nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("已结束任务状态码 = %d", w.Code)
	}

	w = httptest.NewRecorder()
	r.mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v0/task/cancel/missing", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("不存在任务状态码 = %d", w.Code)
	}
}

func TestFinishSendsCancelled(t *testing.T) {
	p := newProgress(10, "test", "测试")
	p.Cancel()
	p.Finish()

	var sawCancelled bool
	for ev := range p.Channel {
		var payload map[string]any
		if json.Unmarshal([]byte(ev), &payload) == nil && payload["step"] == "cancelled" {
			sawCancelled = true
		}
	}
	if !sawCancelled {
		t.Fatal("取消的任务应当发送 cancelled 事件")
	}

	progressMu.Lock()
	delete(progressMap, p.ID)
	progressMu.Unlock()
}

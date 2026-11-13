package events

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleSSEReplaysHistory(t *testing.T) {
	InitBus()
	Bus.Info("hello sse")

	srv := httptest.NewServer(http.HandlerFunc(HandleSSE))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "hello sse") {
		t.Fatalf("SSE 首行 = %q", line)
	}
}

func TestHandleSSENilBus(t *testing.T) {
	old := Bus
	Bus = nil
	defer func() { Bus = old }()

	w := httptest.NewRecorder()
	HandleSSE(w, httptest.NewRequest(http.MethodGet, "/api/v0/events", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("未初始化状态码 = %d", w.Code)
	}
}

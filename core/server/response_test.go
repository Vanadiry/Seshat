package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatusTitle(t *testing.T) {
	cases := map[int]string{
		http.StatusBadRequest:            "Bad Request",
		http.StatusNotFound:              "Not Found",
		http.StatusConflict:              "Conflict",
		http.StatusRequestEntityTooLarge: "Payload Too Large",
		http.StatusInternalServerError:   "Internal Server Error",
	}
	for code, want := range cases {
		if got := statusTitle(code); got != want {
			t.Errorf("statusTitle(%d) = %q, 期望 %q", code, got, want)
		}
	}
}

func TestWriteErrorShape(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v0/subjects/999", nil)
	writeError(w, r, http.StatusNotFound, "条目未找到", map[string]any{
		"path":   "/api/v0/subjects/999",
		"method": http.MethodGet,
	})

	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d", w.Code)
	}
	var body struct {
		Title       string         `json:"title"`
		Description string         `json:"description"`
		Details     map[string]any `json:"details"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if body.Title != "Not Found" {
		t.Errorf("title = %q", body.Title)
	}
	if body.Description != "条目未找到" {
		t.Errorf("description = %q", body.Description)
	}
	if body.Details["path"] != "/api/v0/subjects/999" {
		t.Errorf("details = %v", body.Details)
	}
	if strings.Contains(w.Body.String(), `"error"`) {
		t.Errorf("不应再输出过渡 error 字段: %s", w.Body.String())
	}
}

func TestWriteErrorOmitsEmptyDetails(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v0/fetch/subject", nil)
	writeError(w, r, http.StatusBadRequest, "id 不能为空")
	if strings.Contains(w.Body.String(), `"details"`) {
		t.Fatalf("空 details 不应输出: %s", w.Body.String())
	}
}

func TestWriteNotFoundShape(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v0/subjects/999", nil)
	writeNotFound(w, r, "缓存条目不存在")

	var body struct {
		Title       string         `json:"title"`
		Description string         `json:"description"`
		Details     map[string]any `json:"details"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if w.Code != http.StatusNotFound || body.Title != "Not Found" {
		t.Fatalf("状态码 %d，title %q", w.Code, body.Title)
	}
	if body.Details["path"] != "/api/v0/subjects/999" || body.Details["method"] != http.MethodGet {
		t.Fatalf("details = %v", body.Details)
	}
}

func TestDecodeJSONInvalid(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{"))
	var v map[string]any
	if decodeJSON(w, r, &v) {
		t.Fatal("非法 JSON 应当返回 false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d", w.Code)
	}
}

func TestDecodeJSONTooLarge(t *testing.T) {
	w := httptest.NewRecorder()
	body := `{"x":"` + strings.Repeat("a", maxBodySize) + `"}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	limitBody(w, r)
	var v map[string]any
	if decodeJSON(w, r, &v) {
		t.Fatal("超限请求体应当返回 false")
	}
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("状态码 = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"max_bytes"`) {
		t.Fatalf("响应缺少 max_bytes: %s", w.Body.String())
	}
}

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/config"
)

func TestUpstreamSearch(t *testing.T) {
	var gotMethod, gotPath, gotLimit, gotOffset string
	var gotBody struct {
		Keyword string         `json:"keyword"`
		Sort    string         `json:"sort"`
		Filter  map[string]any `json:"filter"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotLimit = r.URL.Query().Get("limit")
		gotOffset = r.URL.Query().Get("offset")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("解析上游请求失败: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":51,"name":"CLANNAD","name_cn":"CLANNAD","date":"2007-10-04","platform":"TV","images":{"large":"x"}}],"total":12}`))
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.Upstream.BaseURL = srv.URL
	h := handleUpstreamSearch(cfg, bangumi.NewClient("test", srv.URL, nil, nil))
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodPost, "/api/v0/upstream/search",
		strings.NewReader(`{"keyword":" CLANNAD ","limit":2,"offset":1}`)))

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
	if gotMethod != http.MethodPost || gotPath != "/v0/search/subjects" {
		t.Fatalf("上游请求 = %s %s", gotMethod, gotPath)
	}
	if gotLimit != "2" || gotOffset != "1" || gotBody.Keyword != "CLANNAD" || gotBody.Sort != "match" {
		t.Fatalf("上游参数不对: limit=%s offset=%s body=%+v", gotLimit, gotOffset, gotBody)
	}
	types, ok := gotBody.Filter["type"].([]any)
	if !ok || len(types) != 1 || types[0] != float64(2) {
		t.Fatalf("上游类型过滤不对: %+v", gotBody.Filter)
	}

	var resp struct {
		Total  int                  `json:"total"`
		Limit  int                  `json:"limit"`
		Offset int                  `json:"offset"`
		Data   []upstreamSearchItem `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 12 || resp.Limit != 2 || resp.Offset != 1 || len(resp.Data) != 1 {
		t.Fatalf("响应不对: %+v", resp)
	}
	if resp.Data[0].ID != 51 || resp.Data[0].NameCN != "CLANNAD" || resp.Data[0].Platform != "TV" {
		t.Fatalf("条目不对: %+v", resp.Data[0])
	}
}

func TestUpstreamSearchValidatesKeyword(t *testing.T) {
	cfg := &config.Config{}
	cfg.Upstream.BaseURL = "http://127.0.0.1:1"
	h := handleUpstreamSearch(cfg, bangumi.NewClient("test", cfg.Upstream.BaseURL, nil, nil))
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodPost, "/api/v0/upstream/search",
		strings.NewReader(`{"keyword":"  "}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d", w.Code)
	}
}

func TestUpstreamSearchReportsUpstreamFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad search", http.StatusBadRequest)
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.Upstream.BaseURL = srv.URL
	h := handleUpstreamSearch(cfg, bangumi.NewClient("test", srv.URL, nil, nil))
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodPost, "/api/v0/upstream/search",
		strings.NewReader(`{"keyword":"CLANNAD"}`)))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("状态码 = %d, body=%s", w.Code, w.Body.String())
	}
}

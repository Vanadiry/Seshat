package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/vanadiry/seshat/core/config"
)

func TestFeaturesFixedList(t *testing.T) {
	t.Setenv("SESHAT_HOME", t.TempDir())
	r := buildRouter(&config.Config{}, nil)

	want := []string{
		"elo",
		"events",
		"fetch",
		"lists",
		"names",
		"stats",
		"tags",
		"task",
		"tracker",
	}
	if got := r.features(); !reflect.DeepEqual(got, want) {
		t.Fatalf("features = %v，期望 %v", got, want)
	}
}

func TestInfoReturnsFeatures(t *testing.T) {
	t.Setenv("SESHAT_HOME", t.TempDir())
	r := buildRouter(&config.Config{}, nil)

	w := httptest.NewRecorder()
	r.mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v0/info", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d", w.Code)
	}

	var body struct {
		Features []string `json:"features"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if !reflect.DeepEqual(body.Features, r.features()) {
		t.Fatalf("响应 features = %v，router = %v", body.Features, r.features())
	}
}

func TestFeatureRejectsEmptyName(t *testing.T) {
	r := newRouter()
	defer func() {
		if recover() == nil {
			t.Fatal("空 feature 名没有 panic")
		}
	}()
	r.feature("", "GET /x", func(http.ResponseWriter, *http.Request) {})
}

func TestFeaturesOnlyIncludesFeatureKind(t *testing.T) {
	r := newRouter()
	h := func(http.ResponseWriter, *http.Request) {}
	r.page("GET /p", h)
	r.core("GET /c", h)
	r.local("GET /l", h)
	r.feature("b", "GET /b", h)
	r.feature("a", "GET /a1", h)
	r.feature("a", "GET /a2", h)

	want := []string{"a", "b"}
	if got := r.features(); !reflect.DeepEqual(got, want) {
		t.Fatalf("features = %v，期望 %v", got, want)
	}
	if r.routes["GET /c"].kind != kindCore {
		t.Fatalf("core 路由的 kind = %v", r.routes["GET /c"].kind)
	}
	if r.routes["GET /l"].kind != kindLocal {
		t.Fatalf("local 路由的 kind = %v", r.routes["GET /l"].kind)
	}
}

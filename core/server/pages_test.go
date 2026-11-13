package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/vanadiry/seshat/core/config"
)

func TestServePage(t *testing.T) {
	fsys := fstest.MapFS{
		"web-dist/subject.html": &fstest.MapFile{Data: []byte("<html>subject</html>")},
	}
	h := servePage(fsys)

	req := httptest.NewRequest(http.MethodGet, "/subject", nil)
	req.SetPathValue("page", "subject")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "subject") {
		t.Fatalf("正常页面状态码 = %d，body = %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/subject.html", nil)
	req.SetPathValue("page", "subject.html")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != "/subject" {
		t.Fatalf("subject.html 重定向 = %d %q", w.Code, w.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodGet, "/index.html", nil)
	req.SetPathValue("page", "index.html")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != "/" {
		t.Fatalf("index.html 重定向 = %d %q", w.Code, w.Header().Get("Location"))
	}
}

func TestServeIndex(t *testing.T) {
	fsys := fstest.MapFS{
		"web-dist/index.html": &fstest.MapFile{Data: []byte("<html>home</html>")},
	}
	h := serveIndex(fsys)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "home") {
		t.Fatalf("首页状态码 = %d", w.Code)
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/other", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("非根路径状态码 = %d", w.Code)
	}
}

func TestServeAppJSInjectsConfig(t *testing.T) {
	t.Setenv("SESHAT_HOME", t.TempDir())
	fsys := fstest.MapFS{
		"web-dist/assets/app.min.js": &fstest.MapFile{Data: []byte("console.log(1)")},
	}
	cfg := &config.Config{}
	cfg.Server.APIToken = "tok"
	h := serveAppJS(cfg, fsys)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/app.min.js", nil))
	body := w.Body.String()
	if !strings.Contains(body, `window.API_TOKEN="tok"`) {
		t.Fatalf("没有注入 API_TOKEN: %s", body)
	}
	if !strings.Contains(body, "console.log(1)") {
		t.Fatal("没有返回脚本内容")
	}
}

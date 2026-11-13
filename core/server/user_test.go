package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func setupUserAs(t *testing.T, username string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	dir := filepath.Join(home, "user", "settings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := fmt.Sprintf(`{"username":%q}`, username)
	if err := os.WriteFile(filepath.Join(dir, "preferences.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupUser(t *testing.T) { setupUserAs(t, "tester") }

func requestWithUser(path, username string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.SetPathValue("username", username)
	return r
}

func writeUserFile(t *testing.T, dd, name, data string) {
	t.Helper()
	dir := filepath.Join(dd, "user")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHandleUserInjectsAvatar(t *testing.T) {
	setupUser(t)
	dd := t.TempDir()
	writeUserFile(t, dd, "info.json", `{"id":1,"username":"tester"}`)

	w := httptest.NewRecorder()
	handleUser(dd)(w, requestWithUser("/api/v0/users/tester", "tester"))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	avatar, ok := body["avatar"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 avatar: %v", body)
	}
	want := "http://example.com/api/v0/users/tester/avatar?type=large"
	for _, key := range []string{"large", "medium", "small"} {
		if avatar[key] != want {
			t.Errorf("avatar.%s = %v，期望 %s", key, avatar[key], want)
		}
	}
}

func TestHandleUserRejectsOtherName(t *testing.T) {
	setupUser(t)
	dd := t.TempDir()
	writeUserFile(t, dd, "info.json", `{"id":1,"username":"tester"}`)
	w := httptest.NewRecorder()
	handleUser(dd)(w, requestWithUser("/api/v0/users/other", "other"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d", w.Code)
	}
}

func TestHandleUserEmptyConfiguredName(t *testing.T) {
	setupUserAs(t, "")
	dd := t.TempDir()
	writeUserFile(t, dd, "info.json", `{"id":1,"username":"tester"}`)
	w := httptest.NewRecorder()
	handleUser(dd)(w, requestWithUser("/api/v0/users/tester", "tester"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d", w.Code)
	}
}

func TestServeUserAvatarIgnoresType(t *testing.T) {
	setupUser(t)
	dd := t.TempDir()
	writeUserFile(t, dd, "large.jpg", "img")

	for _, typ := range []string{"large", "small", "medium"} {
		w := httptest.NewRecorder()
		r := requestWithUser("/api/v0/users/tester/avatar?type="+typ, "tester")
		serveUserAvatar(dd)(w, r)
		if w.Code != http.StatusOK || w.Body.String() != "img" {
			t.Fatalf("type=%s 状态码 %d，内容 %q", typ, w.Code, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	serveUserAvatar(dd)(w, requestWithUser("/api/v0/users/other/avatar", "other"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("其他用户状态码 = %d", w.Code)
	}
}

func TestHandleUserCollectionsRejectsOtherName(t *testing.T) {
	setupUser(t)
	dd := t.TempDir()
	writeUserFile(t, dd, "collections.json", `{"data":[]}`)
	w := httptest.NewRecorder()
	handleUserCollections(dd)(w, requestWithUser("/api/v0/users/other/collections", "other"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d", w.Code)
	}
}

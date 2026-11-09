package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/vanadiry/seshat/core/bangumi"
)

// collectionsPage 生成指定偏移的收藏分页响应
func collectionsPage(offset, total int) []byte {
	end := offset + 50
	if end > total {
		end = total
	}
	type item struct {
		SubjectID   int `json:"subject_id"`
		SubjectType int `json:"subject_type"`
		Type        int `json:"type"`
	}
	data := make([]item, 0, end-offset)
	for i := offset + 1; i <= end; i++ {
		data = append(data, item{SubjectID: i, SubjectType: 2, Type: 1})
	}
	b, _ := json.Marshal(map[string]any{"data": data, "total": total})
	return b
}

func readCollections(t *testing.T, home string) map[string]int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "data", "user", "collections.json"))
	if err != nil {
		t.Fatalf("读取收藏: %v", err)
	}
	var v struct {
		Subjects map[string]int `json:"subjects"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("解析收藏: %v", err)
	}
	return v.Subjects
}

func TestFetchUserCollectionsWritesAllPages(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		w.Header().Set("Content-Type", "application/json")
		w.Write(collectionsPage(offset, 120))
	}))
	defer srv.Close()

	c := bangumi.NewClient("test", srv.URL, nil, nil)
	if err := fetchUserCollections("tester", c, filepath.Join(home, "data")); err != nil {
		t.Fatalf("拉取失败: %v", err)
	}
	if got := len(readCollections(t, home)); got != 120 {
		t.Errorf("收藏条数为 %d，期望 120", got)
	}
}

func TestFetchUserCollectionsKeepsOldFileOnFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if offset > 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(collectionsPage(0, 120))
	}))
	defer srv.Close()

	old := `{"subjects":{"999":1},"updated_at":"2026-01-01T00:00:00Z"}`
	dir := filepath.Join(home, "data", "user")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "collections.json")
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	c := bangumi.NewClient("test", srv.URL, nil, nil)
	if err := fetchUserCollections("tester", c, filepath.Join(home, "data")); err == nil {
		t.Fatal("第二页失败时应返回错误")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取旧缓存: %v", err)
	}
	if string(data) != old {
		t.Errorf("旧缓存被覆盖，内容为 %q", data)
	}
}

func TestFetchUserInfoWritesOnSuccess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":1,"username":"tester"}`))
	}))
	defer srv.Close()

	c := bangumi.NewClient("test", srv.URL, nil, nil)
	if err := fetchUserInfo("tester", c, filepath.Join(home, "data")); err != nil {
		t.Fatalf("拉取失败: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, "data", "user", "info.json"))
	if err != nil {
		t.Fatalf("读取 info.json: %v", err)
	}
	if !strings.Contains(string(data), `"username":"tester"`) {
		t.Errorf("内容不含 username: %s", data)
	}
}

func TestFetchUserInfoKeepsOldFileOnFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SESHAT_HOME", home)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	old := `{"id":9,"username":"old"}`
	dir := filepath.Join(home, "data", "user")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "info.json")
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	c := bangumi.NewClient("test", srv.URL, nil, nil)
	if err := fetchUserInfo("tester", c, filepath.Join(home, "data")); err == nil {
		t.Fatal("上游失败时应返回错误")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取旧文件: %v", err)
	}
	if string(data) != old {
		t.Errorf("旧文件被覆盖，内容为 %q", data)
	}
}

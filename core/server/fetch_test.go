package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/store"
)

func TestPutDataSuccess(t *testing.T) {
	dd := t.TempDir()
	key := store.Key("subjects", 51, "info.json")
	if !putData(dd, key, []byte(`{"id":51}`), nil) {
		t.Fatal("putData 返回失败")
	}
	if _, err := store.Get(dd, key); err != nil {
		t.Fatalf("读取数据: %v", err)
	}
}

func TestPutDataFailure(t *testing.T) {
	dd := t.TempDir()
	// 在 api 目录的位置放一个文件，让 MkdirAll 失败
	if err := os.WriteFile(filepath.Join(dd, "api"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if putData(dd, store.Key("subjects", 51, "info.json"), []byte(`{"id":51}`), nil) {
		t.Fatal("putData 应当返回失败")
	}
}

func writeSubjectCore(t *testing.T, dd string, sid int, info string) {
	t.Helper()
	files := map[string]string{
		"info.json":       info,
		"characters.json": "[]",
		"persons.json":    "[]",
		"subjects.json":   "[]",
	}
	for name, data := range files {
		if err := store.Put(dd, store.Key("subjects", sid, name), []byte(data)); err != nil {
			t.Fatalf("写入 %s: %v", name, err)
		}
	}
}

func TestSubjectCoreIncompleteMissingInfo(t *testing.T) {
	if !subjectCoreIncomplete(t.TempDir(), 51) {
		t.Fatal("缺少文件时应当返回不完整")
	}
}

func TestSubjectCoreIncompleteNoEpisodes(t *testing.T) {
	dd := t.TempDir()
	writeSubjectCore(t, dd, 51, `{"id":51,"total_episodes":0,"eps":0}`)
	if subjectCoreIncomplete(dd, 51) {
		t.Fatal("没有剧集的条目应当视为完整")
	}
}

func TestSubjectCoreIncompleteMissingEpisodes(t *testing.T) {
	dd := t.TempDir()
	writeSubjectCore(t, dd, 51, `{"id":51,"total_episodes":12,"eps":12}`)
	if !subjectCoreIncomplete(dd, 51) {
		t.Fatal("缺失剧集时应当返回不完整")
	}
}

func TestSubjectCoreIncompleteEpsFallback(t *testing.T) {
	dd := t.TempDir()
	writeSubjectCore(t, dd, 51, `{"id":51,"total_episodes":0,"eps":12}`)
	if !subjectCoreIncomplete(dd, 51) {
		t.Fatal("total_episodes 为 0 时应按 eps 判断")
	}
}

func TestSubjectCoreIncompleteWithEpisodes(t *testing.T) {
	dd := t.TempDir()
	writeSubjectCore(t, dd, 51, `{"id":51,"total_episodes":12,"eps":12}`)
	if err := store.Put(dd, store.Key("subjects", 51, "episodes.json"), []byte("[]")); err != nil {
		t.Fatal(err)
	}
	if subjectCoreIncomplete(dd, 51) {
		t.Fatal("剧集文件存在时应当视为完整")
	}
}

func fetchAllServer(t *testing.T, failPath string) *bangumi.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == failPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.URL.Path {
		case "/v0/subjects/51":
			w.Write([]byte(`{"id":51,"total_episodes":1,"eps":1}`))
		case "/v0/subjects/51/characters", "/v0/subjects/51/persons", "/v0/subjects/51/subjects":
			w.Write([]byte(`[]`))
		case "/v0/episodes":
			w.Write([]byte(`{"data":[{"id":1,"type":0}],"total":1}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return bangumi.NewClient("test", srv.URL, nil, nil)
}

func TestFetchAllFailsWhenRequiredFileMissing(t *testing.T) {
	dd := t.TempDir()
	c := fetchAllServer(t, "/v0/subjects/51/characters")
	if fetchAll(51, c, dd, filepath.Join(dd, "images"), nil, 1) {
		t.Fatal("角色列表失败时应返回 false")
	}
}

func TestFetchAllSucceedsWithCoreFiles(t *testing.T) {
	dd := t.TempDir()
	c := fetchAllServer(t, "")
	if !fetchAll(51, c, dd, filepath.Join(dd, "images"), nil, 1) {
		t.Fatal("核心文件齐全时应返回 true")
	}
	if subjectCoreIncomplete(dd, 51) {
		t.Fatal("成功后核心文件应当齐全")
	}
}

func TestFetchAllSucceedsWithoutEpisodes(t *testing.T) {
	dd := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v0/subjects/51":
			w.Write([]byte(`{"id":51,"total_episodes":0,"eps":0}`))
		case "/v0/subjects/51/characters", "/v0/subjects/51/persons", "/v0/subjects/51/subjects":
			w.Write([]byte(`[]`))
		case "/v0/episodes":
			w.Write([]byte(`{"data":[],"total":0}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := bangumi.NewClient("test", srv.URL, nil, nil)
	if !fetchAll(51, c, dd, filepath.Join(dd, "images"), nil, 1) {
		t.Fatal("没有剧集的条目应当返回 true")
	}
	if subjectCoreIncomplete(dd, 51) {
		t.Fatal("没有剧集的条目不应缺少核心文件")
	}
}

func TestFetchAllKeepsOldEpisodesOnPageFailure(t *testing.T) {
	dd := t.TempDir()
	old := `[{"id":999,"type":0}]`
	if err := store.Put(dd, store.Key("subjects", 51, "episodes.json"), []byte(old)); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v0/subjects/51":
			w.Write([]byte(`{"id":51,"total_episodes":150,"eps":150}`))
		case "/v0/subjects/51/characters", "/v0/subjects/51/persons", "/v0/subjects/51/subjects":
			w.Write([]byte(`[]`))
		case "/v0/episodes":
			if r.URL.Query().Get("offset") == "0" {
				w.Write([]byte(`{"data":[{"id":1,"type":0}],"total":150}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := bangumi.NewClient("test", srv.URL, nil, nil)
	if fetchAll(51, c, dd, filepath.Join(dd, "images"), nil, 1) {
		t.Fatal("第二页失败时应返回 false")
	}
	data, err := store.Get(dd, store.Key("subjects", 51, "episodes.json"))
	if err != nil {
		t.Fatalf("读取旧剧集: %v", err)
	}
	if string(data) != old {
		t.Fatalf("旧剧集被覆盖，内容为 %q", data)
	}
}

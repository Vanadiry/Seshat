package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vanadiry/seshat/core/store"
)

func TestDataReaderAllowedPaths(t *testing.T) {
	dd := t.TempDir()
	files := map[string]string{
		store.Key("subjects", 51, "info.json"):       `{"id":51}`,
		store.Key("subjects", 51, "characters.json"): `{"data":[]}`,
		store.Key("subjects", 51, "persons.json"):    `{"data":[]}`,
		store.Key("subjects", 51, "subjects.json"):   `{"data":[]}`,
		store.Key("subjects", 51, "episodes.json"):   `{"data":[]}`,
	}
	for key, body := range files {
		if err := store.Put(dd, key, []byte(body)); err != nil {
			t.Fatalf("写入 %s: %v", key, err)
		}
	}

	cases := []struct {
		path string
		want int
	}{
		{"/api/v0/subjects/51", http.StatusOK},
		{"/api/v0/subjects/51/characters", http.StatusOK},
		{"/api/v0/subjects/51/persons", http.StatusOK},
		{"/api/v0/subjects/51/subjects", http.StatusOK},
		{"/api/v0/subjects/51/info", http.StatusNotFound},
		{"/api/v0/subjects/51/episodes", http.StatusNotFound},
		{"/api/v0/subjects/51/unknown", http.StatusNotFound},
		{"/api/v0/subjects/51/characters/foo", http.StatusNotFound},
		{"/api/v0/subjects/51/characters/", http.StatusOK},
		{"/api/v0/episodes/51", http.StatusNotFound},
		{"/api/v0/subjects/abc", http.StatusNotFound},
		{"/api/v0/subjects/0", http.StatusNotFound},
	}

	h := handleDataReader(dd)
	for _, c := range cases {
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest(http.MethodGet, c.path, nil))
		if w.Code != c.want {
			t.Errorf("%s 状态码 = %d，期望 %d", c.path, w.Code, c.want)
		}
	}
}

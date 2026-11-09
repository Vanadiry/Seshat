package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vanadiry/seshat/core/store"
)

type episodePage struct {
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
	Data   []json.RawMessage `json:"data"`
}

func writeEpisodes(t *testing.T, dd string, sid int, data string) {
	t.Helper()
	if err := store.Put(dd, store.Key("subjects", sid, "episodes.json"), []byte(data)); err != nil {
		t.Fatal(err)
	}
}

func requestEpisodes(t *testing.T, dd, query string) (*httptest.ResponseRecorder, episodePage) {
	t.Helper()
	w := httptest.NewRecorder()
	handleEpisodes(dd)(w, httptest.NewRequest(http.MethodGet, "/api/v0/episodes?"+query, nil))
	var page episodePage
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatalf("解析响应: %v", err)
		}
	}
	return w, page
}

const episodesFixture = `[
  {"id":1,"sort":1,"ep":1,"type":0},
  {"id":2,"sort":2,"ep":2,"type":0},
  {"id":3,"sort":3,"ep":3,"type":1}
]`

func TestEpisodesReturnsAllWithoutLimit(t *testing.T) {
	dd := t.TempDir()
	writeEpisodes(t, dd, 51, episodesFixture)
	w, page := requestEpisodes(t, dd, "subject_id=51")
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d", w.Code)
	}
	if page.Total != 3 || page.Limit != 3 || page.Offset != 0 || len(page.Data) != 3 {
		t.Fatalf("page = %+v", page)
	}
}

func TestEpisodesFiltersType(t *testing.T) {
	dd := t.TempDir()
	writeEpisodes(t, dd, 51, episodesFixture)
	w, page := requestEpisodes(t, dd, "subject_id=51&type=1")
	if w.Code != http.StatusOK || page.Total != 1 || len(page.Data) != 1 {
		t.Fatalf("状态码 %d，page %+v", w.Code, page)
	}
	var meta struct {
		Type int `json:"type"`
	}
	if err := json.Unmarshal(page.Data[0], &meta); err != nil || meta.Type != 1 {
		t.Fatalf("type = %d，err %v", meta.Type, err)
	}
}

func TestEpisodesPagination(t *testing.T) {
	dd := t.TempDir()
	writeEpisodes(t, dd, 51, episodesFixture)
	w, page := requestEpisodes(t, dd, "subject_id=51&limit=1&offset=1")
	if w.Code != http.StatusOK || page.Total != 3 || page.Limit != 1 || page.Offset != 1 || len(page.Data) != 1 {
		t.Fatalf("状态码 %d，page %+v", w.Code, page)
	}
}

func TestEpisodesInvalidParams(t *testing.T) {
	dd := t.TempDir()
	writeEpisodes(t, dd, 51, episodesFixture)
	for _, query := range []string{
		"",
		"subject_id=x",
		"subject_id=51&type=x",
		"subject_id=51&limit=0",
		"subject_id=51&limit=201",
		"subject_id=51&offset=-1",
	} {
		w, _ := requestEpisodes(t, dd, query)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%q 状态码 = %d，期望 400", query, w.Code)
		}
	}
}

func TestEpisodesMissingFile(t *testing.T) {
	dd := t.TempDir()
	w, page := requestEpisodes(t, dd, "subject_id=51")
	if w.Code != http.StatusOK || page.Total != 0 || len(page.Data) != 0 {
		t.Fatalf("状态码 %d，page %+v", w.Code, page)
	}
}

func TestEpisodesCorruptFile(t *testing.T) {
	dd := t.TempDir()
	writeEpisodes(t, dd, 51, `{`)
	w, _ := requestEpisodes(t, dd, "subject_id=51")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("状态码 = %d", w.Code)
	}
}

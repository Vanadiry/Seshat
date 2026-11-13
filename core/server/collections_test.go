package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/vanadiry/seshat/core/store"
)

const collectionsFullFixture = `{
  "updated_at": "2026-11-09T12:00:00+08:00",
  "data": [
    {"subject_id": 1, "subject_type": 2, "type": 1, "rate": 7, "subject": {"id": 1, "name": "a"}},
    {"subject_id": 2, "subject_type": 2, "type": 2, "rate": 8, "subject": {"id": 2, "name": "b"}},
    {"subject_id": 3, "subject_type": 1, "type": 1, "rate": 0, "subject": {"id": 3, "name": "c"}}
  ]
}`

const collectionsSummaryFixture = `{
  "updated_at": "2026-11-09T12:00:00+08:00",
  "data": [
    {"subject_id": 1, "subject_type": 2, "type": 1, "rate": 7},
    {"subject_id": 2, "subject_type": 2, "type": 2, "rate": 8},
    {"subject_id": 3, "subject_type": 1, "type": 1, "rate": 0}
  ]
}`

type collectionListPage struct {
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
	Data   []map[string]any `json:"data"`
}

func writeCollections(t *testing.T, dd, full, summary string) {
	t.Helper()
	dir := filepath.Join(dd, "user")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if full != "" {
		if err := os.WriteFile(filepath.Join(dir, "collections.json"), []byte(full), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if summary != "" {
		if err := os.WriteFile(filepath.Join(dir, "collections_summary.json"), []byte(summary), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func requestCollections(t *testing.T, dd, query string) (*httptest.ResponseRecorder, collectionListPage) {
	t.Helper()
	setupUser(t)
	w := httptest.NewRecorder()
	handleUserCollections(dd)(w, requestWithUser("/api/v0/users/tester/collections?"+query, "tester"))
	var page collectionListPage
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatalf("解析响应: %v", err)
		}
	}
	return w, page
}

func TestUserCollectionsPaged(t *testing.T) {
	dd := t.TempDir()
	writeCollections(t, dd, collectionsFullFixture, collectionsSummaryFixture)
	putImageIndex(t, dd, "subjects_image.json", map[int]store.ImageEntry{1: {Large: "l.jpg"}})

	w, page := requestCollections(t, dd, "")
	if w.Code != http.StatusOK || page.Total != 3 || page.Limit != 3 || page.Offset != 0 || len(page.Data) != 3 {
		t.Fatalf("状态码 %d，page %+v", w.Code, page)
	}
	subject, ok := page.Data[0]["subject"].(map[string]any)
	if !ok {
		t.Fatalf("subject = %v", page.Data[0]["subject"])
	}
	images, ok := subject["images"].(map[string]any)
	if !ok || len(images) != 5 {
		t.Fatalf("subject images = %v", subject["images"])
	}
	if images["large"] != "http://example.com/api/v0/subjects/1/image?type=large" {
		t.Fatalf("large = %v", images["large"])
	}
}

func TestUserCollectionsFilters(t *testing.T) {
	dd := t.TempDir()
	writeCollections(t, dd, collectionsFullFixture, collectionsSummaryFixture)

	cases := []struct {
		query string
		total int
	}{
		{"subject_type=2", 2},
		{"type=2", 1},
		{"subject_type=2&type=1", 1},
	}
	for _, c := range cases {
		w, page := requestCollections(t, dd, c.query)
		if w.Code != http.StatusOK || page.Total != c.total || len(page.Data) != c.total {
			t.Errorf("%q 状态码 %d，page %+v", c.query, w.Code, page)
		}
	}
}

func TestUserCollectionsPagination(t *testing.T) {
	dd := t.TempDir()
	writeCollections(t, dd, collectionsFullFixture, collectionsSummaryFixture)
	w, page := requestCollections(t, dd, "limit=1&offset=1")
	if w.Code != http.StatusOK || page.Total != 3 || page.Limit != 1 || page.Offset != 1 || len(page.Data) != 1 {
		t.Fatalf("状态码 %d，page %+v", w.Code, page)
	}
	if page.Data[0]["subject_id"] != float64(2) {
		t.Fatalf("subject_id = %v", page.Data[0]["subject_id"])
	}
}

func TestUserCollectionsSummary(t *testing.T) {
	dd := t.TempDir()
	writeCollections(t, dd, collectionsFullFixture, collectionsSummaryFixture)
	w, page := requestCollections(t, dd, "view=summary")
	if w.Code != http.StatusOK || page.Total != 3 || len(page.Data) != 3 {
		t.Fatalf("状态码 %d，page %+v", w.Code, page)
	}
	if _, ok := page.Data[0]["subject"]; ok {
		t.Fatalf("摘要不应包含 subject: %v", page.Data[0])
	}
	if page.Data[0]["rate"] != float64(7) {
		t.Fatalf("rate = %v", page.Data[0]["rate"])
	}
}

func TestUserCollectionsSummaryFallback(t *testing.T) {
	dd := t.TempDir()
	writeCollections(t, dd, collectionsFullFixture, "")
	w, page := requestCollections(t, dd, "view=summary")
	if w.Code != http.StatusOK || page.Total != 3 || len(page.Data) != 3 {
		t.Fatalf("状态码 %d，page %+v", w.Code, page)
	}
}

func TestUserCollectionsInvalid(t *testing.T) {
	dd := t.TempDir()
	writeCollections(t, dd, collectionsFullFixture, collectionsSummaryFixture)
	for _, query := range []string{
		"limit=0",
		"limit=101",
		"offset=-1",
		"subject_type=x",
		"type=x",
		"view=foo",
	} {
		w, _ := requestCollections(t, dd, query)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%q 状态码 = %d，期望 400", query, w.Code)
		}
	}
}

func TestUserCollectionsMissingFile(t *testing.T) {
	dd := t.TempDir()
	w, page := requestCollections(t, dd, "")
	if w.Code != http.StatusOK || page.Total != 0 || len(page.Data) != 0 {
		t.Fatalf("状态码 %d，page %+v", w.Code, page)
	}
}

func TestUserCollectionsCorruptFile(t *testing.T) {
	dd := t.TempDir()
	writeCollections(t, dd, `{`, "")
	w, _ := requestCollections(t, dd, "")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("状态码 = %d", w.Code)
	}
}

package Test

import (
	"path/filepath"
	"testing"

	"github.com/vanadiry/seshat/Core/cache"
)

func TestPutGet(t *testing.T) {
	dir := t.TempDir()
	cache.Put(dir, "subjects/51.json", []byte(`{"id":51,"name":"CLANNAD"}`))
	if !cache.Has(dir, "subjects/51.json") {
		t.Error("Has should return true")
	}
	got, _ := cache.Get(dir, "subjects/51.json")
	if string(got) != `{"id":51,"name":"CLANNAD"}` {
		t.Errorf("unexpected: %s", got)
	}
}

func TestHasNotFound(t *testing.T) {
	if cache.Has(t.TempDir(), "nonexistent.json") {
		t.Error("should be false")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	// New cache layout: subjects/{id%10}/{id}/info.json
	cache.Put(dir, "subjects/1/51/info.json", []byte(`{}`))
	cache.Put(dir, "subjects/8/288/info.json", []byte(`{}`))
	list, _ := cache.List(dir, "subjects")
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}
}

func TestStripImages(t *testing.T) {
	data := []byte(`{"id":51,"images":{"large":"http://x.jpg"},"name":"test"}`)
	result := cache.StripImages(data)
	if string(result) == string(data) {
		t.Error("images should be stripped")
	}
}

func TestIndexDir(t *testing.T) {
	d := cache.IndexDir("/data")
	if d != "/data/index" {
		t.Errorf("expected /data/index, got %s", d)
	}
}

func TestIndexFile(t *testing.T) {
	p := cache.IndexFile("/data", "test.json")
	expected := filepath.Join("/data", "index", "test.json")
	if p != expected {
		t.Errorf("expected %s, got %s", expected, p)
	}
}

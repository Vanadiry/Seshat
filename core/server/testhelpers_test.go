package server

import (
	"testing"

	"github.com/vanadiry/seshat/core/store"
)

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

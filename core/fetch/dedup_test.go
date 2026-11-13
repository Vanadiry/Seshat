package fetch

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/vanadiry/seshat/core/bangumi"
)

func TestFetchCharacterInfosDeduplicatesPerTask(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Write([]byte(`{"id":1}`))
	}))
	defer srv.Close()

	dd := t.TempDir()
	c := bangumi.NewClient("test", srv.URL, nil, nil)
	ids := []subjectID{{ID: 1}, {ID: 2}}
	var failed atomic.Bool

	dedup := newFetchDedup()
	fetchCharacterInfos(ids, dd, c, nil, true, 2, dedup, &failed)
	fetchCharacterInfos(ids, dd, c, nil, true, 2, dedup, &failed)
	if got := requests.Load(); got != 2 {
		t.Fatalf("同一任务内应只请求 2 个角色，实际 %d", got)
	}

	fresh := newFetchDedup()
	fetchCharacterInfos(ids, dd, c, nil, true, 2, fresh, &failed)
	if got := requests.Load(); got != 4 {
		t.Fatalf("新任务应重新请求，实际 %d", got)
	}
	if failed.Load() {
		t.Fatal("写入不应失败")
	}
}

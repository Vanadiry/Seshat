package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

// putData 写入数据并在失败时上报，返回是否成功
func putData(dd, key string, data []byte, p *Progress) bool {
	if err := store.Put(dd, key, data); err != nil {
		log.Error("data write failed", "key", key, "err", err)
		if p != nil {
			p.SetError(fmt.Sprintf("数据写入失败 %s: %v", key, err))
		}
		return false
	}
	return true
}

func fetchSubjectList(ids []int, bg *bangumi.Client, dd, imgDir string, p *Progress, concurrency int) []int {
	if len(ids) == 0 {
		return nil
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	var done int
	var mu sync.Mutex
	success := map[int]bool{}
	ctx := context.Background()
	if p != nil {
		ctx = p.Context()
	}
	for _, sid := range ids {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(sid int) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
				}
			}()
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			if fetchAll(sid, bg, dd, imgDir, p, concurrency, true) {
				mu.Lock()
				success[sid] = true
				mu.Unlock()
			}
			if p != nil {
				mu.Lock()
				done++
				p.SendPhaseProgress(done, len(ids), "")
				mu.Unlock()
			}
		}(sid)
	}
	wg.Wait()

	result := make([]int, 0, len(success))
	for _, sid := range ids {
		if success[sid] {
			result = append(result, sid)
		}
	}
	return result
}

// loadOrFetchData 读取已有文件，缺失时拉取并写入
func loadOrFetchData(dd, key, path string, bg *bangumi.Client, overwrite bool) ([]byte, error) {
	if !overwrite && store.Has(dd, key) {
		return store.Get(dd, key)
	}
	data, err := bg.GetRaw(path)
	if err != nil {
		return nil, err
	}
	if err := store.Put(dd, key, store.StripImages(data)); err != nil {
		return nil, err
	}
	return data, nil
}

// subjectCoreIncomplete 检查条目的核心文件是否缺失
func subjectCoreIncomplete(dd string, sid int) bool {
	for _, file := range []string{"info.json", "characters.json", "persons.json", "subjects.json"} {
		if !store.Has(dd, store.Key("subjects", sid, file)) {
			return true
		}
	}
	data, err := store.Get(dd, store.Key("subjects", sid, "info.json"))
	if err != nil {
		return true
	}
	var info struct {
		TotalEpisodes int `json:"total_episodes"`
		Eps           int `json:"eps"`
	}
	if json.Unmarshal(data, &info) != nil {
		return true
	}
	expected := info.TotalEpisodes
	if expected == 0 {
		expected = info.Eps
	}
	if expected > 0 && !store.Has(dd, store.Key("subjects", sid, "episodes.json")) {
		return true
	}
	return false
}

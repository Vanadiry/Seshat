package server

import (
	"net/http"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vanadiry/seshat/Core/bangumi"
	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/config"
	"github.com/vanadiry/seshat/Core/log"
	"os"
	"path/filepath"
)

func extractNameCN(data []byte) string {
	var v struct {
		Infobox []struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		} `json:"infobox"`
	}
	if json.Unmarshal(data, &v) != nil {
		return ""
	}
	for _, ib := range v.Infobox {
		if ib.Key == "简体中文名" {
			var s string
			if json.Unmarshal(ib.Value, &s) == nil {
				return s
			}
			break
		}
	}
	return ""
}

func getRawWithRetry(bg *bangumi.Client, urlPath string, maxRetries int) ([]byte, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		data, err := bg.GetRaw(urlPath)
		if err == nil {
			return data, nil
		}
		if strings.Contains(err.Error(), "HTTP 404") {
			return nil, err
		}
		lastErr = err
		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}
	return nil, lastErr
}

func fetchSubjectList(ids []int, bg *bangumi.Client, dd, imgDir string, p *Progress) {
	if len(ids) == 0 {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency)
	var done int
	var mu sync.Mutex
	for _, sid := range ids {
		wg.Add(1)
		go func(sid int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			fetchAll(sid, bg, dd, imgDir, p)
			if p != nil {
				mu.Lock()
				done++
				p.Send("subjects", done, len(ids), "")
				mu.Unlock()
			}
		}(sid)
	}
	wg.Wait()
}

// fetchAll 拉取动画的所有数据及关联角色/人物/图片。
func fetchAll(sid int, bg *bangumi.Client, dd, imgDir string, p *Progress) {
	log.Info("Fetching subject #%d", sid)
	charListPath := cache.IndexFile(dd, "characters.json")
	persListPath := cache.IndexFile(dd, "persons.json")
	subjListPath := cache.IndexFile(dd, "subjects.json")

	// Subject
	if p != nil { p.Send("subject", 0, 1, "fetching") }
	data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d", sid))
	if err != nil {
		log.Warn("Subject #%d: %v", sid, err)
		if p != nil { p.Send("subject", 1, 1, "error") }
		return
	}
	cache.Put(dd, cache.Key("subjects", sid, "info.json"), cache.StripImages(data))

	// 提取 subject 摘要写入全局 list
	var ss struct {
		ID       int     `json:"id"`
		Name     string  `json:"name"`
		NameCN   string  `json:"name_cn"`
		Rating   struct{ Score float64 `json:"score"` } `json:"rating"`
		Platform string  `json:"platform"`
		Date     string  `json:"date"`
	}
	if json.Unmarshal(data, &ss) == nil && ss.ID > 0 {
		mergeSubjectEntry(subjListPath, cache.SubjectSummary{
			ID: ss.ID, Name: ss.Name, NameCN: ss.NameCN,
			Score: ss.Rating.Score, Platform: ss.Platform, Date: ss.Date,
		})
	}
	if p != nil { p.Send("subject", 1, 1, "done") }

	// Characters & persons lists — 边拉取边写入全局 list
	type fullChar struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Relation string `json:"relation"`
		Actors  []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"actors"`
	}
	type fullPerson struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Relation string `json:"relation"`
	}
	var chars []fullChar
	var persons []fullPerson
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/characters", sid)); err == nil {
			cache.Put(dd, cache.Key("subjects", sid, "characters.json"), cache.StripImages(data))
			json.Unmarshal(data, &chars)
			for _, c := range chars {
				mergeListEntry(charListPath, c.ID, c.Name, "")
				for _, a := range c.Actors {
					if a.ID > 0 {
						mergeListEntry(persListPath, a.ID, a.Name, "")
					}
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/persons", sid)); err == nil {
			cache.Put(dd, cache.Key("subjects", sid, "persons.json"), cache.StripImages(data))
			json.Unmarshal(data, &persons)
			for _, p := range persons {
				mergeListEntry(persListPath, p.ID, p.Name, "")
			}
		}
	}()
	wg.Wait()

	// Character details + Person details + Episodes + Relations — all in parallel
	var wg3 sync.WaitGroup

	// Character details
	type charRef struct{ ID int `json:"id"` }
	var charIDs []charRef
	for _, c := range chars { charIDs = append(charIDs, charRef{ID: c.ID}) }
	wg3.Add(1)
	go func() {
		defer wg3.Done()
		fetchConcurrent(charIDs, func(c charRef) {
			data, err := getRawWithRetry(bg, fmt.Sprintf("v0/characters/%d", c.ID), 3)
			if err != nil {
				if strings.Contains(err.Error(), "HTTP 404") { log.Warn("Character #%d: 404, removing from list", c.ID); removeListEntry(charListPath, c.ID) } else { log.Warn("Character #%d: %v", c.ID, err) }
				return
			}
			cache.Put(dd, cache.Key("characters", c.ID, "info.json"), cache.StripImages(data))
			if cn := extractNameCN(data); cn != "" { mergeListEntry(charListPath, c.ID, "", cn) }
		}, p, "characters", maxConcurrency)
	}()

	// Person details
	type personRef struct{ ID int `json:"id"` }
	personSet := map[int]bool{}
	for _, p := range persons { personSet[p.ID] = true }
	for _, c := range chars { for _, a := range c.Actors { if a.ID > 0 { personSet[a.ID] = true } } }
	var personIDs []personRef
	for id := range personSet { personIDs = append(personIDs, personRef{ID: id}) }
	wg3.Add(1)
	go func() {
		defer wg3.Done()
		fetchConcurrent(personIDs, func(pp personRef) {
			data, err := getRawWithRetry(bg, fmt.Sprintf("v0/persons/%d", pp.ID), 3)
			if err != nil {
				if strings.Contains(err.Error(), "HTTP 404") { log.Warn("Person #%d: 404, removing from list", pp.ID); removeListEntry(persListPath, pp.ID) } else { log.Warn("Person #%d: %v", pp.ID, err) }
				return
			}
			cache.Put(dd, cache.Key("persons", pp.ID, "info.json"), cache.StripImages(data))
			if cn := extractNameCN(data); cn != "" { mergeListEntry(persListPath, pp.ID, "", cn) }
		}, p, "persons", maxConcurrency)
	}()

	// Character subjects
	wg3.Add(1)
	go func() {
		defer wg3.Done()
		fetchConcurrent(charIDs, func(c charRef) {
			data, err := bg.GetRaw(fmt.Sprintf("v0/characters/%d/subjects", c.ID))
			if err != nil { return }
			cache.Put(dd, cache.Key("characters", c.ID, "subjects.json"), cache.StripImages(data))
		}, nil, "", maxConcurrency)
	}()

	// Character persons
	wg3.Add(1)
	go func() {
		defer wg3.Done()
		fetchConcurrent(charIDs, func(c charRef) {
			data, err := bg.GetRaw(fmt.Sprintf("v0/characters/%d/persons", c.ID))
			if err != nil { return }
			cache.Put(dd, cache.Key("characters", c.ID, "persons.json"), cache.StripImages(data))
		}, nil, "", maxConcurrency)
	}()

	// Person subjects
	wg3.Add(1)
	go func() {
		defer wg3.Done()
		fetchConcurrent(personIDs, func(pp personRef) {
			data, err := bg.GetRaw(fmt.Sprintf("v0/persons/%d/subjects", pp.ID))
			if err != nil { return }
			cache.Put(dd, cache.Key("persons", pp.ID, "subjects.json"), cache.StripImages(data))
		}, nil, "", maxConcurrency)
	}()

	// Person characters
	wg3.Add(1)
	go func() {
		defer wg3.Done()
		fetchConcurrent(personIDs, func(pp personRef) {
			data, err := bg.GetRaw(fmt.Sprintf("v0/persons/%d/characters", pp.ID))
			if err != nil { return }
			cache.Put(dd, cache.Key("persons", pp.ID, "characters.json"), cache.StripImages(data))
		}, nil, "", maxConcurrency)
	}()

	// Episodes
	wg3.Add(1)
	go func() {
		defer wg3.Done()
		var allEps []json.RawMessage
		offset := 0
		for {
			data, err := bg.GetRaw(fmt.Sprintf("v0/episodes?subject_id=%d&limit=100&offset=%d", sid, offset))
			if err != nil { break }
			var page struct {
				Data  []json.RawMessage `json:"data"`
				Total int               `json:"total"`
			}
			if json.Unmarshal(data, &page) == nil && len(page.Data) > 0 {
				allEps = append(allEps, page.Data...)
				if len(allEps) >= page.Total { break }
				offset += 100
			} else { break }
		}
		if len(allEps) > 0 {
			result, _ := json.Marshal(allEps)
			cache.Put(dd, cache.Key("subjects", sid, "episodes.json"), cache.StripImages(result))
		}
	}()

	// Relations
	wg3.Add(1)
	go func() {
		defer wg3.Done()
		if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/subjects", sid)); err == nil {
			cache.Put(dd, cache.Key("subjects", sid, "subjects.json"), cache.StripImages(data))
		}
	}()
	wg3.Wait()

	log.Info("Subject #%d API fetch done", sid)
}

// Phase 2: Build index files from all cached API data.
func fetchUserCollections(username string, bg *bangumi.Client, dd string) {
	log.Info("Fetching collections for %s", username)

	all := map[string]int{}

	offset := 0
	for {
		data, err := bg.GetRawPaged(fmt.Sprintf("v0/users/%s/collections", username), offset, 50)
		if err != nil {
			log.Warn("user collections offset %d: %v", offset, err)
			break
		}
		var resp struct {
			Data []struct {
				SubjectID   int `json:"subject_id"`
				SubjectType int `json:"subject_type"`
				Type        int `json:"type"`
			} `json:"data"`
			Total int `json:"total"`
		}
		json.Unmarshal(data, &resp)
		for _, d := range resp.Data {
			if d.SubjectType == 2 {
				all[strconv.Itoa(d.SubjectID)] = d.Type
			}
		}
		if offset+50 >= resp.Total { break }
		offset += 50
	}

	path := filepath.Join(config.Dir(), "tracker", fmt.Sprintf("_user-%s.json", username))
	os.MkdirAll(filepath.Dir(path), 0o755)
	result, _ := json.MarshalIndent(map[string]any{
		"auto":       true,
		"username":   username,
		"updated_at": time.Now().Format(time.RFC3339),
		"subjects":   all,
	}, "", "  ")
	os.WriteFile(path, result, 0o644)
	log.Info("User collections for %s saved (%d items)", username, len(all))
}

// countTrackerTotal 统计所有 tracker 中的 subject 总数。
func fetchConcurrent[T any](items []T, fn func(T), p *Progress, stage string, concurrency int) {
	if len(items) == 0 {
		return
	}
	if concurrency < 1 {
		concurrency = 32
	}
	if p != nil {
		p.Send(stage, 0, len(items), "fetching")
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var done int
	var mu sync.Mutex
	for _, item := range items {
		wg.Add(1)
		go func(item T) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			fn(item)
			if p != nil {
				mu.Lock()
				done++
				p.Send(stage, done, len(items), "")
				mu.Unlock()
			}
		}(item)
	}
	wg.Wait()
}

// ── Helpers ──

func handleFetchAll(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
		if taskLocked() { http.Error(w, `{"error":"a task is already running"}`, 409); return }
		p := newProgress(countTrackerTotal(cfg), "fetch_all", "刷新全部")
		go func() {
			refreshAllTrackers(cfg, bg, dd, imgDir, p)
			p.Send("complete", p.Total, p.Total, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "task_id": p.ID})
	}
}

func handleFetchDeep(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
		if taskLocked() { http.Error(w, `{"error":"a task is already running"}`, 409); return }
		p := newProgress(countTrackerTotal(cfg), "rebuild_all", "重建全部")
		go func() {
			forceRefresh(cfg, bg, dd, imgDir, p)
			p.Send("complete", p.Total, p.Total, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "deep refresh", "task_id": p.ID})
	}
}

func handleFetchTracker(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
		if taskLocked() { http.Error(w, `{"error":"a task is already running"}`, 409); return }
		var req struct{ Names []string `json:"names"` }
		json.NewDecoder(r.Body).Decode(&req)
		for _, n := range req.Names {
			if !validTrackerName(n) {
				writeJSON(w, map[string]any{"error": "invalid tracker name: " + n})
				return
			}
		}
		if countTrackerNames(cfg, req.Names) == 0 {
			writeJSON(w, map[string]any{"error": "tracker not found or empty: " + strings.Join(req.Names, ", ")})
			return
		}
		p := newProgress(countTrackerNames(cfg, req.Names), "fetch_tracker", "拉取 Tracker: "+strings.Join(req.Names, ", "))
		go func() {
			refreshTrackers(cfg, bg, dd, imgDir, req.Names, p)
			p.Send("complete", p.Total, p.Total, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "names": req.Names, "task_id": p.ID})
	}
}

func handleFetchUser(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
		uname := cfg.User.Username
		if uname == "" { http.Error(w, `{"error":"username not configured"}`, 400); return }
		if taskLocked() { http.Error(w, `{"error":"a task is already running"}`, 409); return }
		p := newProgress(1, "fetch_user", "拉取用户收藏")
		go func() {
			fetchUserCollections(uname, bg, dd)
			p.Send("complete", 1, 1, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "username": uname, "task_id": p.ID})
	}
}

func handleFetchSubject(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
		var req struct {
			ID  int   `json:"id"`
			IDs []int `json:"ids"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		ids := req.IDs
		if req.ID != 0 { ids = []int{req.ID} }
		if len(ids) == 0 { http.Error(w, `{"error":"id or ids required"}`, 400); return }
		if taskLocked() { http.Error(w, `{"error":"a task is already running"}`, 409); return }
		var idStrs []string
		for _, sid := range ids { idStrs = append(idStrs, strconv.Itoa(sid)) }
		p := newProgress(len(ids), "fetch_subject", "拉取动画 #"+strings.Join(idStrs, ", "))
		for _, sid := range ids { addToSeshatTracker(cfg, sid) }
		go func() {
			fetchSubjectList(ids, bg, dd, imgDir, p)
			p.Send("phase", 2, 3, "building indexes")
			buildIndexes(dd, p)
			p.Send("phase", 3, 3, "downloading images")
			downloadImages(dd, bg, p)
			p.Send("complete", len(ids), len(ids), "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "count": len(ids), "task_id": p.ID})
	}
}

func handleFetchUpdate(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
		newIDs := diffTrackerIDs(cfg, dd)
		if len(newIDs) == 0 { writeJSON(w, map[string]any{"status": "up-to-date", "count": 0}); return }
		if taskLocked() { http.Error(w, `{"error":"a task is already running"}`, 409); return }
		var idStrs []string
		for _, sid := range newIDs { idStrs = append(idStrs, strconv.Itoa(sid)) }
		p := newProgress(len(newIDs), "fetch_update", "增量更新 "+strconv.Itoa(len(newIDs))+" 部")
		for _, sid := range newIDs { addToSeshatTracker(cfg, sid) }
		go func() {
			fetchSubjectList(newIDs, bg, dd, imgDir, p)
			p.Send("phase", 2, 3, "building indexes")
			buildIndexes(dd, p)
			p.Send("phase", 3, 3, "downloading images")
			downloadImages(dd, bg, p)
			p.Send("complete", len(newIDs), len(newIDs), "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "count": len(newIDs), "task_id": p.ID})
	}
}

func handleFetchImages(cfg *config.Config, bg *bangumi.Client, dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		imgDir := filepath.Join(dd, "images")
		os.RemoveAll(imgDir)
		os.MkdirAll(imgDir, 0o755)
		if len(noImageData) > 0 && noImagePath != "" { os.WriteFile(noImagePath, noImageData, 0o644) }
		for _, name := range []string{"subjects_image.json", "characters_image.json", "persons_image.json"} {
			os.WriteFile(cache.IndexFile(dd, name), []byte("{}"), 0o644)
		}
		total := len(loadNameList(cache.IndexFile(dd, "subjects.json"))) +
			len(loadNameList(cache.IndexFile(dd, "characters.json"))) +
			len(loadNameList(cache.IndexFile(dd, "persons.json")))
		if taskLocked() { http.Error(w, `{"error":"a task is already running"}`, 409); return }
		p := newProgress(total, "rebuild_images", "重建图像")
		go func() {
			downloadImages(dd, bg, p)
			p.Send("complete", total, total, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "downloading", "count": total, "task_id": p.ID})
	}
}

func handleFetchIndex(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if taskLocked() { http.Error(w, `{"error":"a task is already running"}`, 409); return }
		p := newProgress(4, "rebuild_index", "重建索引")
		go func() {
			rebuildFromScan(dd, p)
			p.Send("complete", 4, 4, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "rebuilding", "task_id": p.ID})
	}
}

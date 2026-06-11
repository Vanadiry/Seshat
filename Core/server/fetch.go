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

	// Subject
	if p != nil { p.Send("subject", 0, 1, "fetching") }
	data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d", sid))
	if err != nil {
		log.Warn("Subject #%d: %v", sid, err)
		if p != nil {
			msg := err.Error()
			if strings.Contains(msg, "令牌") || strings.Contains(msg, "HTTP 4") {
				p.SetError(msg)
			}
			p.Send("subject", 1, 1, "error")
		}
		return
	}
	cache.Put(dd, cache.Key("subjects", sid, "info.json"), cache.StripImages(data))
	if p != nil { p.Send("subject", 1, 1, "done") }

	// Characters & persons lists
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
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/persons", sid)); err == nil {
			cache.Put(dd, cache.Key("subjects", sid, "persons.json"), cache.StripImages(data))
			json.Unmarshal(data, &persons)
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
				log.Warn("Character #%d: %v", c.ID, err); if p != nil && (strings.Contains(err.Error(), "令牌") || strings.Contains(err.Error(), "HTTP 4")) { p.SetError(err.Error()) }
				return
			}
			cache.Put(dd, cache.Key("characters", c.ID, "info.json"), cache.StripImages(data))
		}, nil, "", maxConcurrency)
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
				log.Warn("Person #%d: %v", pp.ID, err); if p != nil && (strings.Contains(err.Error(), "令牌") || strings.Contains(err.Error(), "HTTP 4")) { p.SetError(err.Error()) }
				return
			}
			cache.Put(dd, cache.Key("persons", pp.ID, "info.json"), cache.StripImages(data))
		}, nil, "", maxConcurrency)
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
	var ids []int

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
				ids = append(ids, d.SubjectID)
			}
		}
		if offset+50 >= resp.Total { break }
		offset += 50
	}

	// Save collections for frontend display
	userDir := filepath.Join(config.Dir(), "user", "info")
	os.MkdirAll(userDir, 0o755)
	collData, _ := json.Marshal(map[string]any{"subjects": all, "updated_at": time.Now().Format(time.RFC3339)})
	os.WriteFile(filepath.Join(userDir, "collections.json"), collData, 0o644)

	log.Info("User collections for %s saved (%d items)", username, len(all))
}

func fetchUserInfo(username string, bg *bangumi.Client, dd string) {
	log.Info("Fetching user info for %s", username)
	userDir := filepath.Join(config.Dir(), "user", "info")
	os.MkdirAll(userDir, 0o755)
	data, err := bg.GetRaw(fmt.Sprintf("v0/users/%s", username))
	if err != nil {
		log.Warn("User info %s: %v", username, err)
		return
	}
	var userData map[string]any
	json.Unmarshal(data, &userData)
	delete(userData, "avatar")
	clean, _ := json.Marshal(userData)
	os.WriteFile(filepath.Join(userDir, "info.json"), clean, 0o644)
}

func fetchUserAvatar(username string, bg *bangumi.Client, dd string) {
	log.Info("Fetching user avatar for %s", username)
	userDir := filepath.Join(config.Dir(), "user", "info")
	os.MkdirAll(userDir, 0o755)
	imgData, err := bg.GetImage(fmt.Sprintf("v0/users/%s/avatar?type=large", username))
	if err != nil {
		log.Warn("User avatar %s: %v", username, err)
		return
	}
	ext := ".jpg"
	if len(imgData) > 0 && imgData[0] == 0x89 {
		ext = ".png"
	}
	os.WriteFile(filepath.Join(userDir, "large"+ext), imgData, 0o644)
	log.Info("User avatar for %s saved", username)
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
		if taskLocked() { http.Error(w, `{"error":"已有任务正在运行，请等待完成"}`, 409); return }
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
		if taskLocked() { http.Error(w, `{"error":"已有任务正在运行，请等待完成"}`, 409); return }
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
		if taskLocked() { http.Error(w, `{"error":"已有任务正在运行，请等待完成"}`, 409); return }
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
		pref, _ := config.LoadPreferences()
	if pref == nil { pref = &config.DefaultPreferences }
	uname := pref.Username
		if uname == "" { http.Error(w, `{"error":"username not configured"}`, 400); return }
		if taskLocked() { http.Error(w, `{"error":"已有任务正在运行，请等待完成"}`, 409); return }
		p := newProgress(3, "fetch_user", "拉取用户数据")
		go func() {
			p.SetPhase(1, 3, "拉取用户信息")
			p.Send("user_info", 0, 1, "")
			fetchUserInfo(uname, bg, dd)
			p.Send("user_info", 1, 1, "")

			p.SetPhase(2, 3, "拉取用户头像")
			p.Send("user_avatar", 0, 1, "")
			fetchUserAvatar(uname, bg, dd)
			p.Send("user_avatar", 1, 1, "")

			p.SetPhase(3, 3, "拉取用户收藏")
			p.Send("user_collections", 0, 1, "")
			fetchUserCollections(uname, bg, dd)
			p.Send("user_collections", 1, 1, "")

			p.Send("complete", 3, 3, "")
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
		if taskLocked() { http.Error(w, `{"error":"已有任务正在运行，请等待完成"}`, 409); return }
		var idStrs []string
		for _, sid := range ids { idStrs = append(idStrs, strconv.Itoa(sid)) }
		p := newProgress(len(ids), "fetch_subject", "拉取动画 #"+strings.Join(idStrs, ", "))
		for _, sid := range ids { addToSeshatTracker(cfg, sid) }
		go func() {
			p.SetPhase(1, 5, "拉取动画数据")
			fetchSubjectList(ids, bg, dd, imgDir, p)
			downloadImagesScoped(dd, bg, p, 2, 5, ids)
			p.SetPhase(5, 5, "建立索引")
			buildIndexes(dd, p)
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
		if taskLocked() { http.Error(w, `{"error":"已有任务正在运行，请等待完成"}`, 409); return }
		phases := 5
		if len(newIDs) == 0 { phases = 1 }
		p := newProgress(phases, "fetch_update", "增量更新")
		go func() {
			if len(newIDs) > 0 {
				p.SetPhase(1, phases, "拉取动画数据")
				fetchSubjectList(newIDs, bg, dd, imgDir, p)
				downloadImagesScoped(dd, bg, p, 2, phases, newIDs)
				p.SetPhase(phases, phases, "建立索引")
				buildIndexes(dd, p)
			}
			p.Send("complete", phases, phases, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "count": len(newIDs), "task_id": p.ID})
	}
}

func handleFetchIndex(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if taskLocked() { http.Error(w, `{"error":"已有任务正在运行，请等待完成"}`, 409); return }
		p := newProgress(5, "rebuild_index", "重建索引")
		go func() {
			buildIndexes(dd, p)
			p.Send("complete", 5, 5, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "rebuilding", "task_id": p.ID})
	}
}

func handleFetchGap(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
		if taskLocked() { http.Error(w, `{"error":"已有任务正在运行，请等待完成"}`, 409); return }
		allIDs := collectAllSubjectIDs(dd)
		if len(allIDs) == 0 { writeJSON(w, map[string]any{"status": "done", "count": 0}); return }
		p := newProgress(len(allIDs), "fetch_gap", "补充数据")
		go func() {
			var done int
			for _, sid := range allIDs {
				// Check characters
				if data, err := os.ReadFile(filepath.Join(cache.Dir(dd), cache.Key("subjects", sid, "characters.json"))); err == nil {
					var chars []struct{ ID int `json:"id"` }
					if json.Unmarshal(data, &chars) == nil {
						for _, c := range chars {
							if _, err := os.Stat(filepath.Join(cache.Dir(dd), cache.Key("characters", c.ID, "info.json"))); os.IsNotExist(err) {
								if d, e := bg.GetRaw(fmt.Sprintf("v0/characters/%d", c.ID)); e == nil {
									cache.Put(dd, cache.Key("characters", c.ID, "info.json"), cache.StripImages(d))
								}
							}
						}
					}
				}
				// Check persons
				if data, err := os.ReadFile(filepath.Join(cache.Dir(dd), cache.Key("subjects", sid, "persons.json"))); err == nil {
					var persons []struct{ ID int `json:"id"` }
					if json.Unmarshal(data, &persons) == nil {
						for _, pp := range persons {
							if _, err := os.Stat(filepath.Join(cache.Dir(dd), cache.Key("persons", pp.ID, "info.json"))); os.IsNotExist(err) {
								if d, e := bg.GetRaw(fmt.Sprintf("v0/persons/%d", pp.ID)); e == nil {
									cache.Put(dd, cache.Key("persons", pp.ID, "info.json"), cache.StripImages(d))
								}
							}
						}
					}
				}
				done++
				p.Send("gap", done, len(allIDs), "")
			}
			fillImageGaps(dd, bg, p)
			downloadImages(dd, bg, p, 0, 0)
			buildIndexes(dd, p)
			p.Send("complete", len(allIDs), len(allIDs), "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "count": len(allIDs), "task_id": p.ID})
	}
}

func handleFetchMeta(cfg *config.Config, bg *bangumi.Client, dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" { http.Error(w, `{"error":"base_url not configured"}`, 400); return }
		if taskLocked() { http.Error(w, `{"error":"已有任务正在运行，请等待完成"}`, 409); return }
		allIDs := collectAllSubjectIDs(dd)
		charIDs := collectNameIDs(cache.IndexFile(dd, "characters.json"))
		persIDs := collectNameIDs(cache.IndexFile(dd, "persons.json"))
		total := len(allIDs) + len(charIDs) + len(persIDs)
		p := newProgress(total, "fetch_meta", "刷新元数据")
		go func() {
			var done int
			var mu sync.Mutex
			var wg sync.WaitGroup
			sem := make(chan struct{}, maxConcurrency)

			process := func(kind string, id int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				url := fmt.Sprintf("v0/%ss/%d", kind, id)
				data, err := bg.GetRaw(url)
				if err != nil {
					log.Warn("Meta %s #%d: %v", kind, id, err)
				} else {
					cache.Put(dd, cache.Key(kind+"s", id, "info.json"), cache.StripImages(data))
				}
				mu.Lock()
				done++
				p.Send("meta", done, total, "")
				mu.Unlock()
			}

			for _, id := range allIDs { wg.Add(1); go process("subject", id) }
			for _, id := range charIDs { wg.Add(1); go process("character", id) }
			for _, id := range persIDs { wg.Add(1); go process("person", id) }
			wg.Wait()

			buildIndexes(dd, p)
			p.Send("complete", total, total, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "count": total, "task_id": p.ID})
	}
}

func collectAllSubjectIDs(dd string) []int {
	return collectNameIDs(cache.IndexFile(dd, "subjects.json"))
}

func collectNameIDs(path string) []int {
	list := loadNameList(path)
	ids := make([]int, len(list))
	for i, e := range list { ids[i] = e.ID }
	return ids
}

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/cache"
	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
	"os"
	"path/filepath"
)

// putCache 写入缓存并在失败时上报，返回是否成功
func putCache(dd, key string, data []byte, p *Progress) bool {
	if err := cache.Put(dd, key, data); err != nil {
		log.Error("cache put failed", "key", key, "err", err)
		if p != nil {
			p.SetError(fmt.Sprintf("缓存写入失败 %s: %v", key, err))
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
	for _, sid := range ids {
		wg.Add(1)
		go func(sid int) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
				}
			}()
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if fetchAll(sid, bg, dd, imgDir, p, concurrency) {
				mu.Lock()
				success[sid] = true
				mu.Unlock()
			}
			if p != nil {
				mu.Lock()
				done++
				p.Send("subjects", done, len(ids), "")
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

// fetchAll 拉取条目全部数据及关联角色/人物/图片
func fetchAll(sid int, bg *bangumi.Client, dd, imgDir string, p *Progress, concurrency int) bool {
	log.Info("fetching subject", "id", sid)
	var writeFailed atomic.Bool
	var requiredFailed atomic.Bool

	// 条目
	if p != nil {
		p.Send("subject", 0, 1, "fetching")
	}
	data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d", sid))
	if err != nil {
		log.Warn("subject fetch failed", "id", sid, "err", err)
		if p != nil {
			p.SetError(err.Error())
			p.Send("subject", 1, 1, "error")
		}
		return false
	}
	if !putCache(dd, cache.Key("subjects", sid, "info.json"), cache.StripImages(data), p) {
		if p != nil {
			p.Send("subject", 1, 1, "error")
		}
		return false
	}
	if p != nil {
		p.Send("subject", 1, 1, "done")
	}

	// 角色与人物列表
	type fullChar struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Relation string `json:"relation"`
		Actors   []struct {
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
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
		defer wg.Done()
		data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/characters", sid))
		if err != nil {
			log.Warn("subject characters fetch failed", "subject", sid, "err", err)
			requiredFailed.Store(true)
			if p != nil {
				p.SetError(fmt.Sprintf("角色列表拉取失败 #%d: %v", sid, err))
			}
			return
		}
		if !putCache(dd, cache.Key("subjects", sid, "characters.json"), cache.StripImages(data), p) {
			writeFailed.Store(true)
			return
		}
		if err := json.Unmarshal(data, &chars); err != nil {
			log.Warn("unmarshal chars failed", "subject", sid, "err", err)
			requiredFailed.Store(true)
			if p != nil {
				p.SetError(fmt.Sprintf("角色列表解析失败 #%d: %v", sid, err))
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
		defer wg.Done()
		data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/persons", sid))
		if err != nil {
			log.Warn("subject persons fetch failed", "subject", sid, "err", err)
			requiredFailed.Store(true)
			if p != nil {
				p.SetError(fmt.Sprintf("人物列表拉取失败 #%d: %v", sid, err))
			}
			return
		}
		if !putCache(dd, cache.Key("subjects", sid, "persons.json"), cache.StripImages(data), p) {
			writeFailed.Store(true)
			return
		}
		if err := json.Unmarshal(data, &persons); err != nil {
			log.Warn("unmarshal persons failed", "subject", sid, "err", err)
			requiredFailed.Store(true)
			if p != nil {
				p.SetError(fmt.Sprintf("人物列表解析失败 #%d: %v", sid, err))
			}
		}
	}()
	wg.Wait()

	// 角色详情、人物详情、剧集、关联条目并行拉取
	var wg3 sync.WaitGroup

	// 角色详情
	type charRef struct {
		ID int `json:"id"`
	}
	var charIDs []charRef
	for _, c := range chars {
		charIDs = append(charIDs, charRef{ID: c.ID})
	}
	wg3.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
		defer wg3.Done()
		fetchConcurrent(charIDs, func(c charRef) {
			data, err := bg.GetRaw(fmt.Sprintf("v0/characters/%d", c.ID))
			if err != nil {
				log.Warn("character fetch failed", "id", c.ID, "err", err)
				p.SetError(err.Error())
				return
			}
			if !putCache(dd, cache.Key("characters", c.ID, "info.json"), cache.StripImages(data), p) {
				writeFailed.Store(true)
			}
		}, nil, "", concurrency)
	}()

	// 人物详情
	type personRef struct {
		ID int `json:"id"`
	}
	personSet := map[int]bool{}
	for _, p := range persons {
		personSet[p.ID] = true
	}
	for _, c := range chars {
		for _, a := range c.Actors {
			if a.ID > 0 {
				personSet[a.ID] = true
			}
		}
	}
	var personIDs []personRef
	for id := range personSet {
		personIDs = append(personIDs, personRef{ID: id})
	}
	log.Debug("subject details", "id", sid, "chars", len(charIDs), "persons_list", len(persons), "persons_total", len(personIDs))

	var allEps []json.RawMessage
	wg3.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
		defer wg3.Done()
		fetchConcurrent(personIDs, func(pp personRef) {
			data, err := bg.GetRaw(fmt.Sprintf("v0/persons/%d", pp.ID))
			if err != nil {
				log.Warn("person fetch failed", "id", pp.ID, "err", err)
				p.SetError(err.Error())
				return
			}
			if !putCache(dd, cache.Key("persons", pp.ID, "info.json"), cache.StripImages(data), p) {
				writeFailed.Store(true)
			}
		}, nil, "", concurrency)
	}()

	// 角色出演条目
	wg3.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
		defer wg3.Done()
		fetchConcurrent(charIDs, func(c charRef) {
			data, err := bg.GetRaw(fmt.Sprintf("v0/characters/%d/subjects", c.ID))
			if err != nil {
				return
			}
			if !putCache(dd, cache.Key("characters", c.ID, "subjects.json"), cache.StripImages(data), p) {
				writeFailed.Store(true)
			}
		}, nil, "", concurrency)
	}()

	// 角色相关人员
	wg3.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
		defer wg3.Done()
		fetchConcurrent(charIDs, func(c charRef) {
			data, err := bg.GetRaw(fmt.Sprintf("v0/characters/%d/persons", c.ID))
			if err != nil {
				return
			}
			if !putCache(dd, cache.Key("characters", c.ID, "persons.json"), cache.StripImages(data), p) {
				writeFailed.Store(true)
			}
		}, nil, "", concurrency)
	}()

	// 人物参与条目
	wg3.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
		defer wg3.Done()
		fetchConcurrent(personIDs, func(pp personRef) {
			data, err := bg.GetRaw(fmt.Sprintf("v0/persons/%d/subjects", pp.ID))
			if err != nil {
				return
			}
			if !putCache(dd, cache.Key("persons", pp.ID, "subjects.json"), cache.StripImages(data), p) {
				writeFailed.Store(true)
			}
		}, nil, "", concurrency)
	}()

	// 人物出演角色
	wg3.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
		defer wg3.Done()
		fetchConcurrent(personIDs, func(pp personRef) {
			data, err := bg.GetRaw(fmt.Sprintf("v0/persons/%d/characters", pp.ID))
			if err != nil {
				return
			}
			if !putCache(dd, cache.Key("persons", pp.ID, "characters.json"), cache.StripImages(data), p) {
				writeFailed.Store(true)
			}
		}, nil, "", concurrency)
	}()

	// 剧集
	wg3.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
		defer wg3.Done()
		offset := 0
		complete := true
		for {
			data, err := bg.GetRaw(fmt.Sprintf("v0/episodes?subject_id=%d&limit=100&offset=%d", sid, offset))
			if err != nil {
				log.Warn("episodes fetch failed", "subject", sid, "offset", offset, "err", err)
				requiredFailed.Store(true)
				complete = false
				break
			}
			var page struct {
				Data  []json.RawMessage `json:"data"`
				Total int               `json:"total"`
			}
			if err := json.Unmarshal(data, &page); err != nil {
				log.Warn("episodes parse failed", "subject", sid, "offset", offset, "err", err)
				requiredFailed.Store(true)
				complete = false
				break
			}
			if len(page.Data) == 0 {
				if page.Total > 0 {
					log.Warn("episodes empty page", "subject", sid, "offset", offset, "total", page.Total)
					requiredFailed.Store(true)
					complete = false
				}
				break
			}
			allEps = append(allEps, page.Data...)
			if len(allEps) >= page.Total {
				break
			}
			offset += 100
		}
		if !complete {
			log.Warn("episodes incomplete, keep old file", "subject", sid, "fetched", len(allEps))
			return
		}
		if len(allEps) > 0 {
			result, err := json.Marshal(allEps)
			if err != nil {
				log.Error("marshal episodes failed", "subject", sid, "err", err)
				requiredFailed.Store(true)
				events.Bus.Error(fmt.Sprintf("剧集数据序列化失败 #%d", sid))
				return
			}
			if !putCache(dd, cache.Key("subjects", sid, "episodes.json"), cache.StripImages(result), p) {
				writeFailed.Store(true)
			}
		}
	}()

	// 关联条目
	wg3.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
			}
		}()
		defer wg3.Done()
		if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/subjects", sid)); err == nil {
			if !putCache(dd, cache.Key("subjects", sid, "subjects.json"), cache.StripImages(data), p) {
				writeFailed.Store(true)
			}
		} else {
			log.Warn("subject relations fetch failed", "subject", sid, "err", err)
			requiredFailed.Store(true)
		}
	}()
	wg3.Wait()

	if writeFailed.Load() || requiredFailed.Load() {
		log.Warn("subject fetch incomplete", "id", sid, "write_failed", writeFailed.Load(), "required_failed", requiredFailed.Load())
		return false
	}
	log.Info("subject fetch done", "id", sid, "chars", len(charIDs), "persons", len(personIDs), "episodes", len(allEps))
	return true
}

// subjectCoreIncomplete 检查条目的核心文件是否缺失
func subjectCoreIncomplete(dd string, sid int) bool {
	for _, file := range []string{"info.json", "characters.json", "persons.json", "subjects.json"} {
		if !cache.Has(dd, cache.Key("subjects", sid, file)) {
			return true
		}
	}
	data, err := cache.Get(dd, cache.Key("subjects", sid, "info.json"))
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
	if expected > 0 && !cache.Has(dd, cache.Key("subjects", sid, "episodes.json")) {
		return true
	}
	return false
}

// 拉取用户收藏，失败时不做写入
func fetchUserCollections(username string, bg *bangumi.Client, dd string) error {
	log.Info("fetching collections", "user", username)

	all := map[string]int{}

	offset := 0
	for {
		data, err := bg.GetRawPaged(fmt.Sprintf("v0/users/%s/collections", username), offset, 50)
		if err != nil {
			return fmt.Errorf("拉取收藏失败，偏移 %d: %w", offset, err)
		}
		var resp struct {
			Data []struct {
				SubjectID   int `json:"subject_id"`
				SubjectType int `json:"subject_type"`
				Type        int `json:"type"`
			} `json:"data"`
			Total int `json:"total"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("解析收藏失败: %w", err)
		}
		for _, d := range resp.Data {
			if d.SubjectType == 2 {
				all[strconv.Itoa(d.SubjectID)] = d.Type
			}
		}
		if offset+50 >= resp.Total {
			break
		}
		offset += 50
	}

	// 全部页拉取成功后才写盘，中途失败会留下残缺列表
	userDir := filepath.Join(dd, "user")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return fmt.Errorf("创建用户目录失败: %w", err)
	}
	collData, err := json.Marshal(map[string]any{"subjects": all, "updated_at": time.Now().Format(time.RFC3339)})
	if err != nil {
		return fmt.Errorf("序列化收藏失败: %w", err)
	}
	if err := cache.WriteAtomic(filepath.Join(userDir, "collections.json"), collData, 0o644); err != nil {
		return fmt.Errorf("保存收藏失败: %w", err)
	}

	log.Info("user collections saved", "user", username, "count", len(all))
	return nil
}

func fetchUserInfo(username string, bg *bangumi.Client, dd string) error {
	log.Info("fetching user info", "user", username)
	userDir := filepath.Join(dd, "user")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return fmt.Errorf("创建用户目录失败: %w", err)
	}
	data, err := bg.GetRaw(fmt.Sprintf("v0/users/%s", username))
	if err != nil {
		return fmt.Errorf("拉取用户信息失败: %w", err)
	}
	var userData map[string]any
	if err := json.Unmarshal(data, &userData); err != nil {
		return fmt.Errorf("解析用户信息失败: %w", err)
	}
	delete(userData, "avatar")
	clean, err := json.Marshal(userData)
	if err != nil {
		return fmt.Errorf("序列化用户信息失败: %w", err)
	}
	if err := cache.WriteAtomic(filepath.Join(userDir, "info.json"), clean, 0o644); err != nil {
		return fmt.Errorf("保存用户信息失败: %w", err)
	}
	return nil
}

func fetchUserAvatar(username string, bg *bangumi.Client, dd string) error {
	log.Info("fetching user avatar", "user", username)
	userDir := filepath.Join(dd, "user")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return fmt.Errorf("创建用户目录失败: %w", err)
	}
	imgData, err := bg.GetImage(fmt.Sprintf("v0/users/%s/avatar?type=large", username))
	if err != nil {
		return fmt.Errorf("拉取用户头像失败: %w", err)
	}
	ext := ".jpg"
	if len(imgData) > 0 && imgData[0] == 0x89 {
		ext = ".png"
	}
	if err := cache.WriteAtomic(filepath.Join(userDir, "large"+ext), imgData, 0o644); err != nil {
		return fmt.Errorf("保存用户头像失败: %w", err)
	}
	log.Info("user avatar saved", "user", username)
	return nil
}

// countTrackerTotal 统计全部 tracker 中的条目总数
func fetchConcurrent[T any](items []T, fn func(T), p *Progress, stage string, concurrency int) {
	if len(items) == 0 {
		return
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
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
				}
			}()
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

// 辅助函数

func handleFetchAll(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" {
			writeError(w, r, 400, "base_url not configured")
			return
		}
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}
		p := newProgress(countTrackerTotal(cfg), "fetch_all", "刷新全部")
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			refreshAllTrackers(cfg, bg, dd, imgDir, p)
			p.Send("complete", p.Total, p.Total, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "task_id": p.ID})
	}
}

func handleFetchDeep(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" {
			writeError(w, r, 400, "base_url not configured")
			return
		}
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}
		p := newProgress(countTrackerTotal(cfg), "rebuild_all", "重建全部")
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			forceRefresh(cfg, bg, dd, imgDir, p)
			p.Send("complete", p.Total, p.Total, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "deep refresh", "task_id": p.ID})
	}
}

func handleFetchTracker(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" {
			writeError(w, r, 400, "base_url not configured")
			return
		}
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}
		var req struct {
			Names []string `json:"names"`
		}
		limitBody(w, r)
		if !decodeJSON(w, r, &req) {
			return
		}
		for _, n := range req.Names {
			if !validTrackerName(n) {
				writeError(w, r, http.StatusBadRequest, "invalid tracker name: "+n)
				return
			}
		}
		if countTrackerNames(cfg, req.Names) == 0 {
			writeError(w, r, http.StatusNotFound, "tracker not found or empty: "+strings.Join(req.Names, ", "))
			return
		}
		p := newProgress(countTrackerNames(cfg, req.Names), "fetch_tracker", "拉取 Tracker: "+strings.Join(req.Names, ", "))
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			refreshTrackers(cfg, bg, dd, imgDir, req.Names, p)
			p.Send("complete", p.Total, p.Total, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "names": req.Names, "task_id": p.ID})
	}
}

func handleFetchUser(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" {
			writeError(w, r, 400, "base_url not configured")
			return
		}
		pref, _ := config.LoadPreferences()
		if pref == nil {
			pref = &config.DefaultPreferences
		}
		uname := pref.Username
		if uname == "" {
			writeError(w, r, 400, "username not configured")
			return
		}
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}
		p := newProgress(3, "fetch_user", "拉取用户数据")
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			p.SetPhase(1, 3, "拉取用户信息")
			p.Send("user_info", 0, 1, "")
			if err := fetchUserInfo(uname, bg, dd); err != nil {
				log.Warn("user info fetch failed", "user", uname, "err", err)
				p.SetError("用户信息拉取失败")
				p.Send("user_info", 1, 1, "error")
			} else {
				p.Send("user_info", 1, 1, "")
			}

			p.SetPhase(2, 3, "拉取用户头像")
			p.Send("user_avatar", 0, 1, "")
			if err := fetchUserAvatar(uname, bg, dd); err != nil {
				log.Warn("user avatar fetch failed", "user", uname, "err", err)
				p.SetError("用户头像拉取失败")
				p.Send("user_avatar", 1, 1, "error")
			} else {
				p.Send("user_avatar", 1, 1, "")
			}

			p.SetPhase(3, 3, "拉取用户收藏")
			p.Send("user_collections", 0, 1, "")
			if err := fetchUserCollections(uname, bg, dd); err != nil {
				log.Warn("user collections fetch failed", "user", uname, "err", err)
				p.SetError("收藏列表拉取失败")
				p.Send("user_collections", 1, 1, "error")
			} else {
				p.Send("user_collections", 1, 1, "")
			}

			p.Send("complete", 3, 3, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "username": uname, "task_id": p.ID})
	}
}

func handleFetchSubject(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" {
			writeError(w, r, 400, "base_url not configured")
			return
		}
		var req struct {
			ID  int   `json:"id"`
			IDs []int `json:"ids"`
		}
		limitBody(w, r)
		if !decodeJSON(w, r, &req) {
			return
		}
		ids := req.IDs
		if req.ID != 0 {
			ids = []int{req.ID}
		}
		if len(ids) == 0 {
			writeError(w, r, 400, "id or ids required")
			return
		}
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}
		var idStrs []string
		for _, sid := range ids {
			idStrs = append(idStrs, strconv.Itoa(sid))
		}
		p := newProgress(len(ids), "fetch_subject", "拉取动画 #"+strings.Join(idStrs, ", "))
		log.Info("pulling subjects", "ids", ids)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			p.SetPhase(1, 5, "拉取动画数据")
			success := fetchSubjectList(ids, bg, dd, imgDir, p, cfg.Server.ConcurrencyInfo)
			for _, sid := range success {
				addToSeshatTracker(sid)
			}
			downloadImagesScoped(dd, bg, p, 2, 5, ids, cfg.Server.ConcurrencyImage)
			p.SetPhase(5, 5, "建立索引")
			buildIndexes(dd, p)
			log.Info("pulling subjects done", "ids", ids)
			p.Send("complete", len(ids), len(ids), "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "count": len(ids), "task_id": p.ID})
	}
}

func handleFetchUpdate(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" {
			writeError(w, r, 400, "base_url not configured")
			return
		}
		newIDs := diffTrackerIDs(cfg, dd)
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}
		phases := 5
		if len(newIDs) == 0 {
			phases = 1
		}
		p := newProgress(phases, "fetch_update", "增量更新")
		log.Info("incremental update", "new_ids", len(newIDs))
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			if len(newIDs) > 0 {
				p.SetPhase(1, phases, "拉取动画数据")
				fetchSubjectList(newIDs, bg, dd, imgDir, p, cfg.Server.ConcurrencyInfo)
				downloadImagesScoped(dd, bg, p, 2, phases, newIDs, cfg.Server.ConcurrencyImage)
				p.SetPhase(phases, phases, "建立索引")
				buildIndexes(dd, p)
				log.Info("incremental update done", "new_ids", len(newIDs))
			}
			p.Send("complete", phases, phases, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "count": len(newIDs), "task_id": p.ID})
	}
}

func handleFetchIndex(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}
		p := newProgress(5, "rebuild_index", "重建索引")
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			buildIndexes(dd, p)
			p.Send("complete", 5, 5, "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "rebuilding", "task_id": p.ID})
	}
}

func handleFetchGap(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" {
			writeError(w, r, 400, "base_url not configured")
			return
		}
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}
		allIDs := collectAllSubjectIDs(dd)
		if len(allIDs) == 0 {
			writeJSON(w, map[string]any{"status": "done", "count": 0})
			return
		}
		p := newProgress(len(allIDs), "fetch_gap", "补充数据")
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			var done int
			log.Info("filling data gaps", "subjects", len(allIDs))
			for _, sid := range allIDs {
				if subjectCoreIncomplete(dd, sid) {
					if !fetchAll(sid, bg, dd, imgDir, p, cfg.Server.ConcurrencyInfo) {
						if p != nil {
							p.SetError(fmt.Sprintf("条目 #%d 修复失败", sid))
						}
					}
				}
				// 检查角色
				if data, err := os.ReadFile(filepath.Join(cache.Dir(dd), cache.Key("subjects", sid, "characters.json"))); err == nil {
					var chars []struct {
						ID int `json:"id"`
					}
					if json.Unmarshal(data, &chars) == nil {
						for _, c := range chars {
							if !cache.Has(dd, cache.Key("characters", c.ID, "info.json")) {
								if d, e := bg.GetRaw(fmt.Sprintf("v0/characters/%d", c.ID)); e == nil {
									putCache(dd, cache.Key("characters", c.ID, "info.json"), cache.StripImages(d), p)
								} else {
									log.Warn("gap character fetch failed", "id", c.ID, "err", e)
									if p != nil {
										p.SetError(fmt.Sprintf("Character #%d: %v", c.ID, e))
									}
								}
							}
						}
					}
				}
				// 检查人物
				if data, err := os.ReadFile(filepath.Join(cache.Dir(dd), cache.Key("subjects", sid, "persons.json"))); err == nil {
					var persons []struct {
						ID int `json:"id"`
					}
					if json.Unmarshal(data, &persons) == nil {
						for _, pp := range persons {
							if !cache.Has(dd, cache.Key("persons", pp.ID, "info.json")) {
								if d, e := bg.GetRaw(fmt.Sprintf("v0/persons/%d", pp.ID)); e == nil {
									putCache(dd, cache.Key("persons", pp.ID, "info.json"), cache.StripImages(d), p)
								} else {
									log.Warn("gap person fetch failed", "id", pp.ID, "err", e)
									if p != nil {
										p.SetError(fmt.Sprintf("Person #%d: %v", pp.ID, e))
									}
								}
							}
						}
					}
				}
				done++
				p.Send("gap", done, len(allIDs), "")
			}
			log.Info("data gaps filled", "subjects", len(allIDs))
			fillImageGaps(dd, bg, p, cfg.Server.ConcurrencyImage)
			downloadImages(dd, bg, p, 0, 0, cfg.Server.ConcurrencyImage)
			buildIndexes(dd, p)
			p.Send("complete", len(allIDs), len(allIDs), "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "count": len(allIDs), "task_id": p.ID})
	}
}

func handleFetchMeta(cfg *config.Config, bg *bangumi.Client, dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" {
			writeError(w, r, 400, "base_url not configured")
			return
		}
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}

		// 重新拉取全部 tracker 条目，不含图片
		files, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.json"))
		files2, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.toml"))
		files = append(files, files2...)
		seen := map[int]bool{}
		var allIDs []int
		for _, f := range files {
			for _, sid := range loadTrackerIDs(f) {
				if !seen[sid] {
					seen[sid] = true
					allIDs = append(allIDs, sid)
				}
			}
		}
		if len(allIDs) == 0 {
			writeJSON(w, map[string]any{"status": "done", "count": 0})
			return
		}
		log.Info("refreshing metadata", "subjects", len(allIDs))
		imgDir := filepath.Join(dd, "images")
		p := newProgress(len(allIDs), "fetch_meta", "刷新元数据")
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			p.SetPhase(1, 3, "拉取动画数据")
			fetchSubjectList(allIDs, bg, dd, imgDir, p, cfg.Server.ConcurrencyInfo)
			p.SetPhase(2, 3, "建立索引")
			buildIndexes(dd, p)
			log.Info("metadata refresh done", "subjects", len(allIDs))
			p.SetPhase(3, 3, "完成")
			p.Send("complete", len(allIDs), len(allIDs), "")
			p.Close()
		}()
		writeJSON(w, map[string]any{"status": "fetching", "count": len(allIDs), "task_id": p.ID})
	}
}

func collectAllSubjectIDs(dd string) []int {
	list, _ := loadCachedIndex[[]cache.SubjectSummary](cache.IndexFile(dd, "subjects.json"))
	ids := make([]int, len(list))
	for i, e := range list {
		ids[i] = e.ID
	}
	return ids
}

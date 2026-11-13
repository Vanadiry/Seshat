package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
	"os"
	"path/filepath"
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

// fetchAll 拉取条目全部数据及关联角色/人物/图片
func fetchAll(sid int, bg *bangumi.Client, dd, imgDir string, p *Progress, concurrency int, overwrite bool) bool {
	if taskCanceled(p) {
		return false
	}
	log.Info("fetching subject", "id", sid)
	var writeFailed atomic.Bool
	var requiredFailed atomic.Bool

	// 条目
	if p != nil {
		p.Send("subject", 0, 1, "fetching")
	}
	_, err := loadOrFetchData(
		dd,
		store.Key("subjects", sid, "info.json"),
		fmt.Sprintf("v0/subjects/%d", sid),
		bg,
		overwrite,
	)
	if err != nil {
		if isCanceled(err) {
			return false
		}
		log.Warn("subject fetch failed", "id", sid, "err", err)
		if p != nil {
			p.SetError(err.Error())
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
		data, err := loadOrFetchData(
			dd,
			store.Key("subjects", sid, "characters.json"),
			fmt.Sprintf("v0/subjects/%d/characters", sid),
			bg,
			overwrite,
		)
		if err != nil {
			if isCanceled(err) {
				return
			}
			log.Warn("subject characters fetch failed", "subject", sid, "err", err)
			requiredFailed.Store(true)
			if p != nil {
				p.SetError(fmt.Sprintf("角色列表拉取失败 #%d: %v", sid, err))
			}
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
		data, err := loadOrFetchData(
			dd,
			store.Key("subjects", sid, "persons.json"),
			fmt.Sprintf("v0/subjects/%d/persons", sid),
			bg,
			overwrite,
		)
		if err != nil {
			if isCanceled(err) {
				return
			}
			log.Warn("subject persons fetch failed", "subject", sid, "err", err)
			requiredFailed.Store(true)
			if p != nil {
				p.SetError(fmt.Sprintf("人物列表拉取失败 #%d: %v", sid, err))
			}
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
			key := store.Key("characters", c.ID, "info.json")
			if !overwrite && store.Has(dd, key) {
				return
			}
			data, err := bg.GetRaw(fmt.Sprintf("v0/characters/%d", c.ID))
			if err != nil {
				log.Warn("character fetch failed", "id", c.ID, "err", err)
				p.SetError(err.Error())
				return
			}
			if !putData(dd, key, store.StripImages(data), p) {
				writeFailed.Store(true)
			}
		}, nil, concurrency)
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
			key := store.Key("persons", pp.ID, "info.json")
			if !overwrite && store.Has(dd, key) {
				return
			}
			data, err := bg.GetRaw(fmt.Sprintf("v0/persons/%d", pp.ID))
			if err != nil {
				log.Warn("person fetch failed", "id", pp.ID, "err", err)
				p.SetError(err.Error())
				return
			}
			if !putData(dd, key, store.StripImages(data), p) {
				writeFailed.Store(true)
			}
		}, nil, concurrency)
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
			key := store.Key("characters", c.ID, "subjects.json")
			if !overwrite && store.Has(dd, key) {
				return
			}
			data, err := bg.GetRaw(fmt.Sprintf("v0/characters/%d/subjects", c.ID))
			if err != nil {
				return
			}
			if !putData(dd, key, store.StripImages(data), p) {
				writeFailed.Store(true)
			}
		}, nil, concurrency)
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
			key := store.Key("characters", c.ID, "persons.json")
			if !overwrite && store.Has(dd, key) {
				return
			}
			data, err := bg.GetRaw(fmt.Sprintf("v0/characters/%d/persons", c.ID))
			if err != nil {
				return
			}
			if !putData(dd, key, store.StripImages(data), p) {
				writeFailed.Store(true)
			}
		}, nil, concurrency)
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
			key := store.Key("persons", pp.ID, "subjects.json")
			if !overwrite && store.Has(dd, key) {
				return
			}
			data, err := bg.GetRaw(fmt.Sprintf("v0/persons/%d/subjects", pp.ID))
			if err != nil {
				return
			}
			if !putData(dd, key, store.StripImages(data), p) {
				writeFailed.Store(true)
			}
		}, nil, concurrency)
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
			key := store.Key("persons", pp.ID, "characters.json")
			if !overwrite && store.Has(dd, key) {
				return
			}
			data, err := bg.GetRaw(fmt.Sprintf("v0/persons/%d/characters", pp.ID))
			if err != nil {
				return
			}
			if !putData(dd, key, store.StripImages(data), p) {
				writeFailed.Store(true)
			}
		}, nil, concurrency)
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
		epKey := store.Key("subjects", sid, "episodes.json")
		if !overwrite && store.Has(dd, epKey) {
			return
		}
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
			if !putData(dd, store.Key("subjects", sid, "episodes.json"), store.StripImages(result), p) {
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
		relKey := store.Key("subjects", sid, "subjects.json")
		if !overwrite && store.Has(dd, relKey) {
			return
		}
		if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/subjects", sid)); err == nil {
			if !putData(dd, relKey, store.StripImages(data), p) {
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

// 拉取用户收藏，失败时不做写入
func fetchUserCollections(username string, bg *bangumi.Client, dd string) error {
	log.Info("fetching collections", "user", username)

	var items []json.RawMessage

	offset := 0
	for {
		data, err := bg.GetRawPaged(fmt.Sprintf("v0/users/%s/collections", username), offset, 50)
		if err != nil {
			return fmt.Errorf("拉取收藏失败，偏移 %d: %w", offset, err)
		}
		var resp struct {
			Data  []json.RawMessage `json:"data"`
			Total int               `json:"total"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("解析收藏失败: %w", err)
		}
		items = append(items, resp.Data...)
		if offset+50 >= resp.Total {
			break
		}
		offset += 50
	}

	full := make([]json.RawMessage, 0, len(items))
	summary := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		clean := store.StripImages(raw)
		full = append(full, clean)
		var item struct {
			SubjectID   int `json:"subject_id"`
			SubjectType int `json:"subject_type"`
			Type        int `json:"type"`
			Rate        int `json:"rate"`
		}
		if json.Unmarshal(clean, &item) == nil {
			summary = append(summary, map[string]any{
				"subject_id":   item.SubjectID,
				"subject_type": item.SubjectType,
				"type":         item.Type,
				"rate":         item.Rate,
			})
		}
	}

	// 全部页拉取成功后才写盘，中途失败会留下残缺列表
	userDir := filepath.Join(dd, "user")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return fmt.Errorf("创建用户目录失败: %w", err)
	}
	updated := time.Now().Format(time.RFC3339)
	fullData, err := json.Marshal(map[string]any{"updated_at": updated, "data": full})
	if err != nil {
		return fmt.Errorf("序列化收藏失败: %w", err)
	}
	if err := store.WriteAtomic(filepath.Join(userDir, "collections.json"), fullData, 0o644); err != nil {
		return fmt.Errorf("保存收藏失败: %w", err)
	}
	summaryData, err := json.Marshal(map[string]any{"updated_at": updated, "data": summary})
	if err != nil {
		return fmt.Errorf("序列化收藏摘要失败: %w", err)
	}
	if err := store.WriteAtomic(filepath.Join(userDir, "collections_summary.json"), summaryData, 0o644); err != nil {
		return fmt.Errorf("保存收藏摘要失败: %w", err)
	}

	log.Info("user collections saved", "user", username, "count", len(full))
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
	if err := store.WriteAtomic(filepath.Join(userDir, "info.json"), clean, 0o644); err != nil {
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
	if err := store.WriteAtomic(filepath.Join(userDir, "large"+ext), imgData, 0o644); err != nil {
		return fmt.Errorf("保存用户头像失败: %w", err)
	}
	log.Info("user avatar saved", "user", username)
	return nil
}

// countTrackerTotal 统计全部 tracker 中的条目总数
func fetchConcurrent[T any](items []T, fn func(T), p *Progress, concurrency int) {
	if len(items) == 0 {
		return
	}
	if p != nil {
		p.SendPhaseProgress(0, len(items), "fetching")
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var done int
	var mu sync.Mutex
	ctx := context.Background()
	if p != nil {
		ctx = p.Context()
	}
	for _, item := range items {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(item T) {
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
			fn(item)
			if p != nil {
				mu.Lock()
				done++
				p.SendPhaseProgress(done, len(items), "")
				mu.Unlock()
			}
		}(item)
	}
	wg.Wait()
}

// 辅助函数

func handleFetchAll(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveFetchRequest(w, r, cfg, bg, dd, imgDir, fetchRequest{
			Source: fetchSourceTrackerAll,
			JSON:   fetchJSONOverwrite,
			Images: fetchImagesFill,
		})
	}
}

func handleFetchDeep(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" {
			writeError(w, r, 400, "base_url not configured")
			return
		}
		var req struct {
			ConfirmClean bool `json:"confirm_clean"`
		}
		limitBody(w, r)
		if !decodeJSON(w, r, &req) {
			return
		}
		if !req.ConfirmClean {
			writeError(w, r, http.StatusBadRequest, "重建全部需要确认")
			return
		}
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}
		p := newProgress(100, "rebuild_all", "重建全部")
		go func() {
			defer p.Finish()
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			forceRefresh(cfg, bg.WithContext(p.Context()), dd, imgDir, p)
		}()
		writeJSON(w, map[string]any{"status": "deep refresh", "task_id": p.ID})
	}
}

func handleFetchTracker(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Names []string `json:"names"`
		}
		limitBody(w, r)
		if !decodeJSON(w, r, &req) {
			return
		}
		serveFetchRequest(w, r, cfg, bg, dd, imgDir, fetchRequest{
			Source: fetchSourceTrackerNames,
			Names:  req.Names,
			JSON:   fetchJSONOverwrite,
			Images: fetchImagesFill,
		})
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
		p := newProgress(100, "fetch_user", "拉取用户数据")
		p.SetPlan(splitPhaseRanges(0, 100, "拉取用户信息", "拉取用户头像", "拉取用户收藏"))
		go func() {
			defer p.Finish()
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			taskBg := bg.WithContext(p.Context())
			p.BeginPhase(1)
			p.SendPhaseStep(0, 1, "")
			if err := fetchUserInfo(uname, taskBg, dd); err != nil {
				if isCanceled(err) {
					return
				}
				log.Warn("user info fetch failed", "user", uname, "err", err)
				p.SetError("用户信息拉取失败")
				p.SendPhaseStep(1, 1, "失败")
			} else {
				p.SendPhaseStep(1, 1, "")
			}
			if taskCanceled(p) {
				return
			}

			p.BeginPhase(2)
			p.SendPhaseStep(0, 1, "")
			if err := fetchUserAvatar(uname, taskBg, dd); err != nil {
				if isCanceled(err) {
					return
				}
				log.Warn("user avatar fetch failed", "user", uname, "err", err)
				p.SetError("用户头像拉取失败")
				p.SendPhaseStep(1, 1, "失败")
			} else {
				p.SendPhaseStep(1, 1, "")
			}
			if taskCanceled(p) {
				return
			}

			p.BeginPhase(3)
			p.SendPhaseStep(0, 1, "")
			if err := fetchUserCollections(uname, taskBg, dd); err != nil {
				if isCanceled(err) {
					return
				}
				log.Warn("user collections fetch failed", "user", uname, "err", err)
				p.SetError("收藏列表拉取失败")
				p.SendPhaseStep(1, 1, "失败")
			} else {
				p.SendPhaseStep(1, 1, "")
			}
		}()
		writeJSON(w, map[string]any{"status": "fetching", "username": uname, "task_id": p.ID})
	}
}

func handleFetchSubject(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		serveFetchRequest(w, r, cfg, bg, dd, imgDir, fetchRequest{
			Source: fetchSourceIDs,
			IDs:    ids,
			JSON:   fetchJSONOverwrite,
			Images: fetchImagesFill,
		})
	}
}

func handleFetchUpdate(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveFetchRequest(w, r, cfg, bg, dd, imgDir, fetchRequest{
			Source: fetchSourceTrackerNew,
			JSON:   fetchJSONFill,
			Images: fetchImagesFill,
		})
	}
}

func handleFetchIndex(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}
		p := newProgress(100, "rebuild_index", "重建索引")
		p.SetPlan([]phaseSpec{
			{Name: "扫描条目", Start: 0, End: 40},
			{Name: "扫描角色", Start: 40, End: 55},
			{Name: "扫描人物", Start: 55, End: 70},
			{Name: "保存标签和名称索引", Start: 70, End: 80},
			{Name: "重建图片索引", Start: 80, End: 100},
		})
		go func() {
			defer p.Finish()
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			buildIndexesPhased(dd, p)
		}()
		writeJSON(w, map[string]any{"status": "rebuilding", "task_id": p.ID})
	}
}

func handleFetchGap(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveFetchRequest(w, r, cfg, bg, dd, imgDir, fetchRequest{
			Source: fetchSourceLocal,
			JSON:   fetchJSONFill,
			Images: fetchImagesFill,
		})
	}
}

func handleFetchMeta(cfg *config.Config, bg *bangumi.Client, dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveFetchRequest(w, r, cfg, bg, dd, filepath.Join(dd, "images"), fetchRequest{
			Source: fetchSourceTrackerAll,
			JSON:   fetchJSONOverwrite,
			Images: fetchImagesNone,
		})
	}
}

func collectAllSubjectIDs(dd string) []int {
	list, _ := loadCachedIndex[[]store.SubjectSummary](store.IndexFile(dd, "subjects.json"))
	ids := make([]int, len(list))
	for i, e := range list {
		ids[i] = e.ID
	}
	return ids
}

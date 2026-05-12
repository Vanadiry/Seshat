package server

import (
	"encoding/json"
	"fmt"
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
	cache.Put(dd, fmt.Sprintf("subjects/%d.json", sid), cache.StripImages(data))

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
			cache.Put(dd, fmt.Sprintf("subjects/%d/characters.json", sid), cache.StripImages(data))
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
			cache.Put(dd, fmt.Sprintf("subjects/%d/persons.json", sid), cache.StripImages(data))
			json.Unmarshal(data, &persons)
			for _, p := range persons {
				mergeListEntry(persListPath, p.ID, p.Name, "")
			}
		}
	}()
	wg.Wait()

	// Character details — retry 3 times, remove from list on 404
	type charRef struct{ ID int `json:"id"` }
	var charIDs []charRef
	for _, c := range chars { charIDs = append(charIDs, charRef{ID: c.ID}) }
	fetchConcurrent(charIDs, func(c charRef) {
		data, err := getRawWithRetry(bg, fmt.Sprintf("v0/characters/%d", c.ID), 3)
		if err != nil {
			if strings.Contains(err.Error(), "HTTP 404") {
				log.Warn("Character #%d: 404, removing from list", c.ID)
				removeListEntry(charListPath, c.ID)
			} else {
				log.Warn("Character #%d: %v", c.ID, err)
			}
			return
		}
		cache.Put(dd, fmt.Sprintf("characters/%d.json", c.ID), cache.StripImages(data))
		if cn := extractNameCN(data); cn != "" {
			mergeListEntry(charListPath, c.ID, "", cn)
		}
	}, p, "characters", maxConcurrency)

	// Person details — crew + actors, retry 3 times, remove from list on 404
	type personRef struct{ ID int `json:"id"` }
	personSet := map[int]bool{}
	for _, p := range persons { personSet[p.ID] = true }
	for _, c := range chars {
		for _, a := range c.Actors {
			if a.ID > 0 { personSet[a.ID] = true }
		}
	}
	var personIDs []personRef
	for id := range personSet { personIDs = append(personIDs, personRef{ID: id}) }
	fetchConcurrent(personIDs, func(pp personRef) {
		data, err := getRawWithRetry(bg, fmt.Sprintf("v0/persons/%d", pp.ID), 3)
		if err != nil {
			if strings.Contains(err.Error(), "HTTP 404") {
				log.Warn("Person #%d: 404, removing from list", pp.ID)
				removeListEntry(persListPath, pp.ID)
			} else {
				log.Warn("Person #%d: %v", pp.ID, err)
			}
			return
		}
		cache.Put(dd, fmt.Sprintf("persons/%d.json", pp.ID), cache.StripImages(data))
		if cn := extractNameCN(data); cn != "" {
			mergeListEntry(persListPath, pp.ID, "", cn)
		}
	}, p, "persons", maxConcurrency)

	// Episodes
	if data, err := bg.GetRaw(fmt.Sprintf("v0/episodes?subject_id=%d&limit=200", sid)); err == nil {
		cache.Put(dd, fmt.Sprintf("subjects/%d/episodes.json", sid), cache.StripImages(data))
	}

	// Relations
	if data, err := bg.GetRaw(fmt.Sprintf("v0/subjects/%d/subjects", sid)); err == nil {
		cache.Put(dd, fmt.Sprintf("subjects/%d/relations.json", sid), cache.StripImages(data))
	}

	log.Info("Subject #%d API fetch done", sid)
}

// Phase 2: Build index files from all cached API data.
func fetchUserCollections(username string, bg *bangumi.Client, dd string) {
	log.Info("Fetching collections for %s", username)

	type item struct {
		SubjectID int `json:"subject_id"`
		Type      int `json:"type"`
	}
	var all []item

	offset := 0
	for {
		data, err := bg.GetRawPaged(fmt.Sprintf("v0/users/%s/collections", username), offset, 50)
		if err != nil {
			log.Warn("user collections offset %d: %v", offset, err)
			break
		}
		// Parse just the data array
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
				all = append(all, item{SubjectID: d.SubjectID, Type: d.Type})
			}
		}
		if offset+50 >= resp.Total {
			break
		}
		offset += 50
	}

	// Save to tracker with underscore prefix
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

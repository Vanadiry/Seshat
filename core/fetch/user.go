package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

// 拉取用户收藏，失败时不做写入
func UserCollections(username string, bg *bangumi.Client, dd string) error {
	log.Info("fetching collections", "user", username)

	var items []json.RawMessage

	offset := 0
	for {
		data, err := bg.GetRawPaged(fmt.Sprintf("v0/users/%s/collections", username), offset, 50)
		if err != nil {
			return fmt.Errorf("拉取收藏失败，偏移 %d：%w", offset, err)
		}
		var resp struct {
			Data  []json.RawMessage `json:"data"`
			Total int               `json:"total"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("解析收藏失败：%w", err)
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
		return fmt.Errorf("创建用户目录失败：%w", err)
	}
	updated := time.Now().Format(time.RFC3339)
	fullData, err := json.Marshal(map[string]any{"updated_at": updated, "data": full})
	if err != nil {
		return fmt.Errorf("序列化收藏失败：%w", err)
	}
	if err := store.WriteAtomic(filepath.Join(userDir, "collections.json"), fullData, 0o644); err != nil {
		return fmt.Errorf("保存收藏失败：%w", err)
	}
	summaryData, err := json.Marshal(map[string]any{"updated_at": updated, "data": summary})
	if err != nil {
		return fmt.Errorf("序列化收藏摘要失败：%w", err)
	}
	if err := store.WriteAtomic(filepath.Join(userDir, "collections_summary.json"), summaryData, 0o644); err != nil {
		return fmt.Errorf("保存收藏摘要失败：%w", err)
	}

	log.Info("user collections saved", "user", username, "count", len(full))
	return nil
}

func UserInfo(username string, bg *bangumi.Client, dd string) error {
	log.Info("fetching user info", "user", username)
	userDir := filepath.Join(dd, "user")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return fmt.Errorf("创建用户目录失败：%w", err)
	}
	data, err := bg.GetRaw(fmt.Sprintf("v0/users/%s", username))
	if err != nil {
		return fmt.Errorf("拉取用户信息失败：%w", err)
	}
	var userData map[string]any
	if err := json.Unmarshal(data, &userData); err != nil {
		return fmt.Errorf("解析用户信息失败：%w", err)
	}
	delete(userData, "avatar")
	clean, err := json.Marshal(userData)
	if err != nil {
		return fmt.Errorf("序列化用户信息失败：%w", err)
	}
	if err := store.WriteAtomic(filepath.Join(userDir, "info.json"), clean, 0o644); err != nil {
		return fmt.Errorf("保存用户信息失败：%w", err)
	}
	return nil
}

func UserAvatar(username string, bg *bangumi.Client, dd string) error {
	log.Info("fetching user avatar", "user", username)
	userDir := filepath.Join(dd, "user")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return fmt.Errorf("创建用户目录失败：%w", err)
	}
	imgData, err := bg.GetImage(fmt.Sprintf("v0/users/%s/avatar?type=large", username))
	if err != nil {
		return fmt.Errorf("拉取用户头像失败：%w", err)
	}
	ext := ".jpg"
	if len(imgData) > 0 && imgData[0] == 0x89 {
		ext = ".png"
	}
	if err := store.WriteAtomic(filepath.Join(userDir, "large"+ext), imgData, 0o644); err != nil {
		return fmt.Errorf("保存用户头像失败：%w", err)
	}
	log.Info("user avatar saved", "user", username)
	return nil
}

// countTrackerTotal 统计全部 Tracker 中的条目总数
func fetchConcurrent[T any](items []T, fn func(T), p Progress, concurrency int) {
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

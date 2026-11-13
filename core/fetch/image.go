package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

func DownloadImages(dd string, bg *bangumi.Client, p Progress, phaseBase, concurrency int) {
	downloadImagesWithPhase(dd, bg, p, phaseBase, nil, concurrency, false)
}

// downloadImagesScoped 仅下载指定条目及其关联角色/人物的图像
func DownloadImagesScoped(dd string, bg *bangumi.Client, p Progress, phaseBase int, subjectIDs []int, concurrency int) {
	downloadImagesWithPhase(dd, bg, p, phaseBase, subjectIDs, concurrency, false)
}

// downloadImagesScopedForce 覆盖指定条目及其关联角色/人物的全部图像
func DownloadImagesScopedForce(dd string, bg *bangumi.Client, p Progress, phaseBase int, subjectIDs []int, concurrency int) {
	downloadImagesWithPhase(dd, bg, p, phaseBase, subjectIDs, concurrency, true)
}

func downloadImagesWithPhase(dd string, bg *bangumi.Client, p Progress, phaseBase int, subjectFilter []int, concurrency int, force bool) {
	log.Info("downloading images")
	if err := os.MkdirAll(store.IndexDir(dd), 0o755); err != nil {
		log.Error("index dir mkdir failed", "err", err)
	}
	imgBase := filepath.Join(dd, "images")

	// 构建实体列表：scoped 按指定条目过滤，full 扫描 API 数据目录
	var subjIDs []int
	var charIDs []int
	var persIDs []int

	if subjectFilter != nil {
		subjIDs = subjectFilter
		charSet := map[int]bool{}
		persSet := map[int]bool{}
		for _, sid := range subjectFilter {
			if data, err := os.ReadFile(filepath.Join(store.Dir(dd), store.Key("subjects", sid, "characters.json"))); err == nil {
				var chars []struct {
					ID     int `json:"id"`
					Actors []struct {
						ID int `json:"id"`
					} `json:"actors"`
				}
				if json.Unmarshal(data, &chars) == nil {
					for _, c := range chars {
						charSet[c.ID] = true
						for _, a := range c.Actors {
							if a.ID > 0 {
								persSet[a.ID] = true
							}
						}
					}
				}
			}
			if data, err := os.ReadFile(filepath.Join(store.Dir(dd), store.Key("subjects", sid, "persons.json"))); err == nil {
				var persons []struct {
					ID int `json:"id"`
				}
				if json.Unmarshal(data, &persons) == nil {
					for _, p := range persons {
						persSet[p.ID] = true
					}
				}
			}
		}
		for id := range charSet {
			charIDs = append(charIDs, id)
		}
		for id := range persSet {
			persIDs = append(persIDs, id)
		}
	} else {
		subjIDs, _ = store.ListIDs(dd, "subjects")
		charIDs, _ = store.ListIDs(dd, "characters")
		persIDs, _ = store.ListIDs(dd, "persons")
	}

	// 条目
	if p != nil {
		p.BeginPhase(phaseBase)
		p.SendPhaseProgress(0, len(subjIDs), "")
	}
	dlImageList(subjIDs, "subject", nil, imgBase, bg, p, "images_subjects", concurrency, force)
	if taskCanceled(p) {
		return
	}

	// 角色
	if p != nil {
		p.BeginPhase(phaseBase + 1)
		p.SendPhaseProgress(0, len(charIDs), "")
	}
	dlImageList(charIDs, "character", nil, imgBase, bg, p, "images_characters", concurrency, force)
	if taskCanceled(p) {
		return
	}

	// 人物
	if p != nil {
		p.BeginPhase(phaseBase + 2)
		p.SendPhaseProgress(0, len(persIDs), "")
	}
	dlImageList(persIDs, "person", nil, imgBase, bg, p, "images_persons", concurrency, force)

	log.Info("images download complete")
}

// missingImageSizes 返回指定条目在磁盘上缺失的图片尺寸
func missingImageSizes(imgBase, kind string, id int) []string {
	var missing []string
	for _, size := range []string{"large", "grid", "small"} {
		path := filepath.Join(imgBase, fmt.Sprintf("%ss_%s/%d/%d.jpg", kind, size, id%10, id))
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, size)
		}
	}
	return missing
}

func dlImageList(ids []int, kind string, imgMap map[int]store.ImageEntry, imgBase string, bg *bangumi.Client, p Progress, stage string, concurrency int, force bool) {
	if len(ids) == 0 {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	var done int
	var mu sync.Mutex
	ctx := context.Background()
	if p != nil {
		ctx = p.Context()
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(id int) {
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
			dlImage(bg, kind, id, imgMap, imgBase, &mu, p, force)
			if p != nil {
				mu.Lock()
				done++
				if p.HasPhaseRange() {
					p.SendPhaseProgress(done, len(ids), "")
				} else {
					p.Send(stage, done, len(ids), "")
				}
				mu.Unlock()
			}
		}(id)
	}
	wg.Wait()
}

func dlImage(bg *bangumi.Client, kind string, id int, imgMap map[int]store.ImageEntry, imgBase string, mu *sync.Mutex, p Progress, force bool) {
	// 默认只补缺失的尺寸，force 时整组重下
	var missing []string
	if force {
		missing = []string{"large", "grid", "small"}
	} else if imgMap != nil {
		mu.Lock()
		entry, ok := imgMap[id]
		mu.Unlock()
		if ok {
			if entry.Large == "" {
				missing = append(missing, "large")
			}
			if entry.Grid == "" {
				missing = append(missing, "grid")
			}
			if entry.Small == "" {
				missing = append(missing, "small")
			}
		} else {
			missing = []string{"large", "grid", "small"}
		}
	} else {
		missing = missingImageSizes(imgBase, kind, id)
	}
	if len(missing) == 0 {
		return
	}
	if imgMap == nil {
		imgMap = map[int]store.ImageEntry{id: store.ImageEntry{}}
	}
	dlMissingSizes(bg, kind, id, missing, imgMap, imgBase, mu, p)
}

// dlMissingSizes 仅下载指定条目缺失的图片尺寸
func dlMissingSizes(bg *bangumi.Client, kind string, id int, sizes []string, imgMap map[int]store.ImageEntry, imgBase string, mu *sync.Mutex, p Progress) {
	mu.Lock()
	entry := imgMap[id]
	mu.Unlock()

	type result struct {
		size, path string
	}
	var wg sync.WaitGroup
	results := make(chan result, len(sizes))

	for _, size := range sizes {
		if taskCanceled(p) {
			break
		}
		wg.Add(1)
		go func(size string) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
				}
			}()
			defer wg.Done()
			data, err := bg.GetImage(fmt.Sprintf("v0/%ss/%d/image?type=%s", kind, id, size))
			if err != nil {
				if isCanceled(err) {
					return
				}
				if !strings.Contains(err.Error(), "placeholder") && !strings.Contains(err.Error(), "HTTP 404") {
					log.Warn("image download failed", "kind", kind, "id", id, "size", size, "err", err)
					if p != nil {
						p.SetError(fmt.Sprintf("图片 %s #%d %s：%v", kind, id, size, err))
					}
				}
				return
			}
			relPath := fmt.Sprintf("%ss_%s/%d/%d.jpg", kind, size, id%10, id)
			fullPath := filepath.Join(imgBase, relPath)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				log.Error("image mkdir failed", "kind", kind, "id", id, "size", size, "err", err)
				return
			}
			if err := store.WriteAtomic(fullPath, data, 0o644); err != nil {
				log.Error("image write failed", "kind", kind, "id", id, "size", size, "err", err)
				events.Bus.Error(fmt.Sprintf("图片写入失败 %s #%d %s", kind, id, size))
				return
			}
			results <- result{size, relPath}
		}(size)
	}
	wg.Wait()
	close(results)

	for r := range results {
		switch r.size {
		case "large":
			entry.Large = r.path
		case "grid":
			entry.Grid = r.path
		case "small":
			entry.Small = r.path
		}
	}

	mu.Lock()
	imgMap[id] = entry
	mu.Unlock()
}

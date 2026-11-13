package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

func downloadImages(dd string, bg *bangumi.Client, p *Progress, phaseBase, concurrency int) {
	downloadImagesWithPhase(dd, bg, p, phaseBase, nil, concurrency, false)
}

// downloadImagesScoped 仅下载指定条目及其关联角色/人物的图像
func downloadImagesScoped(dd string, bg *bangumi.Client, p *Progress, phaseBase int, subjectIDs []int, concurrency int) {
	downloadImagesWithPhase(dd, bg, p, phaseBase, subjectIDs, concurrency, false)
}

// downloadImagesScopedForce 覆盖指定条目及其关联角色/人物的全部图像
func downloadImagesScopedForce(dd string, bg *bangumi.Client, p *Progress, phaseBase int, subjectIDs []int, concurrency int) {
	downloadImagesWithPhase(dd, bg, p, phaseBase, subjectIDs, concurrency, true)
}

func downloadImagesWithPhase(dd string, bg *bangumi.Client, p *Progress, phaseBase int, subjectFilter []int, concurrency int, force bool) {
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

func dlImageList(ids []int, kind string, imgMap map[int]store.ImageEntry, imgBase string, bg *bangumi.Client, p *Progress, stage string, concurrency int, force bool) {
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

func dlImage(bg *bangumi.Client, kind string, id int, imgMap map[int]store.ImageEntry, imgBase string, mu *sync.Mutex, p *Progress, force bool) {
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
func dlMissingSizes(bg *bangumi.Client, kind string, id int, sizes []string, imgMap map[int]store.ImageEntry, imgBase string, mu *sync.Mutex, p *Progress) {
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
						p.SetError(fmt.Sprintf("Image %s #%d %s: %v", kind, id, size, err))
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

// fillImageGaps 补充缺失的图片尺寸及索引中不存在的图片
func fillImageGaps(dd string, bg *bangumi.Client, p *Progress, concurrency int) {
	imgBase := filepath.Join(dd, "images")
	domains := []struct {
		kind       string
		labelSizes string
		labelMiss  string
	}{
		{"subject", "检查遗漏的subject图像规格", "检查缺失的subject图像"},
		{"character", "检查遗漏的character图像规格", "检查缺失的character图像"},
		{"person", "检查遗漏的person图像规格", "检查缺失的person图像"},
	}

	phaseNum := 1
	for _, d := range domains {
		if taskCanceled(p) {
			return
		}
		imgMap := loadImageIndex(dd, d.kind+"s_image.json")
		nameList := loadNameList(store.IndexFile(dd, d.kind+"s.json"))

		// 阶段一：补充已有条目缺失的图片尺寸
		type partial struct {
			id    int
			sizes []string
		}
		var partials []partial
		for id, entry := range imgMap {
			var missing []string
			if entry.Large == "" {
				missing = append(missing, "large")
			}
			if entry.Grid == "" {
				missing = append(missing, "grid")
			}
			if entry.Small == "" {
				missing = append(missing, "small")
			}
			if len(missing) > 0 {
				partials = append(partials, partial{id: id, sizes: missing})
			}
		}
		if p != nil {
			p.SetPhase(phaseNum, 11, d.labelSizes)
		}
		if len(partials) > 0 {
			if p != nil {
				p.Send("fill_"+d.kind+"_sizes", 0, len(partials), "")
			}
			var wg sync.WaitGroup
			sem := make(chan struct{}, concurrency)
			var done int
			var mu sync.Mutex
			for _, pt := range partials {
				if taskCanceled(p) {
					break
				}
				wg.Add(1)
				go func(id int, sizes []string) {
					defer func() {
						if r := recover(); r != nil {
							log.Error("goroutine panic", "panic", r)
						}
					}()
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					dlMissingSizes(bg, d.kind, id, sizes, imgMap, imgBase, &mu, p)
					if p != nil {
						mu.Lock()
						done++
						p.Send("fill_"+d.kind+"_sizes", done, len(partials), "")
						mu.Unlock()
					}
				}(pt.id, pt.sizes)
			}
			wg.Wait()
		}
		phaseNum++

		// 阶段二：下载图片索引中完全缺失的条目
		var missingIDs []int
		for _, e := range nameList {
			if taskCanceled(p) {
				return
			}
			if _, ok := imgMap[e.ID]; !ok {
				missingIDs = append(missingIDs, e.ID)
			}
		}
		if p != nil {
			p.SetPhase(phaseNum, 11, d.labelMiss)
		}
		if len(missingIDs) > 0 {
			if p != nil {
				p.Send("fill_"+d.kind+"_miss", 0, len(missingIDs), "")
			}
			dlImageList(missingIDs, d.kind, imgMap, imgBase, bg, p, "fill_"+d.kind+"_miss", concurrency, false)
		}
		phaseNum++

		saveJSON(store.IndexFile(dd, d.kind+"s_image.json"), imgMap)
	}
}

// RebuildImageIndex 扫描 images/ 目录重建图片索引
func RebuildImageIndex(dd string) {
	imgBase := filepath.Join(dd, "images")
	kinds := []string{"subject", "character", "person"}
	for _, kind := range kinds {
		m := map[int]store.ImageEntry{}
		for _, size := range []string{"large", "grid", "small"} {
			dir := filepath.Join(imgBase, kind+"s_"+size)
			digits, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, d := range digits {
				if !d.IsDir() {
					continue
				}
				files, _ := os.ReadDir(filepath.Join(dir, d.Name()))
				for _, f := range files {
					name := f.Name()
					if ext := filepath.Ext(name); ext != ".jpg" {
						continue
					}
					id, err := strconv.Atoi(strings.TrimSuffix(name, ".jpg"))
					if err != nil {
						continue
					}
					entry := m[id]
					relPath := fmt.Sprintf("%ss_%s/%s/%s", kind, size, d.Name(), name)
					switch size {
					case "large":
						entry.Large = relPath
					case "grid":
						entry.Grid = relPath
					case "small":
						entry.Small = relPath
					}
					m[id] = entry
				}
			}
		}
		saveJSON(store.IndexFile(dd, kind+"s_image.json"), m)
		log.Info("image index rebuilt", "kind", kind+"s", "count", len(m))
	}
}

// 辅助函数

func loadImageIndex(dd, name string) map[int]store.ImageEntry {
	m, err := loadCachedIndex[map[int]store.ImageEntry](store.IndexFile(dd, name))
	if err != nil || m == nil {
		return map[int]store.ImageEntry{}
	}
	return m
}

func entryPath(entry store.ImageEntry, size string) string {
	switch size {
	case "large":
		return entry.Large
	case "grid":
		return entry.Grid
	case "small":
		return entry.Small
	default:
		return ""
	}
}

// imageSizeOrder 返回请求尺寸的尝试顺序，优先临近的更高档位
func imageSizeOrder(requested string) []string {
	switch requested {
	case "large":
		return []string{"large", "small", "grid"}
	case "small":
		return []string{"small", "large", "grid"}
	case "grid":
		return []string{"grid", "small", "large"}
	case "medium", "common":
		return []string{"large", "small", "grid"}
	default:
		return []string{"grid", "small", "large"}
	}
}

// pickImageSize 返回实际可用的尺寸，没有可用图片时返回空字符串
func pickImageSize(entry store.ImageEntry, requested string) string {
	for _, size := range imageSizeOrder(requested) {
		if entryPath(entry, size) != "" {
			return size
		}
	}
	return ""
}

func serveImage(w http.ResponseWriter, r *http.Request, dd, kind, size string) {
	if size == "" {
		size = "grid"
	}
	idStr := r.PathValue("id")
	id, _ := strconv.Atoi(idStr)
	images := loadImageIndex(dd, kind+"s_image.json")
	entry, ok := images[id]
	if ok {
		for _, candidate := range imageSizeOrder(size) {
			path := entryPath(entry, candidate)
			if path == "" {
				continue
			}
			fullPath := filepath.Join(dd, "images", path)
			if _, err := os.Stat(fullPath); err == nil {
				w.Header().Set("Cache-Control", "no-store")
				http.ServeFile(w, r, fullPath)
				return
			}
		}
	}
	// 404 触发前端 onerror 回退
	writeNotFound(w, r, "图片不存在")
}

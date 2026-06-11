package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/vanadiry/seshat/Core/bangumi"
	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/log"
)

func downloadImages(dd string, bg *bangumi.Client, p *Progress, phaseBase, totalPhases int) {
	downloadImagesWithPhase(dd, bg, p, phaseBase, totalPhases, nil)
}

// downloadImagesScoped 仅下载指定 subject 及其关联角色/人物的图像。
func downloadImagesScoped(dd string, bg *bangumi.Client, p *Progress, phaseBase, totalPhases int, subjectIDs []int) {
	downloadImagesWithPhase(dd, bg, p, phaseBase, totalPhases, subjectIDs)
}

func downloadImagesWithPhase(dd string, bg *bangumi.Client, p *Progress, phaseBase, totalPhases int, subjectFilter []int) {
	log.Info("downloading images")
	os.MkdirAll(cache.IndexDir(dd), 0o755)
	imgBase := filepath.Join(dd, "images")

	// Build entity lists: scoped = filter by given subject IDs, full = scan API data dir
	var subjIDs []int
	var charIDs []int
	var persIDs []int

	if subjectFilter != nil {
		subjIDs = subjectFilter
		charSet := map[int]bool{}
		persSet := map[int]bool{}
		for _, sid := range subjectFilter {
			if data, err := os.ReadFile(filepath.Join(cache.Dir(dd), cache.Key("subjects", sid, "characters.json"))); err == nil {
				var chars []struct {
					ID     int `json:"id"`
					Actors []struct{ ID int `json:"id"` } `json:"actors"`
				}
				if json.Unmarshal(data, &chars) == nil {
					for _, c := range chars {
						charSet[c.ID] = true
						for _, a := range c.Actors {
							if a.ID > 0 { persSet[a.ID] = true }
						}
					}
				}
			}
			if data, err := os.ReadFile(filepath.Join(cache.Dir(dd), cache.Key("subjects", sid, "persons.json"))); err == nil {
				var persons []struct{ ID int `json:"id"` }
				if json.Unmarshal(data, &persons) == nil {
					for _, p := range persons { persSet[p.ID] = true }
				}
			}
		}
		for id := range charSet { charIDs = append(charIDs, id) }
		for id := range persSet { persIDs = append(persIDs, id) }
	} else {
		subjIDs, _ = cache.ListIDs(dd, "subjects")
		charIDs, _ = cache.ListIDs(dd, "characters")
		persIDs, _ = cache.ListIDs(dd, "persons")
	}

	// Subjects
	if p != nil && totalPhases > 0 { p.SetPhase(phaseBase, totalPhases, "下载条目图像") }
	if p != nil { p.Send("images_subjects", 0, len(subjIDs), "downloading") }
	dlImageList(subjIDs, "subject", nil, imgBase, bg, p, "images_subjects")

	// Characters
	if p != nil && totalPhases > 0 { p.SetPhase(phaseBase+1, totalPhases, "下载角色图像") }
	if p != nil { p.Send("images_characters", 0, len(charIDs), "downloading") }
	dlImageList(charIDs, "character", nil, imgBase, bg, p, "images_characters")

	// Persons
	if p != nil && totalPhases > 0 { p.SetPhase(phaseBase+2, totalPhases, "下载人物图像") }
	if p != nil { p.Send("images_persons", 0, len(persIDs), "downloading") }
	dlImageList(persIDs, "person", nil, imgBase, bg, p, "images_persons")

	log.Info("images download complete")
}

// imageExists checks if all three sizes of an image exist on disk.
func imageExists(imgBase, kind string, id int) bool {
	for _, size := range []string{"large", "grid", "small"} {
		path := filepath.Join(imgBase, fmt.Sprintf("%ss_%s/%d/%d.jpg", kind, size, id%10, id))
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

func dlImageList(ids []int, kind string, imgMap map[int]cache.ImageEntry, imgBase string, bg *bangumi.Client, p *Progress, stage string) {
	if len(ids) == 0 {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency)
	var done int
	var mu sync.Mutex
	for _, id := range ids {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			dlImage(bg, kind, id, imgMap, imgBase, &mu, p)
			if p != nil {
				mu.Lock()
				done++
				p.Send(stage, done, len(ids), "")
				mu.Unlock()
			}
		}(id)
	}
	wg.Wait()
}

func dlImage(bg *bangumi.Client, kind string, id int, imgMap map[int]cache.ImageEntry, imgBase string, mu *sync.Mutex, p *Progress) {
	// Dedup: check map (legacy path) or disk (new path)
	if imgMap != nil {
		mu.Lock()
		entry, hasAll := imgMap[id]
		mu.Unlock()
		if hasAll && entry.Large != "" && entry.Grid != "" && entry.Small != "" {
			return
		}
	} else {
		if imageExists(imgBase, kind, id) {
			return
		}
	}

	// 三个尺寸并发下载
	type result struct {
		size, path string
	}
	var wg sync.WaitGroup
	results := make(chan result, 3)

	for _, size := range []string{"large", "grid", "small"} {
		wg.Add(1)
		go func(size string) {
			defer wg.Done()
			data, err := bg.GetImage(fmt.Sprintf("v0/%ss/%d/image?type=%s", kind, id, size))
			if err != nil {
				if !strings.Contains(err.Error(), "placeholder") {
					log.Warn("image download failed", "kind", kind, "id", id, "size", size, "err", err)
					if p != nil { p.SetError(fmt.Sprintf("Image %s #%d %s: %v", kind, id, size, err)) }
				}
				return
			}
			relPath := fmt.Sprintf("%ss_%s/%d/%d.jpg", kind, size, id%10, id)
			fullPath := filepath.Join(imgBase, relPath)
			os.MkdirAll(filepath.Dir(fullPath), 0o755)
			os.WriteFile(fullPath, data, 0o644)
			results <- result{size, relPath}
		}(size)
	}
	wg.Wait()
	close(results)

	var entry cache.ImageEntry
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

	if imgMap != nil && (entry.Large != "" || entry.Grid != "" || entry.Small != "") {
		mu.Lock()
		imgMap[id] = entry
		mu.Unlock()
	}
}

// dlMissingSizes downloads only the specified missing sizes for an image entry.
func dlMissingSizes(bg *bangumi.Client, kind string, id int, sizes []string, imgMap map[int]cache.ImageEntry, imgBase string, mu *sync.Mutex, p *Progress) {
	mu.Lock()
	entry := imgMap[id]
	mu.Unlock()

	type result struct {
		size, path string
	}
	var wg sync.WaitGroup
	results := make(chan result, len(sizes))

	for _, size := range sizes {
		wg.Add(1)
		go func(size string) {
			defer wg.Done()
			data, err := bg.GetImage(fmt.Sprintf("v0/%ss/%d/image?type=%s", kind, id, size))
			if err != nil {
				if !strings.Contains(err.Error(), "placeholder") {
					log.Warn("image download failed", "kind", kind, "id", id, "size", size, "err", err)
					if p != nil { p.SetError(fmt.Sprintf("Image %s #%d %s: %v", kind, id, size, err)) }
				}
				return
			}
			relPath := fmt.Sprintf("%ss_%s/%d/%d.jpg", kind, size, id%10, id)
			fullPath := filepath.Join(imgBase, relPath)
			os.MkdirAll(filepath.Dir(fullPath), 0o755)
			os.WriteFile(fullPath, data, 0o644)
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

// fillImageGaps fills missing image sizes and downloads images for entities not yet in the image index.
func fillImageGaps(dd string, bg *bangumi.Client, p *Progress) {
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
		imgMap := loadImageIndex(dd, d.kind+"s_image.json")
		nameList := loadNameList(cache.IndexFile(dd, d.kind+"s.json"))

		// Phase 1: Fill missing sizes for existing entries
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
			sem := make(chan struct{}, maxConcurrency)
			var done int
			var mu sync.Mutex
			for _, pt := range partials {
				wg.Add(1)
				go func(id int, sizes []string) {
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

		// Phase 2: Download all sizes for entries missing entirely from image index
		var missingIDs []int
		for _, e := range nameList {
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
			dlImageList(missingIDs, d.kind, imgMap, imgBase, bg, p, "fill_"+d.kind+"_miss")
		}
		phaseNum++

		saveJSON(cache.IndexFile(dd, d.kind+"s_image.json"), imgMap)
	}
}

// RebuildImageIndex scans the images/ directory and rebuilds *_image.json files.
func RebuildImageIndex(dd string) {
	imgBase := filepath.Join(dd, "images")
	kinds := []string{"subject", "character", "person"}
	for _, kind := range kinds {
		m := map[int]cache.ImageEntry{}
		for _, size := range []string{"large", "grid", "small"} {
			dir := filepath.Join(imgBase, kind+"s_"+size)
			digits, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, d := range digits {
				if !d.IsDir() { continue }
				files, _ := os.ReadDir(filepath.Join(dir, d.Name()))
				for _, f := range files {
					name := f.Name()
					if ext := filepath.Ext(name); ext != ".jpg" {
						continue
					}
					id, err := strconv.Atoi(strings.TrimSuffix(name, ".jpg"))
					if err != nil { continue }
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
		saveJSON(cache.IndexFile(dd, kind+"s_image.json"), m)
		log.Info("image index rebuilt", "kind", kind+"s", "count", len(m))
	}
}

// ── Helpers ──

func loadImageIndex(dd, name string) map[int]cache.ImageEntry {
	data, err := os.ReadFile(cache.IndexFile(dd, name))
	if err != nil {
		return map[int]cache.ImageEntry{}
	}
	var m map[int]cache.ImageEntry
	json.Unmarshal(data, &m)
	if m == nil { m = map[int]cache.ImageEntry{} }
	return m
}

func serveImage(w http.ResponseWriter, r *http.Request, dd, kind, size string) {
	if size == "" { size = "grid" }
	idStr := r.PathValue("id")
	imgFile := cache.IndexFile(dd, kind+"s_image.json")
	data, err := os.ReadFile(imgFile)
	if err == nil {
		var images map[int]cache.ImageEntry
		json.Unmarshal(data, &images)
		id, _ := strconv.Atoi(idStr)
		entry, ok := images[id]
		if ok {
			path := entry.Large
			if size == "grid" {
				path = entry.Grid
			} else if size == "small" {
				path = entry.Small
			}
			if path != "" {
				fullPath := filepath.Join(dd, "images", path)
				if _, err := os.Stat(fullPath); err == nil {
					w.Header().Set("Cache-Control", "no-store")
					http.ServeFile(w, r, fullPath)
					return
				}
			}
		}
	}
	// 返回 404 以触发前端 onerror 回退逻辑
	http.NotFound(w, r)
}

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/vanadiry/seshat/Core/bangumi"
	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/log"
)

func downloadImages(dd string, bg *bangumi.Client, p *Progress) {
	downloadImagesWithPhase(dd, bg, p, 3, 5)
}

func downloadImagesWithPhase(dd string, bg *bangumi.Client, p *Progress, phaseBase, totalPhases int) {
	log.Info("Downloading images...")
	os.MkdirAll(cache.IndexDir(dd), 0o755)

	subjImg := loadImageIndex(dd, "subjects_image.json")
	charImg := loadImageIndex(dd, "characters_image.json")
	persImg := loadImageIndex(dd, "persons_image.json")
	imgBase := filepath.Join(dd, "images")

	// Subjects
	subjList := loadNameList(cache.IndexFile(dd, "subjects.json"))
	if p != nil { p.SetPhase(phaseBase, totalPhases, "下载Subject图像"); p.Send("images_subjects", 0, len(subjList), "downloading") }
	dlImageList(subjList, "subject", subjImg, imgBase, bg, dd, p, "images_subjects")
	saveJSON(cache.IndexFile(dd, "subjects_image.json"), subjImg)

	// Characters
	charList := loadNameList(cache.IndexFile(dd, "characters.json"))
	if p != nil { p.SetPhase(phaseBase+1, totalPhases, "下载角色图像"); p.Send("images_characters", 0, len(charList), "downloading") }
	dlImageList(charList, "character", charImg, imgBase, bg, dd, p, "images_characters")
	saveJSON(cache.IndexFile(dd, "characters_image.json"), charImg)

	// Persons
	persList := loadNameList(cache.IndexFile(dd, "persons.json"))
	if p != nil { p.SetPhase(phaseBase+2, totalPhases, "下载人物图像"); p.Send("images_persons", 0, len(persList), "downloading") }
	dlImageList(persList, "person", persImg, imgBase, bg, dd, p, "images_persons")
	saveJSON(cache.IndexFile(dd, "persons_image.json"), persImg)

	log.Info("Images download complete")
}

func dlImageList(list []cache.NameEntry, kind string, imgMap map[int]cache.ImageEntry, imgBase string, bg *bangumi.Client, dd string, p *Progress, stage string) {
	if len(list) == 0 {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency)
	var done int
	var mu sync.Mutex
	for _, entry := range list {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			dlImage(bg, kind, id, imgMap, imgBase, &mu)
			if p != nil {
				mu.Lock()
				done++
				if done%10 == 0 || done == len(list) {
					p.Send(stage, done, len(list), "")
				}
				mu.Unlock()
			}
		}(entry.ID)
	}
	wg.Wait()
}

func dlImage(bg *bangumi.Client, kind string, id int, imgMap map[int]cache.ImageEntry, imgBase string, mu *sync.Mutex) {
	var entry cache.ImageEntry
	for _, size := range []string{"large", "grid", "small"} {
		data, err := bg.GetImage(fmt.Sprintf("v0/%ss/%d/image?type=%s", kind, id, size))
		if err != nil {
			continue
		}
		relPath := fmt.Sprintf("%ss_%s/%d/%d.jpg", kind, size, id%10, id)
		fullPath := filepath.Join(imgBase, relPath)
		os.MkdirAll(filepath.Dir(fullPath), 0o755)
		os.WriteFile(fullPath, data, 0o644)
		if size == "large" {
			entry.Large = relPath
		} else if size == "grid" {
			entry.Grid = relPath
		} else if size == "small" {
			entry.Small = relPath
		}
	}
	if entry.Large != "" || entry.Grid != "" || entry.Small != "" {
		mu.Lock()
		imgMap[id] = entry
		mu.Unlock()
	}
}

// dlMissingSizes downloads only the specified missing sizes for an image entry.
func dlMissingSizes(bg *bangumi.Client, kind string, id int, sizes []string, imgMap map[int]cache.ImageEntry, imgBase string, mu *sync.Mutex) {
	mu.Lock()
	entry := imgMap[id]
	mu.Unlock()
	for _, size := range sizes {
		data, err := bg.GetImage(fmt.Sprintf("v0/%ss/%d/image?type=%s", kind, id, size))
		if err != nil {
			continue
		}
		relPath := fmt.Sprintf("%ss_%s/%d/%d.jpg", kind, size, id%10, id)
		fullPath := filepath.Join(imgBase, relPath)
		os.MkdirAll(filepath.Dir(fullPath), 0o755)
		os.WriteFile(fullPath, data, 0o644)
		switch size {
		case "large":
			entry.Large = relPath
		case "grid":
			entry.Grid = relPath
		case "small":
			entry.Small = relPath
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
		nameSet := make(map[int]bool)
		for _, e := range nameList {
			nameSet[e.ID] = true
		}

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
					dlMissingSizes(bg, d.kind, id, sizes, imgMap, imgBase, &mu)
					if p != nil {
						mu.Lock()
						done++
						if done%10 == 0 || done == len(partials) {
							p.Send("fill_"+d.kind+"_sizes", done, len(partials), "")
						}
						mu.Unlock()
					}
				}(pt.id, pt.sizes)
			}
			wg.Wait()
		}
		phaseNum++

		// Phase 2: Download all sizes for entries missing entirely from image index
		var missingIDs []cache.NameEntry
		for _, e := range nameList {
			if _, ok := imgMap[e.ID]; !ok {
				missingIDs = append(missingIDs, e)
			}
		}
		if p != nil {
			p.SetPhase(phaseNum, 11, d.labelMiss)
		}
		if len(missingIDs) > 0 {
			if p != nil {
				p.Send("fill_"+d.kind+"_miss", 0, len(missingIDs), "")
			}
			dlImageList(missingIDs, d.kind, imgMap, imgBase, bg, dd, p, "fill_"+d.kind+"_miss")
		}
		phaseNum++

		saveJSON(cache.IndexFile(dd, d.kind+"s_image.json"), imgMap)
	}
}

// ── Search ──

// quickSearch 使用 list 文件快速搜索。
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
					http.ServeFile(w, r, fullPath)
					return
				}
			}
		}
	}
	// 回退
	if len(noImageData) > 0 {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(noImageData)
		return
	}
	http.NotFound(w, r)
}

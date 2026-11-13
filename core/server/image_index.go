package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

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

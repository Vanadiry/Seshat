package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/vanadiry/seshat/core/store"
)

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

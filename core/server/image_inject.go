package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/vanadiry/seshat/core/cache"
)

// injectSpec 返回文件对应的图片类型和是否为数组
func injectSpec(domain, file string) (kind string, array bool, ok bool) {
	switch domain + "/" + file {
	case "subjects/info.json":
		return "subject", false, true
	case "subjects/characters.json":
		return "character", true, true
	case "subjects/persons.json":
		return "person", true, true
	case "subjects/subjects.json":
		return "subject", true, true
	case "characters/info.json":
		return "character", false, true
	case "characters/persons.json":
		return "person", true, true
	case "persons/info.json":
		return "person", false, true
	case "persons/characters.json":
		return "character", true, true
	}
	return "", false, false
}

func jsonInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p == "http" || p == "https" {
		scheme = p
	}
	host := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		if i := strings.IndexByte(h, ','); i >= 0 {
			h = h[:i]
		}
		host = strings.TrimSpace(h)
	}
	return scheme + "://" + host
}

// buildImages 生成本地图片地址，没有图片时返回占位图或空字符串
func buildImages(kind string, id int, base string, index map[int]cache.ImageEntry) map[string]string {
	entry := index[id]
	keys := []string{"large", "common", "medium", "small", "grid"}
	if kind != "subject" {
		keys = []string{"large", "medium", "small", "grid"}
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		picked := pickImageSize(entry, key)
		if picked == "" {
			if kind == "subject" {
				out[key] = base + "/assets/no-image.png"
			} else {
				out[key] = ""
			}
			continue
		}
		out[key] = base + "/api/v0/" + kind + "s/" + strconv.Itoa(id) + "/image?type=" + picked
	}
	return out
}

// injectImages 在响应时补齐 images，不修改缓存文件
func injectImages(data []byte, domain, file string, id int, r *http.Request, dd string) []byte {
	kind, array, ok := injectSpec(domain, file)
	if !ok {
		return data
	}
	var root any
	if json.Unmarshal(data, &root) != nil {
		return data
	}
	index := loadImageIndex(dd, kind+"s_image.json")
	base := requestBaseURL(r)
	switch v := root.(type) {
	case map[string]any:
		if !array {
			v["images"] = buildImages(kind, id, base, index)
		}
	case []any:
		if array {
			for _, item := range v {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				itemID := jsonInt(m["id"])
				if itemID <= 0 {
					continue
				}
				m["images"] = buildImages(kind, itemID, base, index)
			}
		}
	}
	result, err := json.Marshal(root)
	if err != nil {
		return data
	}
	return result
}

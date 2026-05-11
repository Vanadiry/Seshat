package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func Dir(dataDir string) string { return filepath.Join(dataDir, "api") }
func Get(dataDir, key string) ([]byte, error) {
	return os.ReadFile(filepath.Join(Dir(dataDir), key))
}
func Put(dataDir, key string, data []byte) error {
	p := filepath.Join(Dir(dataDir), key)
	os.MkdirAll(filepath.Dir(p), 0o755)
	return os.WriteFile(p, compress(data), 0o644)
}
func Has(dataDir, key string) bool {
	_, err := os.Stat(filepath.Join(Dir(dataDir), key))
	return err == nil
}
func List(dataDir, dir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(Dir(dataDir), dir))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names, nil
}
func compress(data []byte) []byte {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return data
	}
	r, _ := json.Marshal(v)
	return r
}

// ── Image handling ──

var imgURLRe = regexp.MustCompile(`https?://lain\.bgm\.tv/([^"'\s]+)`)

// classifyImage determines the local directory for a Bangumi image URL.
// cover → subject, crt with _crt_ → character, crt with _prsn_ → person
func classifyImage(url string) (kind string, id int) {
	u := strings.TrimPrefix(url, "https://lain.bgm.tv/")
	u = strings.TrimPrefix(u, "http://lain.bgm.tv/")
	base := filepath.Base(u)
	base = strings.TrimSuffix(base, filepath.Ext(base))

	// Extract ID from filename (first numeric segment)
	for _, seg := range strings.Split(base, "_") {
		if n, err := strconv.Atoi(seg); err == nil && n > 0 {
			id = n
			break
		}
	}
	if id == 0 {
		return "", 0
	}

	// Determine type + size
	size := "large"
	if strings.Contains(u, "/g/") || strings.Contains(u, "_grid") {
		size = "grid"
	}
	if strings.Contains(u, "/cover/") {
		kind = "subject_" + size
	} else if strings.Contains(u, "_prsn_") {
		kind = "person_" + size
	} else {
		kind = "character_" + size
	}
	return
}

func localPath(kind string, id int) string {
	return fmt.Sprintf("%s/%d/%d.jpg", kind, id%10, id)
}

// ReplaceImageURLs replaces Bangumi image URLs with short local paths.
func ReplaceImageURLs(data []byte) []byte {
	return imgURLRe.ReplaceAllFunc(data, func(match []byte) []byte {
		kind, id := classifyImage(string(match))
		if id == 0 {
			return match
		}
		return []byte(fmt.Sprintf("/images/%s", localPath(kind, id)))
	})
}

// ProcessImages downloads all Bangumi images referenced in JSON data.
func ProcessImages(data []byte, imgDir string) {
	imgURLRe.ReplaceAllFunc(data, func(match []byte) []byte {
		url := string(match)
		kind, id := classifyImage(url)
		if id == 0 {
			return match
		}
		p := filepath.Join(imgDir, localPath(kind, id))
		if _, err := os.Stat(p); err == nil {
			return match
		}
		os.MkdirAll(filepath.Dir(p), 0o755)
		resp, err := http.Get(url)
		if err != nil {
			return match
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return match
		}
		f, _ := os.Create(p)
		if f != nil {
			defer f.Close()
			io.Copy(f, resp.Body)
		}
		return match
	})
}

package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func Dir(dataDir string) string      { return filepath.Join(dataDir, "api") }
func IndexDir(dataDir string) string { return filepath.Join(dataDir, "index") }

// Key builds a cache path: {subdomain}/{id%10}/{id}/{file}
// e.g. Key("subjects", 51, "info.json") → "subjects/1/51/info.json"
func Key(subdomain string, id int, file string) string {
	return fmt.Sprintf("%s/%d/%d/%s", subdomain, id%10, id, file)
}

func Get(dataDir, key string) ([]byte, error) { return os.ReadFile(filepath.Join(Dir(dataDir), key)) }
func Put(dataDir, key string, data []byte) error {
	p := filepath.Join(Dir(dataDir), key)
	os.MkdirAll(filepath.Dir(p), 0o755)
	return os.WriteFile(p, data, 0o644)
}
func Has(dataDir, key string) bool {
	_, err := os.Stat(filepath.Join(Dir(dataDir), key))
	return err == nil
}
func List(dataDir, dir string) ([]string, error) {
	base := filepath.Join(Dir(dataDir), dir)
	var names []string
	digits, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	for _, d := range digits {
		if !d.IsDir() { continue }
		ids, _ := os.ReadDir(filepath.Join(base, d.Name()))
		for _, id := range ids {
			if !id.IsDir() { continue }
			names = append(names, id.Name())
		}
	}
	return names, nil
}

// ListIDs returns all cached IDs for a domain by scanning the API data directory.
func ListIDs(dataDir, domain string) ([]int, error) {
	strs, err := List(dataDir, domain)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(strs))
	for _, s := range strs {
		if id, err := strconv.Atoi(s); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// ── Index generation ──

// StripImages returns JSON data with all "images" fields removed recursively.
func StripImages(data []byte) []byte {
	// Try as object first
	var obj map[string]any
	if json.Unmarshal(data, &obj) == nil {
		stripImagesRecursive(obj)
		r, _ := json.Marshal(obj)
		return r
	}
	// Try as array
	var arr []any
	if json.Unmarshal(data, &arr) == nil {
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				stripImagesRecursive(m)
			}
		}
		r, _ := json.Marshal(arr)
		return r
	}
	return data
}

func stripImagesRecursive(v map[string]any) {
	delete(v, "images")
	delete(v, "image")
	for _, key := range []string{"actors", "characters", "subjects", "persons", "responses", "data", "items"} {
		if arr, ok := v[key].([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					stripImagesRecursive(m)
				}
			}
		}
	}
}

// SubjectSummary is a lightweight subject entry for list files.
type SubjectSummary struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	NameCN   string  `json:"name_cn"`
	Score    float64 `json:"score"`
	Platform string  `json:"platform"`
	Date     string  `json:"date"`
}

// NameEntry is a simple name+id entry for character/person lists.
type NameEntry struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	NameCN string `json:"name_cn"`
}

// ImageEntry records local image paths.
type ImageEntry struct {
	Large string `json:"large"`
	Grid  string `json:"grid"`
	Small string `json:"small"`
}

func localImagePath(kind string, id int) string {
	return fmt.Sprintf("%s/%d/%d.jpg", kind, id%10, id)
}

func IndexFile(dd, name string) string { return filepath.Join(IndexDir(dd), name) }

package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Dir(dataDir string) string          { return filepath.Join(dataDir, "api") }
func IndexDir(dataDir string) string     { return filepath.Join(dataDir, "index") }
func Get(dataDir, key string) ([]byte, error) { return os.ReadFile(filepath.Join(Dir(dataDir), key)) }
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

// ── Index generation ──

// StripImages returns JSON data with the "images" field removed.
func StripImages(data []byte) []byte {
	var v map[string]any
	if json.Unmarshal(data, &v) != nil {
		return data
	}
	delete(v, "images")
	// Also strip nested images in array items
	if actors, ok := v["actors"].([]any); ok {
		for _, a := range actors {
			if am, ok := a.(map[string]any); ok {
				delete(am, "images")
			}
		}
	}
	// Strip images from array items
	for _, key := range []string{"data", "items"} {
		if arr, ok := v[key].([]any); ok {
			for _, item := range arr {
				if im, ok := item.(map[string]any); ok {
					delete(im, "images")
				}
			}
		}
	}
	r, _ := json.Marshal(v)
	return r
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
}

func localImagePath(kind string, id int) string {
	return fmt.Sprintf("%s/%d/%d.jpg", kind, id%10, id)
}

func IndexFile(dd, name string) string { return filepath.Join(IndexDir(dd), name) }

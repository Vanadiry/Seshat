package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func Dir(dataDir string) string      { return filepath.Join(dataDir, "api") }
func IndexDir(dataDir string) string { return filepath.Join(dataDir, "index") }

// Key 生成数据路径 {subdomain}/{id%10}/{id}/{file}
// 例如 Key("subjects", 51, "info.json") 得到 "subjects/1/51/info.json"
func Key(subdomain string, id int, file string) string {
	return fmt.Sprintf("%s/%d/%d/%s", subdomain, id%10, id, file)
}

func Get(dataDir, key string) ([]byte, error) { return os.ReadFile(filepath.Join(Dir(dataDir), key)) }
func Put(dataDir, key string, data []byte) error {
	return WriteAtomic(filepath.Join(Dir(dataDir), key), data, 0o644)
}

// WriteAtomic 写同目录临时文件后 rename，避免中途失败留下半截文件
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// WriteAtomicFast 与 WriteAtomic 相同，但跳过 fsync
// 用于批量导入这类可重试、更看重速度的场景
func WriteAtomicFast(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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
		if !d.IsDir() {
			continue
		}
		ids, err := os.ReadDir(filepath.Join(base, d.Name()))
		if err != nil {
			continue
		}
		for _, id := range ids {
			if !id.IsDir() {
				continue
			}
			names = append(names, id.Name())
		}
	}
	return names, nil
}

// ListIDs 扫描 API 数据目录，返回某个领域已保存的全部 ID
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

// 索引生成

// StripImages 递归删除 JSON 中的 images 字段
func StripImages(data []byte) []byte {
	// 按对象解析
	var obj map[string]any
	if json.Unmarshal(data, &obj) == nil {
		stripImagesRecursive(obj)
		r, err := json.Marshal(obj)
		if err != nil {
			return data
		}
		return r
	}
	// 对象解析失败后按数组解析
	var arr []any
	if json.Unmarshal(data, &arr) == nil {
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				stripImagesRecursive(m)
			}
		}
		r, err := json.Marshal(arr)
		if err != nil {
			return data
		}
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

// SubjectSummary 为列表文件使用的轻量条目
type SubjectSummary struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	NameCN   string  `json:"name_cn"`
	Score    float64 `json:"score"`
	Platform string  `json:"platform"`
	Date     string  `json:"date"`
}

// NameEntry 为角色和人物列表使用的名称与 ID
type NameEntry struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	NameCN string `json:"name_cn"`
}

// ImageEntry 记录本地图片路径
type ImageEntry struct {
	Large string `json:"large"`
	Grid  string `json:"grid"`
	Small string `json:"small"`
}

func localImagePath(kind string, id int) string {
	return fmt.Sprintf("%s/%d/%d.jpg", kind, id%10, id)
}

func IndexFile(dd, name string) string { return filepath.Join(IndexDir(dd), name) }

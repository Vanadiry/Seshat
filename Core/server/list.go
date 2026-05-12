package server

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/vanadiry/seshat/Core/cache"
)

func mergeListEntry(path string, id int, name, nameCN string) {
	listMutex.Lock()
	defer listMutex.Unlock()
	os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := os.ReadFile(path)
	var list []cache.NameEntry
	json.Unmarshal(data, &list)
	for i, e := range list {
		if e.ID == id {
			if name != "" { list[i].Name = name }
			if nameCN != "" {
				list[i].NameCN = nameCN
			}
			data, _ = json.Marshal(list)
			os.WriteFile(path, data, 0o644)
			return
		}
	}
	list = append(list, cache.NameEntry{ID: id, Name: name, NameCN: nameCN})
	data, _ = json.Marshal(list)
	os.WriteFile(path, data, 0o644)
}

// mergeSubjectEntry 将一个 subject 条目合并到 subjects 列表（去重更新）。
func mergeSubjectEntry(path string, s cache.SubjectSummary) {
	listMutex.Lock()
	defer listMutex.Unlock()
	os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := os.ReadFile(path)
	var list []cache.SubjectSummary
	json.Unmarshal(data, &list)
	for i, e := range list {
		if e.ID == s.ID {
			list[i] = s
			data, _ = json.Marshal(list)
			os.WriteFile(path, data, 0o644)
			return
		}
	}
	list = append(list, s)
	data, _ = json.Marshal(list)
	os.WriteFile(path, data, 0o644)
}

// removeListEntry 从 list 文件中移除指定 ID。
func removeListEntry(path string, id int) {
	listMutex.Lock()
	defer listMutex.Unlock()
	os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := os.ReadFile(path)
	var list []cache.NameEntry
	json.Unmarshal(data, &list)
	for i, e := range list {
		if e.ID == id {
			list = append(list[:i], list[i+1:]...)
			data, _ = json.Marshal(list)
			os.WriteFile(path, data, 0o644)
			return
		}
	}
}

// getRawWithRetry 拉取 API 数据，网络错误重试 maxRetries 次，404 不重试。
// extractNameCN 从 JSON 的 infobox 中提取简体中文名。
func saveJSON(path string, v any) {
	data, _ := json.Marshal(v)
	os.WriteFile(path, data, 0o644)
}

// ── Tags index ──

type tagInfo struct {
	Count    int   `json:"count"`
	Subjects []int `json:"subjects"`
}

func loadNameList(path string) []cache.NameEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var list []cache.NameEntry
	json.Unmarshal(data, &list)
	return list
}

// (saveJSON already declared above)

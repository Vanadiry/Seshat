package server

import (
	"strings"
	"fmt"
	"net/http"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/config"
	"github.com/vanadiry/seshat/Core/bangumi"
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

func handleListSubjects(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(cache.IndexFile(dd, "subjects.json"))
		if err != nil { writeJSON(w, []any{}); return }
		var list []cache.SubjectSummary
		json.Unmarshal(data, &list)
		writeJSON(w, list)
	}
}

func handleListCharacters(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(cache.IndexFile(dd, "characters.json"))
		if err != nil { writeJSON(w, []any{}); return }
		var list []cache.NameEntry
		json.Unmarshal(data, &list)
		writeJSON(w, list)
	}
}

func handleListPersons(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(cache.IndexFile(dd, "persons.json"))
		if err != nil { writeJSON(w, []any{}); return }
		var list []cache.NameEntry
		json.Unmarshal(data, &list)
		writeJSON(w, list)
	}
}

func handleCacheReader(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/api/v0/") + ".json"
		data, err := cache.Get(dd, key)
		if err != nil { http.NotFound(w, r); return }
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

func handleUserProfile(cfg *config.Config, bg *bangumi.Client, dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uname := cfg.User.Username
		if uname == "" { http.Error(w, `{"error":"no username"}`, 404); return }
		data, err := cache.Get(dd, fmt.Sprintf("users/%s.json", uname))
		if err != nil {
			raw, err := bg.GetRaw(fmt.Sprintf("v0/users/%s", uname))
			if err != nil { http.Error(w, `{"error":"user not found"}`, 404); return }
			cache.Put(dd, fmt.Sprintf("users/%s.json", uname), raw)
			w.Header().Set("Content-Type", "application/json")
			w.Write(raw)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

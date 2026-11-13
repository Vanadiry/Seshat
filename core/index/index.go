// Package index 负责扫描本地数据并生成列表、标签和图片索引
package index

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

// TagInfo 标签的条目数量和条目 ID
type TagInfo struct {
	Count    int   `json:"count"`
	Subjects []int `json:"subjects"`
}

func tagsPath(dd string) string {
	return store.IndexFile(dd, "tags.json")
}

// LoadTags 读取标签索引
func LoadTags(dd string) map[string]TagInfo {
	tags, _ := LoadCached[map[string]TagInfo](tagsPath(dd))
	if tags == nil {
		tags = map[string]TagInfo{}
	}
	return tags
}

// SaveJSON 写入索引文件并清除对应缓存
func SaveJSON(path string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Error("saveJSON marshal", "path", path, "err", err)
		events.Bus.Error("索引文件保存失败")
		return
	}
	if err := store.WriteAtomic(path, data, 0o644); err != nil {
		log.Error("saveJSON write", "path", path, "err", err)
		events.Bus.Error("索引文件保存失败")
		return
	}
	ClearCache(path)
}

// ScanAPIDir 遍历数据目录 {domain}/{id%10}/{id}/*.json，对每个文件调用 fn
func ScanAPIDir(dd, domain string, fn func(path string)) {
	base := filepath.Join(store.Dir(dd), domain)
	digits, _ := os.ReadDir(base)
	for _, d := range digits {
		if !d.IsDir() {
			continue
		}
		ids, _ := os.ReadDir(filepath.Join(base, d.Name()))
		for _, id := range ids {
			if !id.IsDir() {
				continue
			}
			idDir := filepath.Join(base, d.Name(), id.Name())
			files, _ := os.ReadDir(idDir)
			for _, f := range files {
				if strings.HasSuffix(f.Name(), ".json") && !f.IsDir() {
					fn(filepath.Join(idDir, f.Name()))
				}
			}
		}
	}
}

// Build 扫描已保存的 API JSON，重建列表、标签和名称索引。
// report 用于上报阶段进度，canceled 返回 true 时提前结束。
func Build(dd string, report func(step, total int, name string), canceled func() bool) {
	reportStep := func(step, total int, name string) {
		if report != nil {
			report(step, total, name)
		}
	}
	stopped := func() bool {
		return canceled != nil && canceled()
	}
	log.Info("rebuilding indexes from scan")
	if err := os.MkdirAll(store.IndexDir(dd), 0o755); err != nil {
		log.Error("index dir mkdir failed", "err", err)
	}

	var subjects []store.SubjectSummary
	var chars []store.NameEntry
	var persons []store.NameEntry
	tags := map[string]TagInfo{}

	// 扫描条目
	reportStep(0, 5, "扫描条目")
	ScanAPIDir(dd, "subjects", func(path string) {
		if stopped() {
			return
		}
		if !strings.HasSuffix(path, "info.json") {
			return
		}
		data, _ := os.ReadFile(path)
		var s struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			NameCN string `json:"name_cn"`
			Rating struct {
				Score float64 `json:"score"`
			} `json:"rating"`
			Platform string `json:"platform"`
			Date     string `json:"date"`
			Tags     []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"tags"`
		}
		if json.Unmarshal(data, &s) == nil && s.ID > 0 {
			subjects = append(subjects, store.SubjectSummary{ID: s.ID, Name: s.Name, NameCN: s.NameCN, Score: s.Rating.Score, Platform: s.Platform, Date: s.Date})
			for _, t := range s.Tags {
				info := tags[t.Name]
				info.Count++
				info.Subjects = append(info.Subjects, s.ID)
				tags[t.Name] = info
			}
		}
	})
	SaveJSON(store.IndexFile(dd, "subjects.json"), subjects)

	if stopped() {
		return
	}
	// 扫描角色
	reportStep(1, 5, "扫描角色")
	ScanAPIDir(dd, "characters", func(path string) {
		if stopped() {
			return
		}
		if !strings.HasSuffix(path, "info.json") {
			return
		}
		data, _ := os.ReadFile(path)
		var c struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Infobox []struct {
				Key   string          `json:"key"`
				Value json.RawMessage `json:"value"`
			} `json:"infobox"`
		}
		if json.Unmarshal(data, &c) == nil && c.ID > 0 {
			nameCN := ""
			for _, ib := range c.Infobox {
				if ib.Key == "简体中文名" {
					var v string
					if json.Unmarshal(ib.Value, &v) == nil {
						nameCN = v
					}
					break
				}
			}
			chars = append(chars, store.NameEntry{ID: c.ID, Name: c.Name, NameCN: nameCN})
		}
	})
	SaveJSON(store.IndexFile(dd, "characters.json"), chars)

	if stopped() {
		return
	}
	// 扫描人物
	reportStep(2, 5, "扫描人物")
	ScanAPIDir(dd, "persons", func(path string) {
		if stopped() {
			return
		}
		if !strings.HasSuffix(path, "info.json") {
			return
		}
		data, _ := os.ReadFile(path)
		var p struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Infobox []struct {
				Key   string          `json:"key"`
				Value json.RawMessage `json:"value"`
			} `json:"infobox"`
		}
		if json.Unmarshal(data, &p) == nil && p.ID > 0 {
			nameCN := ""
			for _, ib := range p.Infobox {
				if ib.Key == "简体中文名" {
					var v string
					if json.Unmarshal(ib.Value, &v) == nil {
						nameCN = v
					}
					break
				}
			}
			persons = append(persons, store.NameEntry{ID: p.ID, Name: p.Name, NameCN: nameCN})
		}
	})
	SaveJSON(store.IndexFile(dd, "persons.json"), persons)

	if stopped() {
		return
	}
	// 保存标签
	reportStep(3, 5, "保存标签")
	SaveJSON(store.IndexFile(dd, "tags.json"), tags)
	buildNameIndex(dd, "subjects")
	buildNameIndex(dd, "characters")
	buildPersonNames(dd)

	if stopped() {
		return
	}
	// 从磁盘重建图片索引
	reportStep(4, 5, "重建图片索引")
	RebuildImageIndex(dd)

	log.Info("rebuild complete", "subjects", len(subjects), "chars", len(chars), "persons", len(persons), "tags", len(tags))
}

// buildPersonNames 从 persons.json 生成名称到 ID 的映射
func buildPersonNames(dd string) {
	list, err := LoadCached[[]store.NameEntry](store.IndexFile(dd, "persons.json"))
	if err != nil {
		log.Warn("persons_name: persons.json not found")
		return
	}
	m := map[int][]string{}
	for _, p := range list {
		if p.Name != "" {
			m[p.ID] = append(m[p.ID], p.Name)
		}
		if p.NameCN != "" && p.NameCN != p.Name {
			m[p.ID] = append(m[p.ID], p.NameCN)
		}
	}
	result, err := json.Marshal(m)
	if err != nil {
		log.Error("persons name index marshal", "err", err)
		events.Bus.Error("人物名称索引生成失败")
		return
	}
	namePath := store.IndexFile(dd, "persons_name.json")
	if err := store.WriteAtomic(namePath, result, 0o644); err != nil {
		log.Error("persons name index write", "err", err)
		events.Bus.Error("人物名称索引写入失败")
	}
	ClearCache(namePath)
	log.Info("persons_name.json built", "count", len(m))
}

// buildNameIndex 为指定领域生成名称到 ID 的映射
func buildNameIndex(dd, domain string) {
	list, err := LoadCached[[]store.NameEntry](store.IndexFile(dd, domain+".json"))
	if err != nil {
		log.Warn("name index file not found", "domain", domain)
		return
	}
	m := map[int][]string{}
	for _, e := range list {
		if e.Name != "" {
			m[e.ID] = append(m[e.ID], e.Name)
		}
		if e.NameCN != "" && e.NameCN != e.Name {
			m[e.ID] = append(m[e.ID], e.NameCN)
		}
	}
	result, err := json.Marshal(m)
	if err != nil {
		log.Error("name index marshal", "domain", domain, "err", err)
		events.Bus.Error("名称索引生成失败")
		return
	}
	namePath := store.IndexFile(dd, domain+"_name.json")
	if err := store.WriteAtomic(namePath, result, 0o644); err != nil {
		log.Error("name index write", "domain", domain, "err", err)
		events.Bus.Error("名称索引写入失败")
	}
	ClearCache(namePath)
	log.Info("name index built", "domain", domain, "count", len(m))
}

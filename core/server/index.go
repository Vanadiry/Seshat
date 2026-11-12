package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

// scanAPIDir 遍历数据目录 {domain}/{id%10}/{id}/*.json，对每个文件调用 fn
func scanAPIDir(dd, domain string, fn func(path string)) {
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

// 阶段三：通过官方端点下载图片
func tagsPath(dd string) string {
	return store.IndexFile(dd, "tags.json")
}

func loadTags(dd string) map[string]tagInfo {
	tags, _ := loadCachedIndex[map[string]tagInfo](tagsPath(dd))
	if tags == nil {
		tags = map[string]tagInfo{}
	}
	return tags
}

// rebuildTags 扫描所有已保存的 subject JSON，重建 tags.json
func rebuildTags(dd string) {
	tags := map[string]tagInfo{}
	scanAPIDir(dd, "subjects", func(path string) {
		if !strings.HasSuffix(path, "info.json") {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var s struct {
			ID   int `json:"id"`
			Tags []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"tags"`
		}
		if err := json.Unmarshal(data, &s); err != nil {
			return
		}
		for _, t := range s.Tags {
			info := tags[t.Name]
			info.Count++
			info.Subjects = append(info.Subjects, s.ID)
			tags[t.Name] = info
		}
	})
	result, err := json.Marshal(tags)
	if err != nil {
		log.Error("tags index marshal", "err", err)
		events.Bus.Error("标签索引生成失败")
		return
	}
	if err := store.WriteAtomic(tagsPath(dd), result, 0o644); err != nil {
		log.Error("tags index write", "err", err)
		events.Bus.Error("标签索引写入失败")
	}
	buildNameIndex(dd, "subjects")
	buildNameIndex(dd, "characters")
	buildPersonNames(dd)
	log.Info("tags index rebuilt", "count", len(tags))
}

// buildPersonNames 从 persons.json 生成名称到 ID 的映射
func buildPersonNames(dd string) {
	list, err := loadCachedIndex[[]store.NameEntry](store.IndexFile(dd, "persons.json"))
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
	clearIndexCache(namePath)
	log.Info("persons_name.json built", "count", len(m))
}

// buildNameIndex 为指定领域生成名称到 ID 的映射，领域为 subjects、characters 或 persons
func buildNameIndex(dd, domain string) {
	list, err := loadCachedIndex[[]store.NameEntry](store.IndexFile(dd, domain+".json"))
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
	clearIndexCache(namePath)
	log.Info("name index built", "domain", domain, "count", len(m))
}

// buildIndexes 扫描已保存的 API JSON，重建列表、标签和名称索引
func buildIndexes(dd string, p *Progress) {
	buildIndexesWithSteps(dd, func(step, total int, name string) {
		if p != nil {
			p.SendPhaseStep(step, total, name)
		}
	})
}

// buildIndexesPhased 把索引重建的每个步骤作为独立阶段上报
func buildIndexesPhased(dd string, p *Progress) {
	buildIndexesWithSteps(dd, func(step, total int, name string) {
		if p != nil {
			p.BeginPhase(step + 1)
		}
	})
}

// buildIndexesWithSteps 与 buildIndexes 相同，但通过 report 上报阶段进度
func buildIndexesWithSteps(dd string, report func(step, total int, name string)) {
	reportStep := func(step, total int, name string) {
		if report != nil {
			report(step, total, name)
		}
	}
	log.Info("rebuilding indexes from scan")
	if err := os.MkdirAll(store.IndexDir(dd), 0o755); err != nil {
		log.Error("index dir mkdir failed", "err", err)
	}

	var subjects []store.SubjectSummary
	var chars []store.NameEntry
	var persons []store.NameEntry
	tags := map[string]tagInfo{}

	// 扫描条目
	reportStep(0, 5, "扫描条目")
	scanAPIDir(dd, "subjects", func(path string) {
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
	saveJSON(store.IndexFile(dd, "subjects.json"), subjects)

	// 扫描角色
	reportStep(1, 5, "扫描角色")
	scanAPIDir(dd, "characters", func(path string) {
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
	saveJSON(store.IndexFile(dd, "characters.json"), chars)

	// 扫描人物
	reportStep(2, 5, "扫描人物")
	scanAPIDir(dd, "persons", func(path string) {
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
	saveJSON(store.IndexFile(dd, "persons.json"), persons)

	// 保存标签
	reportStep(3, 5, "保存标签")
	saveJSON(store.IndexFile(dd, "tags.json"), tags)
	buildNameIndex(dd, "subjects")
	buildNameIndex(dd, "characters")
	buildPersonNames(dd)

	// 从磁盘重建图片索引
	reportStep(4, 5, "重建图片索引")
	RebuildImageIndex(dd)

	log.Info("rebuild complete", "subjects", len(subjects), "chars", len(chars), "persons", len(persons), "tags", len(tags))
}

func handleTags(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tags := loadTags(dd)
		type tagItem struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}
		var list []tagItem
		for name, info := range tags {
			list = append(list, tagItem{Name: name, Count: info.Count})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Count > list[j].Count })
		writeList(w, list)
	}
}

func handleTagSubjects(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		tags := loadTags(dd)
		info, ok := tags[name]
		if !ok {
			writeList(w, []any{})
			return
		}
		var subjects []any
		for _, sid := range info.Subjects {
			data, err := store.Get(dd, store.Key("subjects", sid, "info.json"))
			if err != nil {
				continue
			}
			var s struct {
				ID     int    `json:"id"`
				Name   string `json:"name"`
				NameCN string `json:"name_cn"`
				Rating struct {
					Score float64 `json:"score"`
				} `json:"rating"`
				Platform string `json:"platform"`
			}
			json.Unmarshal(data, &s)
			subjects = append(subjects, s)
		}
		writeList(w, subjects)
	}
}

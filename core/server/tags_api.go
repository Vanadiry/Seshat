package server

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/vanadiry/seshat/core/index"
	"github.com/vanadiry/seshat/core/store"
)

// buildIndexes 扫描本地数据重建索引
func buildIndexes(dd string, p *Progress) {
	index.Build(dd, func(step, total int, name string) {
		if p != nil {
			p.SendPhaseStep(step, total, name)
		}
	}, func() bool { return taskCanceled(p) })
}

// buildIndexesPhased 把索引重建的每个步骤作为独立阶段上报
func buildIndexesPhased(dd string, p *Progress) {
	index.Build(dd, func(step, total int, name string) {
		if p != nil {
			p.BeginPhase(step + 1)
		}
	}, func() bool { return taskCanceled(p) })
}

func handleTags(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tags := index.LoadTags(dd)
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
		tags := index.LoadTags(dd)
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

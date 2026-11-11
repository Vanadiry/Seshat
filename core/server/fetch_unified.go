package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

const (
	fetchSourceTrackerNew   = "tracker_new"
	fetchSourceTrackerAll   = "tracker_all"
	fetchSourceTrackerNames = "tracker_names"
	fetchSourceIDs          = "ids"
	fetchSourceLocal        = "local"

	fetchJSONFill      = "fill"
	fetchJSONOverwrite = "overwrite"

	fetchImagesNone      = "none"
	fetchImagesFill      = "fill"
	fetchImagesOverwrite = "overwrite"
)

type fetchRequest struct {
	Source string   `json:"source"`
	Names  []string `json:"names"`
	IDs    []int    `json:"ids"`
	JSON   string   `json:"json"`
	Images string   `json:"images"`
}

func (r *fetchRequest) validate() error {
	switch r.Source {
	case fetchSourceTrackerNew, fetchSourceTrackerAll, fetchSourceLocal:
	case fetchSourceTrackerNames:
		if len(r.Names) == 0 {
			return fmt.Errorf("source=tracker_names 时 names 不能为空")
		}
		for _, name := range r.Names {
			if !validTrackerName(name) {
				return fmt.Errorf("Tracker 名称不合法: %s", name)
			}
		}
	case fetchSourceIDs:
		if len(r.IDs) == 0 {
			return fmt.Errorf("source=ids 时 ids 不能为空")
		}
		for _, id := range r.IDs {
			if id <= 0 {
				return fmt.Errorf("ID 必须大于 0")
			}
		}
	default:
		return fmt.Errorf("source 不合法")
	}
	switch r.JSON {
	case fetchJSONFill, fetchJSONOverwrite:
	default:
		return fmt.Errorf("json 不合法")
	}
	switch r.Images {
	case fetchImagesNone, fetchImagesFill, fetchImagesOverwrite:
	default:
		return fmt.Errorf("images 不合法")
	}
	return nil
}

func trackerIDsByNames(cfg *config.Config, names []string) []int {
	seen := map[int]bool{}
	var ids []int
	for _, name := range names {
		for _, sid := range trackerIDsByID(cfg, name) {
			if !seen[sid] {
				seen[sid] = true
				ids = append(ids, sid)
			}
		}
	}
	return ids
}

func resolveFetchIDs(cfg *config.Config, dd string, req fetchRequest) ([]int, error) {
	switch req.Source {
	case fetchSourceTrackerNew:
		all := allTrackerIDs(cfg)
		existing := map[int]bool{}
		if list, err := loadCachedIndex[[]store.SubjectSummary](store.IndexFile(dd, "subjects.json")); err == nil {
			for _, s := range list {
				existing[s.ID] = true
			}
		}
		ids := make([]int, 0, len(all))
		for _, sid := range all {
			if !existing[sid] || subjectCoreIncomplete(dd, sid) {
				ids = append(ids, sid)
			}
		}
		return ids, nil
	case fetchSourceTrackerAll:
		return allTrackerIDs(cfg), nil
	case fetchSourceTrackerNames:
		ids := trackerIDsByNames(cfg, req.Names)
		if len(ids) == 0 {
			return nil, fmt.Errorf("Tracker 不存在或为空")
		}
		return ids, nil
	case fetchSourceIDs:
		return req.IDs, nil
	case fetchSourceLocal:
		return collectAllSubjectIDs(dd), nil
	default:
		return nil, fmt.Errorf("source 不合法")
	}
}

// fetchSubjectListFill 只重拉不完整条目，并补齐已有条目的角色和人物详情
func fetchSubjectListFill(ids []int, bg *bangumi.Client, dd, imgDir string, p *Progress, concurrency int) []int {
	success := make([]int, 0, len(ids))
	for i, sid := range ids {
		if subjectCoreIncomplete(dd, sid) {
			if fetchAll(sid, bg, dd, imgDir, p, concurrency, false) {
				success = append(success, sid)
			}
		} else {
			success = append(success, sid)
		}
		if p != nil {
			p.Send("subjects", i+1, len(ids), "")
		}
	}
	fillRelatedGaps(dd, bg, ids, p)
	return success
}

// fillRelatedGaps 补齐现有条目的角色和人物详情
func fillRelatedGaps(dd string, bg *bangumi.Client, ids []int, p *Progress) {
	for _, sid := range ids {
		if data, err := os.ReadFile(filepath.Join(store.Dir(dd), store.Key("subjects", sid, "characters.json"))); err == nil {
			var chars []struct {
				ID int `json:"id"`
			}
			if json.Unmarshal(data, &chars) == nil {
				for _, c := range chars {
					if store.Has(dd, store.Key("characters", c.ID, "info.json")) {
						continue
					}
					d, e := bg.GetRaw(fmt.Sprintf("v0/characters/%d", c.ID))
					if e != nil {
						log.Warn("gap character fetch failed", "id", c.ID, "err", e)
						if p != nil {
							p.SetError(fmt.Sprintf("Character #%d: %v", c.ID, e))
						}
						continue
					}
					putData(dd, store.Key("characters", c.ID, "info.json"), store.StripImages(d), p)
				}
			}
		}
		if data, err := os.ReadFile(filepath.Join(store.Dir(dd), store.Key("subjects", sid, "persons.json"))); err == nil {
			var persons []struct {
				ID int `json:"id"`
			}
			if json.Unmarshal(data, &persons) == nil {
				for _, pp := range persons {
					if store.Has(dd, store.Key("persons", pp.ID, "info.json")) {
						continue
					}
					d, e := bg.GetRaw(fmt.Sprintf("v0/persons/%d", pp.ID))
					if e != nil {
						log.Warn("gap person fetch failed", "id", pp.ID, "err", e)
						if p != nil {
							p.SetError(fmt.Sprintf("Person #%d: %v", pp.ID, e))
						}
						continue
					}
					putData(dd, store.Key("persons", pp.ID, "info.json"), store.StripImages(d), p)
				}
			}
		}
	}
}

func handleFetchUnified(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req fetchRequest
		limitBody(w, r)
		if !decodeJSON(w, r, &req) {
			return
		}
		serveFetchRequest(w, r, cfg, bg, dd, imgDir, req)
	}
}

func serveFetchRequest(w http.ResponseWriter, r *http.Request, cfg *config.Config, bg *bangumi.Client, dd, imgDir string, req fetchRequest) {
	resp, code, err := dispatchFetch(cfg, bg, dd, imgDir, req)
	if err != nil {
		writeError(w, r, code, err.Error())
		return
	}
	writeJSON(w, resp)
}

func dispatchFetch(cfg *config.Config, bg *bangumi.Client, dd, imgDir string, req fetchRequest) (map[string]any, int, error) {
	if cfg.Upstream.BaseURL == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("base_url 未配置")
	}
	if err := req.validate(); err != nil {
		return nil, http.StatusBadRequest, err
	}
	ids, err := resolveFetchIDs(cfg, dd, req)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	if taskLocked() {
		return nil, http.StatusConflict, fmt.Errorf("已有任务正在运行，请等待完成")
	}
	if len(ids) == 0 {
		return map[string]any{"status": "done", "count": 0}, http.StatusOK, nil
	}

	p := newProgress(len(ids), "fetch", "拉取数据")
	log.Info("unified fetch", "source", req.Source, "count", len(ids), "json", req.JSON, "images", req.Images)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
				events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
			}
		}()
		p.SetPhase(1, 3, "拉取条目")
		var success []int
		if req.JSON == fetchJSONOverwrite {
			success = fetchSubjectList(ids, bg, dd, imgDir, p, cfg.Server.ConcurrencyInfo)
		} else {
			success = fetchSubjectListFill(ids, bg, dd, imgDir, p, cfg.Server.ConcurrencyInfo)
		}

		if req.Images != fetchImagesNone {
			p.SetPhase(2, 3, "下载图片")
			if req.Images == fetchImagesOverwrite {
				downloadImagesScopedForce(dd, bg, p, 2, 3, ids, cfg.Server.ConcurrencyImage)
			} else {
				downloadImagesScoped(dd, bg, p, 2, 3, ids, cfg.Server.ConcurrencyImage)
			}
		}

		p.SetPhase(3, 3, "重建索引")
		buildIndexes(dd, p)
		if req.Source == fetchSourceIDs {
			for _, sid := range success {
				addToSeshatTracker(sid)
			}
		}
		p.Send("complete", len(ids), len(ids), "")
		p.Close()
	}()
	return map[string]any{"status": "fetching", "count": len(ids), "task_id": p.ID}, http.StatusOK, nil
}

// 旧的按 ID 拉取仍保留独立入口

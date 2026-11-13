package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/fetch"
	"github.com/vanadiry/seshat/core/index"
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
		if list, err := index.LoadCached[[]store.SubjectSummary](store.IndexFile(dd, "subjects.json")); err == nil {
			for _, s := range list {
				existing[s.ID] = true
			}
		}
		ids := make([]int, 0, len(all))
		for _, sid := range all {
			if !existing[sid] || fetch.SubjectCoreIncomplete(dd, sid) {
				ids = append(ids, sid)
			}
		}
		return ids, nil
	case fetchSourceTrackerAll:
		return allTrackerIDs(cfg), nil
	case fetchSourceTrackerNames:
		if missing := missingTrackerNames(cfg, req.Names); len(missing) > 0 {
			return nil, fmt.Errorf("未找到 Tracker：%s", strings.Join(missing, "、"))
		}
		ids := trackerIDsByNames(cfg, req.Names)
		if len(ids) == 0 {
			return nil, fmt.Errorf("Tracker 为空")
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

func fetchPhasePlan(withImages bool) []phaseSpec {
	var specs []phaseSpec
	if withImages {
		specs = splitPhaseRanges(0, 95, "拉取条目", "下载条目图像", "下载角色图像", "下载人物图像")
	} else {
		specs = splitPhaseRanges(0, 95, "拉取条目")
	}
	return append(specs, phaseSpec{Name: "重建索引", Start: 95, End: 100})
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

	p := newProgress(100, "fetch", "拉取数据")
	p.SetPlan(fetchPhasePlan(req.Images != fetchImagesNone))
	log.Info("unified fetch", "source", req.Source, "count", len(ids), "json", req.JSON, "images", req.Images)
	go func() {
		defer p.Finish()
		defer func() {
			if r := recover(); r != nil {
				log.Error("goroutine panic", "panic", r)
				events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
			}
		}()
		taskBg := bg.WithContext(p.Context())
		p.BeginPhase(1)
		if taskCanceled(p) {
			return
		}
		var success []int
		if req.JSON == fetchJSONOverwrite {
			success = fetch.SubjectList(ids, taskBg, dd, imgDir, p, cfg.Server.ConcurrencyInfo)
		} else {
			success = fetch.SubjectListFill(ids, taskBg, dd, imgDir, p, cfg.Server.ConcurrencyInfo)
		}

		if taskCanceled(p) {
			return
		}
		if req.Images != fetchImagesNone {
			if req.Images == fetchImagesOverwrite {
				fetch.DownloadImagesScopedForce(dd, taskBg, p, 2, ids, cfg.Server.ConcurrencyImage)
			} else {
				fetch.DownloadImagesScoped(dd, taskBg, p, 2, ids, cfg.Server.ConcurrencyImage)
			}
		}

		if taskCanceled(p) {
			return
		}
		p.BeginPhase(p.Phases)
		buildIndexes(dd, p)
		if taskCanceled(p) {
			return
		}
		if req.Source == fetchSourceIDs {
			for _, sid := range success {
				addToSeshatTracker(sid)
			}
		}
	}()
	return map[string]any{"status": "fetching", "count": len(ids), "task_id": p.ID}, http.StatusOK, nil
}

// 旧的按 ID 拉取仍保留独立入口

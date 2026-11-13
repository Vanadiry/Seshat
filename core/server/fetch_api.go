package server

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/fetch"
	"github.com/vanadiry/seshat/core/index"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

func handleFetchAll(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveFetchRequest(w, r, cfg, bg, dd, imgDir, fetchRequest{
			Source: fetchSourceTrackerAll,
			JSON:   fetchJSONOverwrite,
			Images: fetchImagesFill,
		})
	}
}

func handleFetchDeep(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" {
			writeError(w, r, 400, "base_url not configured")
			return
		}
		var req struct {
			ConfirmClean bool `json:"confirm_clean"`
		}
		limitBody(w, r)
		if !decodeJSON(w, r, &req) {
			return
		}
		if !req.ConfirmClean {
			writeError(w, r, http.StatusBadRequest, "重建全部需要确认")
			return
		}
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}
		p := newProgress(100, "rebuild_all", "重建全部")
		go func() {
			defer p.Finish()
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			forceRefresh(cfg, bg.WithContext(p.Context()), dd, imgDir, p)
		}()
		writeJSON(w, map[string]any{"status": "deep refresh", "task_id": p.ID})
	}
}

func handleFetchTracker(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Names []string `json:"names"`
		}
		limitBody(w, r)
		if !decodeJSON(w, r, &req) {
			return
		}
		serveFetchRequest(w, r, cfg, bg, dd, imgDir, fetchRequest{
			Source: fetchSourceTrackerNames,
			Names:  req.Names,
			JSON:   fetchJSONOverwrite,
			Images: fetchImagesFill,
		})
	}
}

func handleFetchUser(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Upstream.BaseURL == "" {
			writeError(w, r, 400, "base_url not configured")
			return
		}
		pref, _ := config.LoadPreferences()
		if pref == nil {
			pref = &config.DefaultPreferences
		}
		uname := pref.Username
		if uname == "" {
			writeError(w, r, 400, "username not configured")
			return
		}
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}
		p := newProgress(100, "fetch_user", "拉取用户数据")
		p.SetPlan(splitPhaseRanges(0, 100, "拉取用户信息", "拉取用户头像", "拉取用户收藏"))
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
			p.SendPhaseStep(0, 1, "")
			if err := fetch.UserInfo(uname, taskBg, dd); err != nil {
				if isCanceled(err) {
					return
				}
				log.Warn("user info fetch failed", "user", uname, "err", err)
				p.SetError("用户信息拉取失败")
				p.SendPhaseStep(1, 1, "失败")
			} else {
				p.SendPhaseStep(1, 1, "")
			}
			if taskCanceled(p) {
				return
			}

			p.BeginPhase(2)
			p.SendPhaseStep(0, 1, "")
			if err := fetch.UserAvatar(uname, taskBg, dd); err != nil {
				if isCanceled(err) {
					return
				}
				log.Warn("user avatar fetch failed", "user", uname, "err", err)
				p.SetError("用户头像拉取失败")
				p.SendPhaseStep(1, 1, "失败")
			} else {
				p.SendPhaseStep(1, 1, "")
			}
			if taskCanceled(p) {
				return
			}

			p.BeginPhase(3)
			p.SendPhaseStep(0, 1, "")
			if err := fetch.UserCollections(uname, taskBg, dd); err != nil {
				if isCanceled(err) {
					return
				}
				log.Warn("user collections fetch failed", "user", uname, "err", err)
				p.SetError("收藏列表拉取失败")
				p.SendPhaseStep(1, 1, "失败")
			} else {
				p.SendPhaseStep(1, 1, "")
			}
		}()
		writeJSON(w, map[string]any{"status": "fetching", "username": uname, "task_id": p.ID})
	}
}

func handleFetchSubject(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID  int   `json:"id"`
			IDs []int `json:"ids"`
		}
		limitBody(w, r)
		if !decodeJSON(w, r, &req) {
			return
		}
		ids := req.IDs
		if req.ID != 0 {
			ids = []int{req.ID}
		}
		serveFetchRequest(w, r, cfg, bg, dd, imgDir, fetchRequest{
			Source: fetchSourceIDs,
			IDs:    ids,
			JSON:   fetchJSONOverwrite,
			Images: fetchImagesFill,
		})
	}
}

func handleFetchUpdate(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveFetchRequest(w, r, cfg, bg, dd, imgDir, fetchRequest{
			Source: fetchSourceTrackerNew,
			JSON:   fetchJSONFill,
			Images: fetchImagesFill,
		})
	}
}

func handleFetchIndex(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if taskLocked() {
			writeError(w, r, 409, "已有任务正在运行，请等待完成")
			return
		}
		p := newProgress(100, "rebuild_index", "重建索引")
		p.SetPlan([]phaseSpec{
			{Name: "扫描条目", Start: 0, End: 40},
			{Name: "扫描角色", Start: 40, End: 55},
			{Name: "扫描人物", Start: 55, End: 70},
			{Name: "保存标签和名称索引", Start: 70, End: 80},
			{Name: "重建图片索引", Start: 80, End: 100},
		})
		go func() {
			defer p.Finish()
			defer func() {
				if r := recover(); r != nil {
					log.Error("goroutine panic", "panic", r)
					events.Bus.Error(fmt.Sprintf("任务发生意外错误：%v", r))
				}
			}()
			buildIndexesPhased(dd, p)
		}()
		writeJSON(w, map[string]any{"status": "rebuilding", "task_id": p.ID})
	}
}

func handleFetchGap(cfg *config.Config, bg *bangumi.Client, dd, imgDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveFetchRequest(w, r, cfg, bg, dd, imgDir, fetchRequest{
			Source: fetchSourceLocal,
			JSON:   fetchJSONFill,
			Images: fetchImagesFill,
		})
	}
}

func handleFetchMeta(cfg *config.Config, bg *bangumi.Client, dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveFetchRequest(w, r, cfg, bg, dd, filepath.Join(dd, "images"), fetchRequest{
			Source: fetchSourceTrackerAll,
			JSON:   fetchJSONOverwrite,
			Images: fetchImagesNone,
		})
	}
}

func collectAllSubjectIDs(dd string) []int {
	list, _ := index.LoadCached[[]store.SubjectSummary](store.IndexFile(dd, "subjects.json"))
	ids := make([]int, len(list))
	for i, e := range list {
		ids[i] = e.ID
	}
	return ids
}

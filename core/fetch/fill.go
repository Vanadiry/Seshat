package fetch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

// SubjectListFill 只重拉不完整条目，并补齐已有条目的角色和人物详情
func SubjectListFill(ids []int, bg *bangumi.Client, dd, imgDir string, p Progress, concurrency int) []int {
	success := make([]int, 0, len(ids))
	for i, sid := range ids {
		if taskCanceled(p) {
			break
		}
		if SubjectCoreIncomplete(dd, sid) {
			if fetchAll(sid, bg, dd, imgDir, p, concurrency, false) {
				success = append(success, sid)
			}
		} else {
			success = append(success, sid)
		}
		if p != nil {
			p.SendPhaseProgress(i+1, len(ids), "")
		}
	}
	if taskCanceled(p) {
		return success
	}
	if p != nil {
		p.SendPhaseStatus("补齐关联数据")
	}
	fillRelatedGaps(dd, bg, ids, p)
	return success
}

// fillRelatedGaps 补齐现有条目的角色和人物详情
func fillRelatedGaps(dd string, bg *bangumi.Client, ids []int, p Progress) {
	for _, sid := range ids {
		if taskCanceled(p) {
			return
		}
		if data, err := os.ReadFile(filepath.Join(store.Dir(dd), store.Key("subjects", sid, "characters.json"))); err == nil {
			var chars []struct {
				ID int `json:"id"`
			}
			if json.Unmarshal(data, &chars) == nil {
				for _, c := range chars {
					if taskCanceled(p) {
						return
					}
					if store.Has(dd, store.Key("characters", c.ID, "info.json")) {
						continue
					}
					d, e := bg.GetRaw(fmt.Sprintf("v0/characters/%d", c.ID))
					if e != nil {
						if isCanceled(e) {
							return
						}
						log.Warn("gap character fetch failed", "id", c.ID, "err", e)
						if p != nil {
							p.SetError(fmt.Sprintf("角色 #%d：%v", c.ID, e))
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
					if taskCanceled(p) {
						return
					}
					if store.Has(dd, store.Key("persons", pp.ID, "info.json")) {
						continue
					}
					d, e := bg.GetRaw(fmt.Sprintf("v0/persons/%d", pp.ID))
					if e != nil {
						if isCanceled(e) {
							return
						}
						log.Warn("gap person fetch failed", "id", pp.ID, "err", e)
						if p != nil {
							p.SetError(fmt.Sprintf("人物 #%d：%v", pp.ID, e))
						}
						continue
					}
					putData(dd, store.Key("persons", pp.ID, "info.json"), store.StripImages(d), p)
				}
			}
		}
	}
}

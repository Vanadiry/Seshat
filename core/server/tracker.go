package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"

	"github.com/vanadiry/seshat/core/bangumi"
	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

// seshatTrackerMu 串行化 _seshat.json 的读改写，避免并发追加互相覆盖
var seshatTrackerMu sync.Mutex

const (
	trackerTypeAuto   = "auto"
	trackerTypeManual = "manual"
)

// trackerInfo 为合并同名文件后的 Tracker 视图
type trackerInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// reservedTrackerID 返回 id 是否保留给内部 Tracker
func reservedTrackerID(id string) bool {
	return strings.HasPrefix(id, "_")
}

// trackerFilePaths 返回指定 id 实际存在的 TOML 和 JSON 文件
func trackerFilePaths(td, id string) []string {
	var paths []string
	for _, ext := range []string{".toml", ".json"} {
		path := filepath.Join(td, id+ext)
		if _, err := os.Stat(path); err == nil {
			paths = append(paths, path)
		}
	}
	return paths
}

// loadTrackerName 读取 Tracker 文件中的展示名称
func loadTrackerName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if strings.HasSuffix(path, ".json") {
		var v struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &v) == nil {
			return strings.TrimSpace(v.Name)
		}
		return ""
	}
	var v struct {
		Name string `toml:"name"`
	}
	if _, err := toml.Decode(string(data), &v); err == nil {
		return strings.TrimSpace(v.Name)
	}
	return ""
}

// loadTrackerInfos 扫描并合并同名 TOML 和 JSON，按 id 排序
func loadTrackerInfos(cfg *config.Config) []trackerInfo {
	td := cfg.TrackerDir()
	files, _ := filepath.Glob(filepath.Join(td, "*.json"))
	files2, _ := filepath.Glob(filepath.Join(td, "*.toml"))
	files = append(files, files2...)

	type merged struct {
		ids       map[int]bool
		jsonName  string
		tomlName  string
		hasManual bool
	}
	byID := map[string]*merged{}
	for _, path := range files {
		base := filepath.Base(path)
		id := strings.TrimSuffix(strings.TrimSuffix(base, ".json"), ".toml")
		if id == "" || strings.HasPrefix(id, ".") {
			continue
		}
		e := byID[id]
		if e == nil {
			e = &merged{ids: map[int]bool{}}
			byID[id] = e
		}
		manual := strings.HasSuffix(path, ".toml")
		if manual {
			e.hasManual = true
		}
		for _, sid := range loadTrackerIDs(path) {
			if sid > 0 {
				e.ids[sid] = true
			}
		}
		name := loadTrackerName(path)
		if name == "" {
			continue
		}
		if manual {
			e.tomlName = name
		} else if e.jsonName == "" {
			e.jsonName = name
		}
	}

	infos := make([]trackerInfo, 0, len(byID))
	for id, e := range byID {
		name := e.tomlName
		if name == "" {
			name = e.jsonName
		}
		if name == "" {
			name = id
		}
		typ := trackerTypeAuto
		if e.hasManual {
			typ = trackerTypeManual
		}
		infos = append(infos, trackerInfo{
			ID:    id,
			Name:  name,
			Type:  typ,
			Count: len(e.ids),
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })
	return infos
}

func countTrackerTotal(cfg *config.Config) int {
	files, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.json"))
	files2, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.toml"))
	files = append(files, files2...)
	seen := map[int]bool{}
	for _, f := range files {
		for _, sid := range loadTrackerIDs(f) {
			seen[sid] = true
		}
	}
	return len(seen)
}

// countTrackerNames 统计指定 tracker 列表中的条目总数
func countTrackerNames(cfg *config.Config, names []string) int {
	td := cfg.TrackerDir()
	seen := map[int]bool{}
	for _, name := range names {
		for _, path := range trackerFilePaths(td, name) {
			for _, sid := range loadTrackerIDs(path) {
				if sid > 0 {
					seen[sid] = true
				}
			}
		}
	}
	return len(seen)
}

// clearContentData 删除内容数据，保留 data/user
func clearContentData(dd string) {
	for _, name := range []string{"api", "images", "index"} {
		if err := os.RemoveAll(filepath.Join(dd, name)); err != nil {
			log.Error("clear content data failed", "dir", name, "err", err)
		}
	}
}

// forceRefresh 删除全部内容数据后完整重建
func forceRefresh(cfg *config.Config, bg *bangumi.Client, dd, imgDir string, p *Progress) {
	log.Info("force refresh: clearing content data")
	clearContentData(dd)
	log.Info("force refresh: content data cleared, starting rebuild")
	refreshAllTrackers(cfg, bg, dd, imgDir, p)
}

// refreshAllTrackers 刷新全部 tracker 列表
func refreshAllTrackers(cfg *config.Config, bg *bangumi.Client, dd, imgDir string, p *Progress) {
	files, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.json"))
	files2, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.toml"))
	files = append(files, files2...)
	seen := map[int]bool{}
	var allIDs []int
	for _, f := range files {
		for _, sid := range loadTrackerIDs(f) {
			if !seen[sid] {
				seen[sid] = true
				allIDs = append(allIDs, sid)
			}
		}
	}
	p.SetPhase(1, 5, "拉取动画数据")
	fetchSubjectList(allIDs, bg, dd, imgDir, p, cfg.Server.ConcurrencyInfo)
	log.Info("all trackers refreshed", "subjects", len(seen))
	downloadImages(dd, bg, p, 2, 5, cfg.Server.ConcurrencyImage)
	p.SetPhase(5, 5, "建立索引")
	buildIndexes(dd, p)
}

// refreshTrackers 刷新指定 tracker 列表
func refreshTrackers(cfg *config.Config, bg *bangumi.Client, dd, imgDir string, names []string, p *Progress) {
	td := cfg.TrackerDir()
	seen := map[int]bool{}
	var allIDs []int
	for _, name := range names {
		for _, path := range trackerFilePaths(td, name) {
			for _, sid := range loadTrackerIDs(path) {
				if sid > 0 && !seen[sid] {
					seen[sid] = true
					allIDs = append(allIDs, sid)
				}
			}
		}
	}
	if len(allIDs) == 0 {
		log.Warn("refreshTrackers: no IDs found", "names", names)
		return
	}
	p.SetPhase(1, 5, "拉取动画数据")
	fetchSubjectList(allIDs, bg, dd, imgDir, p, cfg.Server.ConcurrencyInfo)
	log.Info("trackers refreshed", "names", names, "subjects", len(seen))
	downloadImages(dd, bg, p, 2, 5, cfg.Server.ConcurrencyImage)
	p.SetPhase(5, 5, "建立索引")
	buildIndexes(dd, p)
}

// addToSeshatTracker 将用户手动添加的 subject ID 记录到 _seshat.json
// 文件不存在时会创建，内容损坏时保留原文件并报错，不覆盖用户数据
func addToSeshatTracker(sid int) {
	seshatTrackerMu.Lock()
	defer seshatTrackerMu.Unlock()

	path := filepath.Join(config.Dir(), "tracker", "_seshat.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Error("tracker mkdir", "err", err)
		events.Bus.Error("Tracker 写入失败")
		return
	}
	var data struct {
		Name     string `json:"name"`
		Subjects []int  `json:"subjects"`
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Error("tracker read", "sid", sid, "err", err)
			events.Bus.Error("Tracker 写入失败")
			return
		}
	} else if err := json.Unmarshal(raw, &data); err != nil {
		log.Error("tracker parse", "sid", sid, "err", err)
		events.Bus.Error("Tracker 文件格式错误，已忽略")
		return
	}
	if data.Name == "" {
		data.Name = "seshat"
	}
	for _, existing := range data.Subjects {
		if existing == sid {
			return
		}
	}
	data.Subjects = append(data.Subjects, sid)
	result, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Error("tracker marshal indent", "sid", sid, "err", err)
		events.Bus.Error("Tracker 写入失败")
		return
	}
	if err := store.WriteAtomic(path, result, 0o644); err != nil {
		log.Error("tracker write", "sid", sid, "err", err)
		events.Bus.Error("Tracker 写入失败")
	}
}

// loadTrackerIDs 从 tracker 文件读取条目 ID 列表
func loadTrackerIDs(path string) []int {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	// JSON: {"subjects": [1,2,3]} 或 {"subjects":[{"subject_id":1}]}
	if strings.HasSuffix(path, ".json") {
		var list struct {
			Subjects []int `json:"subjects"`
		}
		if json.Unmarshal(data, &list) == nil && len(list.Subjects) > 0 {
			return list.Subjects
		}
		var userMap struct {
			Subjects map[string]int `json:"subjects"`
		}
		if json.Unmarshal(data, &userMap) == nil && len(userMap.Subjects) > 0 {
			var ids []int
			for k := range userMap.Subjects {
				id, _ := strconv.Atoi(k)
				if id > 0 {
					ids = append(ids, id)
				}
			}
			return ids
		}
		return nil
	}
	// TOML: ids = [1, 2, 3] 或 ids = [\n1,\n2\n]
	return parseTOMLIDArray(data)
}

// parseTOMLIDArray 解析 TOML 格式的 ids = [...] 数组，支持多行逗号分隔
func parseTOMLIDArray(data []byte) []int {
	var ids []int
	inArray := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ids = [") {
			inArray = true
			rest := strings.TrimPrefix(trimmed, "ids = [")
			if idx := strings.Index(rest, "]"); idx >= 0 {
				rest = rest[:idx]
				inArray = false
			}
			parseIDList(rest, &ids)
			continue
		}
		if inArray {
			if idx := strings.Index(trimmed, "]"); idx >= 0 {
				rest := trimmed[:idx]
				inArray = false
				parseIDList(rest, &ids)
			} else {
				parseIDList(trimmed, &ids)
			}
		}
	}
	return ids
}

func parseIDList(s string, ids *[]int) {
	for _, part := range strings.Split(s, ",") {
		var id int
		fmt.Sscanf(strings.TrimSpace(part), "%d", &id)
		if id > 0 {
			*ids = append(*ids, id)
		}
	}
}

// diffTrackerIDs 返回 tracker 中有但本地没有的 ID
func diffTrackerIDs(cfg *config.Config, dd string) []int {
	seen := map[int]bool{}
	var allIDs []int
	files, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.json"))
	files2, _ := filepath.Glob(filepath.Join(cfg.TrackerDir(), "*.toml"))
	for _, f := range append(files, files2...) {
		for _, sid := range loadTrackerIDs(f) {
			if !seen[sid] {
				seen[sid] = true
				allIDs = append(allIDs, sid)
			}
		}
	}
	// 读取已有条目 ID
	existing := map[int]bool{}
	if list, err := loadCachedIndex[[]store.SubjectSummary](store.IndexFile(dd, "subjects.json")); err == nil {
		for _, s := range list {
			existing[s.ID] = true
		}
	}
	var newIDs []int
	for _, sid := range allIDs {
		if !existing[sid] {
			newIDs = append(newIDs, sid)
		}
	}
	return newIDs
}

// fetchConcurrent 并发拉取

// validTrackerName 校验 tracker 名称合法性
func validTrackerName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

func handleTrackerCreate(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		limitBody(w, r)
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" || !validTrackerName(req.Name) {
			writeError(w, r, 400, "tracker name must be alphanumeric, dash, or underscore")
			return
		}
		path := filepath.Join(cfg.TrackerDir(), req.Name+".toml")
		if _, err := os.Stat(path); err == nil {
			writeError(w, r, 409, "tracker already exists")
			return
		}
		tmpl := fmt.Sprintf(config.TrackerTemplate, req.Name, req.Name)
		if err := os.MkdirAll(cfg.TrackerDir(), 0o755); err != nil {
			writeError(w, r, 500, "无法创建 tracker 目录")
			return
		}
		if err := store.WriteAtomic(path, []byte(tmpl), 0o644); err != nil {
			log.Error("tracker create write", "err", err)
			writeError(w, r, 500, "无法写入 tracker 文件")
			return
		}
		writeJSON(w, map[string]string{"status": "created", "name": req.Name})
	}
}

func handleTrackerList(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeList(w, loadTrackerInfos(cfg))
	}
}

func handleImportCollections(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join(userDir(dd), "collections.json"))
		if err != nil {
			writeError(w, r, 400, "collections not found")
			return
		}
		var coll struct {
			Data []struct {
				SubjectID   int `json:"subject_id"`
				SubjectType int `json:"subject_type"`
			} `json:"data"`
		}
		if json.Unmarshal(data, &coll) != nil {
			writeError(w, r, 400, "invalid collections data")
			return
		}
		seen := map[int]bool{}
		var ids []int
		for _, item := range coll.Data {
			if item.SubjectType != 2 || item.SubjectID <= 0 || seen[item.SubjectID] {
				continue
			}
			seen[item.SubjectID] = true
			ids = append(ids, item.SubjectID)
		}
		td := filepath.Join(config.Dir(), "tracker")
		if err := os.MkdirAll(td, 0o755); err != nil {
			writeError(w, r, 500, "无法创建 tracker 目录")
			return
		}
		trackerData, err := json.Marshal(map[string]any{"name": "user", "subjects": ids})
		if err != nil {
			log.Error("import collections marshal", "err", err)
			events.Bus.Error("收藏列表导入失败")
			writeError(w, r, 500, "internal error")
			return
		}
		if err := store.WriteAtomic(filepath.Join(td, "user.json"), trackerData, 0o644); err != nil {
			log.Error("import collections write", "err", err)
			writeError(w, r, 500, "无法写入 tracker 文件")
			return
		}
		writeJSON(w, map[string]any{"status": "ok", "count": len(ids)})
	}
}

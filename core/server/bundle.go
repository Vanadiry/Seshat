package server

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

const bundleFormat = "seshat-bundle"

type bundleExportRequest struct {
	Source      string   `json:"source"`
	IDs         []int    `json:"ids"`
	Trackers    []string `json:"trackers"`
	Description string   `json:"description"`
}

type bundleManifest struct {
	Format      string        `json:"format"`
	Description string        `json:"description,omitempty"`
	ExportedAt  string        `json:"exported_at"`
	Subjects    []int         `json:"subjects"`
	Characters  []int         `json:"characters"`
	Persons     []int         `json:"persons"`
	Images      []bundleImage `json:"images"`
}

type bundleImage struct {
	Kind  string   `json:"kind"`
	ID    int      `json:"id"`
	Sizes []string `json:"sizes"`
}

type bundleMissing struct {
	Kind   string `json:"kind"`
	Entity string `json:"entity,omitempty"`
	ID     int    `json:"id"`
	Size   string `json:"size,omitempty"`
}

type bundleCounts struct {
	Subjects   int `json:"subjects"`
	Characters int `json:"characters"`
	Persons    int `json:"persons"`
	Images     int `json:"images"`
}

type bundleFile struct {
	tarPath string
	fsPath  string
}

type bundleCollector struct {
	dd         string
	subjects   []int
	characters []int
	persons    []int
	images     []bundleImage
	files      []bundleFile
	missing    []bundleMissing
	seenSub    map[int]bool
	seenChar   map[int]bool
	seenPerson map[int]bool
	seenFile   map[string]bool
}

func handleBundleExport(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if taskLocked() {
			writeError(w, r, http.StatusConflict, "已有任务正在运行，请等待完成")
			return
		}
		var req bundleExportRequest
		limitBody(w, r)
		if !decodeJSON(w, r, &req) {
			return
		}
		ids, err := bundleRootIDs(cfg, req)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		if len(ids) == 0 {
			writeError(w, r, http.StatusBadRequest, "没有可导出的条目")
			return
		}
		result, err := exportBundle(cfg, ids, strings.TrimSpace(req.Description))
		if err != nil {
			if err == errNoBundleData {
				writeError(w, r, http.StatusBadRequest, "没有可导出的本地条目")
				return
			}
			log.Error("bundle export", "err", err)
			writeError(w, r, http.StatusInternalServerError, "导出数据包失败")
			return
		}
		writeJSON(w, result)
	}
}

var errNoBundleData = fmt.Errorf("没有可导出的本地条目")

func bundleRootIDs(cfg *config.Config, req bundleExportRequest) ([]int, error) {
	seen := map[int]bool{}
	var ids []int
	switch req.Source {
	case "ids":
		if len(req.IDs) == 0 {
			return nil, fmt.Errorf("ids 不能为空")
		}
		for _, id := range req.IDs {
			if id <= 0 {
				return nil, fmt.Errorf("ID 必须大于 0")
			}
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	case "trackers":
		if len(req.Trackers) == 0 {
			return nil, fmt.Errorf("trackers 不能为空")
		}
		for _, name := range req.Trackers {
			if !validTrackerName(name) {
				return nil, fmt.Errorf("Tracker 名称不合法: %s", name)
			}
		}
		if missing := missingTrackerNames(cfg, req.Trackers); len(missing) > 0 {
			return nil, fmt.Errorf("未找到 Tracker：%s", strings.Join(missing, "、"))
		}
		for _, id := range trackerIDsByNames(cfg, req.Trackers) {
			if id > 0 && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	default:
		return nil, fmt.Errorf("source 不合法")
	}
	sort.Ints(ids)
	return ids, nil
}

func exportBundle(cfg *config.Config, rootIDs []int, description string) (map[string]any, error) {
	dd := cfg.DataDir()
	c := &bundleCollector{
		dd:         dd,
		seenSub:    map[int]bool{},
		seenChar:   map[int]bool{},
		seenPerson: map[int]bool{},
		seenFile:   map[string]bool{},
		missing:    []bundleMissing{},
		images:     []bundleImage{},
	}

	for _, id := range rootIDs {
		if c.seenSub[id] {
			continue
		}
		c.seenSub[id] = true
		if !c.addEntity("subjects", id, &c.subjects, c.missingEntity("subjects", id)) {
			continue
		}
		c.addImages("subject", "subjects", id)
		c.addCharacters(id)
		c.addPersons(id)
	}
	if len(c.files) == 0 {
		return nil, errNoBundleData
	}

	sort.Ints(c.subjects)
	sort.Ints(c.characters)
	sort.Ints(c.persons)
	sort.Slice(c.images, func(i, j int) bool {
		if c.images[i].Kind != c.images[j].Kind {
			return c.images[i].Kind < c.images[j].Kind
		}
		return c.images[i].ID < c.images[j].ID
	})
	sort.Slice(c.missing, func(i, j int) bool {
		a, b := c.missing[i], c.missing[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Entity != b.Entity {
			return a.Entity < b.Entity
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Size < b.Size
	})
	sort.Slice(c.files, func(i, j int) bool { return c.files[i].tarPath < c.files[j].tarPath })

	manifest := bundleManifest{
		Format:      bundleFormat,
		Description: description,
		ExportedAt:  time.Now().Format(time.RFC3339),
		Subjects:    c.subjects,
		Characters:  c.characters,
		Persons:     c.persons,
		Images:      c.images,
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}

	exportsDir := filepath.Join(config.Dir(), "exports")
	if err := os.MkdirAll(exportsDir, 0o755); err != nil {
		return nil, err
	}
	filename := nextBundleFilename(exportsDir)
	finalPath := filepath.Join(exportsDir, filename)
	tmp, err := os.CreateTemp(exportsDir, ".seshat-export-*.tmp")
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	keep := false
	defer func() {
		tmp.Close()
		if !keep {
			os.Remove(tmpName)
		}
	}()

	tw := tar.NewWriter(tmp)
	if err := writeTarFile(tw, "manifest.json", manifestData, time.Now()); err != nil {
		return nil, err
	}
	for _, f := range c.files {
		if err := writeTarPath(tw, f.tarPath, f.fsPath); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		return nil, err
	}
	keep = true
	stat, err := os.Stat(finalPath)
	if err != nil {
		return nil, err
	}

	imageFiles := 0
	for _, img := range c.images {
		imageFiles += len(img.Sizes)
	}
	return map[string]any{
		"status":   "ok",
		"path":     finalPath,
		"filename": filename,
		"size":     stat.Size(),
		"counts": bundleCounts{
			Subjects:   len(c.subjects),
			Characters: len(c.characters),
			Persons:    len(c.persons),
			Images:     imageFiles,
		},
		"missing": c.missing,
	}, nil
}

func (c *bundleCollector) addEntity(domain string, id int, list *[]int, missing bundleMissing) bool {
	dir := filepath.Join(store.Dir(c.dd), domain, fmt.Sprintf("%d", id%10), fmt.Sprintf("%d", id))
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		c.missing = append(c.missing, missing)
		return false
	}
	added := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(store.Dir(c.dd), path)
		if err != nil {
			return nil
		}
		tarPath := "api/" + filepath.ToSlash(rel)
		if !c.seenFile[tarPath] {
			c.seenFile[tarPath] = true
			c.files = append(c.files, bundleFile{tarPath: tarPath, fsPath: path})
			added = true
		}
		return nil
	})
	if !added {
		c.missing = append(c.missing, missing)
		return false
	}
	*list = append(*list, id)
	return true
}

func (c *bundleCollector) addCharacters(sid int) {
	dir := filepath.Join(store.Dir(c.dd), "subjects", fmt.Sprintf("%d", sid%10), fmt.Sprintf("%d", sid))
	data, err := os.ReadFile(filepath.Join(dir, "characters.json"))
	if err != nil {
		return
	}
	var chars []struct {
		ID     int `json:"id"`
		Actors []struct {
			ID int `json:"id"`
		} `json:"actors"`
	}
	if json.Unmarshal(data, &chars) != nil {
		return
	}
	for _, ch := range chars {
		if ch.ID <= 0 || c.seenChar[ch.ID] {
			continue
		}
		c.seenChar[ch.ID] = true
		if !c.addEntity("characters", ch.ID, &c.characters, c.missingEntity("characters", ch.ID)) {
			continue
		}
		c.addImages("character", "characters", ch.ID)
		for _, actor := range ch.Actors {
			c.addPerson(actor.ID)
		}
	}
}

func (c *bundleCollector) addPersons(sid int) {
	dir := filepath.Join(store.Dir(c.dd), "subjects", fmt.Sprintf("%d", sid%10), fmt.Sprintf("%d", sid))
	data, err := os.ReadFile(filepath.Join(dir, "persons.json"))
	if err != nil {
		return
	}
	var persons []struct {
		ID int `json:"id"`
	}
	if json.Unmarshal(data, &persons) != nil {
		return
	}
	for _, p := range persons {
		c.addPerson(p.ID)
	}
}

func (c *bundleCollector) addPerson(id int) {
	if id <= 0 || c.seenPerson[id] {
		return
	}
	c.seenPerson[id] = true
	if !c.addEntity("persons", id, &c.persons, c.missingEntity("persons", id)) {
		return
	}
	c.addImages("person", "persons", id)
}

func (c *bundleCollector) addImages(kind, entity string, id int) {
	sizes := []string{}
	for _, size := range []string{"large", "grid", "small"} {
		path := filepath.Join(c.dd, "images", fmt.Sprintf("%ss_%s", kind, size), fmt.Sprintf("%d", id%10), fmt.Sprintf("%d.jpg", id))
		if _, err := os.Stat(path); err != nil {
			c.missing = append(c.missing, bundleMissing{Kind: "images", Entity: entity, ID: id, Size: size})
			continue
		}
		rel, err := filepath.Rel(c.dd, path)
		if err != nil {
			continue
		}
		tarPath := filepath.ToSlash(rel)
		if !c.seenFile[tarPath] {
			c.seenFile[tarPath] = true
			c.files = append(c.files, bundleFile{tarPath: tarPath, fsPath: path})
		}
		sizes = append(sizes, size)
	}
	if len(sizes) > 0 {
		c.images = append(c.images, bundleImage{Kind: entity, ID: id, Sizes: sizes})
	}
}

func (c *bundleCollector) missingEntity(kind string, id int) bundleMissing {
	return bundleMissing{Kind: kind, ID: id}
}

func nextBundleFilename(dir string) string {
	base := "seshat-export-" + time.Now().Format("20060102-150405")
	name := base + ".tar"
	for i := 1; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, name)); os.IsNotExist(err) {
			return name
		}
		name = fmt.Sprintf("%s-%d.tar", base, i)
	}
}

func writeTarFile(tw *tar.Writer, name string, data []byte, modTime time.Time) error {
	h := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), ModTime: modTime}
	if err := tw.WriteHeader(h); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func writeTarPath(tw *tar.Writer, tarPath, fsPath string) error {
	f, err := os.Open(fsPath)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	h := &tar.Header{Name: tarPath, Mode: 0o644, Size: info.Size(), ModTime: info.ModTime()}
	if err := tw.WriteHeader(h); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

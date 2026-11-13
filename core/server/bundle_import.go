package server

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/store"
)

const (
	bundleImportMax   = 1 << 30
	bundleManifestMax = 1 << 20
	bundleMaxEntries  = 200000
)

func handleBundleImport(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if taskLocked() {
			writeError(w, r, http.StatusConflict, "已有任务正在运行，请等待完成")
			return
		}
		if !bundleMu.TryLock() {
			writeError(w, r, http.StatusConflict, "数据包操作正在进行")
			return
		}
		p := newProgress(100, "bundle_import", "导入数据包")
		p.SetPlan(bundleImportPhasePlan())
		ok := false
		defer func() {
			if !ok {
				p.Close()
				bundleMu.Unlock()
			}
		}()

		importsDir := filepath.Join(config.Dir(), "imports")
		if err := os.MkdirAll(importsDir, 0o755); err != nil {
			writeError(w, r, http.StatusInternalServerError, "无法创建导入目录")
			return
		}
		tmp, err := os.CreateTemp(importsDir, "upload-*.tar")
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "无法创建临时文件")
			return
		}
		tmpPath := tmp.Name()
		r.Body = http.MaxBytesReader(w, r.Body, bundleImportMax)
		if _, err := io.Copy(tmp, r.Body); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeError(w, r, http.StatusRequestEntityTooLarge, "数据包不能超过 1 GiB")
				return
			}
			writeError(w, r, http.StatusBadRequest, "读取数据包失败")
			return
		}
		if err := tmp.Close(); err != nil {
			os.Remove(tmpPath)
			writeError(w, r, http.StatusInternalServerError, "写入临时文件失败")
			return
		}
		stat, err := os.Stat(tmpPath)
		if err != nil || stat.Size() == 0 {
			os.Remove(tmpPath)
			writeError(w, r, http.StatusBadRequest, "数据包为空")
			return
		}

		ok = true
		go processBundleImport(cfg, tmpPath, p)
		writeJSON(w, map[string]any{"status": "importing", "task_id": p.ID})
	}
}

func processBundleImport(cfg *config.Config, tarPath string, p *Progress) {
	defer bundleMu.Unlock()
	defer p.Close()
	defer os.Remove(tarPath)
	defer func() {
		if r := recover(); r != nil {
			p.Send("error", 0, 0, fmt.Sprintf("导入失败: %v", r))
		}
	}()

	importsDir := filepath.Join(config.Dir(), "imports")
	staging, err := os.MkdirTemp(importsDir, "unpack-*")
	if err != nil {
		p.Send("error", 0, 0, "无法创建解包目录")
		return
	}
	defer os.RemoveAll(staging)

	specs := bundleImportPhasePlan()
	p.BeginPhase(1)
	extractLast := -1
	if err := extractBundleTar(tarPath, staging, func(read, total int64) {
		percent := specs[0].Start
		if total > 0 {
			span := int64(specs[0].End - specs[0].Start)
			percent += int(read * span / total)
		}
		if percent == extractLast {
			return
		}
		extractLast = percent
		p.SendPercent(percent, "")
	}, func() bool { return taskCanceled(p) }); err != nil {
		if errors.Is(err, errTaskCanceled) {
			p.Send("cancelled", p.Done, p.Total, "")
			return
		}
		p.Send("error", 0, 0, err.Error())
		return
	}
	if taskCanceled(p) {
		p.Send("cancelled", p.Done, p.Total, "")
		return
	}

	p.BeginPhase(2)
	commitLast := -1
	if _, err := commitBundle(staging, cfg.DataDir(), func(done, total int) {
		percent := specs[1].Start
		if total > 0 {
			percent += done * (specs[1].End - specs[1].Start) / total
		}
		if percent == commitLast {
			return
		}
		commitLast = percent
		p.SendPercent(percent, "")
	}, func() bool { return taskCanceled(p) }); err != nil {
		if errors.Is(err, errTaskCanceled) {
			p.Send("cancelled", p.Done, p.Total, "")
			return
		}
		buildIndexes(cfg.DataDir(), nil)
		p.Send("error", 0, 0, err.Error())
		return
	}
	if taskCanceled(p) {
		p.Send("cancelled", p.Done, p.Total, "")
		return
	}

	p.BeginPhase(3)
	buildIndexes(cfg.DataDir(), p)
	if taskCanceled(p) {
		p.Send("cancelled", p.Done, p.Total, "")
		return
	}
	p.Send("complete", 100, 100, "")
}

func bundleImportPhasePlan() []phaseSpec {
	specs := splitPhaseRanges(0, 95, "解包", "写入")
	return append(specs, phaseSpec{Name: "重建索引", Start: 95, End: 100})
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func extractBundleTar(tarPath, staging string, progress func(read, total int64), canceled func() bool) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}
	tarSize := stat.Size()
	cr := &countingReader{r: f}
	lastReport := time.Time{}
	report := func(force bool) {
		if progress == nil {
			return
		}
		now := time.Now()
		if !force && now.Sub(lastReport) < 100*time.Millisecond {
			return
		}
		lastReport = now
		progress(cr.n, tarSize)
	}

	tr := tar.NewReader(cr)
	seen := map[string]bool{}
	var total int64
	files := 0
	hasManifest := false
	hasData := false
	for {
		if canceled != nil && canceled() {
			return errTaskCanceled
		}
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取 tar 失败: %w", err)
		}
		name := path.Clean(h.Name)
		if name == "." || path.IsAbs(name) || strings.HasPrefix(name, "../") ||
			strings.Contains(h.Name, "\\") || strings.ContainsRune(h.Name, 0) {
			return fmt.Errorf("数据包路径不合法: %s", h.Name)
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		if h.Typeflag != tar.TypeReg {
			return fmt.Errorf("不支持的条目类型: %s", h.Name)
		}
		if !safeBundleFile(name) {
			return fmt.Errorf("不允许的数据文件: %s", name)
		}
		if seen[name] {
			return fmt.Errorf("重复条目: %s", name)
		}
		seen[name] = true
		if h.Size < 0 || h.Size > bundleImportMax {
			return fmt.Errorf("文件过大: %s", name)
		}
		total += h.Size
		if total > bundleImportMax {
			return fmt.Errorf("解包后体积超过 1 GiB")
		}
		files++
		if files > bundleMaxEntries {
			return fmt.Errorf("文件数量超过限制")
		}
		if name == "manifest.json" {
			if h.Size > bundleManifestMax {
				return fmt.Errorf("manifest 过大")
			}
			hasManifest = true
		} else if strings.HasPrefix(name, "api/") || strings.HasPrefix(name, "images/") {
			hasData = true
		}

		target := filepath.Join(staging, filepath.FromSlash(name))
		if !pathWithin(staging, target) {
			return fmt.Errorf("数据包路径不合法: %s", name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.CopyN(out, tr, h.Size); err != nil {
			out.Close()
			return fmt.Errorf("读取文件失败: %s", name)
		}
		if err := out.Close(); err != nil {
			return err
		}
		report(false)
	}
	report(true)
	if !hasManifest {
		return fmt.Errorf("缺少 manifest.json")
	}
	if !hasData {
		return fmt.Errorf("数据包没有可导入的数据")
	}
	data, err := os.ReadFile(filepath.Join(staging, "manifest.json"))
	if err != nil {
		return err
	}
	var m struct {
		Format string `json:"format"`
	}
	if json.Unmarshal(data, &m) != nil || m.Format != bundleFormat {
		return fmt.Errorf("manifest 格式不合法")
	}
	return nil
}

func safeBundleFile(name string) bool {
	if name == "manifest.json" {
		return true
	}
	parts := strings.Split(name, "/")
	if len(parts) == 5 && parts[0] == "api" {
		if parts[1] != "subjects" && parts[1] != "characters" && parts[1] != "persons" {
			return false
		}
		if !safeEntityDir(parts[2], parts[3]) {
			return false
		}
		return safeJSONName(parts[4])
	}
	if len(parts) == 4 && parts[0] == "images" {
		kind, size := splitImageKind(parts[1])
		if kind == "" || size == "" {
			return false
		}
		if !safeEntityDir(parts[2], strings.TrimSuffix(parts[3], ".jpg")) {
			return false
		}
		return strings.HasSuffix(parts[3], ".jpg")
	}
	return false
}

func safeEntityDir(dir, idText string) bool {
	dirID, err1 := strconv.Atoi(dir)
	id, err2 := strconv.Atoi(idText)
	if err1 != nil || err2 != nil || dirID < 0 || id <= 0 {
		return false
	}
	return dirID == id%10
}

func splitImageKind(s string) (string, string) {
	for _, kind := range []string{"subjects", "characters", "persons"} {
		prefix := kind + "_"
		if strings.HasPrefix(s, prefix) {
			size := strings.TrimPrefix(s, prefix)
			if size == "large" || size == "grid" || size == "small" {
				return kind, size
			}
		}
	}
	return "", ""
}

func safeJSONName(name string) bool {
	if !strings.HasSuffix(name, ".json") || len(name) <= len(".json") {
		return false
	}
	for _, c := range name {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') &&
			c != '.' && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func commitBundle(staging, dd string, progress func(done, total int), canceled func() bool) (int, error) {
	total := 0
	_ = filepath.WalkDir(staging, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(staging, path)
		if err != nil {
			return nil
		}
		if filepath.ToSlash(rel) != "manifest.json" {
			total++
		}
		return nil
	})

	count := 0
	err := filepath.WalkDir(staging, func(path string, d os.DirEntry, err error) error {
		if canceled != nil && canceled() {
			return errTaskCanceled
		}
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(staging, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		var target string
		switch {
		case rel == "manifest.json":
			return nil
		case strings.HasPrefix(rel, "api/"):
			target = filepath.Join(store.Dir(dd), filepath.FromSlash(strings.TrimPrefix(rel, "api/")))
		case strings.HasPrefix(rel, "images/"):
			target = filepath.Join(dd, filepath.FromSlash(rel))
		default:
			return fmt.Errorf("未知数据文件: %s", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := store.WriteAtomicFast(target, data, 0o644); err != nil {
			return err
		}
		count++
		if progress != nil {
			progress(count, total)
		}
		return nil
	})
	return count, err
}

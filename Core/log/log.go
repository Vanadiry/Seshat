// Package log 提供基于 slog 的结构化分级日志系统。
// 日志同时输出到 stdout 和文件，文件超过 10MB 自动轮转，最多保留 10 个。
package log

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vanadiry/seshat/Core/config"
)

var logger *slog.Logger

var levelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

const maxLogSize = 10 << 20

type rotatingWriter struct {
	mu      sync.Mutex
	dir     string
	file    *os.File
	written int64
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil && w.written >= maxLogSize {
		w.file.Close()
		w.file = nil
		w.written = 0
		ts := time.Now().Format("20060102_150405")
		path := filepath.Join(w.dir, "seshat_"+ts+".log")
		f, err := os.Create(path)
		if err == nil {
			w.file = f
		}
		go rotateLogs(w.dir, 10)
	}

	if w.file == nil {
		return len(p), nil
	}

	n, err := w.file.Write(p)
	w.written += int64(n)
	return n, err
}

func (w *rotatingWriter) openFirst() error {
	ts := time.Now().Format("20060102_150405")
	path := filepath.Join(w.dir, "seshat_"+ts+".log")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w.file = f
	w.written = 0
	return nil
}

// Init 初始化日志系统。level 为 "debug" / "info" / "warn" / "error"。
func Init(level string) {
	logDir := filepath.Join(config.Dir(), "logs")
	os.MkdirAll(logDir, 0o755)

	rw := &rotatingWriter{dir: logDir}
	if err := rw.openFirst(); err != nil {
		slog.Error("无法创建日志文件", "dir", logDir, "err", err)
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
		return
	}

	lvl, ok := levelMap[strings.ToLower(level)]
	if !ok {
		lvl = slog.LevelInfo
	}

	handler := slog.NewTextHandler(io.MultiWriter(os.Stdout, rw), &slog.HandlerOptions{Level: lvl})
	logger = slog.New(handler)

	rotateLogs(logDir, 10)
	logger.Info("Log file: "+rw.file.Name(), "level", lvl.String())
}

func rotateLogs(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var logs []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "seshat_") && strings.HasSuffix(e.Name(), ".log") {
			logs = append(logs, e.Name())
		}
	}
	if len(logs) <= keep {
		return
	}
	sort.Sort(sort.Reverse(sort.StringSlice(logs)))
	for _, name := range logs[keep:] {
		os.Remove(filepath.Join(dir, name))
	}
}

// Debug 输出调试日志。
func Debug(msg string, args ...any) {
	log(slog.LevelDebug, msg, args...)
}

// Info 输出信息日志。
func Info(msg string, args ...any) {
	log(slog.LevelInfo, msg, args...)
}

// Warn 输出警告日志。
func Warn(msg string, args ...any) {
	log(slog.LevelWarn, msg, args...)
}

// Error 输出错误日志。
func Error(msg string, args ...any) {
	log(slog.LevelError, msg, args...)
}

func log(level slog.Level, msg string, args ...any) {
	if logger != nil {
		logger.LogAttrs(nil, level, msg, toAttrs(args)...)
	} else {
		slog.New(slog.NewTextHandler(os.Stdout, nil)).LogAttrs(nil, level, msg, toAttrs(args)...)
	}
}

func toAttrs(args []any) []slog.Attr {
	if len(args)%2 != 0 {
		args = append(args, "!MISSING")
	}
	var attrs []slog.Attr
	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			key = "!BADKEY"
		}
		attrs = append(attrs, slog.Any(key, args[i+1]))
	}
	return attrs
}

// HTTP 记录 HTTP 请求日志。
func HTTP(method, path string, status int, duration time.Duration) {
	Debug("http request", "method", method, "path", path, "status", status, "duration", duration)
}

// Package log 提供基于 slog 的结构化分级日志系统。
// 每次启动创建一个新的日志文件，最多保留 10 个。
package log

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// Init 初始化日志系统。level 为 "debug" / "info" / "warn" / "error"。
func Init(level string) {
	logDir := filepath.Join(config.Dir(), "logs")
	os.MkdirAll(logDir, 0o755)

	ts := time.Now().Format("20060102_150405")
	logPath := filepath.Join(logDir, "seshat_"+ts+".log")

	f, err := os.Create(logPath)
	if err != nil {
		slog.Error("无法创建日志文件", "path", logPath, "err", err)
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
		return
	}

	lvl, ok := levelMap[strings.ToLower(level)]
	if !ok {
		lvl = slog.LevelInfo
	}

	handler := slog.NewTextHandler(io.MultiWriter(os.Stdout, f), &slog.HandlerOptions{Level: lvl})
	logger = slog.New(handler)

	rotateLogs(logDir, 10)
	logger.Info("Log file: "+logPath, "level", lvl.String())
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

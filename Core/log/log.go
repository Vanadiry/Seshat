// Package log 提供同时写入控制台和文件的日志系统。
// 每次启动创建一个新的日志文件，最多保留 10 个文件。
package log

import (
	"fmt"
	"io"
	stdlog "log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var logger *stdlog.Logger

// Init 初始化日志系统，同时输出到控制台和 SESHAT_HOME/Logs/seshat_YYYYMMDD_HHMMSS.log。
func Init(dataDir string) {
	logDir := filepath.Join(dataDir, "logs")
	os.MkdirAll(logDir, 0o755)

	ts := time.Now().Format("20060102_150405")
	logPath := filepath.Join(logDir, "seshat_"+ts+".log")

	f, err := os.Create(logPath)
	if err != nil {
		stdlog.Printf("无法创建日志文件 %s: %v", logPath, err)
		logger = stdlog.New(os.Stdout, "", stdlog.LstdFlags)
		return
	}

	writer := io.MultiWriter(os.Stdout, f)
	logger = stdlog.New(writer, "", stdlog.LstdFlags)

	rotateLogs(logDir, 10)

	logger.Printf("Log file: %s", logPath)
}

// rotateLogs 清理多余的日志文件，只保留最新的 keep 个。
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

// Info 输出信息日志。
func Info(format string, v ...any) {
	if logger != nil {
		logger.Printf("[INFO] "+format, v...)
	} else {
		stdlog.Printf("[INFO] "+format, v...)
	}
}

// Warn 输出警告日志。
func Warn(format string, v ...any) {
	if logger != nil {
		logger.Printf("[WARN] "+format, v...)
	} else {
		stdlog.Printf("[WARN] "+format, v...)
	}
}

// Error 输出错误日志。
func Error(format string, v ...any) {
	if logger != nil {
		logger.Printf("[ERROR] "+format, v...)
	} else {
		stdlog.Printf("[ERROR] "+format, v...)
	}
}

// HTTP 记录 HTTP 请求日志。
func HTTP(method, path string, status int, duration time.Duration) {
	if logger != nil {
		logger.Printf("[HTTP] %s %s %d %s", method, path, status, duration)
	} else {
		stdlog.Printf("[HTTP] %s %s %d %s", method, path, status, duration)
	}
}

var _ = fmt.Sprintf // keep fmt import

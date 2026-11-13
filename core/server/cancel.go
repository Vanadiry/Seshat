package server

import (
	"context"
	"errors"
)

// errTaskCanceled 表示任务被用户取消
var errTaskCanceled = errors.New("task canceled")

// taskCanceled 判断任务是否已被取消
func taskCanceled(p *Progress) bool {
	return p != nil && p.Cancelled()
}

// isCanceled 判断错误是否来自上下文取消
func isCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}

// CancelAllTasks 取消所有未结束的任务，返回取消数量
func CancelAllTasks() int {
	progressMu.Lock()
	defer progressMu.Unlock()
	n := 0
	for _, p := range progressMap {
		if !p.Closed() {
			p.Cancel()
			n++
		}
	}
	return n
}

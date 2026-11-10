package server

import "testing"

func TestTaskLockedIgnoresBufferedEvents(t *testing.T) {
	p := newProgress(1, "test", "测试")
	for len(p.Channel) < cap(p.Channel) {
		p.Channel <- "{}"
	}
	if !taskLocked() {
		t.Fatal("通道有缓冲事件时仍应视为有任务运行")
	}
	if len(activeTasks()) == 0 {
		t.Fatal("运行中的任务应当出现在任务列表")
	}

	p.Close()
	if taskLocked() {
		t.Fatal("任务关闭后不应再锁定")
	}
	if len(activeTasks()) != 0 {
		t.Fatal("任务关闭后不应出现在任务列表")
	}

	progressMu.Lock()
	delete(progressMap, p.ID)
	progressMu.Unlock()
}

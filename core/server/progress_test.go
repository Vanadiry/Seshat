package server

import (
	"encoding/json"
	"testing"
)

func TestTaskLockedIgnoresBufferedEvents(t *testing.T) {
	p := newProgress(10, "test", "测试")
	for len(p.Channel) < cap(p.Channel) {
		p.Channel <- "{}"
	}
	if !taskLocked() {
		t.Fatal("通道有缓冲事件时仍应视为有任务运行")
	}
	p.Send("test", 3, 10, "")
	tasks := activeTasks()
	if len(tasks) != 1 {
		t.Fatal("运行中的任务应当出现在任务列表")
	}
	if tasks[0]["done"] != 3 || tasks[0]["total"] != 10 || tasks[0]["label"] != "测试" {
		t.Fatalf("任务列表进度不对: %v", tasks[0])
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

func TestProgressSnapshot(t *testing.T) {
	p := newProgress(10, "fetch", "拉取数据")
	p.SetPhase(2, 3, "下载图片")
	p.Send("images", 4, 10, "")

	var snap map[string]any
	if err := json.Unmarshal([]byte(p.Snapshot()), &snap); err != nil {
		t.Fatal(err)
	}
	if snap["step"] != "snapshot" || snap["label"] != "拉取数据" {
		t.Fatalf("快照内容不对: %v", snap)
	}
	if snap["done"] != float64(4) || snap["total"] != float64(10) {
		t.Fatalf("快照进度不对: %v", snap)
	}
	if snap["phase"] != float64(2) || snap["phases"] != float64(3) || snap["phase_name"] != "下载图片" {
		t.Fatalf("快照阶段不对: %v", snap)
	}

	p.Close()
	progressMu.Lock()
	delete(progressMap, p.ID)
	progressMu.Unlock()
}

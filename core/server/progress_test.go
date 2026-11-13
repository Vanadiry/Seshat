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
		t.Fatalf("任务列表进度应为 3/10，实际为 %v", tasks[0])
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

func TestSplitPhaseRanges(t *testing.T) {
	specs := splitPhaseRanges(0, 95, "a", "b", "c", "d")
	if len(specs) != 4 || specs[0].Start != 0 || specs[3].End != 95 {
		t.Fatalf("阶段区间应为 4 段且覆盖 0 到 95，实际为 %v", specs)
	}
	for i := 1; i < len(specs); i++ {
		if specs[i].Start != specs[i-1].End {
			t.Fatalf("阶段区间不连续: %v", specs)
		}
	}
}

func TestFetchPhasePlan(t *testing.T) {
	withImages := fetchPhasePlan(true)
	if len(withImages) != 5 {
		t.Fatalf("阶段数应为 5，实际为 %d", len(withImages))
	}
	if withImages[4].Name != "重建索引" || withImages[4].Start != 95 || withImages[4].End != 100 {
		t.Fatalf("索引阶段应占 5%%: %v", withImages[4])
	}
	for i := 0; i < 4; i++ {
		span := withImages[i].End - withImages[i].Start
		if span != 23 && span != 24 {
			t.Fatalf("其余阶段应平均分配: %v", withImages)
		}
	}
	withoutImages := fetchPhasePlan(false)
	if len(withoutImages) != 2 || withoutImages[0].Start != 0 || withoutImages[0].End != 95 {
		t.Fatalf("无图片阶段应为 0 到 95 再到 100，实际为 %v", withoutImages)
	}
}

func drainProgress(t *testing.T, p *Progress) map[string]any {
	t.Helper()
	var last map[string]any
	for len(p.Channel) > 0 {
		var ev map[string]any
		if err := json.Unmarshal([]byte(<-p.Channel), &ev); err != nil {
			t.Fatal(err)
		}
		if ev["step"] == "progress" {
			last = ev
		}
	}
	return last
}

func TestPhasePlanProgress(t *testing.T) {
	p := newProgress(100, "test", "测试")
	p.SetPlan([]phaseSpec{
		{Name: "拉取条目", Start: 0, End: 24},
		{Name: "下载图片", Start: 24, End: 48},
		{Name: "重建索引", Start: 95, End: 100},
	})
	p.BeginPhase(1)
	p.SendPhaseProgress(1, 2, "")
	withSpeed := drainProgress(t, p)
	if withSpeed["percent"] != float64(12) || withSpeed["done"] != float64(1) || withSpeed["total"] != float64(2) {
		t.Fatalf("阶段计数应为 1/2，实际为 %v", withSpeed)
	}
	if withSpeed["speed"] == nil {
		t.Fatalf("实体进度应包含 speed，实际为 %v", withSpeed)
	}

	p.SendPhaseStep(2, 2, "")
	last := drainProgress(t, p)
	if last["percent"] != float64(24) {
		t.Fatalf("阶段结束时整体百分比应为 24，实际为 %v", last)
	}
	if last["done"] != float64(2) || last["total"] != float64(2) {
		t.Fatalf("阶段计数应为 2/2，实际为 %v", last)
	}
	if last["speed"] != nil {
		t.Fatalf("步骤进度不应包含 speed，实际为 %v", last)
	}
	if last["phase"] != float64(1) || last["phase_name"] != "拉取条目" {
		t.Fatalf("阶段信息应为 1 拉取条目，实际为 %v", last)
	}

	p.Close()
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
		t.Fatalf("快照步骤应为 snapshot，label 应为 拉取数据，实际为 %v", snap)
	}
	if snap["done"] != float64(4) || snap["total"] != float64(10) {
		t.Fatalf("快照进度应为 4/10，实际为 %v", snap)
	}
	if snap["phase"] != float64(2) || snap["phases"] != float64(3) || snap["phase_name"] != "下载图片" {
		t.Fatalf("快照阶段应为 2/3 下载图片，实际为 %v", snap)
	}

	p.Close()
	progressMu.Lock()
	delete(progressMap, p.ID)
	progressMu.Unlock()
}

func TestProgressSnapshotStepCounts(t *testing.T) {
	p := newProgress(100, "fetch", "拉取数据")
	defer func() {
		p.Close()
		progressMu.Lock()
		delete(progressMap, p.ID)
		progressMu.Unlock()
	}()
	p.SetPlan([]phaseSpec{{Name: "拉取条目", Start: 0, End: 24}})
	p.BeginPhase(1)
	p.SendPhaseProgress(4, 652, "")

	var snap map[string]any
	if err := json.Unmarshal([]byte(p.Snapshot()), &snap); err != nil {
		t.Fatal(err)
	}
	if snap["percent"] != float64(0) || snap["done"] != float64(4) || snap["total"] != float64(652) {
		t.Fatalf("快照应带整体百分比和阶段计数，实际为 %v", snap)
	}

	tasks := activeTasks()
	if len(tasks) != 1 {
		t.Fatalf("任务数应为 1，实际为 %d", len(tasks))
	}
	if tasks[0]["percent"] != 0 || tasks[0]["done"] != 4 || tasks[0]["total"] != 652 {
		t.Fatalf("任务列表应带整体百分比和阶段计数，实际为 %v", tasks[0])
	}
}

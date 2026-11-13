package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vanadiry/seshat/core/store"
)

func TestCancelAllTasks(t *testing.T) {
	p1 := newProgress(10, "test", "任务一")
	p2 := newProgress(10, "test", "任务二")
	defer func() {
		p1.Close()
		p2.Close()
		progressMu.Lock()
		delete(progressMap, p1.ID)
		delete(progressMap, p2.ID)
		progressMu.Unlock()
	}()

	if n := CancelAllTasks(); n < 2 {
		t.Fatalf("取消任务数 = %d", n)
	}
	for _, p := range []*Progress{p1, p2} {
		if !p.Cancelled() {
			t.Fatal("任务应被标记为已取消")
		}
		select {
		case <-p.Context().Done():
		default:
			t.Fatal("任务上下文应已取消")
		}
	}
}

func TestProgressPhasesAndSnapshot(t *testing.T) {
	p := newProgress(0, "test", "测试")
	defer func() {
		p.Close()
		progressMu.Lock()
		delete(progressMap, p.ID)
		progressMu.Unlock()
	}()

	p.SetPlan(splitPhaseRanges(0, 100, "拉取", "写入"))
	p.BeginPhase(1)
	p.SendPhaseProgress(5, 10, "进行中")

	var snap map[string]any
	if err := json.Unmarshal([]byte(p.Snapshot()), &snap); err != nil {
		t.Fatal(err)
	}
	if snap["phase"] != float64(1) || snap["phases"] != float64(2) {
		t.Fatalf("阶段 = %v", snap)
	}
	if snap["percent"] != float64(25) {
		t.Fatalf("百分比 = %v", snap["percent"])
	}

	p.SendPercent(120, "")
	if err := json.Unmarshal([]byte(p.Snapshot()), &snap); err != nil {
		t.Fatal(err)
	}
	if snap["percent"] != float64(100) {
		t.Fatalf("超限百分比应夹到 100，实际 %v", snap["percent"])
	}
}

func TestImageSizeOrder(t *testing.T) {
	cases := map[string]string{
		"large":   "large",
		"small":   "small",
		"grid":    "grid",
		"medium":  "large",
		"unknown": "grid",
	}
	for requested, want := range cases {
		order := imageSizeOrder(requested)
		if len(order) != 3 || order[0] != want {
			t.Errorf("imageSizeOrder(%q) = %v，首项期望 %q", requested, order, want)
		}
	}

	full := store.ImageEntry{Large: "l.jpg", Grid: "g.jpg", Small: "s.jpg"}
	if got := pickImageSize(full, "small"); got != "small" {
		t.Fatalf("pickImageSize = %q", got)
	}
	onlyGrid := store.ImageEntry{Grid: "g.jpg"}
	if got := pickImageSize(onlyGrid, "large"); got != "grid" {
		t.Fatalf("回退尺寸 = %q", got)
	}
	if got := pickImageSize(store.ImageEntry{}, "large"); got != "" {
		t.Fatalf("无图片时应为空，实际 %q", got)
	}
}

func TestHandleOpenURLRejectsBadURL(t *testing.T) {
	cases := []string{
		`{}`,
		`{"url":"ftp://example.com"}`,
		`{"url":"javascript:alert(1)"}`,
		`not-json`,
	}
	for _, body := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v0/open-url", strings.NewReader(body))
		handleOpenURL(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body=%q 状态码 = %d", body, w.Code)
		}
	}
}

func TestKFactor(t *testing.T) {
	if got := kFactor(0); got != 32 {
		t.Fatalf("kFactor(0) = %v", got)
	}
	if got := kFactor(8); got != 16 {
		t.Fatalf("kFactor(8) = %v", got)
	}
	if got := kFactor(1 << 20); got != 6 {
		t.Fatalf("kFactor 下限 = %v", got)
	}
}

func TestUpdateELO(t *testing.T) {
	t.Setenv("SESHAT_HOME", t.TempDir())
	updateELO(t.TempDir(), 1, 2)

	data := loadEloData()
	if data.Subjects[1].Rating <= eloDefault {
		t.Fatalf("胜者评分应上升: %v", data.Subjects[1].Rating)
	}
	if data.Subjects[2].Rating >= eloDefault {
		t.Fatalf("败者评分应下降: %v", data.Subjects[2].Rating)
	}
	if data.Subjects[1].Count != 1 || data.GlobalCompareCount != 1 {
		t.Fatalf("计数 = %+v", data)
	}

	history := loadELOHistory()
	if len(history) != 1 || history[0].Winner != 1 || history[0].Loser != 2 {
		t.Fatalf("历史 = %+v", history)
	}
}

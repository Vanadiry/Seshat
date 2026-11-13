package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
)

type phaseSpec struct {
	Name  string
	Start int
	End   int
}

type Progress struct {
	ID           string
	Channel      chan string
	Total        int
	Done         int
	Task         string `json:"task"`   // e.g. "fetch", "rebuild"
	Label        string `json:"label"`  // e.g. "拉取动画 #51", "刷新全部"
	Phase        int    `json:"phase"`  // current phase number (1-based)
	Phases       int    `json:"phases"` // total number of phases
	PhaseName    string `json:"name"`   // human-readable phase name, e.g. "下载角色图像"
	Percent      int    `json:"percent"`
	started      time.Time
	closed       bool
	usePercent   bool
	percentStart int
	percentEnd   int
	stepDone     int
	stepTotal    int
	hasStep      bool
	plan         []phaseSpec
	ctx          context.Context
	cancel       context.CancelFunc
	cancelled    bool
	mu           sync.Mutex
}

var (
	progressMap = map[string]*Progress{}
	progressMu  sync.Mutex
)

func taskLocked() bool {
	progressMu.Lock()
	defer progressMu.Unlock()
	for _, p := range progressMap {
		if !p.Closed() {
			return true
		}
	}
	return false
}

func newProgress(total int, task, label string) *Progress {
	b := make([]byte, 4)
	rand.Read(b)
	ctx, cancel := context.WithCancel(context.Background())
	p := &Progress{
		ID:      hex.EncodeToString(b),
		Channel: make(chan string, 64),
		Total:   total,
		Task:    task,
		Label:   label,
		started: time.Now(),
		ctx:     ctx,
		cancel:  cancel,
	}
	progressMu.Lock()
	progressMap[p.ID] = p
	progressMu.Unlock()
	startMsg, err := json.Marshal(map[string]any{"step": "start", "done": 0, "total": total, "task": task, "label": label, "phase": p.Phase, "phases": p.Phases, "phase_name": p.PhaseName})
	if err != nil {
		startMsg = []byte(`{"step":"start","error":"marshal failed"}`)
	}
	p.Channel <- string(startMsg)
	return p
}

// Context 返回任务上下文，供请求和循环响应取消
func (p *Progress) Context() context.Context {
	return p.ctx
}

// Cancel 取消任务，幂等
func (p *Progress) Cancel() {
	p.mu.Lock()
	if p.cancelled {
		p.mu.Unlock()
		return
	}
	p.cancelled = true
	p.mu.Unlock()
	p.cancel()
}

// Cancelled 返回任务是否已被取消
func (p *Progress) Cancelled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cancelled
}

// Finish 发送结束事件并关闭任务
func (p *Progress) Finish() {
	if p.Cancelled() {
		p.Send("cancelled", p.Done, p.Total, "")
	} else {
		p.Send("complete", 100, 100, "")
	}
	p.Close()
}

func (p *Progress) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()
	p.cancel()
	select {
	case p.Channel <- `{"step":"done","status":"closed"}`:
	default:
	}
	close(p.Channel)
	time.AfterFunc(30*time.Second, func() {
		progressMu.Lock()
		delete(progressMap, p.ID)
		progressMu.Unlock()
	})
}

// Closed 返回任务是否已经结束
func (p *Progress) Closed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

func (p *Progress) SetError(msg string) {
	events.Bus.Error(msg)
}

func (p *Progress) SetPhase(phase int, phases int, name string) {
	p.mu.Lock()
	p.Phase = phase
	p.Phases = phases
	p.PhaseName = name
	p.mu.Unlock()
}

// splitPhaseRanges 把整体百分比平均分配给各阶段
func splitPhaseRanges(start, end int, names ...string) []phaseSpec {
	if len(names) == 0 {
		return nil
	}
	span := end - start
	specs := make([]phaseSpec, len(names))
	for i, name := range names {
		specs[i] = phaseSpec{
			Name:  name,
			Start: start + span*i/len(names),
			End:   start + span*(i+1)/len(names),
		}
	}
	return specs
}

// SetPlan 设置阶段划分，每个阶段对应整体百分比的区间
func (p *Progress) SetPlan(specs []phaseSpec) {
	p.mu.Lock()
	p.plan = specs
	p.Phases = len(specs)
	p.mu.Unlock()
}

// BeginPhase 进入指定阶段并上报阶段起点
func (p *Progress) BeginPhase(index int) {
	p.mu.Lock()
	if index < 1 || index > len(p.plan) {
		p.mu.Unlock()
		return
	}
	spec := p.plan[index-1]
	p.Phase = index
	p.Phases = len(p.plan)
	p.PhaseName = spec.Name
	p.percentStart = spec.Start
	p.percentEnd = spec.End
	p.mu.Unlock()
	p.SendPercent(spec.Start, "")
}

// SendPhaseProgress 按当前阶段内的完成比例上报整体进度和速度
func (p *Progress) SendPhaseProgress(done, total int, status string) {
	p.sendPhaseProgress(done, total, status, true)
}

// SendPhaseStep 按当前阶段内的完成比例上报整体进度，不带速度
func (p *Progress) SendPhaseStep(done, total int, status string) {
	p.sendPhaseProgress(done, total, status, false)
}

func (p *Progress) sendPhaseProgress(done, total int, status string, withSpeed bool) {
	p.mu.Lock()
	start, end := p.percentStart, p.percentEnd
	p.mu.Unlock()
	if done < 0 {
		done = 0
	}
	if total > 0 && done > total {
		done = total
	}
	percent := start
	if total > 0 {
		percent += (end - start) * done / total
	}
	p.sendProgress(percent, done, total, status, true, withSpeed)
}

// SendPhaseStatus 在当前阶段上报一条状态
func (p *Progress) SendPhaseStatus(status string) {
	p.mu.Lock()
	percent := p.Percent
	p.mu.Unlock()
	p.SendPercent(percent, status)
}

// HasPhaseRange 返回当前阶段是否设置了百分比区间
func (p *Progress) HasPhaseRange() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.percentEnd > p.percentStart
}

func (p *Progress) Send(step string, done, total int, status string) {
	p.mu.Lock()
	p.Done = done
	if step == "complete" && p.usePercent {
		p.Percent = done
	}
	phase := p.Phase
	phases := p.Phases
	phaseName := p.PhaseName
	p.mu.Unlock()
	elapsed := time.Since(p.started).Seconds()
	speed := float64(0)
	if elapsed > 0 {
		speed = float64(done) / elapsed
	}
	data, e := json.Marshal(map[string]any{
		"step":       step,
		"done":       done,
		"total":      total,
		"status":     status,
		"speed":      fmt.Sprintf("%.1f/s", speed),
		"phase":      phase,
		"phases":     phases,
		"phase_name": phaseName,
	})
	if e != nil {
		log.Error("progress send marshal", "err", e)
		events.Bus.Error("进度数据序列化失败")
		return
	}
	select {
	case p.Channel <- string(data):
	default:
	}
}

// SendPercent 上报整体百分比进度
func (p *Progress) SendPercent(percent int, status string) {
	p.sendProgress(percent, 0, 0, status, false, false)
}

func (p *Progress) sendProgress(percent int, done, total int, status string, hasStep bool, withSpeed bool) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	p.mu.Lock()
	p.Done = percent
	p.Total = 100
	p.Percent = percent
	p.usePercent = true
	p.hasStep = hasStep
	if hasStep {
		p.stepDone = done
		p.stepTotal = total
	}
	phase := p.Phase
	phases := p.Phases
	phaseName := p.PhaseName
	p.mu.Unlock()
	payload := map[string]any{
		"step":       "progress",
		"percent":    percent,
		"status":     status,
		"phase":      phase,
		"phases":     phases,
		"phase_name": phaseName,
	}
	if hasStep {
		payload["done"] = done
		payload["total"] = total
		if withSpeed {
			elapsed := time.Since(p.started).Seconds()
			speed := float64(0)
			if elapsed > 0 {
				speed = float64(done) / elapsed
			}
			payload["speed"] = fmt.Sprintf("%.1f/s", speed)
		}
	}
	data, e := json.Marshal(payload)
	if e != nil {
		log.Error("progress send marshal", "err", e)
		events.Bus.Error("进度数据序列化失败")
		return
	}
	select {
	case p.Channel <- string(data):
	default:
	}
}

// Snapshot 返回当前进度的 SSE 数据
func (p *Progress) Snapshot() string {
	p.mu.Lock()
	payload := map[string]any{
		"step":       "snapshot",
		"id":         p.ID,
		"done":       p.Done,
		"total":      p.Total,
		"task":       p.Task,
		"label":      p.Label,
		"phase":      p.Phase,
		"phases":     p.Phases,
		"phase_name": p.PhaseName,
	}
	if p.usePercent {
		payload["percent"] = p.Percent
	}
	if p.hasStep {
		payload["done"] = p.stepDone
		payload["total"] = p.stepTotal
	}
	p.mu.Unlock()
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func getProgress(id string) *Progress {
	progressMu.Lock()
	defer progressMu.Unlock()
	return progressMap[id]
}

func activeTasks() []map[string]any {
	progressMu.Lock()
	defer progressMu.Unlock()
	var tasks []map[string]any
	for _, p := range progressMap {
		if p.Closed() {
			continue
		}
		p.mu.Lock()
		done, total := p.Done, p.Total
		phase, phases, phaseName := p.Phase, p.Phases, p.PhaseName
		p.mu.Unlock()
		tasks = append(tasks, map[string]any{
			"id":         p.ID,
			"task":       p.Task,
			"label":      p.Label,
			"done":       done,
			"total":      total,
			"phase":      phase,
			"phases":     phases,
			"phase_name": phaseName,
		})
	}
	return tasks
}

func handleActiveTasks(w http.ResponseWriter, r *http.Request) {
	writeList(w, activeTasks())
}

func handleProgress(w http.ResponseWriter, r *http.Request) {
	p := getProgress(r.PathValue("id"))
	if p == nil {
		writeError(w, r, http.StatusNotFound, "task not found")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}
	if snapshot := p.Snapshot(); snapshot != "" {
		fmt.Fprintf(w, "data: %s\n\n", snapshot)
		flusher.Flush()
	}
	for event := range p.Channel {
		fmt.Fprintf(w, "data: %s\n\n", event)
		flusher.Flush()
	}
}

package server

import (
	"net/http"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Progress struct {
	ID        string
	Channel   chan string
	Total     int
	Done      int
	Task      string `json:"task"`   // e.g. "fetch", "rebuild"
	Label     string `json:"label"`  // e.g. "拉取动画 #51", "刷新全部"
	Phase     int    `json:"phase"`  // current phase number (1-based)
	Phases    int    `json:"phases"` // total number of phases
	PhaseName string `json:"name"`   // human-readable phase name, e.g. "下载角色图像"
	started   time.Time
	mu        sync.Mutex
}

var (
	progressMap = map[string]*Progress{}
	progressMu  sync.Mutex
)

// taskLocked returns true if any task is currently active (not yet completed).
func taskLocked() bool {
	progressMu.Lock()
	defer progressMu.Unlock()
	for _, p := range progressMap {
		select {
		case <-p.Channel:
		default:
			return true // channel still open = task still running
		}
	}
	return false
}

func newProgress(total int, task, label string) *Progress {
	b := make([]byte, 4)
	rand.Read(b)
	p := &Progress{
		ID:      hex.EncodeToString(b),
		Channel: make(chan string, 64),
		Total:   total,
		Task:    task,
		Label:   label,
		started: time.Now(),
	}
	progressMu.Lock()
	progressMap[p.ID] = p
	progressMu.Unlock()
	startMsg, _ := json.Marshal(map[string]any{"step":"start","done":0,"total":total,"task":task,"label":label,"phase":p.Phase,"phases":p.Phases,"phase_name":p.PhaseName})
	p.Channel <- string(startMsg)
	return p
}

func (p *Progress) Close() {
	// Send final event before closing
	select {
	case p.Channel <- `{"step":"done","status":"closed"}`:
	default:
	}
	close(p.Channel)
	// Keep in map briefly so late SSE subscribers get the final event
	time.AfterFunc(30*time.Second, func() {
		progressMu.Lock()
		delete(progressMap, p.ID)
		progressMu.Unlock()
	})
}

func (p *Progress) SetPhase(phase int, phases int, name string) {
	p.mu.Lock()
	p.Phase = phase
	p.Phases = phases
	p.PhaseName = name
	p.mu.Unlock()
}

func (p *Progress) Send(step string, done, total int, status string) {
	p.mu.Lock()
	p.Done = done
	phase := p.Phase
	phases := p.Phases
	phaseName := p.PhaseName
	p.mu.Unlock()
	elapsed := time.Since(p.started).Seconds()
	speed := float64(0)
	if elapsed > 0 {
		speed = float64(done) / elapsed
	}
	data, _ := json.Marshal(map[string]any{
		"step":       step,
		"done":       done,
		"total":      total,
		"status":     status,
		"speed":      fmt.Sprintf("%.1f/s", speed),
		"phase":      phase,
		"phases":     phases,
		"phase_name": phaseName,
	})
	select {
	case p.Channel <- string(data):
	default: // channel full, drop event to avoid blocking fetch
	}
}

func getProgress(id string) *Progress {
	progressMu.Lock()
	defer progressMu.Unlock()
	return progressMap[id]
}

func activeTasks() []map[string]string {
	progressMu.Lock()
	defer progressMu.Unlock()
	var tasks []map[string]string
	for _, p := range progressMap {
		select {
		case <-p.Channel:
			continue // closed channel = done, skip
		default:
		}
		tasks = append(tasks, map[string]string{"id": p.ID, "task": p.Task, "label": p.Label})
	}
	return tasks
}

func handleActiveTasks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"tasks": activeTasks()})
}

func handleProgress(w http.ResponseWriter, r *http.Request) {
	p := getProgress(r.PathValue("id"))
	if p == nil { writeJSON(w, map[string]string{"error": "task not found"}); return }
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok { return }
	for event := range p.Channel {
		fmt.Fprintf(w, "data: %s\n\n", event)
		flusher.Flush()
	}
}

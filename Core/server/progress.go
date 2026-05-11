package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Progress struct {
	ID       string
	Channel  chan string
	Total    int
	Done     int
	started  time.Time
	mu       sync.Mutex
}

var (
	progressMap = map[string]*Progress{}
	progressMu  sync.Mutex
)

func newProgress(total int) *Progress {
	b := make([]byte, 4)
	rand.Read(b)
	p := &Progress{
		ID:      hex.EncodeToString(b),
		Channel: make(chan string, 64),
		Total:   total,
		started: time.Now(),
	}
	progressMu.Lock()
	progressMap[p.ID] = p
	progressMu.Unlock()
	// Send initial event immediately so SSE connects without waiting
	startMsg, _ := json.Marshal(map[string]any{"step":"start","done":0,"total":total,"speed":"0.0/s","status":"connecting"})
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

func (p *Progress) Send(step string, done, total int, status string) {
	p.mu.Lock()
	p.Done = done
	p.mu.Unlock()
	elapsed := time.Since(p.started).Seconds()
	speed := float64(0)
	if elapsed > 0 {
		speed = float64(done) / elapsed
	}
	data, _ := json.Marshal(map[string]any{
		"step":  step,
		"done":  done,
		"total": total,
		"status": status,
		"speed": fmt.Sprintf("%.1f/s", speed),
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

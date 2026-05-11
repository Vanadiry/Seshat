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
	return p
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
	p.Channel <- string(data)
}

func (p *Progress) Close() {
	close(p.Channel)
	progressMu.Lock()
	delete(progressMap, p.ID)
	progressMu.Unlock()
}

func getProgress(id string) *Progress {
	progressMu.Lock()
	defer progressMu.Unlock()
	return progressMap[id]
}

// Package task 提供异步任务管理和 SSE（Server-Sent Events）进度推送。
package task

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type Status string

const (
	StatusRunning  Status = "running"
	StatusComplete Status = "complete"
	StatusFailed   Status = "failed"
)

type Task struct {
	ID        string    `json:"id"`
	Status    Status    `json:"status"`
	Progress  string    `json:"progress"`
	SubjectID int       `json:"subject_id"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	events    chan string
	closed    bool
	mu        sync.Mutex
}

func (t *Task) Send(event string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	select {
	case t.events <- event:
	default:
	}
}

func (t *Task) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.closed = true
		close(t.events)
	}
}

type Manager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

func NewManager() *Manager { return &Manager{tasks: make(map[string]*Task)} }

func (m *Manager) Create(subjectID int) *Task {
	b := make([]byte, 4)
	rand.Read(b)
	t := &Task{
		ID:        hex.EncodeToString(b),
		Status:    StatusRunning,
		SubjectID: subjectID,
		CreatedAt: time.Now(),
		events:    make(chan string, 64),
	}
	m.mu.Lock()
	m.tasks[t.ID] = t
	m.mu.Unlock()
	return t
}

func (m *Manager) Get(id string) *Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[id]
}

func (m *Manager) Events(id string) (<-chan string, error) {
	t := m.Get(id)
	if t == nil {
		return nil, fmt.Errorf("任务 %s 不存在", id)
	}
	return t.events, nil
}

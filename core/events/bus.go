package events

import (
	"sync"
	"time"
)

type EventType string

const (
	Error EventType = "error"
	Warn  EventType = "warn"
	Info  EventType = "info"
)

type Event struct {
	Type    EventType `json:"type"`
	Message string    `json:"message"`
	Time    int64     `json:"time"`
}

type EventBus struct {
	mu      sync.Mutex
	clients map[chan Event]struct{}
	history []Event
}

var Bus *EventBus

// historyLimit 回放给新订阅者的历史事件条数上限
const historyLimit = 50

func InitBus() {
	Bus = &EventBus{
		clients: make(map[chan Event]struct{}),
	}
}

func (b *EventBus) Subscribe() chan Event {
	ch := make(chan Event, 64)
	b.mu.Lock()
	defer b.mu.Unlock()
	// 回放订阅前的事件（如启动期的配置警告）
	for _, e := range b.history {
		select {
		case ch <- e:
		default:
		}
	}
	b.clients[ch] = struct{}{}
	return ch
}

func (b *EventBus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.clients, ch)
	close(ch)
	b.mu.Unlock()
}

func (b *EventBus) Publish(typ EventType, msg string) {
	e := Event{Type: typ, Message: msg, Time: time.Now().Unix()}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.history = append(b.history, e)
	if len(b.history) > historyLimit {
		b.history = b.history[len(b.history)-historyLimit:]
	}
	for ch := range b.clients {
		select {
		case ch <- e:
		default:
		}
	}
}

func (b *EventBus) Error(msg string) {
	if b == nil {
		return
	}
	b.Publish(Error, msg)
}
func (b *EventBus) Warn(msg string) {
	if b == nil {
		return
	}
	b.Publish(Warn, msg)
}
func (b *EventBus) Info(msg string) {
	if b == nil {
		return
	}
	b.Publish(Info, msg)
}

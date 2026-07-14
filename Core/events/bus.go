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
	mu      sync.RWMutex
	clients map[chan Event]struct{}
}

var Bus *EventBus

func InitBus() {
	Bus = &EventBus{
		clients: make(map[chan Event]struct{}),
	}
}

func (b *EventBus) Subscribe() chan Event {
	ch := make(chan Event, 32)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *EventBus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *EventBus) Publish(typ EventType, msg string) {
	e := Event{Type: typ, Message: msg, Time: time.Now().Unix()}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- e:
		default:
		}
	}
}

func (b *EventBus) Error(msg string) { b.Publish(Error, msg) }
func (b *EventBus) Warn(msg string)  { b.Publish(Warn, msg) }
func (b *EventBus) Info(msg string)  { b.Publish(Info, msg) }

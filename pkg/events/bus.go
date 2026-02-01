package events

import (
	"sync"
	"time"
)

type EventType string

const (
	EventAgentStarted EventType = "agent.started"
	EventAgentStopped EventType = "agent.stopped"
	EventAgentPaused  EventType = "agent.paused"
	EventAgentResumed EventType = "agent.resumed"
	EventOutputChunk  EventType = "output.chunk"
	EventError        EventType = "error"
	EventSessionSaved EventType = "session.saved"
)

type Event struct {
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	AgentName string    `json:"agent_name,omitempty"`
	Data      any       `json:"data,omitempty"`
}

type Bus interface {
	Subscribe(eventType EventType) <-chan Event
	Unsubscribe(eventType EventType, ch <-chan Event)
	Publish(event Event)
}

type EventBus struct {
	subscribers map[EventType][]chan Event
	mu          sync.RWMutex
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]chan Event),
	}
}

func (b *EventBus) Subscribe(eventType EventType) <-chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Event, 100)
	b.subscribers[eventType] = append(b.subscribers[eventType], ch)
	return ch
}

func (b *EventBus) Unsubscribe(eventType EventType, ch <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, c := range b.subscribers[eventType] {
		if c == ch {
			b.subscribers[eventType] = append(b.subscribers[eventType][:i], b.subscribers[eventType][i+1:]...)
			break
		}
	}
}

func (b *EventBus) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	event.Timestamp = time.Now()
	for _, ch := range b.subscribers[event.Type] {
		select {
		case ch <- event:
		default:
		}
	}
}
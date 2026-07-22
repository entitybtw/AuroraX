package console

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type Subscriber struct {
	ID     string
	Events chan Event
}

type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[string]*Subscriber
	recent      []Event
	bufferSize  int
	nextID      atomic.Int64
}

func NewBroadcaster(bufferSize int) *Broadcaster {
	if bufferSize <= 0 {
		bufferSize = 200
	}
	return &Broadcaster{
		subscribers: make(map[string]*Subscriber),
		bufferSize:  bufferSize,
		recent:      make([]Event, 0, bufferSize),
	}
}

func (b *Broadcaster) Subscribe(bufferSize int) *Subscriber {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	sub := &Subscriber{
		ID:     fmt.Sprintf("console_sub_%d", b.nextID.Add(1)),
		Events: make(chan Event, bufferSize),
	}
	b.mu.Lock()
	b.subscribers[sub.ID] = sub
	b.mu.Unlock()
	return sub
}

func (b *Broadcaster) Unsubscribe(id string) {
	b.mu.Lock()
	if sub, ok := b.subscribers[id]; ok {
		close(sub.Events)
		delete(b.subscribers, id)
	}
	b.mu.Unlock()
}

func (b *Broadcaster) Publish(event Event) {
	b.mu.Lock()
	if len(b.recent) >= b.bufferSize {
		copy(b.recent, b.recent[1:])
		b.recent[len(b.recent)-1] = event
	} else {
		b.recent = append(b.recent, event)
	}
	subscribers := make([]*Subscriber, 0, len(b.subscribers))
	for _, sub := range b.subscribers {
		subscribers = append(subscribers, sub)
	}
	b.mu.Unlock()

	for _, sub := range subscribers {
		select {
		case sub.Events <- event:
		default:
		}
	}
}

func (b *Broadcaster) Recent(limit, offset int) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	total := len(b.recent)
	if offset >= total {
		return []Event{}
	}
	available := total - offset
	if limit <= 0 || limit > available {
		limit = available
	}
	start := total - offset - limit
	out := make([]Event, limit)
	copy(out, b.recent[start:start+limit])
	return out
}

func (b *Broadcaster) LenRecent() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.recent)
}

func (b *Broadcaster) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

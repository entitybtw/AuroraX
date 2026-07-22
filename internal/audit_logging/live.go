package auditlog

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type LogSubscriber struct {
	ID      string
	Entries chan *LogEntry
}

type LiveBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[string]*LogSubscriber
	nextID      atomic.Int64
}

func NewLiveBroadcaster() *LiveBroadcaster {
	return &LiveBroadcaster{
		subscribers: make(map[string]*LogSubscriber),
	}
}

func (b *LiveBroadcaster) Subscribe(bufferSize int) *LogSubscriber {
	id := fmt.Sprintf("sub_%d", b.nextID.Add(1))
	sub := &LogSubscriber{
		ID:      id,
		Entries: make(chan *LogEntry, bufferSize),
	}
	b.mu.Lock()
	b.subscribers[id] = sub
	b.mu.Unlock()
	return sub
}

func (b *LiveBroadcaster) Unsubscribe(id string) {
	b.mu.Lock()
	if sub, ok := b.subscribers[id]; ok {
		close(sub.Entries)
		delete(b.subscribers, id)
	}
	b.mu.Unlock()
}

func (b *LiveBroadcaster) Publish(entry *LogEntry) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, sub := range b.subscribers {
		select {
		case sub.Entries <- entry:
		default:
		}
	}
}

func (b *LiveBroadcaster) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

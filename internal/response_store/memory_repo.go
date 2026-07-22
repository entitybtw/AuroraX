package responsestore

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

const (
	DefaultMemoryStoreTTL            = 24 * time.Hour
	DefaultMemoryStoreMaxEntries     = 10000
	defaultRepoCleanupTick    = time.Minute
)

type MemoryStore struct {
	mu              sync.RWMutex
	items           map[string]*StoredResponse
	ttl             time.Duration
	maxEntries      int
	lastCleanup     time.Time
	cleanupInterval time.Duration
}

type MemoryStoreOption func(*MemoryStore)

func WithTTL(ttl time.Duration) MemoryStoreOption {
	return func(s *MemoryStore) {
		s.ttl = ttl
	}
}

func WithMaxEntries(maxEntries int) MemoryStoreOption {
	return func(s *MemoryStore) {
		s.maxEntries = maxEntries
	}
}

func WithUnboundedRetention() MemoryStoreOption {
	return func(s *MemoryStore) {
		s.ttl = 0
		s.maxEntries = 0
	}
}

func NewMemoryStore(options ...MemoryStoreOption) *MemoryStore {
	store := &MemoryStore{
		items:           make(map[string]*StoredResponse),
		ttl:             DefaultMemoryStoreTTL,
		maxEntries:      DefaultMemoryStoreMaxEntries,
		cleanupInterval: defaultRepoCleanupTick,
	}
	for _, opt := range options {
		if opt != nil {
			opt(store)
		}
	}
	return store
}

func (s *MemoryStore) Create(_ context.Context, response *StoredResponse) error {
	if response == nil || response.Response == nil || response.Response.ID == "" {
		return fmt.Errorf("response id is required")
	}
	c, err := copyResponse(response)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	applyRetentionPolicy(c, now, s.ttl)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(now)
	if isExpired(c, now) {
		return nil
	}
	if existing, exists := s.items[c.Response.ID]; exists {
		if !isExpired(existing, now) {
			return fmt.Errorf("response already exists: %s", c.Response.ID)
		}
		delete(s.items, c.Response.ID)
	}
	s.items[c.Response.ID] = c
	s.evictExcessLocked()
	return nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (*StoredResponse, error) {
	now := time.Now().UTC()
	s.mu.Lock()
	s.purgeExpiredLocked(now)
	entry, ok := s.items[id]
	if !ok {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	if isExpired(entry, now) {
		delete(s.items, id)
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	s.mu.Unlock()
	return copyResponse(entry)
}

func (s *MemoryStore) Update(_ context.Context, response *StoredResponse) error {
	if response == nil || response.Response == nil || response.Response.ID == "" {
		return fmt.Errorf("response id is required")
	}
	c, err := copyResponse(response)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(now)
	existing, exists := s.items[c.Response.ID]
	if !exists {
		return ErrNotFound
	}
	if isExpired(existing, now) {
		delete(s.items, c.Response.ID)
		return ErrNotFound
	}
	if c.StoredAt.IsZero() {
		c.StoredAt = existing.StoredAt
	}
	if c.ExpiresAt.IsZero() {
		c.ExpiresAt = existing.ExpiresAt
	}
	applyRetentionPolicy(c, now, s.ttl)
	if isExpired(c, now) {
		delete(s.items, c.Response.ID)
		return ErrNotFound
	}
	s.items[c.Response.ID] = c
	s.evictExcessLocked()
	return nil
}

func (s *MemoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(time.Now().UTC())
	if _, exists := s.items[id]; !exists {
		return ErrNotFound
	}
	delete(s.items, id)
	return nil
}

func (s *MemoryStore) Close() error {
	return nil
}

func applyRetentionPolicy(response *StoredResponse, now time.Time, ttl time.Duration) {
	if response.StoredAt.IsZero() {
		response.StoredAt = now
	}
	if ttl > 0 && response.ExpiresAt.IsZero() {
		response.ExpiresAt = response.StoredAt.Add(ttl)
	}
}

func (s *MemoryStore) purgeExpiredLocked(now time.Time) {
	if s.ttl <= 0 {
		return
	}
	if s.cleanupInterval > 0 && !s.lastCleanup.IsZero() && now.Sub(s.lastCleanup) < s.cleanupInterval {
		return
	}
	s.lastCleanup = now
	for id, entry := range s.items {
		if isExpired(entry, now) {
			delete(s.items, id)
		}
	}
}

func (s *MemoryStore) evictExcessLocked() {
	if s.maxEntries <= 0 {
		return
	}
	over := len(s.items) - s.maxEntries
	if over <= 0 {
		return
	}
	entries := make([]repoEntry, 0, len(s.items))
	for id, entry := range s.items {
		entries = append(entries, repoEntry{
			id:       id,
			storedAt: entryStoredAt(entry),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].storedAt.Equal(entries[j].storedAt) {
			return entries[i].id < entries[j].id
		}
		return entries[i].storedAt.Before(entries[j].storedAt)
	})
	for i := 0; i < over && i < len(entries); i++ {
		delete(s.items, entries[i].id)
	}
}

type repoEntry struct {
	id       string
	storedAt time.Time
}

func isExpired(entry *StoredResponse, now time.Time) bool {
	return entry != nil && !entry.ExpiresAt.IsZero() && !entry.ExpiresAt.After(now)
}

func entryStoredAt(entry *StoredResponse) time.Time {
	if entry == nil || entry.StoredAt.IsZero() {
		return time.Time{}
	}
	return entry.StoredAt
}

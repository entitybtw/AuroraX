package batch

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]*StoredBatch
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items: make(map[string]*StoredBatch),
	}
}

func (s *MemoryStore) Create(_ context.Context, batch *StoredBatch) error {
	if batch == nil || batch.Batch == nil || batch.Batch.ID == "" {
		return fmt.Errorf("batch id is required")
	}
	c, err := deepCopy(batch)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.items[c.Batch.ID]; exists {
		return fmt.Errorf("batch already exists: %s", c.Batch.ID)
	}
	s.items[c.Batch.ID] = c
	return nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (*StoredBatch, error) {
	s.mu.RLock()
	b, ok := s.items[id]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	return deepCopy(b)
}

func (s *MemoryStore) List(_ context.Context, limit int, after string) ([]*StoredBatch, error) {
	limit = clampPageSize(limit)
	s.mu.RLock()
	all := make([]*StoredBatch, 0, len(s.items))
	for _, b := range s.items {
		c, err := deepCopy(b)
		if err != nil {
			s.mu.RUnlock()
			return nil, err
		}
		all = append(all, c)
	}
	s.mu.RUnlock()
	sort.Slice(all, func(i, j int) bool {
		if all[i].Batch.CreatedAt == all[j].Batch.CreatedAt {
			return all[i].Batch.ID > all[j].Batch.ID
		}
		return all[i].Batch.CreatedAt > all[j].Batch.CreatedAt
	})
	start := 0
	if after != "" {
		idx := -1
		for i := range all {
			if all[i].Batch.ID == after {
				idx = i
				break
			}
		}
		if idx == -1 {
			return nil, ErrNotFound
		}
		start = idx + 1
	}
	if start >= len(all) {
		return []*StoredBatch{}, nil
	}
	end := min(start+limit, len(all))
	return all[start:end], nil
}

func (s *MemoryStore) Update(_ context.Context, batch *StoredBatch) error {
	if batch == nil || batch.Batch == nil || batch.Batch.ID == "" {
		return fmt.Errorf("batch id is required")
	}
	c, err := deepCopy(batch)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.items[c.Batch.ID]; !exists {
		return ErrNotFound
	}
	s.items[c.Batch.ID] = c
	return nil
}

func (s *MemoryStore) Close() error {
	return nil
}

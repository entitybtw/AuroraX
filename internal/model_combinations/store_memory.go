package combos

import (
	"context"
	"sort"
	"strings"
	"sync"
)

type MemoryStore struct {
	mu     sync.RWMutex
	combos map[string]Combo
}

func NewMemoryStore(initial []Combo) *MemoryStore {
	store := &MemoryStore{combos: make(map[string]Combo)}
	for _, combo := range initial {
		if combo.ID == "" {
			combo.ID = combo.Name
		}
		store.combos[combo.ID] = cloneCombo(combo)
	}
	return store
}

func (s *MemoryStore) List(_ context.Context) ([]Combo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Combo, 0, len(s.combos))
	for _, combo := range s.combos {
		out = append(out, cloneCombo(combo))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *MemoryStore) Get(_ context.Context, idOrName string) (*Combo, error) {
	idOrName = strings.TrimSpace(idOrName)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, combo := range s.combos {
		if combo.ID == idOrName || combo.Name == idOrName {
			copy := cloneCombo(combo)
			return &copy, nil
		}
	}
	return nil, ErrNotFound
}

func (s *MemoryStore) Upsert(_ context.Context, combo Combo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if combo.ID == "" {
		combo.ID = combo.Name
	}
	s.combos[combo.ID] = cloneCombo(combo)
	return nil
}

func (s *MemoryStore) Delete(_ context.Context, idOrName string) error {
	idOrName = strings.TrimSpace(idOrName)
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, combo := range s.combos {
		if combo.ID == idOrName || combo.Name == idOrName {
			delete(s.combos, id)
			return nil
		}
	}
	return ErrNotFound
}

func (s *MemoryStore) Close() error { return nil }

func cloneCombo(combo Combo) Combo {
	combo.Models = append([]string(nil), combo.Models...)
	return combo
}

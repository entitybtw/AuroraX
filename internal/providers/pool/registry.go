package pool

import (
	json "github.com/goccy/go-json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Registry holds all configured pools keyed by pool name.
//
// Pool names share the same namespace as configured provider instance names,
// so pool resolution must take precedence at the router but a pool name MUST
// NOT collide with an existing provider instance name. The Build* helpers in
// this file enforce that.
type Registry struct {
	mu    sync.RWMutex
	pools map[string]*Pool
}

// NewRegistry creates an empty pool registry.
func NewRegistry() *Registry {
	return &Registry{pools: make(map[string]*Pool)}
}

// Replace copies all pools from another registry into this registry, carrying
// forward accumulated member counters (total_requests, total_errors) from
// matching members in the existing registry so that runtime pool rebuilds do
// not reset diagnostic stats.
//
// Existing pointers to this registry remain valid, which keeps admin
// snapshots aligned after runtime pool rebuilds.
func (r *Registry) Replace(other *Registry) {
	if r == nil {
		return
	}
	next := make(map[string]*Pool)
	if other != nil {
		other.mu.RLock()
		for name, p := range other.pools {
			next[name] = p
		}
		other.mu.RUnlock()
	}
	r.mu.Lock()
	// Merge counters from old members into new ones before the swap.
	for name, oldPool := range r.pools {
		newPool, exists := next[name]
		if !exists {
			continue
		}
		mergePoolCounters(oldPool, newPool)
	}
	r.pools = next
	r.mu.Unlock()
}

// mergePoolCounters carries forward accumulated request/error counters from
// matching members in src to dst so that stats survive runtime rebuilds.
func mergePoolCounters(src, dst *Pool) {
	for _, sm := range src.members {
		for _, dm := range dst.members {
			if dm.ProviderName != sm.ProviderName {
				continue
			}
			oldReq := atomic.LoadInt64(&sm.totalRequests)
			oldErr := atomic.LoadInt64(&sm.totalErrors)
			atomic.AddInt64(&dm.totalRequests, oldReq)
			atomic.AddInt64(&dm.totalErrors, oldErr)
			break
		}
	}
}

// Register adds a pool. Returns an error if a pool with the same name already exists.
func (r *Registry) Register(p *Pool) error {
	if p == nil {
		return fmt.Errorf("cannot register nil pool")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.pools[p.Name()]; exists {
		return fmt.Errorf("pool %q already registered", p.Name())
	}
	r.pools[p.Name()] = p
	return nil
}

// Get returns the pool registered under name, or nil if none.
func (r *Registry) Get(name string) *Pool {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.pools[name]
}

// Names returns the registered pool names sorted alphabetically.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.pools))
	for name := range r.pools {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Snapshot returns a defensive copy of all pools' member metadata. Used by
// the admin dashboard and Prometheus exporter.
func (r *Registry) Snapshot() []PoolSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PoolSnapshot, 0, len(r.pools))
	for _, p := range r.pools {
		out = append(out, PoolSnapshot{
			Name:     p.Name(),
			Strategy: string(p.Strategy()),
			Members:  p.Members(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// PoolSnapshot is a read-only diagnostic view of a configured pool.
type PoolSnapshot struct {
	Name     string           `json:"name"`
	Strategy string           `json:"strategy"`
	Members  []MemberSnapshot `json:"members"`
}

// Count returns the number of registered pools.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.pools)
}

// HasPool reports whether name maps to a registered pool.
func (r *Registry) HasPool(name string) bool {
	return r.Get(name) != nil
}

// --- Pool counter persistence ----------------------------------------------

// persistedPoolCounters is the on-disk snapshot format for pool member stats.
type persistedPoolCounters struct {
	Version int                             `json:"version"`
	Pools   map[string][]persistedMemberStat `json:"pools"`
}

type persistedMemberStat struct {
	ProviderName  string `json:"provider_name"`
	TotalRequests int64  `json:"total_requests"`
	TotalErrors   int64  `json:"total_errors"`
}

// SaveCounters writes current pool member counters to a JSON file using an
// atomic write (temp file + rename) to prevent partial writes.
func (r *Registry) SaveCounters(path string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data := persistedPoolCounters{
		Version: 1,
		Pools:   make(map[string][]persistedMemberStat, len(r.pools)),
	}
	for name, p := range r.pools {
		members := p.Members()
		stats := make([]persistedMemberStat, len(members))
		for i, m := range members {
			stats[i] = persistedMemberStat{
				ProviderName:  m.ProviderName,
				TotalRequests: m.TotalRequests,
				TotalErrors:   m.TotalErrors,
			}
		}
		data.Pools[name] = stats
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("pool counters: marshal: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("pool counters: mkdir: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return fmt.Errorf("pool counters: write tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("pool counters: rename: %w", err)
	}
	return nil
}

// LoadCounters restores pool member counters from a previously-saved JSON
// file. Members that no longer exist are silently skipped.
func (r *Registry) LoadCounters(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // first boot — nothing to restore
		}
		return fmt.Errorf("pool counters: read: %w", err)
	}

	var data persistedPoolCounters
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("pool counters: unmarshal: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for poolName, stats := range data.Pools {
		p, ok := r.pools[poolName]
		if !ok {
			continue // pool was removed from config — skip
		}
		for _, stat := range stats {
			for _, dm := range p.members {
				if dm.ProviderName != stat.ProviderName {
					continue
				}
				atomic.AddInt64(&dm.totalRequests, stat.TotalRequests)
				atomic.AddInt64(&dm.totalErrors, stat.TotalErrors)
				break
			}
		}
	}
	return nil
}

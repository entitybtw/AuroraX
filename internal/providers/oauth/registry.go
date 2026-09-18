// Package oauth provides RFC 8628 OAuth 2.0 device authorization grant support.
package oauth

import (
	"log/slog"
	"sync"
)

// Registry holds OAuth managers keyed by provider name. It provides a
// central lookup so the admin API can start device flows for any provider
// that has OAuth configured.
type Registry struct {
	mu      sync.RWMutex
	managers map[string]*Manager
}

// NewRegistry creates a new empty OAuth registry.
func NewRegistry() *Registry {
	return &Registry{
		managers: make(map[string]*Manager),
	}
}

// Register adds (or replaces) an OAuth manager for the given provider name.
func (r *Registry) Register(providerName string, mgr *Manager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.managers[providerName] = mgr
	slog.Debug("oauth: registered manager", "provider", providerName)
}

// Get returns the OAuth manager for the given provider name, or nil.
func (r *Registry) Get(providerName string) *Manager {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.managers[providerName]
}

// All returns a snapshot of all registered provider names.
func (r *Registry) All() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.managers))
	for name := range r.managers {
		names = append(names, name)
	}
	return names
}

// Len returns the number of registered managers.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.managers)
}

package sessionhub

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/labstack/echo/v5"
)

// Hub is the central session-hub engine. It owns the config, store,
// and applies transformation rules per-provider.
type Hub struct {
	mu     sync.RWMutex
	config *HubConfig
	store  *Store
	// savePath, when set, is a YAML file that the hub loads at startup and
	// rewrites after every rule mutation so rules survive restarts.
	savePath string
	// mappingPath is the file live session mappings are written to/read from
	// when store persistence is enabled.
	mappingPath string
	// providerPool maps a concrete provider instance name to the pools that
	// contain it, so a rule bound to a pool applies to every member provider.
	providerPool map[string][]string
	// rulesIdx is a read-optimized snapshot for the hot Apply path. It is
	// rebuilt only when rules or pool membership change (a rare write).
	rulesIdx atomic.Pointer[map[string]ProviderRule]
}

// New creates a Hub with the given config.
func New(cfg *HubConfig) *Hub {
	if cfg == nil {
		cfg = &HubConfig{Providers: make(map[string]ProviderRule)}
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderRule)
	}
	h := &Hub{
		config:       cfg,
		savePath:     cfg.Path,
		mappingPath:  cfg.MappingsPath,
		store:        newConfiguredStore(cfg),
		providerPool: make(map[string][]string),
	}
	h.rebuildRulesIndex()
	return h
}

// newConfiguredStore builds a session store honouring cfg.MappingStorage. The
// persistence path is always installed so a later runtime toggle to disk can
// enable writing; only MappingStorage == "disk" writes on boot.
func newConfiguredStore(cfg *HubConfig) *Store {
	s := NewStore(WithPersistence(cfg.MappingsPath))
	if cfg.MappingStorage != "disk" {
		s.SetPersists(false)
	}
	return s
}

// rebuildRulesIndex flattens config.Providers plus pool associations into a
// single target->rule snapshot so resolveRule/Apply are lock-free reads.
// Caller must hold h.mu exclusively when mutating.
func (h *Hub) rebuildRulesIndexLocked() {
	idx := make(map[string]ProviderRule, len(h.config.Providers))
	for target, rule := range h.config.Providers {
		idx[target] = rule
		// If target is a pool, also map its member providers to this rule so a
		// member request resolves a pool-bound rule without an extra iteration.
		if members := h.providerPool[target]; members != nil {
			for _, m := range members {
				if _, exists := idx[m]; !exists {
					idx[m] = rule
				}
			}
		}
	}
	h.rulesIdx.Store(&idx)
}

// rebuildRulesIndex is the lock-free-safe public wrapper used only right after
// construction when no concurrency exists yet.
func (h *Hub) rebuildRulesIndex() {
	h.rebuildRulesIndexLocked()
}
// NewWithPersistence creates a hub, seeds it from the YAML file at path if it
// exists (overriding compiled-in defaults with persisted rules), and configures
// it to auto-save after every rule mutation. mappingsPath is where live
// session mappings are persisted when MappingStorage == "disk".
func NewWithPersistence(path, mappingsPath string, cfg *HubConfig) *Hub {
	loaded := cfg
	if path != "" {
		if persisted, err := LoadConfig(path); err == nil && persisted != nil {
			// Merge: base config providers first, then persisted rules override
			if loaded == nil {
				loaded = &HubConfig{}
			}
			if loaded.Providers == nil {
				loaded.Providers = make(map[string]ProviderRule)
			}
			for k, v := range persisted.Providers {
				loaded.Providers[k] = v
			}
			loaded.Enabled = loaded.Enabled || persisted.Enabled
			loaded.Path = path
			if persisted.MappingStorage != "" {
				loaded.MappingStorage = persisted.MappingStorage
			}
		}
	}
	if loaded.MappingsPath == "" {
		loaded.MappingsPath = mappingsPath
	}
	h := New(loaded)
	h.savePath = path
	return h
}

// PersistPath returns the configured persistence file path.
func (h *Hub) PersistPath() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.savePath
}

// save persists the current config to disk. No-op when no save path is set.
func (h *Hub) save() error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.savePath == "" {
		return nil
	}
	raw, err := h.config.MarshalYAML()
	if err != nil {
		return err
	}
	return atomicWrite(h.savePath, raw)
}

// Config returns the current config (read-only snapshot).
func (h *Hub) Config() *HubConfig {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.config
}

// Store returns the session mapping store.
func (h *Hub) Store() *Store {
	return h.store
}

// MappingStorage returns the active mapping storage mode: "memory" or "disk".
func (h *Hub) MappingStorage() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	val := h.config.MappingStorage
	if val == "disk" || h.store.StorageMode() == "disk" {
		return "disk"
	}
	if val == "" {
		return "memory"
	}
	return val
}

// SetMappingStorage toggles whether live session mappings are persisted to
// disk ("disk") or kept only in memory ("memory"/"").
func (h *Hub) SetMappingStorage(mode string) error {
	h.mu.Lock()
	h.config.MappingStorage = mode
	h.mu.Unlock()
	h.store.SetPersists(mode == "disk")
	h.save()
	return nil
}

// Reload replaces the config at runtime (hot reload).
func (h *Hub) Reload(cfg *HubConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cfg == nil {
		return
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderRule)
	}
	cfg.Path = h.savePath
	h.config = cfg
	h.rebuildRulesIndexLocked()
	h.save()
}

// GetProviderRule returns the rule for a specific provider.
func (h *Hub) GetProviderRule(provider string) (ProviderRule, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	r, ok := h.config.Providers[provider]
	return r, ok
}

// SetProviderRule upserts a rule for a provider.
func (h *Hub) SetProviderRule(provider string, rule ProviderRule) {
	h.mu.Lock()
	h.config.Providers[provider] = rule
	h.rebuildRulesIndexLocked()
	h.mu.Unlock()
	h.save()
}

// DeleteProviderRule removes a provider's rule.
func (h *Hub) DeleteProviderRule(provider string) {
	h.mu.Lock()
	delete(h.config.Providers, provider)
	h.rebuildRulesIndexLocked()
	h.mu.Unlock()
	h.save()
}

// Apply applies header transformations for the given provider.
// It returns the generated/mapped values, or nil if no rule matched.
//
// Rule resolution is a single lock-free read of a prebuilt target->rule index
// (pool-bound rules are pre-expanded to their member providers on writes).
func (h *Hub) Apply(headers http.Header, provider string) map[string]string {
	idx := h.rulesIdx.Load()
	var rule ProviderRule
	var ok bool
	if idx != nil {
		if exact, found := (*idx)[provider]; found {
			rule, ok = exact, true
		} else if wild, wfound := (*idx)["*"]; wfound {
			rule, ok = wild, true
		}
	}
	if !ok {
		return nil
	}
	if !rule.Enabled && len(rule.Headers) > 0 {
		return nil
	}
	return Transform(headers, provider, rule, h.store)
}

// SetPoolMembership registers that the given pool contains the listed
// concrete provider names. Rules bound to the pool then apply to every member.
func (h *Hub) SetPoolMembership(poolProvider string, members []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.providerPool[poolProvider] = append([]string(nil), members...)
	// Index reverse lookup provider -> pools for Apply
	for _, m := range members {
		existing := h.providerPool[m]
		for _, e := range existing {
			if e == poolProvider {
				goto next
			}
		}
		h.providerPool[m] = append(h.providerPool[m], poolProvider)
	next:
	}
	h.rebuildRulesIndexLocked()
}

// Stats returns global stats.
func (h *Hub) Stats() HubStats {
	total, byProvider := h.store.Stats()
	h.mu.RLock()
	providerCount := len(h.config.Providers)
	mode := h.config.MappingStorage
	h.mu.RUnlock()
	if mode != "disk" && h.store.StorageMode() == "disk" {
		mode = "disk"
	}
	if mode != "disk" {
		mode = "memory"
	}

	enabledCount := 0
	for _, r := range h.config.Providers {
		if r.Enabled {
			enabledCount++
		}
	}

	return HubStats{
		TotalMappings:       total,
		ByProvider:          byProvider,
		ConfiguredProviders: providerCount,
		EnabledProviders:    enabledCount,
		StorageMode:         mode,
	}
}

// HubStats is a summary of hub state.
type HubStats struct {
	TotalMappings       int            `json:"total_mappings"`
	ByProvider          map[string]int `json:"by_provider"`
	ConfiguredProviders int            `json:"configured_providers"`
	EnabledProviders    int            `json:"enabled_providers"`
	StorageMode         string         `json:"storage_mode"`
}

// ProviderNames returns all configured provider names.
func (h *Hub) ProviderNames() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	names := make([]string, 0, len(h.config.Providers))
	for name := range h.config.Providers {
		names = append(names, name)
	}
	return names
}

// RegisterRoutes satisfies the server.SessionHubRegistrator interface.
func (h *Hub) RegisterRoutes(g interface {
	GET(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo
	POST(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo
	PUT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo
	DELETE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo
}) {
	RegisterSessionHubRoutes(g, h)
}

// Validate checks a ProviderRule for obvious issues.
func ValidateRule(name string, rule ProviderRule) error {
	for i, hr := range rule.Headers {
		if hr.Name == "" {
			return fmt.Errorf("provider %q, header %d: name is required", name, i)
		}
		switch hr.Mode {
		case HeaderModeGenerate, HeaderModePassthrough, HeaderModeStatic,
			HeaderModeRandomFromList, HeaderModeRemove, HeaderModeMap:
			// ok
		case "":
			return fmt.Errorf("provider %q, header %q: mode is required", name, hr.Name)
		default:
			return fmt.Errorf("provider %q, header %q: unsupported mode %q", name, hr.Name, hr.Mode)
		}
		if hr.Mode == HeaderModeStatic && hr.Value == "" {
			return fmt.Errorf("provider %q, header %q: static mode requires value", name, hr.Name)
		}
		if hr.Mode == HeaderModeRandomFromList && len(hr.Values) == 0 {
			return fmt.Errorf("provider %q, header %q: random_from_list requires values", name, hr.Name)
		}
	}
	return nil
}

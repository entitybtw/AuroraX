package sessionhub

import (
	"fmt"
	"net/http"
	"sync"

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
	// providerPool maps a concrete provider instance name to the pools that
	// contain it, so a rule bound to a pool applies to every member provider.
	providerPool map[string][]string
}

// New creates a Hub with the given config.
func New(cfg *HubConfig) *Hub {
	if cfg == nil {
		cfg = &HubConfig{Providers: make(map[string]ProviderRule)}
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderRule)
	}
	return &Hub{
		config:       cfg,
		store:        NewStore(),
		savePath:     cfg.Path,
		providerPool: make(map[string][]string),
	}
}

// NewWithPersistence creates a hub, seeds it from the YAML file at path if it
// exists (overriding compiled-in defaults with persisted rules), and configures
// it to auto-save after every rule mutation.
func NewWithPersistence(path string, cfg *HubConfig) *Hub {
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
		}
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
	h.mu.Unlock()
	h.save()
}

// DeleteProviderRule removes a provider's rule.
func (h *Hub) DeleteProviderRule(provider string) {
	h.mu.Lock()
	delete(h.config.Providers, provider)
	h.mu.Unlock()
	h.save()
}

// Apply applies header transformations for the given provider.
// It returns the generated/mapped values, or nil if no rule matched.
//
// Rule resolution order for the named target:
//  1. exact rule keyed by the target name (provider OR pool OR fallback OR type)
//  2. if the target is a concrete provider, any rule bound to a pool that
//     contains that provider (first match wins)
//  3. the wildcard rule "*"
func (h *Hub) Apply(headers http.Header, provider string) map[string]string {
	rule, ok := h.resolveRule(provider)
	if !ok {
		return nil
	}
	if !rule.Enabled && len(rule.Headers) > 0 {
		return nil
	}
	return Transform(headers, provider, rule, h.store)
}

func (h *Hub) resolveRule(target string) (ProviderRule, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if rule, ok := h.config.Providers[target]; ok {
		return rule, true
	}
	// Try pool associations for the target provider
	for _, pool := range h.providerPool[target] {
		if rule, ok := h.config.Providers[pool]; ok {
			return rule, true
		}
	}
	if rule, ok := h.config.Providers["*"]; ok {
		return rule, true
	}
	return ProviderRule{}, false
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
}

// Stats returns global stats.
func (h *Hub) Stats() HubStats {
	total, byProvider := h.store.Stats()
	h.mu.RLock()
	providerCount := len(h.config.Providers)
	h.mu.RUnlock()

	enabledCount := 0
	for _, r := range h.config.Providers {
		if r.Enabled {
			enabledCount++
		}
	}

	return HubStats{
		TotalMappings:     total,
		ByProvider:        byProvider,
		ConfiguredProviders: providerCount,
		EnabledProviders:    enabledCount,
	}
}

// HubStats is a summary of hub state.
type HubStats struct {
	TotalMappings       int            `json:"total_mappings"`
	ByProvider          map[string]int `json:"by_provider"`
	ConfiguredProviders int            `json:"configured_providers"`
	EnabledProviders    int            `json:"enabled_providers"`
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

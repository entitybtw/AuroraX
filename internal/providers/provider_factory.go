// Package providers provides a factory for creating provider instances.
package providers

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"aurora/configuration"
	"aurora/internal/core"
	"aurora/internal/externalauth"
	"aurora/internal/language_model_client"
)

// ProviderOptions bundles runtime settings passed from the factory to provider constructors.
type ProviderOptions struct {
	Hooks      llmclient.Hooks
	Models     []string
	Resilience config.ResilienceConfig
	// BindIP optionally sets the local outbound IP for the upstream HTTP client.
	BindIP string
	// UserAgent optionally overrides the User-Agent header sent to the upstream provider.
	UserAgent string
	// ProviderName is the configured instance name (e.g. "acc-1" or "my-pool-member").
	// Empty when the creating caller does not supply a name.
	ProviderName string
	// SessionHub optionally provides a header transformer for session mapping.
	// When set, providers wrap their headerSetter to apply session hub rules.
	SessionHub SessionHubTransformer
	// AuthMethod selects authentication: "key" (default) or "external"
	// (bearer tokens from extension auth addons).
	AuthMethod string
	// ExternalAuth bridges to extension auth addons for bearer tokens.
	ExternalAuth *externalauth.Bridge
	// DisableAPIKey keeps the stored API key but stops sending it upstream.
	DisableAPIKey bool
	// UseUTLS enables uTLS fingerprint impersonation for the HTTP client.
	UseUTLS bool
}

// SessionHubTransformer transforms headers for a given provider name.
// Returns true if any transformation was applied.
type SessionHubTransformer func(providerName string, headers http.Header) bool

// ProviderConstructor is the constructor signature for providers.
type ProviderConstructor func(cfg ProviderConfig, opts ProviderOptions) core.Provider

// DiscoveryConfig describes how a provider participates in config resolution.
// Env var names are derived by convention from Registration.Type.
type DiscoveryConfig struct {
	DefaultBaseURL     string
	RequireBaseURL     bool
	AllowAPIKeyless    bool
	SupportsAPIVersion bool
}

// Registration contains metadata for registering a provider with the factory.
type Registration struct {
	Type                        string
	New                         ProviderConstructor
	PassthroughSemanticEnricher core.PassthroughSemanticEnricher
	Discovery                   DiscoveryConfig
}

// ProviderFactory manages provider registration and creation.
type ProviderFactory struct {
	mu                   sync.RWMutex
	builders             map[string]ProviderConstructor
	discoveryConfigs     map[string]DiscoveryConfig
	passthroughEnrichers map[string]core.PassthroughSemanticEnricher
	hooks                llmclient.Hooks
	sessionHub           SessionHubTransformer
	externalAuth         *externalauth.Bridge
}

// NewProviderFactory creates a new provider factory instance.
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		builders:             make(map[string]ProviderConstructor),
		discoveryConfigs:     make(map[string]DiscoveryConfig),
		passthroughEnrichers: make(map[string]core.PassthroughSemanticEnricher),
	}
}

// SetHooks configures observability hooks for all providers created by this factory.
func (f *ProviderFactory) SetHooks(hooks llmclient.Hooks) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hooks = hooks
}

// SetSessionHub configures the session hub transformer for all providers.
func (f *ProviderFactory) SetSessionHub(transformer SessionHubTransformer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessionHub = transformer
}

// SetExternalAuth attaches the extension auth bridge for all providers
// created by this factory.
func (f *ProviderFactory) SetExternalAuth(bridge *externalauth.Bridge) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.externalAuth = bridge
}

// ExternalAuth returns the configured extension auth bridge, or nil.
func (f *ProviderFactory) ExternalAuth() *externalauth.Bridge {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.externalAuth
}

// Add adds a provider constructor to the factory.
// Panics if reg.Type is empty or reg.New is nil — both are programming errors
// caught at startup, not runtime conditions.
func (f *ProviderFactory) Add(reg Registration) {
	if reg.Type == "" {
		panic("providers: Add called with empty Type")
	}
	if reg.New == nil {
		panic(fmt.Sprintf("providers: Add called with nil constructor for type %q", reg.Type))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.builders[reg.Type] = reg.New
	f.discoveryConfigs[reg.Type] = reg.Discovery
	if reg.PassthroughSemanticEnricher != nil {
		f.passthroughEnrichers[reg.Type] = reg.PassthroughSemanticEnricher
	} else {
		delete(f.passthroughEnrichers, reg.Type)
	}
}

// Create instantiates a provider based on its resolved configuration.
func (f *ProviderFactory) Create(cfg ProviderConfig) (core.Provider, error) {
	f.mu.RLock()
	builder, ok := f.builders[cfg.Type]
	hooks := f.hooks
	f.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}

	opts := ProviderOptions{
		Hooks:        hooks,
		Models:       cfg.Models,
		Resilience:   cfg.Resilience,
		BindIP:       cfg.BindIP,
		UserAgent:    cfg.UserAgent,
		ProviderName: cfg.Name,
		SessionHub:   f.sessionHub,
		AuthMethod:   cfg.AuthMethod,
		ExternalAuth: f.externalAuth,
		DisableAPIKey: cfg.DisableAPIKey,
		UseUTLS:      cfg.UseUTLS,
	}

	if strings.TrimSpace(cfg.BaseURL) == "" {
		if def := LoadSidecarDefaults(); def.BaseURL != "" && cfg.Type == "cli-emulation" {
			cfg.BaseURL = def.BaseURL
		}
	}

	return builder(cfg, opts), nil
}

// discoveryConfigsSnapshot returns provider discovery metadata keyed by provider type.
func (f *ProviderFactory) discoveryConfigsSnapshot() map[string]DiscoveryConfig {
	f.mu.RLock()
	defer f.mu.RUnlock()

	snapshot := make(map[string]DiscoveryConfig, len(f.discoveryConfigs))
	for providerType, cfg := range f.discoveryConfigs {
		snapshot[providerType] = cfg
	}
	return snapshot
}

// RegisteredTypes returns a list of all registered provider types.
func (f *ProviderFactory) RegisteredTypes() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	types := make([]string, 0, len(f.builders))
	for t := range f.builders {
		types = append(types, t)
	}
	return types
}

// PassthroughSemanticEnrichers returns registered passthrough semantic
// enrichers in deterministic provider-type order.
func (f *ProviderFactory) PassthroughSemanticEnrichers() []core.PassthroughSemanticEnricher {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if len(f.passthroughEnrichers) == 0 {
		return nil
	}

	types := make([]string, 0, len(f.passthroughEnrichers))
	for providerType := range f.passthroughEnrichers {
		types = append(types, providerType)
	}
	sort.Strings(types)

	enrichers := make([]core.PassthroughSemanticEnricher, 0, len(types))
	for _, providerType := range types {
		if enricher := f.passthroughEnrichers[providerType]; enricher != nil {
			enrichers = append(enrichers, enricher)
		}
	}
	if len(enrichers) == 0 {
		return nil
	}
	return enrichers
}

// optionalRegistrations holds provider types that are NOT registered by
// default. Extensions declare them under provides.provider_types and they
// are activated on import/apply (e.g. the "cli-emulation" type ships only with
// the matching store extension).
var (
	optionalMu       sync.RWMutex
	optionalRegistry = map[string]Registration{}
)

// RegisterOptional stages a provider type for extension-driven activation.
// It does not make the type available to Create until ActivateOptional runs.
func RegisterOptional(reg Registration) {
	if reg.Type == "" || reg.New == nil {
		return
	}
	optionalMu.Lock()
	defer optionalMu.Unlock()
	optionalRegistry[reg.Type] = reg
}

// OptionalTypes returns the staged optional provider type names.
func OptionalTypes() []string {
	optionalMu.RLock()
	defer optionalMu.RUnlock()
	out := make([]string, 0, len(optionalRegistry))
	for t := range optionalRegistry {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// ActivateOptional registers the named optional types on the factory if they
// were staged via RegisterOptional. Returns the types that were newly activated.
func (f *ProviderFactory) ActivateOptional(types ...string) []string {
	optionalMu.RLock()
	defer optionalMu.RUnlock()
	var activated []string
	for _, t := range types {
		reg, ok := optionalRegistry[t]
		if !ok {
			continue
		}
		f.mu.RLock()
		_, already := f.builders[t]
		f.mu.RUnlock()
		if already {
			continue
		}
		f.Add(reg)
		activated = append(activated, t)
	}
	return activated
}

// IsRegistered reports whether a provider type is currently creatable.
func (f *ProviderFactory) IsRegistered(typ string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.builders[typ]
	return ok
}

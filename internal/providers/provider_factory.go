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
	"aurora/internal/language_model_client"
	"aurora/internal/providers/oauth"
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
	// AuthMethod selects authentication: "key" (default) or "oauth".
	AuthMethod string
	// OAuthServer is the base URL of the OAuth server for device flow.
	OAuthServer string
	// OAuthClientID is the OAuth client_id for the device flow.
	OAuthClientID string
	// OAuthAuthorizeURL / OAuthTokenURL / OAuthScopes / OAuthTokenStyle /
	// OAuthStateIsVerifier / OAuthRedirectURI / OAuthGrant configure
	// authorization-code + PKCE when grant is "authorization_code".
	OAuthAuthorizeURL    string
	OAuthTokenURL        string
	OAuthScopes          string
	OAuthTokenStyle      string
	OAuthStateIsVerifier bool
	OAuthRedirectURI     string
	OAuthGrant           string
	// OAuthDataDir is the directory for persisting OAuth tokens.
	OAuthDataDir string
	// OAuthRegistry is the central registry for OAuth token managers.
	OAuthRegistry *oauth.Registry
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
	oauthRegistry        *oauth.Registry
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

// SetOAuthDataDir is replaced by SetOAuthRegistry. Kept for backward compatibility.
func (f *ProviderFactory) SetOAuthDataDir(dir string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// No-op: data dir is now part of the registry setup
}

// SetOAuthRegistry configures the OAuth registry for all providers.
func (f *ProviderFactory) SetOAuthRegistry(registry *oauth.Registry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.oauthRegistry = registry
}

// OAuthRegistry returns the configured OAuth registry, or nil.
func (f *ProviderFactory) OAuthRegistry() *oauth.Registry {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.oauthRegistry
}

func (f *ProviderFactory) oauthDataDir() string {
	return "data"
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
		Hooks:                hooks,
		Models:               cfg.Models,
		Resilience:           cfg.Resilience,
		BindIP:               cfg.BindIP,
		UserAgent:            cfg.UserAgent,
		ProviderName:         cfg.Name,
		SessionHub:           f.sessionHub,
		AuthMethod:           cfg.AuthMethod,
		OAuthServer:          cfg.OAuthServer,
		OAuthClientID:        cfg.OAuthClientID,
		OAuthAuthorizeURL:    cfg.OAuthAuthorizeURL,
		OAuthTokenURL:        cfg.OAuthTokenURL,
		OAuthScopes:          cfg.OAuthScopes,
		OAuthTokenStyle:      cfg.OAuthTokenStyle,
		OAuthStateIsVerifier: cfg.OAuthStateIsVerifier,
		OAuthRedirectURI:     cfg.OAuthRedirectURI,
		OAuthGrant:           cfg.OAuthGrant,
		OAuthDataDir:         f.oauthDataDir(),
		OAuthRegistry:        f.oauthRegistry,
		DisableAPIKey:        cfg.DisableAPIKey,
		UseUTLS:              cfg.UseUTLS,
	}

	// Extension-applied sidecar defaults fill empty OAuth wiring so the
	// gateway itself never hardcodes a provider origin/client.
	if opts.AuthMethod == "oauth" {
		def := LoadSidecarOAuthDefaults()
		if opts.OAuthServer == "" {
			opts.OAuthServer = def.OAuthServer
		}
		if opts.OAuthClientID == "" {
			opts.OAuthClientID = def.OAuthClientID
		}
		if opts.OAuthGrant == "" {
			opts.OAuthGrant = def.OAuthGrant
		}
		if opts.OAuthAuthorizeURL == "" {
			opts.OAuthAuthorizeURL = def.OAuthAuthorizeURL
		}
		if opts.OAuthTokenURL == "" {
			opts.OAuthTokenURL = def.OAuthTokenURL
		}
		if opts.OAuthScopes == "" {
			opts.OAuthScopes = def.OAuthScopes
		}
		if opts.OAuthTokenStyle == "" {
			opts.OAuthTokenStyle = def.OAuthTokenStyle
		}
		if opts.OAuthRedirectURI == "" {
			opts.OAuthRedirectURI = def.OAuthRedirectURI
		}
		opts.OAuthStateIsVerifier = opts.OAuthStateIsVerifier || def.OAuthStateIsVerifier
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		if def := LoadSidecarOAuthDefaults(); def.BaseURL != "" && cfg.Type == "opencode" {
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
// are activated on import/apply (e.g. the "opencode" type ships only with
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

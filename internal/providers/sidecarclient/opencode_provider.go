// Package opencode provides OpenCode Zen integration for the LLM gateway.
// It wraps the vLLM provider but auto-detects OpenCode API keys (sk-*) and
// forces OAuth device flow for free-tier access. When no OAuth token is
// available, falls back to the static API key.
package opencode

import (
	"context"
	"io"
	"os"
	"strings"

	"aurora/internal/core"
	"aurora/internal/providers"
	"aurora/internal/providers/oauth"
	"aurora/internal/providers/vllm"
)

const defaultBaseURL = "https://opencode.ai/zen/v1"

// sidecarEnvURL is the environment variable that points at the local Bun
// sidecar used to reproduce the TLS fingerprint required by the zen free tier.
const sidecarEnvURL = "AURORA_SIDECAR_BASE_URL"

// Registration provides factory registration for the OpenCode provider.
var Registration = providers.Registration{
	Type:  "opencode",
	New:   New,
	Discovery: providers.DiscoveryConfig{
		DefaultBaseURL:  defaultBaseURL,
		RequireBaseURL:  false,
		AllowAPIKeyless: true,
	},
}

// Provider wraps the vLLM provider with OpenCode-specific OAuth logic.
type Provider struct {
	inner    *vllm.Provider
	oauthMgr *oauth.Manager
}

// New creates a new OpenCode provider.
// When an sk-* API key is detected, it automatically enables OAuth device flow
// and registers with the OAuth registry. The API key is kept as fallback.
func New(cfg providers.ProviderConfig, opts providers.ProviderOptions) core.Provider {
	// If no base URL configured, use OpenCode zen default
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultBaseURL
	}

	// Auto-detect: if there's an sk-* API key but auth_method is not set,
	// force OAuth mode for free-tier access
	if opts.AuthMethod == "" && strings.HasPrefix(strings.TrimSpace(cfg.APIKey), "sk-") {
		opts.AuthMethod = "oauth"
	}

	// Route zen traffic through the local Bun sidecar when one is configured.
	// The sidecar reproduces the Bun TLS fingerprint that the zen free tier
	// requires and forwards OAuth/API-key credentials unchanged, so paid
	// accounts continue to work through the same path.
	if sidecar := resolveSidecarURL(cfg); sidecar != "" {
		cfg.BaseURL = sidecar
	}

	// Enable uTLS fingerprint impersonation for opencode.ai zen
	// (JA3 fingerprinting bypass required for free-tier access)
	opts.UseUTLS = true

	// Set OAuth defaults for OpenCode zen
	if opts.AuthMethod == "oauth" {
		if opts.OAuthServer == "" {
			opts.OAuthServer = oauth.DefaultServer
		}
		if opts.OAuthClientID == "" {
			opts.OAuthClientID = oauth.DefaultClientID
		}
	}

	// Create the inner vLLM provider with the resolved config
	inner := vllm.New(cfg, opts).(*vllm.Provider)

	// Get the OAuth manager from the inner provider
	var oauthMgr *oauth.Manager
	if innerOAuth := inner.OAuthManager(); innerOAuth != nil {
		oauthMgr = innerOAuth
	}

	return &Provider{
		inner:    inner,
		oauthMgr: oauthMgr,
	}
}

// OAuthManager returns the OAuth token manager, or nil if OAuth is not configured.
func (p *Provider) OAuthManager() *oauth.Manager {
	return p.oauthMgr
}

// resolveSidecarURL returns the Bun sidecar base URL for this provider, or an
// empty string when the sidecar is not configured. A provider-level
// sidecar_url wins; otherwise the environment default is used, but only when
// the provider actually targets OpenCode zen.
func resolveSidecarURL(cfg providers.ProviderConfig) string {
	if sidecar := strings.TrimSpace(cfg.SidecarURL); sidecar != "" {
		return sidecar
	}
	if !strings.Contains(cfg.BaseURL, "opencode.ai/zen") {
		return ""
	}
	return strings.TrimSpace(os.Getenv(sidecarEnvURL))
}

// ChatCompletion sends a chat completion request via the inner vLLM provider.
func (p *Provider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return p.inner.ChatCompletion(ctx, req)
}

// StreamChatCompletion returns a raw response body for streaming.
func (p *Provider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	return p.inner.StreamChatCompletion(ctx, req)
}

// ListModels retrieves the list of available models.
func (p *Provider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	return p.inner.ListModels(ctx)
}

// Responses sends a Responses API request.
func (p *Provider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	return p.inner.Responses(ctx, req)
}

// StreamResponses streams a Responses API request.
func (p *Provider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	return p.inner.StreamResponses(ctx, req)
}

// Embeddings sends an embeddings request.
func (p *Provider) Embeddings(ctx context.Context, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	return p.inner.Embeddings(ctx, req)
}

// Passthrough routes an opaque provider-native request.
func (p *Provider) Passthrough(ctx context.Context, req *core.PassthroughRequest) (*core.PassthroughResponse, error) {
	return p.inner.Passthrough(ctx, req)
}

// Ensure Provider satisfies the interface at compile time.
var _ core.Provider = (*Provider)(nil)

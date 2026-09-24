// Package sidecarclient provides an optional extension-provided provider type.
// It is NOT registered by default: extensions declare it under
// provides.provider_types and the gateway activates it when that
// extension is installed. device flow is an extension feature and
// only runs when auth_method is explicitly "".
package sidecarclient

import (
	"context"
	"io"
	"os"
	"strings"

	"aurora/internal/core"
	"aurora/internal/providers"
	"aurora/internal/providers/"
	"aurora/internal/providers/vllm"
)

// baseURLOverride is not a hardcoded product origin: Discovery leaves
// DefaultBaseURL empty and RequireBaseURL is true. When the provider config
// has no base_url, the factory injects the extension-applied sidecar base_url.
const baseURLOverride = ""

// sidecarEnvURL points at the local TLS-fingerprint sidecar. The SIDECAR
// suffix is reserved so env-based discovery never creates a provider from
// these keys. AURORA_SIDECAR_BASE_URL points at the local sidecar.
const sidecarEnvURL = "AURORA_SIDECAR_BASE_URL"

func envSidecarURL() string {
	return strings.TrimSpace(os.Getenv(sidecarEnvURL))
}

// Registration provides factory registration for the optional extension type.
// Only staged via providers.RegisterOptional — not Add()ed by default.
// base_url must come from provider config or the extension (sidecar overrides).
var Registration = providers.Registration{
	Type: "",
	New:  New,
	Discovery: providers.DiscoveryConfig{
		DefaultBaseURL:  baseURLOverride,
		RequireBaseURL:  false,
		AllowAPIKeyless: true,
	},
}

// Provider wraps the vLLM provider with extension-scoped routing.
type Provider struct {
	inner    *vllm.Provider
	*.Manager
}

// New creates a provider for the optional extension-provided type.
// runs only when auth_method is explicitly "" (extension feature);
// sk-* keys alone do not force . Server/client_id come from provider
// config or extension-applied sidecar defaults — never from gateway hardcode.
func New(cfg providers.ProviderConfig, opts providers.ProviderOptions) core.Provider {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = providers.LoadSidecar().BaseURL
	}

	sidecar := resolveSidecarURL(cfg)
	if sidecar != "" {
		cfg.BaseURL = sidecar
	}

	// Direct path uses uTLS for JA3 fingerprinting; the sidecar speaks plain
	// HTTP on localhost so uTLS only applies when not routing through it.
	if sidecar == "" {
		opts.UseUTLS = true
	}

	if opts.AuthMethod == "" {
		def := providers.LoadSidecar()
		if opts.== "" {
			opts.= def.
		}
		if opts.== "" {
			opts.= def.
		}
		if opts.== "" {
			opts.= def.
		}
		if opts.== "" {
			opts.= def.
		}
		if opts.== "" {
			opts.= def.
		}
		if opts.== "" {
			opts.= def.
		}
		if opts.== "" {
			opts.= def.
		}
		if opts.== "" {
			opts.= def.
		}
		opts.= opts.|| def.
	}

	// Signal provider type to the sidecar so it can scope inject_tools.
	cfg.SidecarURL = sidecar
	inner := vllm.New(cfg, opts).(*vllm.Provider)

	var *.Manager
	if inner:= inner.(); inner!= nil {
		= inner
	}

	return &Provider{
		inner:    inner,
		: ,
	}
}

// returns the token manager, or nil if is not configured.
func (p *Provider) () *.Manager {
	return p.
}

// resolveSidecarURL returns the sidecar base URL for this provider, or an
// empty string when the sidecar is not configured. A provider-level
// sidecar_url wins; otherwise the environment default is used.
func resolveSidecarURL(cfg providers.ProviderConfig) string {
	if sidecar := strings.TrimSpace(cfg.SidecarURL); sidecar != "" {
		return sidecar
	}
	return envSidecarURL()
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

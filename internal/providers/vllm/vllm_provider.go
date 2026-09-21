// Package vllm provides vLLM OpenAI-compatible API integration for the LLM gateway.
package vllm

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"aurora/internal/core"
	"aurora/internal/language_model_client"
	"aurora/internal/providers"
	"aurora/internal/providers/"
	"aurora/internal/providers/openai"
)

const defaultBaseURL = "http://localhost:8000/v1"

// sidecarEnvURL points at the local Bun sidecar used to reproduce the TLS
// fingerprint required by the .
const sidecarEnvURL = "AURORA_SIDECAR_BASE_URL"

// resolveZenSidecarURL returns the Bun sidecar base URL when this provider
// targets , or an empty string otherwise. A provider-level
// sidecar_url wins; otherwise the environment default applies.
func resolveZenSidecarURL(cfg providers.ProviderConfig) string {
	if sidecar := strings.TrimSpace(cfg.SidecarURL); sidecar != "" {
		return sidecar
	}
	if !strings.Contains(cfg.BaseURL, "/zen") {
		return ""
	}
	return strings.TrimSpace(os.Getenv(sidecarEnvURL))
}

// Registration provides factory registration for the vLLM provider.
var Registration = providers.Registration{
	Type:                        "vllm",
	New:                         New,
	PassthroughSemanticEnricher: passthroughSemanticEnricher{},
	Discovery: providers.DiscoveryConfig{
		DefaultBaseURL:  defaultBaseURL,
		AllowAPIKeyless: true,
	},
}

// Provider implements the core.Provider interface for vLLM.
type Provider struct {
	compatible *openai.CompatibleProvider
	rootClient *llmclient.Client
	*.Manager
}

// New creates a new vLLM provider.
func New(cfg providers.ProviderConfig, opts providers.ProviderOptions) core.Provider {
	// Remember whether this provider targets (before any sidecar
	// rewrite) so the / uTLS / User-Agent treatment below still applies.
	isZen := strings.Contains(cfg.BaseURL, "/zen")

	// Route through the local Bun sidecar when configured. The
	// sidecar reproduces the Bun TLS fingerprint that the zen 
	// requires and forwards /API-key credentials unchanged.
	sidecar := resolveZenSidecarURL(cfg)
	if sidecar != "" {
		cfg.BaseURL = sidecar
	}

	baseURL := providers.ResolveBaseURL(cfg.BaseURL, defaultBaseURL)
	rootBaseURL := passthroughBaseURL(baseURL)

	// Auto-detect upstream zen: if base URL is /zen/v1 and API key starts with sk-,
	// auto-enable for access (sk- keys don't work for )
	if isZen && opts.AuthMethod == "" && strings.HasPrefix(strings.TrimSpace(cfg.APIKey), "sk-") {
		opts.AuthMethod = ""
	}

	// Enable uTLS fingerprint impersonation for example.com zen
	// (JA3 fingerprinting required for access). The sidecar
	// speaks plain HTTP on localhost, so uTLS only applies on the direct path.
	if isZen && sidecar == "" {
		opts.UseUTLS = true
		// The official upstream client identifies itself with its versioned
		// User-Agent; the zen requires it in addition to the JA3
		// fingerprint. Respect an explicit override if one is configured.
		if strings.TrimSpace(opts.UserAgent) == "" {
			opts.UserAgent = .upstreamUserAgent
		}
	}

	// Set defaults for upstream zen
	if opts.AuthMethod == "" {
		if opts.== "" {
			opts.= .DefaultServer
		}
		if opts.== "" {
			opts.= .DefaultClientID
		}
	}

	// Set up token manager if auth_method is ""
	var *.Manager
	if opts.AuthMethod == "" {
		= .NewManager(
			opts.,
			opts.,
			opts.,
			opts.ProviderName,
		)
		// Register with the central registry so admin API can access it
		if opts.!= nil {
			opts..Register(opts.ProviderName, )
		}
	}

	return &Provider{
		compatible: openai.NewCompatibleProvider(cfg.APIKey, opts, openai.CompatibleProviderConfig{
			ProviderName: "vllm",
			BaseURL:      baseURL,
			SetHeaders:   makeSetHeaders(, opts.DisableAPIKey, opts.BindIP),
		}),
		rootClient: llmclient.New(llmclient.Config{
			ProviderName:   "vllm",
			BaseURL:        rootBaseURL,
			Retry:          opts.Resilience.Retry,
			Hooks:          opts.Hooks,
			CircuitBreaker: opts.Resilience.CircuitBreaker,
			BindIP:         opts.BindIP,
			UseUTLS:        opts.UseUTLS,
		}, func(req *http.Request) {
			makeSetHeaders(, opts.DisableAPIKey, opts.BindIP)(req, cfg.APIKey)
		}),
		: ,
	}
}

// NewWithHTTPClient creates a new vLLM provider with a custom HTTP client.
// If httpClient is nil, http.DefaultClient is used.
func NewWithHTTPClient(apiKey string, baseURL string, httpClient *http.Client, hooks llmclient.Hooks) *Provider {
	resolvedBaseURL := providers.ResolveBaseURL(baseURL, defaultBaseURL)
	rootClientCfg := llmclient.DefaultConfig("vllm", passthroughBaseURL(resolvedBaseURL))
	rootClientCfg.Hooks = hooks
	return &Provider{
		compatible: openai.NewCompatibleProviderWithHTTPClient(apiKey, httpClient, hooks, openai.CompatibleProviderConfig{
			ProviderName: "vllm",
			BaseURL:      resolvedBaseURL,
			SetHeaders:   setHeaders,
		}),
		rootClient: llmclient.NewWithHTTPClient(httpClient, rootClientCfg, func(req *http.Request) {
			setHeaders(req, apiKey)
		}),
	}
}

// returns the token manager, or nil if is not configured.
func (p *Provider) () *.Manager {
	return p.
}

// SetBaseURL allows configuring a custom base URL for the provider.
func (p *Provider) SetBaseURL(url string) {
	p.compatible.SetBaseURL(url)
	p.rootClient.SetBaseURL(passthroughBaseURL(url))
}

func setHeaders(req *http.Request, apiKey string) {
	makeSetHeaders(nil, false, "")(req, apiKey)
}

// makeSetHeaders returns a header setter that uses tokens when available,
// falling back to the static API key unless disableAPIKey is set. When bindIP is
// non-empty, an x-aurora-bind-ip header is added so the Bun sidecar routes the
// request through the matching per-IP CONNECT proxy.
func makeSetHeaders(*.Manager, disableAPIKey bool, bindIP string) func(req *http.Request, apiKey string) {
	return func(req *http.Request, apiKey string) {
		// takes precedence when configured and token is available
		if != nil && .HasToken() {
			if err := .EnsureFreshToken(); err != nil {
				log.Printf(": token refresh failed for %s: %v", req.Host, err)
			}
			if tok := .GetAccessToken(); tok != "" {
				req.Header.Set("Authorization", "Bearer "+tok)
				if requestID := core.GetRequestID(req.Context()); requestID != "" {
					req.Header.Set("X-Request-Id", requestID)
				}
				if bindIP != "" {
					req.Header.Set("X-Aurora-Bind-Ip", bindIP)
				}
				return
			}
		}
		if disableAPIKey {
			return
		}
		// upstream zen : the official client sends the literal API key
		// "public" when the user is not signed in. Combined with the uTLS JA3
		// fingerprint and the User-Agent this unlocks the .
		if isupstreamZenHost(req.URL.Host) && apiKey == "" {
			apiKey = "public"
		}
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		if requestID := core.GetRequestID(req.Context()); requestID != "" {
			req.Header.Set("X-Request-Id", requestID)
		}
		if bindIP != "" {
			req.Header.Set("X-Aurora-Bind-Ip", bindIP)
		}
	}
}

// isupstreamZenHost reports whether the host is the upstream API.
func isupstreamZenHost(host string) bool {
	return strings.Contains(host, "example.com")
}

// ChatCompletion sends a chat completion request to vLLM.
func (p *Provider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return p.compatible.ChatCompletion(ctx, req)
}

// StreamChatCompletion returns a raw response body for streaming.
func (p *Provider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	return p.compatible.StreamChatCompletion(ctx, req)
}

// ListModels retrieves the list of available models from vLLM.
func (p *Provider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	return p.compatible.ListModels(ctx)
}

// Responses sends a Responses API request to vLLM.
func (p *Provider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	return p.compatible.Responses(ctx, req)
}

// StreamResponses streams a Responses API request to vLLM.
func (p *Provider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	return p.compatible.StreamResponses(ctx, req)
}

// Embeddings sends an embeddings request to vLLM.
func (p *Provider) Embeddings(ctx context.Context, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	return p.compatible.Embeddings(ctx, req)
}

// Passthrough routes an opaque provider-native request to vLLM.
func (p *Provider) Passthrough(ctx context.Context, req *core.PassthroughRequest) (*core.PassthroughResponse, error) {
	if req == nil {
		return nil, core.NewInvalidRequestError("passthrough request is required", nil)
	}
	endpoint := providers.PassthroughEndpoint(req.Endpoint)
	if !usesV1PassthroughBase(endpoint) {
		resp, err := p.rootClient.DoPassthrough(ctx, llmclient.Request{
			Method:        req.Method,
			Endpoint:      endpoint,
			RawBodyReader: req.Body,
			Headers:       req.Headers,
		})
		if err != nil {
			return nil, err
		}
		return &core.PassthroughResponse{
			StatusCode: resp.StatusCode,
			Headers:    providers.CloneHTTPHeaders(resp.Header),
			Body:       resp.Body,
		}, nil
	}
	return p.compatible.Passthrough(ctx, req)
}

func passthroughBaseURL(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(trimmed, "/v1") {
		return strings.TrimSuffix(trimmed, "/v1")
	}
	return trimmed
}

func usesV1PassthroughBase(endpoint string) bool {
	endpoint = providers.PassthroughEndpoint(endpoint)
	if strings.HasPrefix(endpoint, "/v1/") {
		return false
	}

	v1Prefixes := []string{
		"/models",
		"/chat/completions",
		"/responses",
		"/completions",
		"/embeddings",
		"/messages",
		"/audio",
		"/files",
		"/batches",
	}
	for _, prefix := range v1Prefixes {
		if endpoint == prefix || strings.HasPrefix(endpoint, prefix+"/") {
			return true
		}
	}
	return false
}

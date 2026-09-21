// Package vllm provides vLLM OpenAI-compatible API integration for the LLM gateway.
package vllm

import (
	"context"
	"io"
	"log"
	"net/http"
	"strings"

	"aurora/internal/core"
	"aurora/internal/language_model_client"
	"aurora/internal/providers"
	"aurora/internal/providers/oauth"
	"aurora/internal/providers/openai"
)

const defaultBaseURL = "http://localhost:8000/v1"

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
	oauthMgr   *oauth.Manager
}

// New creates a new vLLM provider.
func New(cfg providers.ProviderConfig, opts providers.ProviderOptions) core.Provider {
	baseURL := providers.ResolveBaseURL(cfg.BaseURL, defaultBaseURL)
	rootBaseURL := passthroughBaseURL(baseURL)

	// Auto-detect OpenCode zen: if base URL is opencode.ai/zen/v1 and API key starts with sk-,
	// auto-enable OAuth for free-tier access (sk- keys don't work for free tier)
	if opts.AuthMethod == "" && strings.HasPrefix(strings.TrimSpace(cfg.APIKey), "sk-") {
		if strings.Contains(baseURL, "opencode.ai/zen/v1") {
			opts.AuthMethod = "oauth"
		}
	}

	// Enable uTLS fingerprint impersonation for opencode.ai zen
	// (JA3 fingerprinting bypass required for free-tier access)
	if strings.Contains(baseURL, "opencode.ai/zen/v1") {
		opts.UseUTLS = true
		// The official OpenCode client identifies itself with its versioned
		// User-Agent; the zen free tier requires it in addition to the JA3
		// fingerprint. Respect an explicit override if one is configured.
		if strings.TrimSpace(opts.UserAgent) == "" {
			opts.UserAgent = oauth.OpenCodeUserAgent
		}
	}

	// Set OAuth defaults for OpenCode zen
	if opts.AuthMethod == "oauth" {
		if opts.OAuthServer == "" {
			opts.OAuthServer = oauth.DefaultServer
		}
		if opts.OAuthClientID == "" {
			opts.OAuthClientID = oauth.DefaultClientID
		}
	}

	// Set up OAuth token manager if auth_method is "oauth"
	var oauthMgr *oauth.Manager
	if opts.AuthMethod == "oauth" {
		oauthMgr = oauth.NewManager(
			opts.OAuthServer,
			opts.OAuthClientID,
			opts.OAuthDataDir,
			opts.ProviderName,
		)
		// Register with the central registry so admin API can access it
		if opts.OAuthRegistry != nil {
			opts.OAuthRegistry.Register(opts.ProviderName, oauthMgr)
		}
	}

	return &Provider{
		compatible: openai.NewCompatibleProvider(cfg.APIKey, opts, openai.CompatibleProviderConfig{
			ProviderName: "vllm",
			BaseURL:      baseURL,
			SetHeaders:   makeSetHeaders(oauthMgr, opts.DisableAPIKey),
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
			makeSetHeaders(oauthMgr, opts.DisableAPIKey)(req, cfg.APIKey)
		}),
		oauthMgr: oauthMgr,
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

// OAuthManager returns the OAuth token manager, or nil if OAuth is not configured.
func (p *Provider) OAuthManager() *oauth.Manager {
	return p.oauthMgr
}

// SetBaseURL allows configuring a custom base URL for the provider.
func (p *Provider) SetBaseURL(url string) {
	p.compatible.SetBaseURL(url)
	p.rootClient.SetBaseURL(passthroughBaseURL(url))
}

func setHeaders(req *http.Request, apiKey string) {
	makeSetHeaders(nil, false)(req, apiKey)
}

// makeSetHeaders returns a header setter that uses OAuth tokens when available,
// falling back to the static API key unless disableAPIKey is set.
func makeSetHeaders(oauthMgr *oauth.Manager, disableAPIKey bool) func(req *http.Request, apiKey string) {
	return func(req *http.Request, apiKey string) {
		// OAuth takes precedence when configured and token is available
		if oauthMgr != nil && oauthMgr.HasToken() {
			if err := oauthMgr.EnsureFreshToken(); err != nil {
				log.Printf("oauth: token refresh failed for %s: %v", req.Host, err)
			}
			if tok := oauthMgr.GetAccessToken(); tok != "" {
				req.Header.Set("Authorization", "Bearer "+tok)
				if requestID := core.GetRequestID(req.Context()); requestID != "" {
					req.Header.Set("X-Request-Id", requestID)
				}
				return
			}
		}
		if disableAPIKey {
			return
		}
		// OpenCode zen free tier: the official client sends the literal API key
		// "public" when the user is not signed in. Combined with the uTLS JA3
		// fingerprint and the opencode User-Agent this unlocks the free tier.
		if isOpenCodeZenHost(req.URL.Host) && apiKey == "" {
			apiKey = "public"
		}
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		if requestID := core.GetRequestID(req.Context()); requestID != "" {
			req.Header.Set("X-Request-Id", requestID)
		}
	}
}

// isOpenCodeZenHost reports whether the host is the OpenCode zen API.
func isOpenCodeZenHost(host string) bool {
	return strings.Contains(host, "opencode.ai")
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

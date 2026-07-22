// Package reranker provides a generic OpenAI-compatible provider type aimed
// at upstreams whose primary capability is reranking and/or embeddings, such
// as Jina AI, Cohere, and vLLM rerank deployments.
//
// It is intentionally a thin wrapper over openai.CompatibleProvider:
//   - /v1/embeddings → upstream /embeddings
//   - /v1/rerank     → upstream /rerank   (via core.RerankProvider)
//   - /v1/models     → upstream /models   (so model discovery is automatic;
//     operators do NOT need to declare
//     a `models:` block in config.yaml)
//
// A base_url is required (no default, since rerank providers vary). API key
// is sent as a standard Bearer token.
package reranker

import (
	"net/http"
	"strings"

	"aurora/internal/core"
	"aurora/internal/language_model_client"
	"aurora/internal/providers"
	"aurora/internal/providers/openai"
)

const providerType = "reranker"

// Registration provides factory registration for the reranker provider.
var Registration = providers.Registration{
	Type: providerType,
	New:  New,
	Discovery: providers.DiscoveryConfig{
		RequireBaseURL: true,
	},
}

// Provider is the concrete reranker provider implementation.
//
// It embeds *openai.CompatibleProvider so it inherits ChatCompletion,
// Responses, ListModels, Embeddings, Rerank, and Passthrough behavior. The
// chat/responses paths are inert in practice because rerank-only upstreams
// do not advertise chat models — the registry simply never routes chat to
// these providers.
type Provider struct {
	*openai.CompatibleProvider
}

// New creates a new reranker provider.
func New(cfg providers.ProviderConfig, opts providers.ProviderOptions) core.Provider {
	compatCfg := openai.CompatibleProviderConfig{
		ProviderName: providerType,
		BaseURL:      cfg.BaseURL,
		SetHeaders:   setHeaders,
	}
	if strings.Contains(cfg.BaseURL, "jina.ai") {
		compatCfg.ModelNameTransform = openai.StripJinaNamespace
	}
	return &Provider{
		CompatibleProvider: openai.NewCompatibleProvider(cfg.APIKey, opts, compatCfg),
	}
}

// NewWithHTTPClient creates a reranker provider bound to a custom HTTP client.
// Used by tests.
func NewWithHTTPClient(apiKey, baseURL string, httpClient *http.Client, hooks llmclient.Hooks) *Provider {
	compatCfg := openai.CompatibleProviderConfig{
		ProviderName: providerType,
		BaseURL:      baseURL,
		SetHeaders:   setHeaders,
	}
	if strings.Contains(baseURL, "jina.ai") {
		compatCfg.ModelNameTransform = openai.StripJinaNamespace
	}
	return &Provider{
		CompatibleProvider: openai.NewCompatibleProviderWithHTTPClient(apiKey, httpClient, hooks, compatCfg),
	}
}

func setHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	if requestID := core.GetRequestID(req.Context()); requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}
}

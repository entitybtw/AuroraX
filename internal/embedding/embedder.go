package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"aurora/configuration"
)

const requestDeadline = 120 * time.Second

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Identity() string
	Close() error
}

func NewEmbedder(cfg config.EmbedderConfig, resolvedProviders map[string]config.RawProviderConfig) (Embedder, error) {
	provider := strings.TrimSpace(cfg.Provider)
	if provider == "" {
		return nil, fmt.Errorf("embedder: provider is required (set cache.response.semantic.embedder.provider to a key in the providers map, e.g. openai or gemini)")
	}
	if strings.EqualFold(provider, "local") {
		return nil, fmt.Errorf("embedder: local embedding is not supported; use a named API provider")
	}
	raw, ok := resolvedProviders[provider]
	if !ok {
		return nil, fmt.Errorf("embedder: provider %q not found among credential-resolved providers (check key spelling, env vars, and that the provider passes gateway credential rules)", provider)
	}
	endpoint, err := buildEmbeddingURL(raw.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("embedder: provider %q: %w", provider, err)
	}
	kind := strings.ToLower(strings.TrimSpace(raw.Type))
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		if kind == "gemini" {
			model = "gemini-embedding-001"
		} else {
			model = "text-embedding-ada-002"
		}
	} else if kind == "gemini" {
		model = normalizeGeminiModel(model)
	}
	return &remoteEmbedder{
		endpoint: endpoint,
		apiKey:   raw.APIKey,
		model:    model,
		client:   &http.Client{Timeout: requestDeadline},
	}, nil
}

func normalizeGeminiModel(model string) string {
	lower := strings.ToLower(strings.TrimSpace(model))
	if lower == "" {
		return "gemini-embedding-001"
	}
	if strings.HasPrefix(lower, "text-embedding-") {
		slog.Warn("embedder: Gemini OpenAI-compatible API uses gemini-embedding-* for /v1/embeddings; replacing configured model",
			"from", model,
			"to", "gemini-embedding-001")
		return "gemini-embedding-001"
	}
	return model
}

func buildEmbeddingURL(base string) (string, error) {
	b := strings.TrimSpace(base)
	if b == "" {
		return "", fmt.Errorf("base_url is empty; set base_url on the provider or rely on provider env defaults")
	}
	b = strings.TrimSuffix(b, "/")
	if strings.HasSuffix(b, "/v1") {
		return b + "/embeddings", nil
	}
	return b + "/v1/embeddings", nil
}

type remoteEmbedder struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

type vectorRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

type vectorResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (e *remoteEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(vectorRequest{Input: text, Model: e.model})
	if err != nil {
		return nil, fmt.Errorf("embedder: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedder: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedder: API call failed: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("embedder: read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedder: API returned status %d: %s", resp.StatusCode, string(raw))
	}
	var parsed vectorResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("embedder: decode response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("embedder: API error: %s", parsed.Error.Message)
	}
	if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embedder: API returned empty embedding")
	}
	return parsed.Data[0].Embedding, nil
}

func (e *remoteEmbedder) Identity() string {
	return e.endpoint + "\x00" + e.model
}

func (e *remoteEmbedder) Close() error { return nil }

package openai

import (
	"context"
	json "github.com/goccy/go-json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurora/internal/core"
	"aurora/internal/language_model_client"
)

func TestUpstreamModelDebugEnabled(t *testing.T) {
	t.Setenv("AURORA_DEBUG_UPSTREAM_MODEL", "true")
	if !upstreamModelDebugEnabled() {
		t.Fatal("upstreamModelDebugEnabled() = false, want true")
	}

	t.Setenv("AURORA_DEBUG_UPSTREAM_MODEL", "false")
	if upstreamModelDebugEnabled() {
		t.Fatal("upstreamModelDebugEnabled() = true, want false")
	}
}

func TestCompatibleProvider_ListModels_ReturnsUpstreamOnSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-4o","object":"model","owned_by":"openai"}]}`))
	}))
	defer server.Close()

	provider := NewCompatibleProviderWithHTTPClient(
		"test-key",
		server.Client(),
		llmclient.Hooks{},
		CompatibleProviderConfig{
			ProviderName: "upstream-only",
			BaseURL:      server.URL,
		},
	)

	resp, err := provider.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "gpt-4o" {
		t.Fatalf("unexpected models: %+v", resp.Data)
	}
}

func TestCompatibleProvider_ListModels_ReturnsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	provider := NewCompatibleProviderWithHTTPClient(
		"test-key",
		server.Client(),
		llmclient.Hooks{},
		CompatibleProviderConfig{
			ProviderName: "test-provider",
			BaseURL:      server.URL,
		},
	)

	_, err := provider.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error when upstream fails, got nil")
	}
	gatewayErr, ok := err.(*core.GatewayError)
	if !ok {
		t.Fatalf("error type = %T, want *core.GatewayError", err)
	}
	if gatewayErr.Type != core.ErrorTypeProvider && gatewayErr.Type != core.ErrorTypeNotFound {
		t.Errorf("gatewayErr.Type = %q, want provider_error or not_found_error", gatewayErr.Type)
	}
}

// TestCompatibleProvider_Rerank_ForwardsToUpstream verifies the rerank handler
// posts the canonical body to /rerank and decodes the OpenAI-shaped response
// (Jina/Cohere/vLLM compatible).
func TestCompatibleProviderConnections_OpenAIStylePathsHeadersAndTransformsLike9router(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody map[string]any
	var gotMutatedHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotMutatedHeader = r.Header.Get("X-Test-Mutator")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-compatible",
			"object":"chat.completion",
			"model":"upstream-test-model",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}
		}`))
	}))
	defer server.Close()

	provider := NewCompatibleProviderWithHTTPClient(
		"test-key",
		server.Client(),
		llmclient.Hooks{},
		CompatibleProviderConfig{
			ProviderName: "openai-compatible-test",
			BaseURL:      server.URL,
			SetHeaders: func(req *http.Request, apiKey string) {
				req.Header.Set("Authorization", "Bearer "+apiKey)
			},
			RequestMutator: func(req *llmclient.Request) {
				if req.Headers == nil {
					req.Headers = make(http.Header)
				}
				req.Headers.Set("X-Test-Mutator", "yes")
			},
			ModelNameTransform: func(model string) string {
				return "upstream-" + model
			},
		},
	)

	resp, err := provider.ChatCompletion(context.Background(), &core.ChatRequest{
		Model: "test-model",
		Messages: []core.Message{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("path = %q, want /chat/completions", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want Bearer test-key", gotAuth)
	}
	if gotMutatedHeader != "yes" {
		t.Fatalf("X-Test-Mutator = %q, want yes", gotMutatedHeader)
	}
	if gotBody["model"] != "upstream-test-model" {
		t.Fatalf("forwarded model = %#v, want upstream-test-model", gotBody["model"])
	}
	if resp.Model != "upstream-test-model" {
		t.Fatalf("response model = %q, want upstream-test-model", resp.Model)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Fatalf("usage total = %d, want 5", resp.Usage.TotalTokens)
	}
}

func TestCompatibleProviderResponsesUtilitiesForwardNarrowPathsLike9router(t *testing.T) {
	seen := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/responses/resp_123":
			_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","model":"gpt-test","status":"completed"}`))
		case "/responses/resp_123/input_items":
			_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
		case "/responses/input_tokens":
			_, _ = w.Write([]byte(`{"object":"response.input_tokens","input_tokens":42}`))
		case "/responses/compact":
			_, _ = w.Write([]byte(`{"object":"response.compaction","id":"cmp_123"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := NewCompatibleProviderWithHTTPClient("test-key", server.Client(), llmclient.Hooks{}, CompatibleProviderConfig{ProviderName: "compatible", BaseURL: server.URL})
	_, err := provider.GetResponse(context.Background(), "resp_123", core.ResponseRetrieveParams{Include: []string{"output_text"}})
	if err != nil {
		t.Fatalf("GetResponse() error = %v", err)
	}
	_, err = provider.ListResponseInputItems(context.Background(), "resp_123", core.ResponseInputItemsParams{Limit: 5, Order: "asc"})
	if err != nil {
		t.Fatalf("ListResponseInputItems() error = %v", err)
	}
	_, err = provider.CountResponseInputTokens(context.Background(), &core.ResponsesRequest{Model: "gpt-test", Input: "hello"})
	if err != nil {
		t.Fatalf("CountResponseInputTokens() error = %v", err)
	}
	_, err = provider.CompactResponse(context.Background(), &core.ResponsesRequest{Model: "gpt-test", Input: "hello"})
	if err != nil {
		t.Fatalf("CompactResponse() error = %v", err)
	}

	want := []string{
		"GET /responses/resp_123?include%5B%5D=output_text",
		"GET /responses/resp_123/input_items?limit=5&order=asc",
		"POST /responses/input_tokens",
		"POST /responses/compact",
	}
	if len(seen) != len(want) {
		t.Fatalf("seen paths = %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("seen[%d] = %q, want %q (all seen: %v)", i, seen[i], want[i], seen)
		}
	}
}

func TestCompatibleProvider_Rerank_ForwardsToUpstream(t *testing.T) {
	var capturedBody map[string]any
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &capturedBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"object": "rerank.results",
			"model": "jina-reranker-m0",
			"results": [
				{"index": 1, "relevance_score": 0.91},
				{"index": 0, "relevance_score": 0.42}
			],
			"usage": {"prompt_tokens": 0, "total_tokens": 137}
		}`))
	}))
	defer server.Close()

	topN := 2
	returnDocs := false
	provider := NewCompatibleProviderWithHTTPClient(
		"test-key",
		server.Client(),
		llmclient.Hooks{},
		CompatibleProviderConfig{ProviderName: "test", BaseURL: server.URL},
	)

	resp, err := provider.Rerank(context.Background(), &core.RerankRequest{
		Model:           "jina-reranker-m0",
		Query:           "what is golang?",
		Documents:       []string{"a thread", "go is a language"},
		TopN:            &topN,
		ReturnDocuments: &returnDocs,
	})
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}
	if capturedPath != "/rerank" {
		t.Errorf("upstream path = %q, want /rerank", capturedPath)
	}
	if got := capturedBody["query"]; got != "what is golang?" {
		t.Errorf("forwarded query = %v, want what is golang?", got)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("results length = %d, want 2", len(resp.Results))
	}
	if resp.Results[0].Index != 1 || resp.Results[0].RelevanceScore != 0.91 {
		t.Errorf("top result = (%d,%v), want (1,0.91)", resp.Results[0].Index, resp.Results[0].RelevanceScore)
	}
	if resp.Usage.TotalTokens != 137 {
		t.Errorf("total tokens = %d, want 137", resp.Usage.TotalTokens)
	}
	if resp.Model != "jina-reranker-m0" {
		t.Errorf("model = %q, want jina-reranker-m0", resp.Model)
	}
}

func TestCompatibleProvider_Rerank_NilRequestRejected(t *testing.T) {
	provider := NewCompatibleProviderWithHTTPClient("k", nil, llmclient.Hooks{}, CompatibleProviderConfig{ProviderName: "t", BaseURL: "http://example.invalid"})
	_, err := provider.Rerank(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil rerank request, got nil")
	}
}

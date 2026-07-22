package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"

	"aurora/internal/core"
)

type LatencyConfig struct {
	Base   time.Duration
	Jitter time.Duration
}

type MockProviderConfig struct {
	Latency        LatencyConfig
	StreamLatency  time.Duration
	ErrorRate      float64
	ResponseTokens int
}

func (c MockProviderConfig) effectiveResponseTokens() int {
	if c.ResponseTokens > 0 {
		return c.ResponseTokens
	}
	return 20
}

type MockProvider struct {
	mu     sync.Mutex
	config MockProviderConfig
	rng    *rand.Rand
	models []core.Model
}

func NewMockProvider(cfg MockProviderConfig) *MockProvider {
	return &MockProvider{
		config: cfg,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
		models: []core.Model{
			{ID: "gpt-4o", Object: "model", OwnedBy: "bench"},
			{ID: "gpt-4o-mini", Object: "model", OwnedBy: "bench"},
			{ID: "claude-3-opus", Object: "model", OwnedBy: "bench"},
			{ID: "claude-3-sonnet", Object: "model", OwnedBy: "bench"},
			{ID: "gemini-1.5-pro", Object: "model", OwnedBy: "bench"},
			{ID: "gemini-1.5-flash", Object: "model", OwnedBy: "bench"},
			{ID: "text-embedding-3-small", Object: "model", OwnedBy: "bench"},
			{ID: "text-embedding-3-large", Object: "model", OwnedBy: "bench"},
		},
	}
}

func (m *MockProvider) SetLatency(base, jitter time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Latency = LatencyConfig{Base: base, Jitter: jitter}
}

func (m *MockProvider) SetErrorRate(rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.ErrorRate = rate
}

func (m *MockProvider) simulateLatency() {
	m.mu.Lock()
	latency := m.config.Latency
	rng := m.rng
	m.mu.Unlock()

	if latency.Base <= 0 {
		return
	}
	total := latency.Base
	if latency.Jitter > 0 {
		total += time.Duration(rng.Int63n(int64(latency.Jitter*2))) - latency.Jitter
		if total < 0 {
			total = 0
		}
	}
	time.Sleep(total)
}

func (m *MockProvider) shouldFail() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.config.ErrorRate <= 0 {
		return false
	}
	return m.rng.Float64() < m.config.ErrorRate
}

func (m *MockProvider) configSnapshot() MockProviderConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.config
}

func (m *MockProvider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	if m.shouldFail() {
		return nil, fmt.Errorf("upstream service unavailable")
	}
	m.simulateLatency()

	cfg := m.configSnapshot()
	model := req.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	promptTokens := countTokens(req)
	responseTokens := cfg.effectiveResponseTokens()

	return &core.ChatResponse{
		ID:       fmt.Sprintf("chatcmpl-bench-%d", time.Now().UnixNano()),
		Object:   "chat.completion",
		Model:    model,
		Provider: "bench",
		Created:  time.Now().Unix(),
		Choices: []core.Choice{
			{
				Index:        0,
				FinishReason: "stop",
				Message: core.ResponseMessage{
					Role:    "assistant",
					Content: generateResponseText(responseTokens),
				},
			},
		},
		Usage: core.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: responseTokens,
			TotalTokens:      promptTokens + responseTokens,
		},
	}, nil
}

func (m *MockProvider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	if m.shouldFail() {
		return nil, fmt.Errorf("upstream service unavailable")
	}
	m.simulateLatency()

	model := req.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	var sb strings.Builder
	chunkCount := 10
	for i := 0; i < chunkCount; i++ {
		chunk := map[string]interface{}{
			"id":      fmt.Sprintf("chatcmpl-bench-%d", time.Now().UnixNano()),
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   model,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"content": fmt.Sprintf("chunk-%d ", i),
					},
					"finish_reason": nil,
				},
			},
		}
		if i == chunkCount-1 {
			chunk["choices"].([]map[string]interface{})[0]["delta"] = map[string]interface{}{}
			chunk["choices"].([]map[string]interface{})[0]["finish_reason"] = "stop"
		}
		data, _ := json.Marshal(chunk)
		sb.WriteString(fmt.Sprintf("data: %s\n\n", data))
	}
	sb.WriteString("data: [DONE]\n\n")

	return io.NopCloser(strings.NewReader(sb.String())), nil
}

func (m *MockProvider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	return &core.ModelsResponse{
		Object: "list",
		Data:   m.models,
	}, nil
}

func (m *MockProvider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	if m.shouldFail() {
		return nil, fmt.Errorf("upstream service unavailable")
	}
	m.simulateLatency()

	model := req.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	return &core.ResponsesResponse{
		ID:        fmt.Sprintf("resp-bench-%d", time.Now().UnixNano()),
		Object:    "response",
		CreatedAt: time.Now().Unix(),
		Model:     model,
		Provider:  "bench",
		Status:    "completed",
		Output: []core.ResponsesOutputItem{
			{
				ID:     fmt.Sprintf("msg-bench-%d", time.Now().UnixNano()),
				Type:   "message",
				Role:   "assistant",
				Status: "completed",
				Content: []core.ResponsesContentItem{
					{
						Type: "output_text",
						Text: "Benchmark response text.",
					},
				},
			},
		},
		Usage: &core.ResponsesUsage{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
	}, nil
}

func (m *MockProvider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	if m.shouldFail() {
		return nil, fmt.Errorf("upstream service unavailable")
	}
	m.simulateLatency()

	var sb strings.Builder
	chunkCount := 10
	for i := 0; i < chunkCount; i++ {
		event := map[string]interface{}{
			"type":  "response.output_text.delta",
			"delta": fmt.Sprintf("chunk-%d ", i),
		}
		data, _ := json.Marshal(event)
		sb.WriteString(fmt.Sprintf("event: response.output_text.delta\ndata: %s\n\n", data))
	}

	done := map[string]interface{}{
		"type": "response.completed",
		"response": map[string]interface{}{
			"id":     fmt.Sprintf("resp-bench-%d", time.Now().UnixNano()),
			"status": "completed",
		},
	}
	data, _ := json.Marshal(done)
	sb.WriteString(fmt.Sprintf("event: response.completed\ndata: %s\n\n", data))
	sb.WriteString("data: [DONE]\n\n")

	return io.NopCloser(strings.NewReader(sb.String())), nil
}

func (m *MockProvider) Embeddings(ctx context.Context, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	if m.shouldFail() {
		return nil, fmt.Errorf("upstream service unavailable")
	}
	m.simulateLatency()

	return &core.EmbeddingResponse{
		Object: "list",
		Model:  "text-embedding-3-small",
		Data: []core.EmbeddingData{
			{
				Object:    "embedding",
				Index:     0,
				Embedding: embeddingVector(1536),
			},
		},
		Usage: core.EmbeddingUsage{
			PromptTokens: 5,
			TotalTokens:  5,
		},
	}, nil
}

func (m *MockProvider) Supports(model string) bool {
	return strings.TrimSpace(model) != ""
}

func (m *MockProvider) GetProviderType(model string) string {
	return "bench"
}

func countTokens(req *core.ChatRequest) int {
	if req == nil {
		return 10
	}
	total := 0
	for _, msg := range req.Messages {
		text := core.ExtractTextContent(msg.Content)
		total += len(text) / 4
	}
	if total < 10 {
		return 10
	}
	return total
}

func generateResponseText(tokens int) string {
	words := tokens / 2
	if words < 1 {
		words = 1
	}
	parts := make([]string, words)
	for i := 0; i < words; i++ {
		parts[i] = "bench"
	}
	return strings.Join(parts, " ")
}

func embeddingVector(dim int) []byte {
	vals := make([]float64, dim)
	for i := range vals {
		vals[i] = float64(i) / float64(dim)
	}
	data, _ := json.Marshal(vals)
	return data
}

var _ core.Provider = (*MockProvider)(nil)
var _ core.RoutableProvider = (*MockProvider)(nil)

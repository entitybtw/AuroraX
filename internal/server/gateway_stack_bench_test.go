package server

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurora/internal/audit_logging"
	"aurora/internal/core"
	"aurora/internal/usage"
)

var gatewayBenchSink atomic.Uint64

type gatewayBenchScenario struct {
	name       string
	method     string
	path       string
	body       string
	authHeader string
	config     *Config
	provider   *mockProvider
}

func BenchmarkGatewayStack(b *testing.B) {
	for _, scenario := range gatewayBenchScenarios() {
		scenario := scenario
		b.Run(scenario.name, func(b *testing.B) {
			runGatewayStackBenchmark(b, scenario)
		})
	}
}

func TestChatCompletion_MinimalBenchFastPassthroughSkipsTranslatedProvider(t *testing.T) {
	t.Setenv("AURORA_MINIMAL_BENCH_MODE", "true")
	t.Setenv("AURORA_CHAT_FAST_PATH_PASSTHROUGH", "true")

	provider := &mockProvider{
		err:             errors.New("translated provider should not be called"),
		supportedModels: []string{"gpt-4o-mini"},
		providerTypes:   map[string]string{"gpt-4o-mini": "openai"},
		providerNames:   map[string]string{"gpt-4o-mini": "openai-primary"},
		passthroughResponse: &core.PassthroughResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string][]string{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl-fast"}`)),
		},
	}
	srv := New(provider, &Config{DisableRequestLogging: true})
	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if provider.lastPassthroughProvider != "openai" {
		t.Fatalf("passthrough provider = %q, want openai", provider.lastPassthroughProvider)
	}
	if got := readPassthroughRequestBody(t, provider.lastPassthroughReq.Body); got != body {
		t.Fatalf("passthrough body = %s, want %s", got, body)
	}
}

func gatewayBenchScenarios() []gatewayBenchScenario {
	return []gatewayBenchScenario{
		{
			name:     "openai_chat_baseline",
			method:   http.MethodPost,
			path:     "/v1/chat/completions",
			body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`,
			config:   &Config{},
			provider: gatewayBenchTextProvider(),
		},
		{
			name:     "openai_chat_fast_passthrough",
			method:   http.MethodPost,
			path:     "/v1/chat/completions",
			body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`,
			config:   &Config{},
			provider: gatewayBenchPassthroughProvider(),
		},
		{
			name:       "openai_chat_master_key_auth",
			method:     http.MethodPost,
			path:       "/v1/chat/completions",
			body:       `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`,
			authHeader: "Bearer bench-master-key",
			config:     &Config{MasterKey: "bench-master-key"},
			provider:   gatewayBenchTextProvider(),
		},
		{
			name:   "openai_chat_usage_audit",
			method: http.MethodPost,
			path:   "/v1/chat/completions",
			body:   `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`,
			config: &Config{
				UsageLogger: &benchUsageLogger{cfg: usage.Config{Enabled: true}},
				AuditLogger: &benchAuditLogger{cfg: auditlog.Config{Enabled: true, LogBodies: true}},
			},
			provider: gatewayBenchTextProvider(),
		},
		{
			name:   "openai_chat_workflow_budget",
			method: http.MethodPost,
			path:   "/v1/chat/completions",
			body:   `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`,
			config: &Config{
				WorkflowPolicyResolver: benchWorkflowResolver{features: core.WorkflowFeatures{Cache: true, Audit: true, Usage: true, Budget: true, Guardrails: true, Fallback: true}},
				UsageLogger:            &benchUsageLogger{cfg: usage.Config{Enabled: true}},
			},
			provider: gatewayBenchTextProvider(),
		},
		{
			name:     "openai_chat_stream",
			method:   http.MethodPost,
			path:     "/v1/chat/completions",
			body:     `{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"hello"}]}`,
			config:   &Config{FallbackResolver: benchFallbackResolver{}},
			provider: gatewayBenchStreamProvider(),
		},
		{
			name:     "anthropic_messages_text_from_openai_provider",
			method:   http.MethodPost,
			path:     "/v1/messages",
			body:     `{"model":"gpt-4o-mini","max_tokens":64,"messages":[{"role":"user","content":"hello"}]}`,
			config:   &Config{EnableAnthropicIngress: true},
			provider: gatewayBenchTextProvider(),
		},
		{
			name:     "anthropic_messages_tool_from_openai_provider",
			method:   http.MethodPost,
			path:     "/v1/messages",
			body:     `{"model":"gpt-4o-mini","max_tokens":64,"messages":[{"role":"user","content":"weather"}]}`,
			config:   &Config{EnableAnthropicIngress: true},
			provider: gatewayBenchToolProvider(),
		},
		{
			name:     "anthropic_messages_stream_from_openai_provider",
			method:   http.MethodPost,
			path:     "/v1/messages",
			body:     `{"model":"gpt-4o-mini","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"hello"}]}`,
			config:   &Config{EnableAnthropicIngress: true},
			provider: gatewayBenchStreamProvider(),
		},
		{
			name:     "responses_api",
			method:   http.MethodPost,
			path:     "/v1/responses",
			body:     `{"model":"gpt-4o-mini","input":"hello"}`,
			config:   &Config{},
			provider: gatewayBenchResponsesProvider(),
		},
		{
			name:     "provider_passthrough",
			method:   http.MethodPost,
			path:     "/p/openai/v1/chat/completions",
			body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`,
			config:   &Config{EnabledPassthroughProviders: []string{"openai"}},
			provider: gatewayBenchPassthroughProvider(),
		},
	}
}

func runGatewayStackBenchmark(b *testing.B, scenario gatewayBenchScenario) {
	b.Helper()
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.Cleanup(func() { slog.SetDefault(oldLogger) })

	srv := New(scenario.provider, scenario.config)
	body := []byte(scenario.body)
	if scenario.name == "openai_chat_fast_passthrough" {
		b.Setenv("AURORA_CHAT_FAST_PATH_PASSTHROUGH", "true")
	} else {
		b.Setenv("AURORA_CHAT_FAST_PATH_PASSTHROUGH", "")
		_ = os.Unsetenv("AURORA_CHAT_FAST_PATH_PASSTHROUGH")
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(scenario.method, scenario.path, strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			if scenario.authHeader != "" {
				req.Header.Set("Authorization", scenario.authHeader)
			}
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("%s status = %d, want 200; body=%s", scenario.name, rec.Code, rec.Body.String())
			}
			gatewayBenchSink.Add(uint64(rec.Body.Len()))
		}
	})
}

func gatewayBenchTextProvider() *mockProvider {
	return &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
		providerTypes:   map[string]string{"gpt-4o-mini": "openai"},
		providerNames:   map[string]string{"gpt-4o-mini": "openai-primary"},
		response: &core.ChatResponse{
			ID:     "chatcmpl-bench",
			Object: "chat.completion",
			Model:  "gpt-4o-mini",
			Choices: []core.Choice{{
				Index:        0,
				FinishReason: "stop",
				Message:      core.ResponseMessage{Role: "assistant", Content: "benchmark response"},
			}},
			Usage: core.Usage{PromptTokens: 8, CompletionTokens: 4, TotalTokens: 12},
		},
	}
}

func gatewayBenchToolProvider() *mockProvider {
	provider := gatewayBenchTextProvider()
	provider.response = &core.ChatResponse{
		ID:    "chatcmpl-bench-tool",
		Model: "gpt-4o-mini",
		Choices: []core.Choice{{
			FinishReason: "tool_calls",
			Message: core.ResponseMessage{ToolCalls: []core.ToolCall{{
				ID:   "call_weather",
				Type: "function",
				Function: core.FunctionCall{
					Name:      "lookup_weather",
					Arguments: `{"city":"Warsaw"}`,
				},
			}}},
		}},
		Usage: core.Usage{PromptTokens: 8, CompletionTokens: 4, TotalTokens: 12},
	}
	return provider
}

func gatewayBenchStreamProvider() *mockProvider {
	provider := gatewayBenchTextProvider()
	provider.streamData = strings.Join([]string{
		`data: {"id":"chatcmpl-bench","model":"gpt-4o-mini","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-bench","model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-bench","model":"gpt-4o-mini","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":4}}`,
		`data: [DONE]`,
		``,
	}, "\n")
	return provider
}

func gatewayBenchResponsesProvider() *mockProvider {
	provider := gatewayBenchTextProvider()
	provider.responsesResponse = &core.ResponsesResponse{
		ID:        "resp-bench",
		Object:    "response",
		CreatedAt: time.Now().Unix(),
		Model:     "gpt-4o-mini",
		Status:    "completed",
		Output: []core.ResponsesOutputItem{{
			ID:      "msg-bench",
			Type:    "message",
			Role:    "assistant",
			Status:  "completed",
			Content: []core.ResponsesContentItem{{Type: "output_text", Text: "benchmark response"}},
		}},
		Usage: &core.ResponsesUsage{InputTokens: 8, OutputTokens: 4, TotalTokens: 12},
	}
	return provider
}

func gatewayBenchPassthroughProvider() *mockProvider {
	provider := gatewayBenchTextProvider()
	provider.passthroughResponse = &core.PassthroughResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl-bench","object":"chat.completion"}`)),
	}
	return provider
}

type benchUsageLogger struct {
	cfg    usage.Config
	writes atomic.Uint64
}

func (l *benchUsageLogger) Write(_ *usage.UsageEntry) { l.writes.Add(1) }
func (l *benchUsageLogger) Config() usage.Config      { return l.cfg }
func (l *benchUsageLogger) Close() error              { return nil }

type benchAuditLogger struct {
	cfg    auditlog.Config
	writes atomic.Uint64
}

func (l *benchAuditLogger) Write(_ *auditlog.LogEntry)         { l.writes.Add(1) }
func (l *benchAuditLogger) BroadcastLive(_ *auditlog.LogEntry) {}
func (l *benchAuditLogger) Config() auditlog.Config            { return l.cfg }
func (l *benchAuditLogger) Close() error                       { return nil }
func (l *benchAuditLogger) SubscribeLive(_ int) *auditlog.LogSubscriber {
	ch := make(chan *auditlog.LogEntry)
	close(ch)
	return &auditlog.LogSubscriber{ID: "bench", Entries: ch}
}
func (l *benchAuditLogger) UnsubscribeLive(_ string) {}
func (l *benchAuditLogger) LiveSubscriberCount() int { return 0 }

type benchWorkflowResolver struct{ features core.WorkflowFeatures }

func (r benchWorkflowResolver) Match(core.WorkflowSelector) (*core.ResolvedWorkflowPolicy, error) {
	return &core.ResolvedWorkflowPolicy{
		VersionID: "bench-workflow",
		Version:   1,
		Name:      "bench",
		Features:  r.features,
	}, nil
}

type benchFallbackResolver struct{}

func (benchFallbackResolver) ResolveFallbacks(*core.RequestModelResolution, core.Operation) []core.ModelSelector {
	return []core.ModelSelector{{Provider: "bench", Model: "unused-fallback"}}
}

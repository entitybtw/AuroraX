package server

import (
	json "github.com/goccy/go-json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurora/internal/core"
)

func TestAnthropicGatewayConversionRouteRequiresIngressFlag(t *testing.T) {
	mock := &mockProvider{supportedModels: []string{"gpt-4o-mini"}}
	srv := New(mock, &Config{})

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"gpt-4o-mini","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when Anthropic ingress is disabled", rec.Code)
	}
}

func TestAnthropicGatewayConversionOpenAITextResponseToAnthropicMessage(t *testing.T) {
	mock := &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
		providerTypes:   map[string]string{"gpt-4o-mini": "openai"},
		response: &core.ChatResponse{
			ID:     "chatcmpl-test",
			Object: "chat.completion",
			Model:  "gpt-4o-mini",
			Choices: []core.Choice{{
				Index:        0,
				FinishReason: "stop",
				Message: core.ResponseMessage{
					Role:    "assistant",
					Content: "hello from openai-shaped provider",
				},
			}},
			Usage: core.Usage{PromptTokens: 11, CompletionTokens: 7, TotalTokens: 18},
		},
	}
	body := postAnthropicMessage(t, mock, `{"model":"gpt-4o-mini","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`)

	if body.Type != "message" || body.Role != "assistant" {
		t.Fatalf("type/role = %q/%q, want message/assistant", body.Type, body.Role)
	}
	if body.Model != "gpt-4o-mini" {
		t.Fatalf("model = %q, want gpt-4o-mini", body.Model)
	}
	if body.StopReason != "end_turn" {
		t.Fatalf("stop_reason = %q, want end_turn", body.StopReason)
	}
	if len(body.Content) != 1 || body.Content[0].Type != "text" || body.Content[0].Text != "hello from openai-shaped provider" {
		t.Fatalf("content = %+v, want one Anthropic text block", body.Content)
	}
	if body.Usage.InputTokens != 11 || body.Usage.OutputTokens != 7 {
		t.Fatalf("usage = %+v, want input=11 output=7", body.Usage)
	}
}

func TestAnthropicGatewayConversionOpenAIToolResponseToAnthropicToolUse(t *testing.T) {
	mock := &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
		providerTypes:   map[string]string{"gpt-4o-mini": "openai"},
		response: &core.ChatResponse{
			ID:    "chatcmpl-tool",
			Model: "gpt-4o-mini",
			Choices: []core.Choice{{
				FinishReason: "tool_calls",
				Message: core.ResponseMessage{ToolCalls: []core.ToolCall{{
					ID:   "call_lookup",
					Type: "function",
					Function: core.FunctionCall{
						Name:      "lookup_weather",
						Arguments: `{"city":"Warsaw","days":1}`,
					},
				}}},
			}},
		},
	}
	body := postAnthropicMessage(t, mock, `{"model":"gpt-4o-mini","max_tokens":64,"messages":[{"role":"user","content":"weather"}]}`)

	if body.StopReason != "tool_use" {
		t.Fatalf("stop_reason = %q, want tool_use", body.StopReason)
	}
	if len(body.Content) != 1 || body.Content[0].Type != "tool_use" {
		t.Fatalf("content = %+v, want one tool_use block", body.Content)
	}
	block := body.Content[0]
	if block.ID != "call_lookup" || block.Name != "lookup_weather" {
		t.Fatalf("tool block = %+v, want call_lookup/lookup_weather", block)
	}
	var input map[string]any
	if err := json.Unmarshal(block.Input, &input); err != nil {
		t.Fatalf("tool input invalid JSON: %v; raw=%s", err, string(block.Input))
	}
	if input["city"] != "Warsaw" || input["days"] != float64(1) {
		t.Fatalf("tool input = %+v, want city Warsaw days 1", input)
	}
}

func TestAnthropicGatewayConversionOpenAIStreamToAnthropicEvents(t *testing.T) {
	mock := &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
		providerTypes:   map[string]string{"gpt-4o-mini": "openai"},
		streamData: strings.Join([]string{
			`data: {"id":"chatcmpl-test","model":"gpt-4o-mini","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-test","model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-test","model":"gpt-4o-mini","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2}}`,
			`data: [DONE]`,
			``,
		}, "\n"),
	}
	srv := New(mock, &Config{EnableAnthropicIngress: true})

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"gpt-4o-mini","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", contentType)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"event: message_start",
		"event: content_block_start",
		"event: content_block_delta",
		`"text":"hello"`,
		"event: message_delta",
		`"stop_reason":"end_turn"`,
		"event: message_stop",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("stream missing %q; body=%s", want, body)
		}
	}
}

func TestAnthropicGatewayConversionFormatsMixedReasoningTextAndToolBlocks(t *testing.T) {
	extra := core.UnknownJSONFieldsFromMap(map[string]json.RawMessage{
		"reasoning_content": json.RawMessage(`"first think"`),
	})

	resp := formatAnthropicResponse(&core.ChatResponse{
		ID: "chatcmpl-mixed",
		Choices: []core.Choice{{
			FinishReason: "tool_calls",
			Message: core.ResponseMessage{
				Role:        "assistant",
				Content:     "then answer",
				ExtraFields: extra,
				ToolCalls: []core.ToolCall{{
					ID:   "call_123",
					Type: "function",
					Function: core.FunctionCall{
						Name:      "lookup_weather",
						Arguments: `{"city":"Warsaw"}`,
					},
				}},
			},
		}},
		Usage: core.Usage{PromptTokens: 10, CompletionTokens: 4},
	}, "claude-sonnet")

	if resp.StopReason != "tool_use" {
		t.Fatalf("StopReason = %q, want tool_use", resp.StopReason)
	}
	if len(resp.Content) != 3 {
		t.Fatalf("len(Content) = %d, want thinking/text/tool_use", len(resp.Content))
	}
	if resp.Content[0].Type != "thinking" || resp.Content[0].Thinking != "first think" {
		t.Fatalf("thinking block = %+v", resp.Content[0])
	}
	if resp.Content[1].Type != "text" || resp.Content[1].Text != "then answer" {
		t.Fatalf("text block = %+v", resp.Content[1])
	}
	if resp.Content[2].Type != "tool_use" || resp.Content[2].ID != "call_123" || resp.Content[2].Name != "lookup_weather" {
		t.Fatalf("tool block = %+v", resp.Content[2])
	}
}

func TestAnthropicGatewayConversionFinishReasonMapping(t *testing.T) {
	for _, tc := range []struct {
		openai    string
		anthropic string
	}{
		{openai: "stop", anthropic: "end_turn"},
		{openai: "tool_calls", anthropic: "tool_use"},
		{openai: "length", anthropic: "max_tokens"},
		{openai: "content_filter", anthropic: "content_filter"},
		{openai: "", anthropic: "end_turn"},
		{openai: "custom", anthropic: "custom"},
	} {
		if got := mapOpenAIFinishToAnthropic(tc.openai); got != tc.anthropic {
			t.Fatalf("mapOpenAIFinishToAnthropic(%q) = %q, want %q", tc.openai, got, tc.anthropic)
		}
	}
}

func postAnthropicMessage(t *testing.T, provider *mockProvider, requestBody string) anthropicIngressResponse {
	t.Helper()
	srv := New(provider, &Config{EnableAnthropicIngress: true})
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body anthropicIngressResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode Anthropic response: %v; body=%s", err, rec.Body.String())
	}
	return body
}

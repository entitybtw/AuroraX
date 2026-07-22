package server

import (
	json "github.com/goccy/go-json"
	"testing"

	"aurora/internal/core"
)

func TestFormatAnthropicResponseHandlesEmptyChoices(t *testing.T) {
	resp := formatAnthropicResponse(&core.ChatResponse{
		ID:      "chatcmpl_empty",
		Choices: nil,
		Usage: core.Usage{
			PromptTokens:     3,
			CompletionTokens: 0,
		},
	}, "claude-sonnet")

	if resp == nil {
		t.Fatal("formatAnthropicResponse returned nil")
	}
	if resp.StopReason != "end_turn" {
		t.Fatalf("StopReason = %q, want end_turn", resp.StopReason)
	}
	if len(resp.Content) != 1 || resp.Content[0].Type != "text" || resp.Content[0].Text != "" {
		t.Fatalf("Content = %#v, want one empty text block", resp.Content)
	}
	if resp.Usage.InputTokens != 3 || resp.Usage.OutputTokens != 0 {
		t.Fatalf("Usage = %+v, want input=3 output=0", resp.Usage)
	}
}

func TestFormatAnthropicResponseNormalizesToolArguments(t *testing.T) {
	resp := formatAnthropicResponse(&core.ChatResponse{
		ID: "chatcmpl_tool",
		Choices: []core.Choice{{
			FinishReason: "tool_calls",
			Message: core.ResponseMessage{
				Role: "assistant",
				ToolCalls: []core.ToolCall{{
					ID:   "call_123",
					Type: "function",
					Function: core.FunctionCall{
						Name:      "lookup_weather",
						Arguments: `{"city":"Warsaw","days":1}`,
					},
				}},
			},
		}},
	}, "claude-sonnet")

	if len(resp.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(resp.Content))
	}
	block := resp.Content[0]
	if block.Type != "tool_use" || block.ID != "call_123" || block.Name != "lookup_weather" {
		t.Fatalf("tool block = %+v, want lookup_weather/call_123", block)
	}
	var input map[string]any
	if err := json.Unmarshal(block.Input, &input); err != nil {
		t.Fatalf("tool input is invalid JSON: %v; raw=%s", err, string(block.Input))
	}
	if input["city"] != "Warsaw" {
		t.Fatalf("city = %#v, want Warsaw", input["city"])
	}
}

func TestFormatAnthropicResponseInvalidToolArgumentsBecomeEmptyObject(t *testing.T) {
	resp := formatAnthropicResponse(&core.ChatResponse{
		ID: "chatcmpl_bad_tool",
		Choices: []core.Choice{{
			FinishReason: "tool_calls",
			Message: core.ResponseMessage{
				Role: "assistant",
				ToolCalls: []core.ToolCall{{
					ID:   "call_123",
					Type: "function",
					Function: core.FunctionCall{
						Name:      "lookup_weather",
						Arguments: `{"city":"Warsaw"`,
					},
				}},
			},
		}},
	}, "claude-sonnet")

	if len(resp.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(resp.Content))
	}
	if string(resp.Content[0].Input) != "{}" {
		t.Fatalf("Input = %s, want {}", string(resp.Content[0].Input))
	}
}

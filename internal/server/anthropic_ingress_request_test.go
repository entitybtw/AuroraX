package server

import (
	"testing"

	"aurora/internal/core"
)

func TestDecodeAnthropicRequestConvertsToolsAndToolChoice(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet",
		"max_tokens":1024,
		"messages":[{"role":"user","content":"weather"}],
		"tools":[{"name":"lookup_weather","description":"Get weather","input_schema":{"type":"object","properties":{"city":{"type":"string"}}}}],
		"tool_choice":{"type":"tool","name":"lookup_weather","disable_parallel_tool_use":true}
	}`)

	req, err := decodeAnthropicRequest(raw)
	if err != nil {
		t.Fatalf("decodeAnthropicRequest() error = %v", err)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(req.Tools))
	}
	function, ok := req.Tools[0]["function"].(map[string]any)
	if !ok {
		t.Fatalf("tool function = %#v, want map", req.Tools[0]["function"])
	}
	if function["name"] != "lookup_weather" {
		t.Fatalf("function name = %#v, want lookup_weather", function["name"])
	}
	choice, ok := req.ToolChoice.(map[string]any)
	if !ok {
		t.Fatalf("ToolChoice = %#v, want map", req.ToolChoice)
	}
	if choice["type"] != "function" {
		t.Fatalf("tool_choice.type = %#v, want function", choice["type"])
	}
	if req.ParallelToolCalls == nil || *req.ParallelToolCalls {
		t.Fatalf("ParallelToolCalls = %#v, want false", req.ParallelToolCalls)
	}
}

func TestDecodeAnthropicRequestSplitsMixedUserToolResultsInOrder(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet",
		"max_tokens":1024,
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"before"},
				{"type":"tool_result","tool_use_id":"toolu_1","content":"result"},
				{"type":"text","text":"after"}
			]}
		]
	}`)

	req, err := decodeAnthropicRequest(raw)
	if err != nil {
		t.Fatalf("decodeAnthropicRequest() error = %v", err)
	}
	if len(req.Messages) != 3 {
		t.Fatalf("len(Messages) = %d, want user/tool/user", len(req.Messages))
	}
	if req.Messages[0].Role != "user" || req.Messages[0].Content != "before" {
		t.Fatalf("first message = %+v", req.Messages[0])
	}
	if req.Messages[1].Role != "tool" || req.Messages[1].ToolCallID != "toolu_1" || req.Messages[1].Content != "result" {
		t.Fatalf("tool message = %+v", req.Messages[1])
	}
	if req.Messages[2].Role != "user" || req.Messages[2].Content != "after" {
		t.Fatalf("third message = %+v", req.Messages[2])
	}
}

func TestDecodeAnthropicRequestConvertsToolUseAndToolResult(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet",
		"max_tokens":1024,
		"messages":[
			{"role":"assistant","content":[{"type":"tool_use","id":"toolu_123","name":"lookup_weather","input":{"city":"Warsaw"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_123","content":[{"type":"text","text":"21C"}]}]}
		]
	}`)

	req, err := decodeAnthropicRequest(raw)
	if err != nil {
		t.Fatalf("decodeAnthropicRequest() error = %v", err)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(req.Messages))
	}
	assistant := req.Messages[0]
	if assistant.Role != "assistant" || len(assistant.ToolCalls) != 1 {
		t.Fatalf("assistant message = %+v, want one tool call", assistant)
	}
	if assistant.ToolCalls[0].ID != "toolu_123" || assistant.ToolCalls[0].Function.Name != "lookup_weather" {
		t.Fatalf("tool call = %+v, want toolu_123/lookup_weather", assistant.ToolCalls[0])
	}
	if assistant.ToolCalls[0].Function.Arguments != `{"city":"Warsaw"}` {
		t.Fatalf("arguments = %q, want canonical city JSON", assistant.ToolCalls[0].Function.Arguments)
	}
	tool := req.Messages[1]
	if tool.Role != "tool" || tool.ToolCallID != "toolu_123" {
		t.Fatalf("tool result message = %+v, want role=tool toolu_123", tool)
	}
	if tool.Content != "21C" {
		t.Fatalf("tool result content = %#v, want 21C", tool.Content)
	}
}

func TestDecodeAnthropicRequestFlattensTextOnlyContentArraysLike9router(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet",
		"max_tokens":1024,
		"messages":[{"role":"user","content":[{"type":"text","text":"hi"},{"type":"text","text":"there"}]}]
	}`)

	req, err := decodeAnthropicRequest(raw)
	if err != nil {
		t.Fatalf("decodeAnthropicRequest() error = %v", err)
	}
	if req.Messages[0].Content != "hi\nthere" {
		t.Fatalf("content = %#v, want hi\\nthere", req.Messages[0].Content)
	}
}

func TestDecodeAnthropicRequestPreservesMultimodalContentArraysLike9router(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet",
		"max_tokens":1024,
		"messages":[{"role":"user","content":[
			{"type":"text","text":"describe"},
			{"type":"image","source":{"type":"base64","media_type":"image/png","data":"ZmFrZQ=="}}
		]}]
	}`)

	req, err := decodeAnthropicRequest(raw)
	if err != nil {
		t.Fatalf("decodeAnthropicRequest() error = %v", err)
	}
	parts, ok := req.Messages[0].Content.([]core.ContentPart)
	if !ok {
		t.Fatalf("content = %T, want []core.ContentPart", req.Messages[0].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("len(parts) = %d, want 2", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "describe" {
		t.Fatalf("unexpected text part: %+v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/png;base64,ZmFrZQ==" {
		t.Fatalf("unexpected image part: %+v", parts[1])
	}
}

func TestDecodeAnthropicRequestAddsMissingToolResponsesAfterRealResponsesLike9router(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet",
		"max_tokens":1024,
		"messages":[
			{"role":"assistant","content":[
				{"type":"tool_use","id":"toolu_real","name":"lookup_weather","input":{"city":"Warsaw"}},
				{"type":"tool_use","id":"toolu_missing","name":"lookup_weather","input":{"city":"Delhi"}}
			]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_real","content":"21C"}]},
			{"role":"user","content":"continue"}
		]
	}`)

	req, err := decodeAnthropicRequest(raw)
	if err != nil {
		t.Fatalf("decodeAnthropicRequest() error = %v", err)
	}
	if len(req.Messages) != 4 {
		t.Fatalf("len(Messages) = %d, want assistant, real tool, synthetic tool, user", len(req.Messages))
	}
	if req.Messages[1].Role != "tool" || req.Messages[1].ToolCallID != "toolu_real" || req.Messages[1].Content != "21C" {
		t.Fatalf("real tool response order/content = %+v", req.Messages[1])
	}
	if req.Messages[2].Role != "tool" || req.Messages[2].ToolCallID != "toolu_missing" || req.Messages[2].Content != "[No response received]" {
		t.Fatalf("synthetic tool response order/content = %+v", req.Messages[2])
	}
}

func TestDecodeAnthropicRequestAddsMissingToolResponsesLike9router(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet",
		"max_tokens":1024,
		"messages":[
			{"role":"assistant","content":[{"type":"tool_use","id":"toolu_missing","name":"lookup_weather","input":{"city":"Warsaw"}}]},
			{"role":"user","content":"continue"}
		]
	}`)

	req, err := decodeAnthropicRequest(raw)
	if err != nil {
		t.Fatalf("decodeAnthropicRequest() error = %v", err)
	}
	if len(req.Messages) != 3 {
		t.Fatalf("len(Messages) = %d, want assistant, synthetic tool, user", len(req.Messages))
	}
	tool := req.Messages[1]
	if tool.Role != "tool" || tool.ToolCallID != "toolu_missing" || tool.Content != "[No response received]" {
		t.Fatalf("synthetic tool response = %+v, want missing response placeholder", tool)
	}
}

func TestDecodeAnthropicRequestConvertsImageContent(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet",
		"max_tokens":1024,
		"messages":[{"role":"user","content":[{"type":"image","source":{"type":"url","url":"https://example.com/image.png"}}]}]
	}`)

	req, err := decodeAnthropicRequest(raw)
	if err != nil {
		t.Fatalf("decodeAnthropicRequest() error = %v", err)
	}
	parts, ok := req.Messages[0].Content.([]core.ContentPart)
	if !ok || len(parts) != 1 {
		t.Fatalf("content = %#v, want one content part", req.Messages[0].Content)
	}
	if parts[0].Type != "image_url" || parts[0].ImageURL == nil || parts[0].ImageURL.URL != "https://example.com/image.png" {
		t.Fatalf("image part = %+v, want image_url", parts[0])
	}
}

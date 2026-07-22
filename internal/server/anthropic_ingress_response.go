package server

import (
	json "github.com/goccy/go-json"
	"strings"
	"time"

	"aurora/internal/core"
)

func formatAnthropicResponse(resp *core.ChatResponse, model string) *anthropicIngressResponse {
	if resp == nil {
		resp = &core.ChatResponse{}
	}

	finishReason := ""
	msg := core.ResponseMessage{Role: "assistant", Content: ""}
	if len(resp.Choices) > 0 {
		finishReason = resp.Choices[0].FinishReason
		msg = resp.Choices[0].Message
	}

	out := &anthropicIngressResponse{
		ID:         resp.ID,
		Type:       "message",
		Role:       "assistant",
		Model:      model,
		StopReason: mapOpenAIFinishToAnthropic(finishReason),
		Content:    make([]anthropicIngressContentBlock, 0),
		Usage: anthropicIngressUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}

	if resp.Usage.RawUsage != nil {
		if v, ok := resp.Usage.RawUsage["cache_creation_input_tokens"]; ok {
			if n, ok := v.(float64); ok {
				out.Usage.CacheCreationInputTokens = int(n)
			}
		}
		if v, ok := resp.Usage.RawUsage["cache_read_input_tokens"]; ok {
			if n, ok := v.(float64); ok {
				out.Usage.CacheReadInputTokens = int(n)
			}
		}
	}

	reasoningContent := extractReasoningContent(msg)
	if reasoningContent != "" {
		out.Content = append(out.Content, anthropicIngressContentBlock{
			Type:     "thinking",
			Thinking: reasoningContent,
		})
	}

	textContent := extractTextContent(msg)
	if textContent != "" {
		out.Content = append(out.Content, anthropicIngressContentBlock{
			Type: "text",
			Text: textContent,
		})
	}

	for _, tc := range msg.ToolCalls {
		input := normalizeAnthropicToolUseInput(tc.Function.Arguments)
		out.Content = append(out.Content, anthropicIngressContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	if len(out.Content) == 0 {
		out.Content = append(out.Content, anthropicIngressContentBlock{
			Type: "text",
			Text: "",
		})
	}

	return out
}

func normalizeAnthropicToolUseInput(arguments string) json.RawMessage {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" {
		return json.RawMessage("{}")
	}

	var parsed any
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&parsed); err != nil {
		return json.RawMessage("{}")
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return json.RawMessage("{}")
	}
	if _, ok := parsed.(map[string]any); !ok {
		return json.RawMessage("{}")
	}
	canonical, err := json.Marshal(parsed)
	if err != nil {
		return json.RawMessage("{}")
	}
	return json.RawMessage(canonical)
}

func extractReasoningContent(msg core.ResponseMessage) string {
	if msg.ExtraFields.IsEmpty() {
		return ""
	}
	raw := msg.ExtraFields.Lookup("reasoning_content")
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

func extractTextContent(msg core.ResponseMessage) string {
	if msg.Content == nil {
		return ""
	}
	switch v := msg.Content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					parts = append(parts, t)
				}
			}
		}
		return strings.Join(parts, "\n\n")
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
}

func mapOpenAIFinishToAnthropic(finish string) string {
	switch finish {
	case "stop":
		return "end_turn"
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	case "content_filter":
		return "content_filter"
	default:
		if finish != "" {
			return finish
		}
		return "end_turn"
	}
}

func anthropicIngressCreatedTimestamp() int64 {
	return time.Now().Unix()
}

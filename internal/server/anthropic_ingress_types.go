package server

import json "github.com/goccy/go-json"

type anthropicIngressRequest struct {
	Model       string                      `json:"model"`
	Messages    []anthropicIngressMessage   `json:"messages"`
	MaxTokens   int                         `json:"max_tokens,omitempty"`
	Temperature *float64                    `json:"temperature,omitempty"`
	System      any                         `json:"system,omitempty"`
	Stream      bool                        `json:"stream,omitempty"`
	Thinking    *anthropicIngressThinking   `json:"thinking,omitempty"`
	Tools       []anthropicIngressTool      `json:"tools,omitempty"`
	ToolChoice  *anthropicIngressToolChoice `json:"tool_choice,omitempty"`
	Metadata    map[string]any              `json:"metadata,omitempty"`
}

type anthropicIngressThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

type anthropicIngressTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicIngressToolChoice struct {
	Type                   string `json:"type"`
	Name                   string `json:"name,omitempty"`
	DisableParallelToolUse *bool  `json:"disable_parallel_tool_use,omitempty"`
}

type anthropicIngressMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicIngressContentBlock struct {
	Type         string                  `json:"type"`
	Text         string                  `json:"text"`
	ID           string                  `json:"id,omitempty"`
	Name         string                  `json:"name,omitempty"`
	Input        json.RawMessage         `json:"input,omitempty"`
	ToolUseID    string                  `json:"tool_use_id,omitempty"`
	Content      any                     `json:"content,omitempty"`
	IsError      bool                    `json:"is_error,omitempty"`
	Source       *anthropicIngressSource `json:"source,omitempty"`
	Thinking     string                  `json:"thinking,omitempty"`
	Signature    string                  `json:"signature,omitempty"`
	CacheControl json.RawMessage         `json:"cache_control,omitempty"`
}

type anthropicIngressSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type anthropicIngressResponse struct {
	ID           string                         `json:"id"`
	Type         string                         `json:"type"`
	Role         string                         `json:"role"`
	Content      []anthropicIngressContentBlock `json:"content"`
	Model        string                         `json:"model"`
	StopReason   string                         `json:"stop_reason"`
	StopSequence string                         `json:"stop_sequence,omitempty"`
	Usage        anthropicIngressUsage          `json:"usage"`
}

type anthropicIngressUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

type anthropicIngressStreamEvent struct {
	Type         string                        `json:"type"`
	Index        int                           `json:"index,omitempty"`
	Delta        *anthropicIngressDelta        `json:"delta,omitempty"`
	ContentBlock *anthropicIngressContentBlock `json:"content_block,omitempty"`
	Message      *anthropicIngressResponse     `json:"message,omitempty"`
	Usage        *anthropicIngressUsage        `json:"usage,omitempty"`
}

type anthropicIngressDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
	Signature   string `json:"signature,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

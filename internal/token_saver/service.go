package tokensaver

import (
	"encoding/json"
	"fmt"
	"strings"

	"aurora/configuration"
	"aurora/internal/core"
)

const (
	EndpointChatCompletions = "chat_completions"

	SkipDisabled         = "disabled"
	SkipEndpointMismatch = "endpoint_mismatch"
	SkipStreaming        = "streaming_disabled"
	SkipModelScope       = "model_scope"
	SkipProviderScope    = "provider_scope"
)

type ChatMeta struct {
	Endpoint string
	Model    string
	Provider string
}

type Metadata struct {
	Enabled              bool
	Applied              bool
	SkipReason           string
	OutputProfileApplied bool
	EmitHeaders          bool
}

type Service struct {
	cfg config.TokenSaverConfig
}

func NewService(cfg config.TokenSaverConfig) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) ApplyChat(req *core.ChatRequest, meta ChatMeta) (*core.ChatRequest, Metadata, error) {
	if req == nil {
		return nil, Metadata{Enabled: s.cfg.Enabled, EmitHeaders: s.cfg.EmitHeaders}, core.NewInvalidRequestError("chat request is required", nil)
	}

	out, err := cloneChatRequest(req)
	if err != nil {
		return nil, Metadata{Enabled: s.cfg.Enabled, EmitHeaders: s.cfg.EmitHeaders}, fmt.Errorf("clone chat request for token saver: %w", err)
	}

	result := Metadata{Enabled: s.cfg.Enabled, EmitHeaders: s.cfg.EmitHeaders}
	if reason := s.skipReason(out, meta); reason != "" {
		result.SkipReason = reason
		return out, result, nil
	}

	if s.cfg.Output.Enabled && !isJSONMode(out) && !hasCavemanInstruction(out.Messages) {
		instruction := s.cfg.Output.OutputInstruction()
		out.Messages = append([]core.Message{{Role: "system", Content: instruction}}, out.Messages...)
		result.OutputProfileApplied = true
	}

	result.Applied = result.OutputProfileApplied
	if !result.Applied {
		result.SkipReason = "output_disabled"
	}
	return out, result, nil
}

func (s *Service) skipReason(req *core.ChatRequest, meta ChatMeta) string {
	if !s.cfg.Enabled {
		return SkipDisabled
	}
	if req.Stream && !s.cfg.ApplyStreaming {
		return SkipStreaming
	}
	endpoint := strings.ToLower(strings.TrimSpace(meta.Endpoint))
	if endpoint == "" {
		endpoint = EndpointChatCompletions
	}
	if len(s.cfg.Endpoints) > 0 && !containsFold(s.cfg.Endpoints, endpoint) {
		return SkipEndpointMismatch
	}
	model := firstNonEmpty(meta.Model, req.Model)
	if !inScope(model, s.cfg.Models.Include, s.cfg.Models.Exclude) {
		return SkipModelScope
	}
	provider := firstNonEmpty(meta.Provider, req.Provider)
	if !inScope(provider, s.cfg.Providers.Include, s.cfg.Providers.Exclude) {
		return SkipProviderScope
	}
	return ""
}

func hasCavemanInstruction(messages []core.Message) bool {
	for _, msg := range messages {
		if msg.Role == "system" {
			if text, ok := msg.Content.(string); ok {
				if strings.Contains(text, config.InstructionFull) ||
					strings.Contains(text, config.InstructionLite) ||
					strings.Contains(text, config.InstructionUltra) ||
					strings.Contains(text, config.InstructionWenyan) {
					return true
				}
			}
		}
	}
	return false
}

func isJSONMode(req *core.ChatRequest) bool {
	if req == nil {
		return false
	}
	return len(req.ExtraFields.Lookup("response_format")) > 0
}

func inScope(value string, include, exclude []string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return true
	}
	if containsFold(exclude, value) {
		return false
	}
	return len(include) == 0 || containsFold(include, value)
}

func containsFold(values []string, value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range values {
		if strings.ToLower(strings.TrimSpace(candidate)) == value {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func cloneChatRequest(req *core.ChatRequest) (*core.ChatRequest, error) {
	if req == nil {
		return nil, nil
	}
	out := &core.ChatRequest{
		Model:             req.Model,
		Provider:          req.Provider,
		Stream:            req.Stream,
		ParallelToolCalls: cloneBoolPtr(req.ParallelToolCalls),
		Temperature:       cloneFloat64Ptr(req.Temperature),
		MaxTokens:         cloneIntPtr(req.MaxTokens),
	}
	if req.StreamOptions != nil {
		so := *req.StreamOptions
		out.StreamOptions = &so
	}
	if req.Reasoning != nil {
		r := *req.Reasoning
		out.Reasoning = &r
	}
	if !req.ExtraFields.IsEmpty() {
		out.ExtraFields = core.CloneUnknownJSONFields(req.ExtraFields)
	}
	if len(req.Messages) > 0 {
		out.Messages = make([]core.Message, len(req.Messages))
		for i := range req.Messages {
			out.Messages[i] = cloneMessage(req.Messages[i])
		}
	}
	if len(req.Tools) > 0 {
		out.Tools = append([]map[string]any(nil), req.Tools...)
	}
	if req.ToolChoice != nil {
		out.ToolChoice = req.ToolChoice
	}
	return out, nil
}

func cloneMessage(msg core.Message) core.Message {
	out := core.Message{
		Role:        msg.Role,
		ToolCallID:  msg.ToolCallID,
		ContentNull: msg.ContentNull,
	}
	switch content := msg.Content.(type) {
	case string:
		out.Content = content
	case []core.ContentPart:
		cloned := make([]core.ContentPart, len(content))
		for i := range content {
			cloned[i] = cloneContentPart(content[i])
		}
		out.Content = cloned
	case nil:
	default:
		if raw, err := json.Marshal(content); err == nil {
			var v any
			if json.Unmarshal(raw, &v) == nil {
				out.Content = v
			} else {
				out.Content = content
			}
		} else {
			out.Content = content
		}
	}
	if len(msg.ToolCalls) > 0 {
		out.ToolCalls = make([]core.ToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			out.ToolCalls[i] = core.ToolCall{
				ID:       tc.ID,
				Type:     tc.Type,
				Function: cloneFunctionCall(tc.Function),
			}
		}
	}
	if !msg.ExtraFields.IsEmpty() {
		out.ExtraFields = core.CloneUnknownJSONFields(msg.ExtraFields)
	}
	return out
}

func cloneFunctionCall(fc core.FunctionCall) core.FunctionCall {
	out := core.FunctionCall{
		Name:      fc.Name,
		Arguments: fc.Arguments,
	}
	if !fc.ExtraFields.IsEmpty() {
		out.ExtraFields = core.CloneUnknownJSONFields(fc.ExtraFields)
	}
	return out
}

func cloneContentPart(p core.ContentPart) core.ContentPart {
	out := core.ContentPart{
		Type: p.Type,
		Text: p.Text,
	}
	if p.ImageURL != nil {
		img := *p.ImageURL
		if !p.ImageURL.ExtraFields.IsEmpty() {
			img.ExtraFields = core.CloneUnknownJSONFields(p.ImageURL.ExtraFields)
		}
		out.ImageURL = &img
	}
	if p.InputAudio != nil {
		audio := *p.InputAudio
		if !p.InputAudio.ExtraFields.IsEmpty() {
			audio.ExtraFields = core.CloneUnknownJSONFields(p.InputAudio.ExtraFields)
		}
		out.InputAudio = &audio
	}
	if !p.ExtraFields.IsEmpty() {
		out.ExtraFields = core.CloneUnknownJSONFields(p.ExtraFields)
	}
	return out
}

func cloneFloat64Ptr(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneBoolPtr(p *bool) *bool {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

package server

import (
	json "github.com/goccy/go-json"
	"fmt"
	"strings"

	"aurora/internal/core"
)

func decodeAnthropicRequest(data []byte) (*core.ChatRequest, error) {
	var req anthropicIngressRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid anthropic request: %w", err)
	}

	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("messages is required")
	}

	chatReq := &core.ChatRequest{
		Model:       req.Model,
		Stream:      req.Stream,
		MaxTokens:   &req.MaxTokens,
		Temperature: req.Temperature,
	}

	if req.MaxTokens == 0 {
		chatReq.MaxTokens = nil
	}

	if req.Thinking != nil {
		switch req.Thinking.Type {
		case "adaptive":
			chatReq.Reasoning = &core.Reasoning{Effort: "medium"}
		case "enabled":
			chatReq.Reasoning = &core.Reasoning{Effort: "high"}
		}
	}

	for _, msg := range req.Messages {
		cm, err := convertAnthropicMessage(msg)
		if err != nil {
			return nil, err
		}
		chatReq.Messages = append(chatReq.Messages, cm...)
	}

	chatReq.Messages = addMissingOpenAIToolResponses(chatReq.Messages)

	if req.System != nil {
		systemContent, err := flattenSystemContent(req.System)
		if err != nil {
			return nil, err
		}
		chatReq.Messages = append([]core.Message{{
			Role:    "system",
			Content: systemContent,
		}}, chatReq.Messages...)
	}

	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.InputSchema,
				},
			})
		}
		chatReq.Tools = tools
	}

	if req.ToolChoice != nil {
		switch req.ToolChoice.Type {
		case "auto":
			chatReq.ToolChoice = "auto"
		case "any":
			chatReq.ToolChoice = "required"
		case "tool":
			chatReq.ToolChoice = map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": req.ToolChoice.Name,
				},
			}
		case "none":
			chatReq.ToolChoice = "none"
		}
		if req.ToolChoice.DisableParallelToolUse != nil {
			chatReq.ParallelToolCalls = boolPtr(!*req.ToolChoice.DisableParallelToolUse)
		}
	}

	return chatReq, nil
}

func convertAnthropicMessage(msg anthropicIngressMessage) ([]core.Message, error) {
	switch msg.Role {
	case "user":
		blocks, err := parseContentBlocks(msg.Content)
		if err != nil {
			return []core.Message{{
				Role:    "user",
				Content: flattenAnthropicContent(msg.Content),
			}}, nil
		}
		return convertAnthropicUserBlocks(blocks)

	case "assistant":
		m := core.Message{Role: "assistant"}
		blocks, err := parseContentBlocks(msg.Content)
		if err != nil {
			return nil, err
		}

		var textParts []string
		for _, block := range blocks {
			switch block.Type {
			case "text":
				textParts = append(textParts, block.Text)
			case "thinking":
				if block.Signature != "" {
					redactedText := block.Thinking
					if redactedText == "" {
						redactedText = block.Text
					}
					raw, err := json.Marshal(redactedText)
					if err != nil {
						return nil, fmt.Errorf("marshal redacted thinking: %w", err)
					}
					m.ExtraFields = core.UnknownJSONFieldsFromMap(map[string]json.RawMessage{
						"redacted_thinking": raw,
					})
				}
			case "tool_use":
				arguments := "{}"
				if len(block.Input) > 0 {
					var parsed any
					if err := json.Unmarshal(block.Input, &parsed); err == nil {
						if canonical, err := json.Marshal(parsed); err == nil {
							arguments = string(canonical)
						}
					} else {
						if trimmed := string(block.Input); trimmed != "" {
							arguments = trimmed
						}
					}
				}
				if m.ToolCalls == nil {
					m.ToolCalls = make([]core.ToolCall, 0)
				}
				m.ToolCalls = append(m.ToolCalls, core.ToolCall{
					ID:   block.ID,
					Type: "function",
					Function: core.FunctionCall{
						Name:      block.Name,
						Arguments: arguments,
					},
				})
			}
		}

		combined := ""
		for i, t := range textParts {
			if i > 0 {
				combined += "\n\n"
			}
			combined += t
		}
		m.Content = combined
		return []core.Message{m}, nil

	case "tool_result":
		blocks, err := parseContentBlocks(msg.Content)
		if err != nil {
			return nil, err
		}

		var toolUseID string
		var resultContent any = ""

		if len(blocks) > 0 {
			if blocks[0].ToolUseID != "" {
				toolUseID = blocks[0].ToolUseID
				resultContent = flattenContentBlockContent(blocks)
			} else {
				toolUseID = blocks[0].ID
				resultContent = flattenContentBlockContent(blocks)
			}
		} else if s, ok := msg.Content.(string); ok {
			toolUseID = s
			resultContent = s
		}

		if toolUseID == "" {
			return nil, fmt.Errorf("tool_result message requires tool_use_id")
		}

		return []core.Message{{
			Role:       "tool",
			ToolCallID: toolUseID,
			Content:    resultContent,
		}}, nil

	default:
		return []core.Message{{
			Role:    msg.Role,
			Content: flattenAnthropicContent(msg.Content),
		}}, nil
	}
}

func convertAnthropicUserBlocks(blocks []anthropicIngressContentBlock) ([]core.Message, error) {
	messages := make([]core.Message, 0)
	userBlocks := make([]anthropicIngressContentBlock, 0, len(blocks))
	flushUserBlocks := func() {
		if len(userBlocks) == 0 {
			return
		}
		messages = append(messages, core.Message{
			Role:    "user",
			Content: flattenParsedAnthropicContent(userBlocks),
		})
		userBlocks = userBlocks[:0]
	}

	for _, block := range blocks {
		if block.Type != "tool_result" {
			userBlocks = append(userBlocks, block)
			continue
		}
		flushUserBlocks()
		toolUseID := strings.TrimSpace(block.ToolUseID)
		if toolUseID == "" {
			toolUseID = strings.TrimSpace(block.ID)
		}
		if toolUseID == "" {
			return nil, fmt.Errorf("tool_result message requires tool_use_id")
		}
		messages = append(messages, core.Message{
			Role:       "tool",
			ToolCallID: toolUseID,
			Content:    flattenAnthropicContent(block.Content),
		})
	}
	flushUserBlocks()
	if len(messages) == 0 {
		return []core.Message{{Role: "user", Content: ""}}, nil
	}
	return messages, nil
}

func flattenAnthropicContent(content any) any {
	if content == nil {
		return ""
	}
	if s, ok := content.(string); ok {
		return s
	}

	blocks, err := parseContentBlocks(content)
	if err != nil || len(blocks) == 0 {
		return ""
	}
	return flattenParsedAnthropicContent(blocks)
}

func flattenParsedAnthropicContent(blocks []anthropicIngressContentBlock) any {
	parts := make([]core.ContentPart, 0, len(blocks))
	textParts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text == "" {
				continue
			}
			textParts = append(textParts, b.Text)
			parts = append(parts, core.ContentPart{Type: "text", Text: b.Text})
		case "image":
			part, err := convertImageBlockToContentPart(b)
			if err == nil {
				parts = append(parts, part)
			}
		case "tool_result":
			return flattenAnthropicContent(b.Content)
		case "tool_use":
			continue
		}
	}
	if len(parts) != len(textParts) {
		return parts
	}
	return strings.Join(textParts, "\n")
}

func flattenSystemContent(system any) (string, error) {
	switch v := system.(type) {
	case string:
		return v, nil
	case []any:
		var sb string
		for i, item := range v {
			if i > 0 {
				sb += "\n\n"
			}
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := block["text"].(string); ok {
				sb += text
			}
		}
		return sb, nil
	case map[string]any:
		if text, ok := v["text"].(string); ok {
			return text, nil
		}
	}
	return "", nil
}

func parseContentBlocks(content any) ([]anthropicIngressContentBlock, error) {
	switch v := content.(type) {
	case string:
		return []anthropicIngressContentBlock{{Type: "text", Text: v}}, nil
	case []any:
		blocks := make([]anthropicIngressContentBlock, 0, len(v))
		for _, item := range v {
			data, err := json.Marshal(item)
			if err != nil {
				return nil, err
			}
			var block anthropicIngressContentBlock
			if err := json.Unmarshal(data, &block); err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
		}
		return blocks, nil
	default:
		data, err := json.Marshal(content)
		if err != nil {
			return nil, err
		}
		var block anthropicIngressContentBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}
		return []anthropicIngressContentBlock{block}, nil
	}
}

func convertImageBlockToContentPart(block anthropicIngressContentBlock) (core.ContentPart, error) {
	part := core.ContentPart{
		Type: "image_url",
	}

	if block.Source != nil {
		switch block.Source.Type {
		case "base64":
			part.ImageURL = &core.ImageURLContent{
				URL: fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, block.Source.Data),
			}
		case "url":
			part.ImageURL = &core.ImageURLContent{
				URL: block.Source.URL,
			}
		}
	}

	return part, nil
}

func flattenContentBlockContent(blocks []anthropicIngressContentBlock) any {
	var textParts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			textParts = append(textParts, b.Text)
		case "image":
			part, err := convertImageBlockToContentPart(b)
			if err == nil {
				return []core.ContentPart{part}
			}
		}
	}

	combined := ""
	for i, t := range textParts {
		if i > 0 {
			combined += "\n\n"
		}
		combined += t
	}
	if combined == "" {
		return "{}"
	}
	return combined
}

func addMissingOpenAIToolResponses(messages []core.Message) []core.Message {
	if len(messages) == 0 {
		return messages
	}

	out := make([]core.Message, 0, len(messages))
	for i := 0; i < len(messages); i++ {
		msg := messages[i]
		out = append(out, msg)
		if msg.Role != "assistant" || len(msg.ToolCalls) == 0 {
			continue
		}

		responded := make(map[string]struct{}, len(msg.ToolCalls))
		j := i + 1
		for j < len(messages) {
			next := messages[j]
			if next.Role != "tool" || strings.TrimSpace(next.ToolCallID) == "" {
				break
			}
			responded[next.ToolCallID] = struct{}{}
			out = append(out, next)
			j++
		}

		for _, toolCall := range msg.ToolCalls {
			toolCallID := strings.TrimSpace(toolCall.ID)
			if toolCallID == "" {
				continue
			}
			if _, ok := responded[toolCallID]; ok {
				continue
			}
			out = append(out, core.Message{
				Role:       "tool",
				ToolCallID: toolCallID,
				Content:    "[No response received]",
			})
		}
		i = j - 1
	}
	return out
}

func boolPtr(b bool) *bool {
	return &b
}

package guardrails

import (
	"strings"

	"aurora/internal/core"
)

func chatResponseToMessages(resp *core.ChatResponse) []Message {
	if resp == nil || len(resp.Choices) == 0 {
		return nil
	}
	msgs := make([]Message, 0, len(resp.Choices))
	for _, choice := range resp.Choices {
		role := choice.Message.Role
		if role == "" {
			role = "assistant"
		}
		msgs = append(msgs, Message{
			Role:        role,
			Content:     core.ExtractTextContent(choice.Message.Content),
			ToolCalls:   cloneToolCalls(choice.Message.ToolCalls),
			ContentNull: choice.Message.Content == nil,
		})
	}
	return msgs
}

func applyMessagesToChatResponse(resp *core.ChatResponse, msgs []Message) (*core.ChatResponse, error) {
	if resp == nil {
		return nil, nil
	}
	if len(msgs) != len(resp.Choices) {
		return nil, core.NewInvalidRequestError("guardrails cannot add or remove chat response choices", nil)
	}
	result := *resp
	result.Choices = make([]core.Choice, len(resp.Choices))
	copy(result.Choices, resp.Choices)
	for i := range result.Choices {
		choice := result.Choices[i]
		modified := msgs[i]
		if modified.Role != "" && modified.Role != choice.Message.Role {
			return nil, core.NewInvalidRequestError("guardrails cannot change chat response roles", nil)
		}
		choice.Message = cloneResponseMessage(choice.Message)
		content, contentNull, err := applyGuardedContentToOriginal(choice.Message.Content, modified.Content, modified.ContentNull)
		if err != nil {
			return nil, err
		}
		choice.Message.Content = content
		if contentNull {
			choice.Message.Content = nil
		}
		choice.Message.ToolCalls = cloneToolCalls(modified.ToolCalls)
		result.Choices[i] = choice
	}
	return &result, nil
}

func cloneResponseMessage(message core.ResponseMessage) core.ResponseMessage {
	return core.ResponseMessage{
		Role:        message.Role,
		Content:     cloneMessageContent(message.Content),
		ToolCalls:   cloneToolCalls(message.ToolCalls),
		ExtraFields: core.CloneUnknownJSONFields(message.ExtraFields),
	}
}

func responsesResponseToMessages(resp *core.ResponsesResponse) []Message {
	if resp == nil || len(resp.Output) == 0 {
		return nil
	}
	msgs := make([]Message, 0, len(resp.Output))
	for _, item := range resp.Output {
		if item.Type != "message" {
			continue
		}
		role := item.Role
		if role == "" {
			role = "assistant"
		}
		msgs = append(msgs, Message{Role: role, Content: responsesOutputText(item)})
	}
	return msgs
}

func responsesOutputText(item core.ResponsesOutputItem) string {
	texts := make([]string, 0, len(item.Content))
	for _, content := range item.Content {
		if content.Type == "output_text" || content.Type == "text" {
			if content.Text != "" {
				texts = append(texts, content.Text)
			}
		}
	}
	return strings.Join(texts, " ")
}

func applyMessagesToResponsesResponse(resp *core.ResponsesResponse, msgs []Message) (*core.ResponsesResponse, error) {
	if resp == nil {
		return nil, nil
	}
	result := *resp
	result.Output = make([]core.ResponsesOutputItem, len(resp.Output))
	copy(result.Output, resp.Output)
	msgIndex := 0
	for i := range result.Output {
		if result.Output[i].Type != "message" {
			continue
		}
		if msgIndex >= len(msgs) {
			return nil, core.NewInvalidRequestError("guardrails cannot remove responses output messages", nil)
		}
		modified := msgs[msgIndex]
		msgIndex++
		role := result.Output[i].Role
		if role == "" {
			role = "assistant"
		}
		if modified.Role != "" && modified.Role != role {
			return nil, core.NewInvalidRequestError("guardrails cannot change responses output roles", nil)
		}
		result.Output[i].Content = rewriteResponsesOutputContent(result.Output[i].Content, modified.Content)
	}
	if msgIndex != len(msgs) {
		return nil, core.NewInvalidRequestError("guardrails cannot add responses output messages", nil)
	}
	return &result, nil
}

func rewriteResponsesOutputContent(content []core.ResponsesContentItem, rewrittenText string) []core.ResponsesContentItem {
	if len(content) == 0 {
		if rewrittenText == "" {
			return nil
		}
		return []core.ResponsesContentItem{{Type: "output_text", Text: rewrittenText}}
	}
	out := make([]core.ResponsesContentItem, 0, len(content))
	inserted := false
	for _, item := range content {
		cloned := item
		if item.Type == "output_text" || item.Type == "text" {
			if !inserted && rewrittenText != "" {
				cloned.Text = rewrittenText
				out = append(out, cloned)
				inserted = true
			}
			continue
		}
		out = append(out, cloned)
	}
	if !inserted && rewrittenText != "" {
		out = append([]core.ResponsesContentItem{{Type: "output_text", Text: rewrittenText}}, out...)
	}
	return out
}

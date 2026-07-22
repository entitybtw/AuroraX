package server

import (
	"bufio"
	"bytes"
	json "github.com/goccy/go-json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurora/internal/streaming"
)

type anthropicStreamState int

const (
	anthropicStateInitial anthropicStreamState = iota
	anthropicStateInText
	anthropicStateInThinking
	anthropicStateInToolUse
	anthropicStateFinished
)

const (
	maxAnthropicIngressSSELineBytes      = 1 << 20
	maxAnthropicIngressToolCalls         = 128
	maxAnthropicIngressToolArgumentBytes = 1 << 20
)

type anthropicToolCallBuild struct {
	ID                   string
	Name                 string
	Arguments            strings.Builder
	EmittedArgumentBytes int
	Index                int
	BlockIndex           int
	Started              bool
}

type openaiToAnthropicStream struct {
	reader *bufio.Reader
	body   io.ReadCloser
	buffer streaming.StreamBuffer
	closed bool

	msgID           string
	model           string
	msgStarted      bool
	textBuilder     strings.Builder
	thinkingBuilder strings.Builder

	nextContentIndex  int
	currentBlockIndex int
	state             anthropicStreamState
	toolCalls         map[int]*anthropicToolCallBuild
	nextToolCallIdx   int

	promptTokens     int
	completionTokens int
	hasUsage         bool
	hasToolUse       bool
}

func newOpenAIToAnthropicStream(body io.ReadCloser, model string) *openaiToAnthropicStream {
	return &openaiToAnthropicStream{
		reader:    bufio.NewReader(body),
		body:      body,
		buffer:    streaming.NewStreamBuffer(1024),
		model:     model,
		toolCalls: make(map[int]*anthropicToolCallBuild),
		state:     anthropicStateInitial,
	}
}

func (s *openaiToAnthropicStream) Read(p []byte) (n int, err error) {
	if s.buffer.Len() > 0 {
		return s.buffer.Read(p), nil
	}

	if s.closed {
		s.release()
		return 0, io.EOF
	}

	for {
		line, err := s.readBoundedLine()
		if err != nil {
			if err == io.EOF {
				if s.state != anthropicStateFinished {
					s.finishStream("")
				}
				n = s.buffer.Read(p)
				s.closed = true
				_ = s.body.Close()
				return n, nil
			}
			return 0, err
		}

		lineStr := string(line)
		lineStr = strings.TrimSpace(lineStr)

		if lineStr == "" || lineStr == "data: [DONE]" {
			if lineStr == "data: [DONE]" {
				if s.state != anthropicStateFinished {
					s.finishStream("")
				}
				n = s.buffer.Read(p)
				s.closed = true
				_ = s.body.Close()
				return n, nil
			}
			continue
		}

		if !strings.HasPrefix(lineStr, "data: ") {
			continue
		}

		data := strings.TrimPrefix(lineStr, "data: ")

		var chunk struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Model   string `json:"model"`
			Created int64  `json:"created"`
			Choices []struct {
				Index        int            `json:"index"`
				Delta        map[string]any `json:"delta"`
				FinishReason any            `json:"finish_reason"`
				Logprobs     any            `json:"logprobs,omitempty"`
			} `json:"choices"`
			Usage map[string]any `json:"usage,omitempty"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			var errEvent struct {
				Error map[string]any `json:"error"`
			}
			if json.Unmarshal([]byte(data), &errEvent) == nil && errEvent.Error != nil {
				slog.Warn("upstream returned SSE error event",
					"error_type", safeStreamErrorField(errEvent.Error["type"]),
					"error_code", safeStreamErrorField(errEvent.Error["code"]),
					"model", s.model,
				)
				continue
			}
			return 0, fmt.Errorf("failed to decode upstream SSE chunk: %w", err)
		}

		if chunk.ID != "" && s.msgID == "" {
			s.msgID = chunk.ID
		}

		if len(chunk.Choices) == 0 {
			if chunk.Usage != nil {
				s.captureUsage(chunk.Usage)
			}
			continue
		}

		delta := chunk.Choices[0].Delta
		finishReason := chunk.Choices[0].FinishReason

		if err := s.processDelta(delta); err != nil {
			return 0, err
		}

		if finishReason != nil {
			reason, _ := finishReason.(string)
			s.finishStream(reason)
			n = s.buffer.Read(p)
			s.closed = true
			_ = s.body.Close()
			return n, nil
		}
	}
}

func (s *openaiToAnthropicStream) Close() error {
	if s.closed {
		s.release()
		return nil
	}
	s.closed = true
	s.release()
	return s.body.Close()
}

func (s *openaiToAnthropicStream) release() {
	s.buffer.Release()
}

func (s *openaiToAnthropicStream) readBoundedLine() ([]byte, error) {
	var out bytes.Buffer
	for {
		fragment, err := s.reader.ReadSlice('\n')
		if len(fragment) > 0 {
			if out.Len()+len(fragment) > maxAnthropicIngressSSELineBytes {
				return nil, fmt.Errorf("anthropic ingress stream SSE line exceeds maximum size")
			}
			_, _ = out.Write(fragment)
		}
		if err == nil {
			return out.Bytes(), nil
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err == io.EOF && out.Len() > 0 {
			return out.Bytes(), nil
		}
		return nil, err
	}
}

func (s *openaiToAnthropicStream) processDelta(delta map[string]any) error {
	if role, ok := delta["role"].(string); ok && role == "assistant" && s.state == anthropicStateInitial {
		s.emitMessageStart()
	}

	if content, ok := delta["content"].(string); ok && content != "" {
		if s.state != anthropicStateInText {
			s.flushPendingContent()
			s.emitContentBlockStartText()
			s.state = anthropicStateInText
		}
		s.textBuilder.WriteString(content)
		s.emitTextDelta(content)
		return nil
	}

	if reasoning, ok := delta["reasoning_content"].(string); ok && reasoning != "" {
		if !s.msgStarted {
			s.emitMessageStart()
		}
		if s.state != anthropicStateInThinking {
			s.flushPendingContent()
			s.emitContentBlockStartThinking()
			s.state = anthropicStateInThinking
		}
		s.thinkingBuilder.WriteString(reasoning)
		s.emitThinkingDelta(reasoning)
		return nil
	}

	if tcData, ok := delta["tool_calls"]; ok {
		tcs, ok := tcData.([]any)
		if !ok {
			return nil
		}
		for _, tcItem := range tcs {
			tc, ok := tcItem.(map[string]any)
			if !ok {
				continue
			}
			if err := s.processToolCallDelta(tc); err != nil {
				return err
			}
		}
		return nil
	}

	_ = delta
	return nil
}

func (s *openaiToAnthropicStream) processToolCallDelta(tc map[string]any) error {
	idx, ok := parseToolCallIndex(tc["index"])
	if !ok || idx < 0 || idx >= maxAnthropicIngressToolCalls {
		return fmt.Errorf("anthropic ingress stream tool call index is out of range")
	}

	state, exists := s.toolCalls[idx]
	if !exists {
		if len(s.toolCalls) >= maxAnthropicIngressToolCalls {
			return fmt.Errorf("anthropic ingress stream exceeded maximum tool calls")
		}
		state = &anthropicToolCallBuild{Index: s.nextToolCallIdx}
		s.nextToolCallIdx++
		s.toolCalls[idx] = state
	}

	if id, ok := tc["id"].(string); ok && id != "" && state.ID == "" {
		state.ID = id
	}

	if fn, ok := tc["function"].(map[string]any); ok {
		if name, ok := fn["name"].(string); ok && name != "" && state.Name == "" {
			state.Name = name
		}
		if args, ok := fn["arguments"].(string); ok && args != "" {
			if state.Arguments.Len()+len(args) > maxAnthropicIngressToolArgumentBytes {
				return fmt.Errorf("anthropic ingress stream tool arguments exceed maximum size")
			}
			state.Arguments.WriteString(args)
		}
	}

	s.emitReadyToolCallDeltas(state)
	return nil
}

func parseToolCallIndex(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v {
			return 0, false
		}
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
}

func (s *openaiToAnthropicStream) emitReadyToolCallDeltas(state *anthropicToolCallBuild) {
	if state == nil || state.ID == "" || state.Name == "" {
		return
	}
	if !state.Started {
		if !s.msgStarted {
			s.emitMessageStart()
		}
		if s.state != anthropicStateInToolUse {
			s.flushPendingContent()
			s.state = anthropicStateInToolUse
		}
		s.emitContentBlockStartTool(state)
		state.Started = true
		s.hasToolUse = true
	}
	if state.Arguments.Len() <= state.EmittedArgumentBytes {
		return
	}
	if isAnthropicIngressReadTool(state.Name) {
		return
	}
	args := state.Arguments.String()
	delta := args[state.EmittedArgumentBytes:]
	state.EmittedArgumentBytes = len(args)
	s.emitInputJSONDelta(state, delta)
}

func (s *openaiToAnthropicStream) flushPendingContent() {
	switch s.state {
	case anthropicStateInText:
		s.textBuilder.Reset()
		s.emitContentBlockStop()
	case anthropicStateInThinking:
		s.thinkingBuilder.Reset()
		s.emitContentBlockStop()
	case anthropicStateInToolUse:
		calls := make([]*anthropicToolCallBuild, 0, len(s.toolCalls))
		for _, tc := range s.toolCalls {
			if tc.Started {
				calls = append(calls, tc)
			}
		}
		sort.Slice(calls, func(i, j int) bool {
			return calls[i].BlockIndex < calls[j].BlockIndex
		})
		for _, tc := range calls {
			s.emitBufferedToolArguments(tc)
			s.emitToolUseContentBlockStop(tc)
		}
		s.toolCalls = make(map[int]*anthropicToolCallBuild)
	case anthropicStateInitial:
	default:
	}
	s.state = anthropicStateInitial
}

func (s *openaiToAnthropicStream) emitBufferedToolArguments(tc *anthropicToolCallBuild) {
	if tc == nil || tc.Arguments.Len() <= tc.EmittedArgumentBytes {
		return
	}
	args := tc.Arguments.String()
	if isAnthropicIngressReadTool(tc.Name) {
		args = sanitizeAnthropicIngressReadToolArgs(args)
	}
	delta := args[tc.EmittedArgumentBytes:]
	tc.EmittedArgumentBytes = len(args)
	if delta != "" {
		s.emitInputJSONDelta(tc, delta)
	}
}

func isAnthropicIngressReadTool(name string) bool {
	return strings.TrimPrefix(name, "proxy_") == "Read"
}

func sanitizeAnthropicIngressReadToolArgs(argsJSON string) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return argsJSON
	}
	if limit, ok := args["limit"].(string); ok {
		if parsed, err := strconv.Atoi(limit); err == nil {
			args["limit"] = parsed
		}
	}
	if offset, ok := args["offset"].(string); ok {
		if parsed, err := strconv.Atoi(offset); err == nil {
			args["offset"] = parsed
		}
	}
	if limit, ok := args["limit"].(float64); ok {
		args["limit"] = int(limit)
	}
	if offset, ok := args["offset"].(float64); ok {
		args["offset"] = int(offset)
	}
	if limit, ok := args["limit"].(int); ok {
		if limit > 2000 {
			args["limit"] = 2000
		} else if limit < 1 {
			delete(args, "limit")
		}
	}
	if offset, ok := args["offset"].(int); ok && offset < 0 {
		args["offset"] = 0
	}
	if !isValidAnthropicIngressPDFPagesArg(args["file_path"], args["pages"]) {
		delete(args, "pages")
	}
	out, err := json.Marshal(args)
	if err != nil {
		return argsJSON
	}
	return string(out)
}

func isValidAnthropicIngressPDFPagesArg(filePathValue, pagesValue any) bool {
	filePath, ok := filePathValue.(string)
	if !ok || !strings.HasSuffix(strings.ToLower(filePath), ".pdf") {
		return false
	}
	pages, ok := pagesValue.(string)
	if !ok || pages == "" {
		return false
	}
	parts := strings.Split(pages, "-")
	if len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}

func (s *openaiToAnthropicStream) finishStream(stopReason string) {
	if s.state == anthropicStateFinished {
		return
	}
	if strings.TrimSpace(stopReason) == "" {
		if s.hasToolUse {
			stopReason = "tool_calls"
		} else {
			stopReason = "stop"
		}
	}
	s.flushPendingContent()
	s.ensureContentBlock()
	s.emitMessageDelta(stopReason)
	s.emitMessageStop()
	s.state = anthropicStateFinished
}

func (s *openaiToAnthropicStream) emitMessageStart() {
	if s.msgStarted {
		return
	}
	s.msgStarted = true
	if s.msgID == "" {
		s.msgID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
	}
	msg := map[string]any{
		"id":            s.msgID,
		"type":          "message",
		"role":          "assistant",
		"content":       []any{},
		"model":         s.model,
		"stop_reason":   nil,
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":  0,
			"output_tokens": 0,
		},
	}
	data, _ := json.Marshal(msg)
	s.writeSSEEvent("message_start", data)
	s.nextContentIndex = 0
}

func (s *openaiToAnthropicStream) emitContentBlockStartText() {
	s.currentBlockIndex = s.nextContentIndex
	s.nextContentIndex++
	block := map[string]any{
		"type": "text",
		"text": "",
	}
	data, _ := json.Marshal(block)
	s.writeContentBlockEvent("content_block_start", data)
}

func (s *openaiToAnthropicStream) emitContentBlockStartThinking() {
	s.currentBlockIndex = s.nextContentIndex
	s.nextContentIndex++
	block := map[string]any{
		"type":      "thinking",
		"thinking":  "",
		"signature": "",
	}
	data, _ := json.Marshal(block)
	s.writeContentBlockEvent("content_block_start", data)
}

func (s *openaiToAnthropicStream) emitContentBlockStartTool(tc *anthropicToolCallBuild) {
	tc.BlockIndex = s.nextContentIndex
	s.currentBlockIndex = tc.BlockIndex
	s.nextContentIndex++
	block := map[string]any{
		"type":  "tool_use",
		"id":    tc.ID,
		"name":  tc.Name,
		"input": "{}",
		"text":  "",
	}
	data, _ := json.Marshal(block)
	s.writeContentBlockEvent("content_block_start", data)
}

func (s *openaiToAnthropicStream) emitTextDelta(text string) {
	delta := map[string]any{
		"type": "text_delta",
		"text": text,
	}
	data, _ := json.Marshal(delta)
	s.writeContentBlockEvent("content_block_delta", data)
}

func (s *openaiToAnthropicStream) emitThinkingDelta(text string) {
	delta := map[string]any{
		"type":     "thinking_delta",
		"thinking": text,
	}
	data, _ := json.Marshal(delta)
	s.writeContentBlockEvent("content_block_delta", data)
}

func (s *openaiToAnthropicStream) emitInputJSONDelta(tc *anthropicToolCallBuild, args string) {
	delta := map[string]any{
		"type":         "input_json_delta",
		"partial_json": args,
	}
	data, _ := json.Marshal(delta)
	s.writeContentBlockEventAtIndex("content_block_delta", tc.BlockIndex, data)
}

func (s *openaiToAnthropicStream) emitContentBlockStop() {
	data, _ := json.Marshal(struct{}{})
	s.writeContentBlockEvent("content_block_stop", data)
}

func (s *openaiToAnthropicStream) emitToolUseContentBlockStop(tc *anthropicToolCallBuild) {
	data, _ := json.Marshal(struct{}{})
	s.writeContentBlockEventAtIndex("content_block_stop", tc.BlockIndex, data)
}

func (s *openaiToAnthropicStream) emitMessageDelta(stopReason string) {
	anthropicReason := mapOpenAIFinishToAnthropic(stopReason)
	delta := map[string]any{
		"stop_reason":   anthropicReason,
		"stop_sequence": nil,
	}
	usage := map[string]any{
		"input_tokens":  s.promptTokens,
		"output_tokens": s.completionTokens,
	}
	payload := map[string]any{
		"type":  "message_delta",
		"delta": delta,
		"usage": usage,
	}
	data, _ := json.Marshal(payload)
	s.buffer.AppendString(fmt.Sprintf("event: message_delta\ndata: %s\n\n", data))
}

func (s *openaiToAnthropicStream) emitMessageStop() {
	data, _ := json.Marshal(struct{}{})
	s.buffer.AppendString(fmt.Sprintf("event: message_stop\ndata: %s\n\n", data))
}

func (s *openaiToAnthropicStream) ensureContentBlock() {
	if s.nextContentIndex == 0 {
		slog.Warn("anthropic stream: upstream returned zero content blocks, emitting empty text block",
			"model", s.model,
		)
		s.emitContentBlockStartText()
		s.emitContentBlockStop()
	}
}

func (s *openaiToAnthropicStream) writeSSEEvent(eventType string, data []byte) {
	s.buffer.AppendString(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data))
}

func (s *openaiToAnthropicStream) writeContentBlockEvent(eventType string, data []byte) {
	s.writeContentBlockEventAtIndex(eventType, s.currentBlockIndex, data)
}

func (s *openaiToAnthropicStream) writeContentBlockEventAtIndex(eventType string, idx int, data []byte) {
	payload := map[string]any{
		"type":          eventType,
		"index":         idx,
		"content_block": nil,
	}
	if eventType == "content_block_start" {
		var cb map[string]any
		if err := json.Unmarshal(data, &cb); err == nil {
			payload["content_block"] = cb
		}
	} else if eventType == "content_block_delta" {
		var delta map[string]any
		if err := json.Unmarshal(data, &delta); err == nil {
			payload["delta"] = delta
		}
	} else if eventType == "content_block_stop" {
	} else {
	}
	raw, _ := json.Marshal(payload)
	s.buffer.AppendString(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, raw))
}

func (s *openaiToAnthropicStream) captureUsage(usage map[string]any) {
	s.hasUsage = true
	if v, ok := usage["prompt_tokens"]; ok {
		switch n := v.(type) {
		case float64:
			s.promptTokens = int(n)
		case int:
			s.promptTokens = n
		}
	}
	if v, ok := usage["completion_tokens"]; ok {
		switch n := v.(type) {
		case float64:
			s.completionTokens = int(n)
		case int:
			s.completionTokens = n
		}
	}
}

func safeStreamErrorField(value any) string {
	s, ok := value.(string)
	if !ok {
		return ""
	}
	return truncateString(s, 120)
}

func extractAnthropicContentIndex(line []byte) (int, bool) {
	var event struct {
		Index int `json:"index"`
	}
	if err := json.Unmarshal(line, &event); err == nil {
		return event.Index, true
	}
	return 0, false
}

func parseIntFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

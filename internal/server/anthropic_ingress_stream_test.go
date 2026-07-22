package server

import (
	json "github.com/goccy/go-json"
	"io"
	"strings"
	"testing"
)

func TestOpenAIToAnthropicStreamEOFWithoutDoneEmitsOneMessageStop(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
		"",
	}, "\n")

	out := readAnthropicStream(t, raw)
	if got := strings.Count(out, "event: message_stop"); got != 1 {
		t.Fatalf("message_stop count = %d, want 1; output=%s", got, out)
	}
}

func TestOpenAIToAnthropicStreamThinkingThenTextUsesSeparateOrderedBlocks(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"reasoning_content":"think"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"content":"answer"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"",
	}, "\n")

	events := parseAnthropicSSEEvents(t, readAnthropicStream(t, raw))
	thinkingStart := findContentBlockStart(t, events, "thinking")
	textStart := findContentBlockStart(t, events, "text")
	if thinkingStart.Index != 0 {
		t.Fatalf("thinking index = %d, want 0", thinkingStart.Index)
	}
	if textStart.Index != 1 {
		t.Fatalf("text index = %d, want 1", textStart.Index)
	}
	if !eventBefore(events, "content_block_stop", 0, "content_block_start", 1) {
		t.Fatalf("thinking block stop should occur before text block start; events=%+v", events)
	}
}

func TestOpenAIToAnthropicStreamParallelToolCallsUseIndependentContentBlockIndexes(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"foo","arguments":"{\"a\":"}},{"index":1,"id":"call_b","function":{"name":"bar","arguments":"{\"b\":"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}},{"index":1,"function":{"arguments":"2}"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		"",
	}, "\n")

	events := parseAnthropicSSEEvents(t, readAnthropicStream(t, raw))
	starts := contentBlockStarts(events, "tool_use")
	if len(starts) != 2 {
		t.Fatalf("tool_use starts = %d, want 2; events=%+v", len(starts), events)
	}
	if starts[0].Index == starts[1].Index {
		t.Fatalf("tool_use block indexes should be distinct, got %d and %d", starts[0].Index, starts[1].Index)
	}
	if !hasDeltaForIndex(events, starts[0].Index, "input_json_delta") || !hasDeltaForIndex(events, starts[1].Index, "input_json_delta") {
		t.Fatalf("expected input_json_delta for both tool indexes; events=%+v", events)
	}
}

func TestOpenAIToAnthropicStreamUsageOnlyChunkBeforeDonePreservedInMessageDelta(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[],"usage":{"prompt_tokens":7,"completion_tokens":3}}`,
		`data: [DONE]`,
		"",
	}, "\n")

	events := parseAnthropicSSEEvents(t, readAnthropicStream(t, raw))
	messageDelta := findEvent(t, events, "message_delta")
	if messageDelta.Usage == nil {
		t.Fatal("message_delta usage is nil")
	}
	if messageDelta.Usage.InputTokens != 7 || messageDelta.Usage.OutputTokens != 3 {
		t.Fatalf("usage = %+v, want input=7 output=3", *messageDelta.Usage)
	}
}

func TestOpenAIToAnthropicStreamToolUseDoneWithoutFinishReasonStopsAsToolUse(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"foo","arguments":"{\"a\":1}"}}]},"finish_reason":null}]}`,
		`data: [DONE]`,
		"",
	}, "\n")

	events := parseAnthropicSSEEvents(t, readAnthropicStream(t, raw))
	messageDelta := findEvent(t, events, "message_delta")
	if messageDelta.Delta == nil || messageDelta.Delta.StopReason != "tool_use" {
		t.Fatalf("message_delta stop_reason = %+v, want tool_use", messageDelta.Delta)
	}
}

func TestOpenAIToAnthropicStreamBuffersToolArgumentsUntilMetadata(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"a\":"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"foo","arguments":"1}"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		"",
	}, "\n")

	events := parseAnthropicSSEEvents(t, readAnthropicStream(t, raw))
	starts := contentBlockStarts(events, "tool_use")
	if len(starts) != 1 {
		t.Fatalf("tool_use starts = %d, want 1; events=%+v", len(starts), events)
	}
	if starts[0].ContentBlock.ID != "call_a" || starts[0].ContentBlock.Name != "foo" {
		t.Fatalf("tool metadata = %+v, want call_a/foo", starts[0].ContentBlock)
	}
	if !hasDeltaForIndex(events, starts[0].Index, "input_json_delta") {
		t.Fatalf("missing buffered input_json_delta for tool block; events=%+v", events)
	}
}

func TestOpenAIToAnthropicStreamEmitsSubsequentToolArgumentDeltas(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"foo","arguments":"{\"a\":"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		"",
	}, "\n")

	events := parseAnthropicSSEEvents(t, readAnthropicStream(t, raw))
	starts := contentBlockStarts(events, "tool_use")
	if len(starts) != 1 {
		t.Fatalf("tool_use starts = %d, want 1; events=%+v", len(starts), events)
	}
	deltas := inputJSONDeltasForIndex(events, starts[0].Index)
	if strings.Join(deltas, "") != `{"a":1}` {
		t.Fatalf("tool argument deltas = %#v, want full JSON", deltas)
	}
}

func TestOpenAIToAnthropicStreamSanitizesReadToolArgumentsLike9router(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_read","function":{"name":"Read","arguments":"{\"file_path\":\"notes.txt\",\"limit\":\"5000\",\"offset\":\"-4\",\"pages\":\"1-2\"}"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		"",
	}, "\n")

	events := parseAnthropicSSEEvents(t, readAnthropicStream(t, raw))
	starts := contentBlockStarts(events, "tool_use")
	if len(starts) != 1 {
		t.Fatalf("tool_use starts = %d, want 1; events=%+v", len(starts), events)
	}
	deltas := inputJSONDeltasForIndex(events, starts[0].Index)
	got := strings.Join(deltas, "")
	for _, want := range []string{`"limit":2000`, `"offset":0`} {
		if !strings.Contains(got, want) {
			t.Fatalf("sanitized Read args missing %s: %s", want, got)
		}
	}
	if strings.Contains(got, `"pages"`) {
		t.Fatalf("sanitized Read args should drop pages for non-PDF path: %s", got)
	}
}

func TestOpenAIToAnthropicStreamNoArgumentToolCallIsPreserved(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"foo"}}]},"finish_reason":null}]}`,
		`data: [DONE]`,
		"",
	}, "\n")

	events := parseAnthropicSSEEvents(t, readAnthropicStream(t, raw))
	starts := contentBlockStarts(events, "tool_use")
	if len(starts) != 1 {
		t.Fatalf("tool_use starts = %d, want 1; events=%+v", len(starts), events)
	}
	messageDelta := findEvent(t, events, "message_delta")
	if messageDelta.Delta == nil || messageDelta.Delta.StopReason != "tool_use" {
		t.Fatalf("message_delta stop_reason = %+v, want tool_use", messageDelta.Delta)
	}
}

func TestOpenAIToAnthropicStreamMalformedToolCallIndexReturnsError(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":1.5,"id":"call_a","function":{"name":"foo"}}]},"finish_reason":null}]}`,
		"",
	}, "\n")

	if err := readAnthropicStreamError(raw); err == nil {
		t.Fatal("expected malformed tool-call index error, got nil")
	}
}

func TestOpenAIToAnthropicStreamMalformedJSONReturnsError(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"id":"chatcmpl_1","model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"broken"`,
		"",
	}, "\n")

	if err := readAnthropicStreamError(raw); err == nil {
		t.Fatal("expected malformed stream error, got nil")
	}
}

func readAnthropicStream(t *testing.T, raw string) string {
	t.Helper()
	stream := newOpenAIToAnthropicStream(io.NopCloser(strings.NewReader(raw)), "claude-sonnet")
	defer func() { _ = stream.Close() }()
	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("failed to read stream: %v", err)
	}
	return string(data)
}

func readAnthropicStreamError(raw string) error {
	stream := newOpenAIToAnthropicStream(io.NopCloser(strings.NewReader(raw)), "claude-sonnet")
	defer func() { _ = stream.Close() }()
	_, err := io.ReadAll(stream)
	return err
}

type anthropicTestEvent struct {
	Name         string
	Index        int
	Delta        *anthropicIngressDelta
	ContentBlock *anthropicIngressContentBlock
	Usage        *anthropicIngressUsage
}

func parseAnthropicSSEEvents(t *testing.T, raw string) []anthropicTestEvent {
	t.Helper()
	var events []anthropicTestEvent
	currentName := ""
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if after, ok := strings.CutPrefix(line, "event:"); ok {
			currentName = strings.TrimSpace(after)
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var event anthropicTestEvent
		event.Name = currentName
		var payload struct {
			Index        int                           `json:"index"`
			Delta        *anthropicIngressDelta        `json:"delta"`
			ContentBlock *anthropicIngressContentBlock `json:"content_block"`
			Usage        *anthropicIngressUsage        `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			t.Fatalf("failed to unmarshal event %q data %q: %v", currentName, data, err)
		}
		event.Index = payload.Index
		event.Delta = payload.Delta
		event.ContentBlock = payload.ContentBlock
		event.Usage = payload.Usage
		events = append(events, event)
		currentName = ""
	}
	return events
}

func findContentBlockStart(t *testing.T, events []anthropicTestEvent, blockType string) anthropicTestEvent {
	t.Helper()
	for _, event := range events {
		if event.Name == "content_block_start" && event.ContentBlock != nil && event.ContentBlock.Type == blockType {
			return event
		}
	}
	t.Fatalf("missing content_block_start for %q; events=%+v", blockType, events)
	return anthropicTestEvent{}
}

func contentBlockStarts(events []anthropicTestEvent, blockType string) []anthropicTestEvent {
	var out []anthropicTestEvent
	for _, event := range events {
		if event.Name == "content_block_start" && event.ContentBlock != nil && event.ContentBlock.Type == blockType {
			out = append(out, event)
		}
	}
	return out
}

func hasDeltaForIndex(events []anthropicTestEvent, index int, deltaType string) bool {
	for _, event := range events {
		if event.Name == "content_block_delta" && event.Index == index && event.Delta != nil && event.Delta.Type == deltaType {
			return true
		}
	}
	return false
}

func inputJSONDeltasForIndex(events []anthropicTestEvent, index int) []string {
	var deltas []string
	for _, event := range events {
		if event.Name == "content_block_delta" && event.Index == index && event.Delta != nil && event.Delta.Type == "input_json_delta" {
			deltas = append(deltas, event.Delta.PartialJSON)
		}
	}
	return deltas
}

func findEvent(t *testing.T, events []anthropicTestEvent, name string) anthropicTestEvent {
	t.Helper()
	for _, event := range events {
		if event.Name == name {
			return event
		}
	}
	t.Fatalf("missing event %q; events=%+v", name, events)
	return anthropicTestEvent{}
}

func eventBefore(events []anthropicTestEvent, firstName string, firstIndex int, secondName string, secondIndex int) bool {
	firstPos := -1
	secondPos := -1
	for i, event := range events {
		if event.Name == firstName && event.Index == firstIndex && firstPos < 0 {
			firstPos = i
		}
		if event.Name == secondName && event.Index == secondIndex && secondPos < 0 {
			secondPos = i
		}
	}
	return firstPos >= 0 && secondPos >= 0 && firstPos < secondPos
}

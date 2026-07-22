package streaming

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

type captureObserver struct {
	events  int
	lastID  string
	payload map[string]any
	done    bool
}

func (o *captureObserver) OnJSONEvent(payload map[string]any) {
	o.events++
	o.payload = payload
	if id, _ := payload["id"].(string); id != "" {
		o.lastID = id
	}
}

func (o *captureObserver) OnStreamClose() {
	o.done = true
}

func TestSSEObserver_PassthroughAndFanout(t *testing.T) {
	input := `data: {"id":"msg-1","choices":[{"delta":{"content":"hi"}}]}

data: {"id":"msg-2","usage":{"total_tokens":3}}

data: [DONE]

`
	a := &captureObserver{}
	b := &captureObserver{}
	stream := NewObservedSSEStream(io.NopCloser(strings.NewReader(input)), a, b)

	raw, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(raw) != input {
		t.Fatalf("bytes mismatch:\ngot:  %q\nwant: %q", string(raw), input)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for i, obs := range []*captureObserver{a, b} {
		if obs.events != 2 {
			t.Fatalf("observer %d events=%d want 2", i, obs.events)
		}
		if obs.lastID != "msg-2" {
			t.Fatalf("observer %d lastID=%q want msg-2", i, obs.lastID)
		}
		if !obs.done {
			t.Fatalf("observer %d not closed", i)
		}
	}
}

func TestSSEObserver_ParsesFragmentOnClose(t *testing.T) {
	input := `data: {"id":"frag-event","usage":{"total_tokens":8}}`
	obs := &captureObserver{}
	stream := NewObservedSSEStream(io.NopCloser(strings.NewReader(input)), obs)

	raw, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(raw) != input {
		t.Fatalf("bytes mismatch")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if obs.events != 1 {
		t.Fatalf("events=%d want 1", obs.events)
	}
	if obs.lastID != "frag-event" {
		t.Fatalf("lastID=%q want frag-event", obs.lastID)
	}
	if !obs.done {
		t.Fatal("not closed")
	}
}

func TestSSEObserver_MultilineData(t *testing.T) {
	input := "data: {\"id\":\"multi-line\",\n" +
		"data: \"usage\":{\"total_tokens\":3}}\n\n" +
		"data: [DONE]\n\n"
	obs := &captureObserver{}
	stream := NewObservedSSEStream(io.NopCloser(strings.NewReader(input)), obs)

	raw, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(raw) != input {
		t.Fatalf("bytes mismatch")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if obs.events != 1 {
		t.Fatalf("events=%d want 1", obs.events)
	}
	if obs.lastID != "multi-line" {
		t.Fatalf("lastID=%q want multi-line", obs.lastID)
	}
	usage, ok := obs.payload["usage"].(map[string]any)
	if !ok {
		t.Fatalf("usage type=%T", obs.payload["usage"])
	}
	if got := usage["total_tokens"]; got != float64(3) {
		t.Fatalf("total_tokens=%v want 3", got)
	}
}

func TestSSEObserver_CRLFVariants(t *testing.T) {
	input := "data:{\"id\":\"first\"}\r\n\r\ndata: {\"id\":\"second\"}\r\n\r\ndata:[DONE]\r\n\r\n"
	obs := &captureObserver{}
	stream := NewObservedSSEStream(io.NopCloser(strings.NewReader(input)), obs)

	raw, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(raw) != input {
		t.Fatalf("bytes mismatch")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if obs.events != 2 {
		t.Fatalf("events=%d want 2", obs.events)
	}
	if obs.lastID != "second" {
		t.Fatalf("lastID=%q want second", obs.lastID)
	}
	if !obs.done {
		t.Fatal("not closed")
	}
}

func TestSSEObserver_NilObserverFiltered(t *testing.T) {
	obs := &captureObserver{}
	stream := NewObservedSSEStream(io.NopCloser(strings.NewReader("data: {}\n\ndata: {}\n\n")), nil, obs, nil)
	raw, _ := io.ReadAll(stream)
	stream.Close()
	if string(raw) == "" {
		t.Fatal("expected passthrough bytes")
	}
	if obs.events != 2 {
		t.Fatalf("events=%d want 2", obs.events)
	}
}

func TestSSEObserver_NoObserversPassthrough(t *testing.T) {
	input := "data: {}\n\n"
	inner := io.NopCloser(strings.NewReader(input))
	stream := NewObservedSSEStream(inner)
	if stream != inner {
		t.Fatal("expected original stream when no observers")
	}
}

func TestSSEObserver_EmptyPayloadIgnored(t *testing.T) {
	obs := &captureObserver{}
	stream := NewObservedSSEStream(io.NopCloser(strings.NewReader("data: {}\n\ndata:  \ndata:{}")), obs)
	_, _ = io.ReadAll(stream)
	stream.Close()
	if obs.events != 2 {
		t.Fatalf("events=%d want 2 (empty data line skipped)", obs.events)
	}
}

func TestSSEObserver_GarbageBeforeBoundaryRecovers(t *testing.T) {
	obs := &captureObserver{}
	input := strings.Repeat("x", sseEventMaxBytes+100) + "\n\ndata: {\"id\":\"recovered\"}\n\n"
	stream := NewObservedSSEStream(io.NopCloser(strings.NewReader(input)), obs)
	_, _ = io.ReadAll(stream)
	stream.Close()
	if obs.events != 1 {
		t.Fatalf("events=%d want 1", obs.events)
	}
	if obs.lastID != "recovered" {
		t.Fatalf("lastID=%q want recovered", obs.lastID)
	}
}

func TestSSEObserver_ChunkedBoundary(t *testing.T) {
	obs := &captureObserver{}
	r := NewObservedSSEStream(io.NopCloser(strings.NewReader("data:{\"id\":\"a\"}\n\n")), obs)
	r.(*SSEEventReader).buf = r.(*SSEEventReader).buf[:0]

	r.(*SSEEventReader).ingest([]byte("data:{\"id\":\"a\"}\n"))
	r.(*SSEEventReader).ingest([]byte("\ndata:{\"id\":\"b\"}\n\n"))

	if obs.events != 2 {
		t.Fatalf("events=%d want 2", obs.events)
	}
	if obs.lastID != "b" {
		t.Fatalf("lastID=%q want b", obs.lastID)
	}
}

func TestSSEObserver_ChunkedCRLF(t *testing.T) {
	obs := &captureObserver{}
	r := &SSEEventReader{
		source:   io.NopCloser(strings.NewReader("")),
		watchers: []Observer{obs},
	}

	r.ingest([]byte("data:{\"id\":\"a\"}\r\n\r"))
	r.ingest([]byte("\ndata:{\"id\":\"b\"}\r\n\r\n"))

	if obs.events != 2 {
		t.Fatalf("events=%d want 2", obs.events)
	}
	if obs.lastID != "b" {
		t.Fatalf("lastID=%q want b", obs.lastID)
	}
}

func TestStripDataPrefix(t *testing.T) {
	cases := []struct {
		input []byte
		want  []byte
	}{
		{[]byte("data: hello"), []byte("hello")},
		{[]byte("data:{\"id\":1}"), []byte("{\"id\":1}")},
		{[]byte("data:  spaced"), []byte(" spaced")},
		{[]byte("event: ping"), nil},
		{[]byte(":comment"), nil},
		{[]byte("data:\r\n"), []byte("")},
	}
	for _, c := range cases {
		got := stripDataPrefix(c.input)
		if !bytes.Equal(got, c.want) {
			t.Errorf("stripDataPrefix(%q) = %q want %q", c.input, got, c.want)
		}
	}
}

func TestBufferFullCycle(t *testing.T) {
	b := NewChunkBuffer(16)
	defer b.Release()

	b.AppendString("hello")
	if b.Len() != 5 {
		t.Fatalf("Len=%d want 5", b.Len())
	}

	out := make([]byte, 2)
	if n := b.Read(out); n != 2 {
		t.Fatalf("Read=%d want 2", n)
	}
	if string(out) != "he" {
		t.Fatalf("Read got %q want he", string(out))
	}

	b.AppendString(" world")
	if got := string(b.Unread()); got != "llo world" {
		t.Fatalf("Unread=%q want llo world", got)
	}

	b.Discard(4)
	if got := string(b.Unread()); got != "world" {
		t.Fatalf("after Discard=%q want world", got)
	}
}

func TestChunkBufferReleaseIdempotent(t *testing.T) {
	b := NewChunkBuffer(8)
	b.AppendString("data")
	b.Release()
	b.Release()
	if b.Len() != 0 {
		t.Fatalf("Len after Release=%d want 0", b.Len())
	}
}

func TestChunkBufferPoolAfterGrowth(t *testing.T) {
	b := NewChunkBuffer(8)
	handle := b.handle
	if handle == nil {
		t.Fatal("handle is nil")
	}
	origCap := cap(*handle)

	b.AppendString(strings.Repeat("x", chunkMaxCached+1))
	b.Release()

	if *handle == nil {
		t.Fatal("pooled slice nil after release")
	}
	if cap(*handle) != origCap {
		t.Fatalf("cap after release=%d want %d", cap(*handle), origCap)
	}
}

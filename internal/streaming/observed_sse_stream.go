package streaming

import (
	"bytes"
	"encoding/json"
	"io"
)

const sseEventMaxBytes = 256 * 1024

var (
	boundaryLF   = []byte{'\n', '\n'}
	boundaryCRLF = []byte{'\r', '\n', '\r', '\n'}
	dataTag      = []byte("data:")
	doneToken    = []byte("[DONE]")
)

type Observer interface {
	OnJSONEvent(payload map[string]any)
	OnStreamClose()
}

type SSEEventReader struct {
	source  io.ReadCloser
	watchers []Observer
	buf     []byte
	closed  bool
}

func NewObservedSSEStream(stream io.ReadCloser, observers ...Observer) io.ReadCloser {
	filtered := make([]Observer, 0, len(observers))
	for _, obs := range observers {
		if obs != nil {
			filtered = append(filtered, obs)
		}
	}
	if len(filtered) == 0 {
		return stream
	}
	return &SSEEventReader{
		source:   stream,
		watchers: filtered,
	}
}

func (r *SSEEventReader) Read(p []byte) (n int, err error) {
	n, err = r.source.Read(p)
	if n > 0 {
		r.ingest(p[:n])
	}
	return n, err
}

func (r *SSEEventReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true

	if len(r.buf) > 0 {
		r.dispatchBuffer(r.buf)
		r.buf = nil
	}

	for _, obs := range r.watchers {
		obs.OnStreamClose()
	}
	return r.source.Close()
}

func (r *SSEEventReader) ingest(data []byte) {
	r.buf = append(r.buf, data...)

	for {
		idx, sepLen := r.findBoundary()
		if idx < 0 {
			if len(r.buf) > sseEventMaxBytes {
				r.buf = r.buf[len(r.buf)-sseEventMaxBytes:]
			}
			return
		}

		if idx > 0 {
			raw := r.buf[:idx]
			r.dispatch(raw)
		}
		r.buf = r.buf[idx+sepLen:]

		if len(r.buf) > sseEventMaxBytes {
			r.buf = r.buf[len(r.buf)-sseEventMaxBytes:]
		}
	}
}

func (r *SSEEventReader) findBoundary() (int, int) {
	lf := bytes.Index(r.buf, boundaryLF)
	cr := bytes.Index(r.buf, boundaryCRLF)

	switch {
	case lf < 0 && cr < 0:
		return -1, 0
	case cr < 0 || (lf >= 0 && lf < cr):
		return lf, 2
	default:
		return cr, 4
	}
}

func (r *SSEEventReader) dispatch(raw []byte) {
	lines := bytes.Split(raw, []byte{'\n'})
	var parts [][]byte
	for _, line := range lines {
		payload := stripDataPrefix(line)
		if payload == nil {
			continue
		}
		parts = append(parts, payload)
	}
	if len(parts) == 0 {
		return
	}

	joined := bytes.Join(parts, []byte{'\n'})
	if bytes.Equal(joined, doneToken) {
		return
	}

	var obj map[string]any
	if err := json.Unmarshal(joined, &obj); err != nil {
		return
	}
	for _, obs := range r.watchers {
		obs.OnJSONEvent(obj)
	}
}

func (r *SSEEventReader) dispatchBuffer(data []byte) {
	for {
		idx, sepLen := r.findBoundaryIn(data)
		if idx < 0 {
			if len(data) > 0 {
				r.dispatch(data)
			}
			return
		}
		if idx > 0 {
			r.dispatch(data[:idx])
		}
		data = data[idx+sepLen:]
	}
}

func (r *SSEEventReader) findBoundaryIn(data []byte) (int, int) {
	lf := bytes.Index(data, boundaryLF)
	cr := bytes.Index(data, boundaryCRLF)
	switch {
	case lf < 0 && cr < 0:
		return -1, 0
	case cr < 0 || (lf >= 0 && lf < cr):
		return lf, 2
	default:
		return cr, 4
	}
}

func stripDataPrefix(line []byte) []byte {
	if !bytes.HasPrefix(line, dataTag) {
		return nil
	}
	payload := bytes.TrimPrefix(line, dataTag)
	if len(payload) > 0 && payload[0] == ' ' {
		payload = payload[1:]
	}
	for len(payload) > 0 && (payload[len(payload)-1] == '\r' || payload[len(payload)-1] == '\n') {
		payload = payload[:len(payload)-1]
	}
	return payload
}

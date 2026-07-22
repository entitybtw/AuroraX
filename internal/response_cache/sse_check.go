package responsecache

import (
	"bytes"
	"encoding/json"
)

func validateCacheableSSE(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}

	sawJSON := false
	sawDone := false

	for len(raw) > 0 {
		remaining := raw
		idx, sepLen := scanSSEBoundary(raw)
		event := remaining
		raw = nil
		if idx != -1 {
			event = remaining[:idx]
			raw = remaining[idx+sepLen:]
		}

		payload, hasData := extractSSEPayload(event)
		if sawDone {
			return false
		}
		if !hasData {
			continue
		}
		if len(bytes.TrimSpace(payload)) == 0 {
			continue
		}
		if bytes.Equal(payload, cacheDonePayload) {
			sawDone = true
			continue
		}
		if !json.Valid(payload) {
			return false
		}
		sawJSON = true
	}

	return sawJSON && sawDone
}

func extractSSEPayload(event []byte) ([]byte, bool) {
	lines := bytes.Split(event, []byte("\n"))
	payloadLines := make([][]byte, 0, len(lines))
	for _, line := range lines {
		data, ok := scanDataLine(line)
		if !ok {
			continue
		}
		payloadLines = append(payloadLines, data)
	}
	if len(payloadLines) == 0 {
		return nil, false
	}
	return bytes.Join(payloadLines, []byte("\n")), true
}

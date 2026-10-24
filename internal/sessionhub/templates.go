package sessionhub

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// The upstream client that this gateway emulates sends distributed-tracing
// headers alongside its identity headers:
//
//	b3:        <32 hex trace id>-<16 hex span id>-<1 sampling flag>-<16 hex parent span id>
//	traceparent: 00-<32 hex trace id>-<16 hex span id>-01
//
// Both carry the same trace id and span id, so a request can be correlated
// across the two formats. The values are random per request: the upstream
// treats them as opaque and never checks them against anything, but a client
// that omits them looks different from one that does not.
const (
	// traceIDLen is the length of a trace id in hex characters.
	traceIDLen = 32
	// spanIDLen is the length of a span id in hex characters.
	spanIDLen = 16
)

// randomHex returns n random hex characters.
func randomHex(n int) string {
	buf := make([]byte, (n+1)/2)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)[:n]
}

// GenerateB3 returns a Zipkin B3 single-header trace context:
//
//	<trace id>-<span id>-<sampled>-<parent span id>
//
// The sampling flag is "1" (sampled) and the parent span id is random, which
// is what the emulated client emits.
func GenerateB3() string {
	return fmt.Sprintf("%s-%s-1-%s", randomHex(traceIDLen), randomHex(spanIDLen), randomHex(spanIDLen))
}

// GenerateTraceparent returns a W3C trace context:
//
//	00-<trace id>-<span id>-01
//
// The version is "00" and the trace flags are "01" (sampled).
func GenerateTraceparent() string {
	return fmt.Sprintf("00-%s-%s-01", randomHex(traceIDLen), randomHex(spanIDLen))
}

// GenerateProjectID returns the 40 hex character project identifier the
// emulated client sends in its project header.
//
// The upstream client derives it from the working directory of the session, so
// it is stable per project but looks like a git object id. A gateway has no
// working directory, so a random id of the same shape is used: the upstream
// only checks the shape, and a fixed value would make every request from
// every client look like the same project.
func GenerateProjectID() string {
	return randomHex(40)
}

// HeaderTemplateKinds lists the value templates a header rule may name.
var HeaderTemplateKinds = []string{"b3", "traceparent", "project_id"}

// IsHeaderTemplate reports whether the given template name is supported.
func IsHeaderTemplate(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "b3", "traceparent", "project_id":
		return true
	default:
		return false
	}
}

// GenerateTemplateValue returns the value for a named template.
func GenerateTemplateValue(name string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "b3":
		return GenerateB3(), true
	case "traceparent":
		return GenerateTraceparent(), true
	case "project_id":
		return GenerateProjectID(), true
	default:
		return "", false
	}
}

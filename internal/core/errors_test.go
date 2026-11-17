package core

import (
	"net/http"
	"testing"
)

// A provider that answers with a bare message must not make the caller read
// its JSON: the text is the reason, and the body is an implementation detail.
func TestParseProviderError_TopLevelMessage(t *testing.T) {
	err := ParseProviderError("sidecar", http.StatusTooManyRequests,
		[]byte(`{"message":"FreeUsageLimitError: Free usage limit reached"}`), nil)
	if err == nil {
		t.Fatal("ParseProviderError returned nil")
	}
	if got, want := err.Message, "FreeUsageLimitError: Free usage limit reached"; got != want {
		t.Fatalf("Message = %q, want %q", got, want)
	}
	if err.Type != ErrorTypeRateLimit {
		t.Fatalf("Type = %q, want %q", err.Type, ErrorTypeRateLimit)
	}
}

// The OpenAI-shaped envelope still wins where it is present.
func TestParseProviderError_ErrorEnvelope(t *testing.T) {
	err := ParseProviderError("p", http.StatusTooManyRequests,
		[]byte(`{"error":{"type":"rate_limit","message":"slow down"}}`), nil)
	if err == nil || err.Message != "slow down" {
		t.Fatalf("Message = %v, want %q", err, "slow down")
	}
}

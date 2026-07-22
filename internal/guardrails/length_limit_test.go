package guardrails

import (
	"context"
	"strings"
	"testing"
)

func TestLengthLimitGuardrail_AllowsWithinLimit(t *testing.T) {
	g, err := NewLengthLimitGuardrail("limit", LengthLimitConfig{MaxChars: 10})
	if err != nil {
		t.Fatal(err)
	}
	out, err := g.Process(context.Background(), []Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Content != "hello" {
		t.Fatalf("content changed = %q", out[0].Content)
	}
}

func TestLengthLimitGuardrail_BlocksOverCharLimit(t *testing.T) {
	g, err := NewLengthLimitGuardrail("limit", LengthLimitConfig{MaxChars: 5})
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.Process(context.Background(), []Message{{Role: "user", Content: "123456"}})
	if err == nil {
		t.Fatal("Process() error = nil, want limit error")
	}
	if !strings.Contains(err.Error(), "character") {
		t.Fatalf("error = %q, want character context", err.Error())
	}
}

func TestLengthLimitGuardrail_BlocksOverEstimatedTokenLimit(t *testing.T) {
	g, err := NewLengthLimitGuardrail("limit", LengthLimitConfig{MaxEstimatedTokens: 2})
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.Process(context.Background(), []Message{{Role: "user", Content: "123456789"}})
	if err == nil {
		t.Fatal("Process() error = nil, want token limit error")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Fatalf("error = %q, want token context", err.Error())
	}
}

func TestLengthLimitGuardrail_RequiresLimit(t *testing.T) {
	_, err := NewLengthLimitGuardrail("limit", LengthLimitConfig{})
	if err == nil {
		t.Fatal("NewLengthLimitGuardrail() error = nil, want required limit error")
	}
}

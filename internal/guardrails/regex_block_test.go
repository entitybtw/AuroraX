package guardrails

import (
	"context"
	"strings"
	"testing"
)

func TestRegexBlockGuardrail_BlocksMatchingContent(t *testing.T) {
	g, err := NewRegexBlockGuardrail("secrets", RegexBlockConfig{
		Action:   RegexBlockActionBlock,
		Patterns: []string{`(?i)api[_-]?key\s*[:=]`},
		Roles:    []string{"user"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = g.Process(context.Background(), []Message{{Role: "user", Content: "api_key=secret"}})
	if err == nil {
		t.Fatal("Process() error = nil, want block error")
	}
	if !strings.Contains(err.Error(), "api") {
		t.Fatalf("Process() error = %q, want pattern context", err.Error())
	}
}

func TestRegexBlockGuardrail_SanitizesMatchingContent(t *testing.T) {
	g, err := NewRegexBlockGuardrail("sanitize", RegexBlockConfig{
		Action:      RegexBlockActionSanitize,
		Patterns:    []string{`sk-[A-Za-z0-9]+`},
		Replacement: "[SECRET]",
		Roles:       []string{"user"},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := g.Process(context.Background(), []Message{
		{Role: "system", Content: "sk-systemvalue"},
		{Role: "user", Content: "token sk-abc123 here"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Content != "sk-systemvalue" {
		t.Fatalf("system content changed = %q", out[0].Content)
	}
	if out[1].Content != "token [SECRET] here" {
		t.Fatalf("sanitized content = %q", out[1].Content)
	}
}

func TestRegexBlockGuardrail_DoesNotMutateOriginal(t *testing.T) {
	g, err := NewRegexBlockGuardrail("sanitize", RegexBlockConfig{
		Action:   RegexBlockActionSanitize,
		Patterns: []string{`secret`},
	})
	if err != nil {
		t.Fatal(err)
	}
	original := []Message{{Role: "user", Content: "secret"}}

	out, err := g.Process(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	if original[0].Content != "secret" {
		t.Fatalf("original mutated = %q", original[0].Content)
	}
	if out[0].Content == original[0].Content {
		t.Fatal("sanitized output did not change")
	}
}

func TestRegexBlockGuardrail_InvalidPattern(t *testing.T) {
	_, err := NewRegexBlockGuardrail("bad", RegexBlockConfig{Patterns: []string{"["}})
	if err == nil {
		t.Fatal("NewRegexBlockGuardrail() error = nil, want invalid pattern error")
	}
}

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

func TestRegexBlockGuardrail_MultiPatternBlocks(t *testing.T) {
	g, err := NewRegexBlockGuardrail("multi", RegexBlockConfig{
		Action:   RegexBlockActionBlock,
		Patterns: []string{`(?i)password\s*[:=]`, `(?i)api[_-]?key\s*[:=]`},
		Roles:    []string{"user"},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"password with colon", "password: secret", true},
		{"password with equals", "PASSWORD = secret", true},
		{"api key", "API_KEY=sk-abc123", true},
		{"safe message", "What is the weather today?", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := g.Process(context.Background(), []Message{{Role: "user", Content: tt.input}})
			if (err != nil) != tt.wantErr {
				t.Fatalf("Process() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegexBlockGuardrail_CaseInsensitivity(t *testing.T) {
	g, err := NewRegexBlockGuardrail("case", RegexBlockConfig{
		Action:   RegexBlockActionBlock,
		Patterns: []string{`(?i)secret`},
		Roles:    []string{"user"},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input string
		block bool
	}{
		{"my secret", true},
		{"my SECRET", true},
		{"my Secret", true},
		{"my sEcReT", true},
		{"safe text", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := g.Process(context.Background(), []Message{{Role: "user", Content: tt.input}})
			if (err != nil) != tt.block {
				t.Fatalf("Process(%q) error = %v, block = %v", tt.input, err, tt.block)
			}
		})
	}
}

func TestRegexBlockGuardrail_RoleScopedBlock(t *testing.T) {
	g, err := NewRegexBlockGuardrail("role-scoped", RegexBlockConfig{
		Action:   RegexBlockActionBlock,
		Patterns: []string{`secret`},
		Roles:    []string{"user"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = g.Process(context.Background(), []Message{
		{Role: "system", Content: "the secret is xyz"},
		{Role: "user", Content: "the secret is xyz"},
	})
	if err == nil {
		t.Fatal("expected block error for user role match")
	}
}

func TestRegexBlockGuardrail_RoleScopedSkipsNonMatchingRole(t *testing.T) {
	g, err := NewRegexBlockGuardrail("role-scoped", RegexBlockConfig{
		Action:   RegexBlockActionBlock,
		Patterns: []string{`secret`},
		Roles:    []string{"user"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = g.Process(context.Background(), []Message{
		{Role: "system", Content: "the secret is xyz"},
	})
	if err != nil {
		t.Fatalf("system role should be skipped: %v", err)
	}
}

func TestRegexBlockGuardrail_AllRolesWhenEmpty(t *testing.T) {
	g, err := NewRegexBlockGuardrail("all-roles", RegexBlockConfig{
		Action:   RegexBlockActionBlock,
		Patterns: []string{`secret`},
	})
	if err != nil {
		t.Fatal(err)
	}

	roles := []string{"system", "user", "assistant", "tool"}
	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			_, err := g.Process(context.Background(), []Message{{Role: role, Content: "secret"}})
			if err == nil {
				t.Fatalf("role %q should have been blocked", role)
			}
		})
	}
}

func TestRegexBlockGuardrail_SanitizeMultiPattern(t *testing.T) {
	g, err := NewRegexBlockGuardrail("sanitize-multi", RegexBlockConfig{
		Action:      RegexBlockActionSanitize,
		Patterns:    []string{`sk-[A-Za-z0-9]+`, `(?i)token[\s:=]+[A-Za-z0-9]+`},
		Replacement: "[REDACTED]",
		Roles:       []string{"user"},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := g.Process(context.Background(), []Message{{
		Role:    "user",
		Content: "my sk-abc123 and Token: xyz789 here",
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := "my [REDACTED] and [REDACTED] here"
	if out[0].Content != want {
		t.Fatalf("content = %q, want %q", out[0].Content, want)
	}
}

func TestRegexBlockGuardrail_EmptyMessage(t *testing.T) {
	g, err := NewRegexBlockGuardrail("empty", RegexBlockConfig{
		Action:   RegexBlockActionBlock,
		Patterns: []string{`secret`},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := g.Process(context.Background(), []Message{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(out))
	}
}

func TestRegexBlockGuardrail_SanitizeWithDefaultReplacement(t *testing.T) {
	g, err := NewRegexBlockGuardrail("default-replace", RegexBlockConfig{
		Action:   RegexBlockActionSanitize,
		Patterns: []string{`secret`},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := g.Process(context.Background(), []Message{{Role: "user", Content: "my secret here"}})
	if err != nil {
		t.Fatal(err)
	}
	want := "my [REDACTED] here"
	if out[0].Content != want {
		t.Fatalf("content = %q, want %q", out[0].Content, want)
	}
}

func TestRegexBlockGuardrail_SanitizeRespectsRoleFilter(t *testing.T) {
	g, err := NewRegexBlockGuardrail("role-filter", RegexBlockConfig{
		Action:      RegexBlockActionSanitize,
		Patterns:    []string{`secret`},
		Replacement: "[REDACTED]",
		Roles:       []string{"user"},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := g.Process(context.Background(), []Message{
		{Role: "system", Content: "keep secret"},
		{Role: "user", Content: "redact secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Content != "keep secret" {
		t.Fatalf("system content changed = %q", out[0].Content)
	}
	if out[1].Content != "redact [REDACTED]" {
		t.Fatalf("user content = %q", out[1].Content)
	}
}

func TestRegexBlockGuardrail_ErrorMessageContainsMatchingPattern(t *testing.T) {
	g, err := NewRegexBlockGuardrail("err-msg", RegexBlockConfig{
		Action:   RegexBlockActionBlock,
		Patterns: []string{`(?i)password`, `sk-[A-Za-z0-9]+`},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = g.Process(context.Background(), []Message{{Role: "user", Content: "my sk-abc123"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "sk-[A-Za-z0-9]+") {
		t.Fatalf("error should contain matching pattern, got: %v", err)
	}
}

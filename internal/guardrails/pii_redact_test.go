package guardrails

import (
	"context"
	"testing"
)

func TestPIIRedactGuardrail_RedactsCommonPII(t *testing.T) {
	g := NewPIIRedactGuardrail("pii", PIIRedactConfig{Kinds: []string{"email", "phone", "ssn", "credit_card"}})
	out, err := g.Process(context.Background(), []Message{{Role: "user", Content: "Email a@example.com, call +1 555-123-4567, SSN 123-45-6789, card 4242 4242 4242 4242"}})
	if err != nil {
		t.Fatal(err)
	}
	want := "Email [EMAIL_REDACTED], call [PHONE_REDACTED], SSN [SSN_REDACTED], card [CARD_REDACTED]"
	if out[0].Content != want {
		t.Fatalf("content = %q, want %q", out[0].Content, want)
	}
}

func TestPIIRedactGuardrail_DefaultsToAllKinds(t *testing.T) {
	g := NewPIIRedactGuardrail("pii", PIIRedactConfig{})
	out, err := g.Process(context.Background(), []Message{{Role: "user", Content: "a@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Content != "[EMAIL_REDACTED]" {
		t.Fatalf("content = %q", out[0].Content)
	}
}

func TestPIIRedactGuardrail_RoleScoped(t *testing.T) {
	g := NewPIIRedactGuardrail("pii", PIIRedactConfig{Kinds: []string{"email"}, Roles: []string{"user"}})
	out, err := g.Process(context.Background(), []Message{
		{Role: "system", Content: "admin@example.com"},
		{Role: "user", Content: "user@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Content != "admin@example.com" {
		t.Fatalf("system content changed = %q", out[0].Content)
	}
	if out[1].Content != "[EMAIL_REDACTED]" {
		t.Fatalf("user content = %q", out[1].Content)
	}
}

func TestPIIRedactGuardrail_SelectiveKinds(t *testing.T) {
	g := NewPIIRedactGuardrail("pii", PIIRedactConfig{Kinds: []string{"email"}})
	out, err := g.Process(context.Background(), []Message{{
		Role:    "user",
		Content: "Email a@example.com, phone +1 555-123-4567",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Content != "Email [EMAIL_REDACTED], phone +1 555-123-4567" {
		t.Fatalf("content = %q, want email-only redaction", out[0].Content)
	}
}

func TestPIIRedactGuardrail_EmptyKindsDefaultsToAll(t *testing.T) {
	g := NewPIIRedactGuardrail("pii", PIIRedactConfig{Kinds: []string{}})
	if len(g.kinds) != 4 {
		t.Fatalf("expected 4 kinds, got %d: %v", len(g.kinds), g.kinds)
	}
}

func TestPIIRedactGuardrail_UnknownKindIsIgnored(t *testing.T) {
	g := NewPIIRedactGuardrail("pii", PIIRedactConfig{Kinds: []string{"email", "unknown_kind"}})
	out, err := g.Process(context.Background(), []Message{{
		Role:    "user",
		Content: "a@example.com",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Content != "[EMAIL_REDACTED]" {
		t.Fatalf("content = %q", out[0].Content)
	}
}

func TestPIIRedactGuardrail_DuplicateKindIsDeduplicated(t *testing.T) {
	g := NewPIIRedactGuardrail("pii", PIIRedactConfig{Kinds: []string{"email", "email", "phone", "email"}})
	if len(g.kinds) != 2 {
		t.Fatalf("expected 2 unique kinds, got %d: %v", len(g.kinds), g.kinds)
	}
}

func TestPIIRedactGuardrail_AllRolesWhenEmpty(t *testing.T) {
	g := NewPIIRedactGuardrail("pii", PIIRedactConfig{Kinds: []string{"email"}})
	out, err := g.Process(context.Background(), []Message{
		{Role: "system", Content: "sys@example.com"},
		{Role: "user", Content: "user@example.com"},
		{Role: "assistant", Content: "asst@example.com"},
		{Role: "tool", Content: "tool@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range out {
		if m.Content != "[EMAIL_REDACTED]" {
			t.Fatalf("role %q was not redacted: %q", m.Role, m.Content)
		}
	}
}

func TestPIIRedactGuardrail_NoPIIToRedact(t *testing.T) {
	g := NewPIIRedactGuardrail("pii", PIIRedactConfig{Kinds: []string{"email"}})
	out, err := g.Process(context.Background(), []Message{{
		Role:    "user",
		Content: "Hello, this is a safe message with no PII.",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Content != "Hello, this is a safe message with no PII." {
		t.Fatalf("content changed unexpectedly = %q", out[0].Content)
	}
}

func TestPIIRedactGuardrail_MultipleEmails(t *testing.T) {
	g := NewPIIRedactGuardrail("pii", PIIRedactConfig{Kinds: []string{"email"}})
	out, err := g.Process(context.Background(), []Message{{
		Role:    "user",
		Content: "contact a@b.com or c@d.org for support",
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := "contact [EMAIL_REDACTED] or [EMAIL_REDACTED] for support"
	if out[0].Content != want {
		t.Fatalf("content = %q, want %q", out[0].Content, want)
	}
}

func TestPIIRedactGuardrail_EmptyMessages(t *testing.T) {
	g := NewPIIRedactGuardrail("pii", PIIRedactConfig{Kinds: []string{"email"}})
	out, err := g.Process(context.Background(), []Message{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(out))
	}
}

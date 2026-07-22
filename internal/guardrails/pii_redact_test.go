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

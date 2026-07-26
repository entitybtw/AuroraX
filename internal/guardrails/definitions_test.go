package guardrails

import (
	"encoding/json"
	"testing"
)

func TestDecodeRegexBlockConfig_ValidFullConfig(t *testing.T) {
	raw := json.RawMessage(`{
		"action": "block",
		"patterns": ["(?i)api[_-]?key\\s*[:=]", "(?i)password\\s*[:=]"],
		"replacement": "[REDACTED]",
		"roles": ["user"],
		"on_error": "block"
	}`)
	cfg, err := decodeRegexBlockDefinitionConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Action != "block" {
		t.Fatalf("action = %q, want block", cfg.Action)
	}
	if len(cfg.Patterns) != 2 {
		t.Fatalf("patterns = %v, want 2 patterns", cfg.Patterns)
	}
	if cfg.Patterns[0] != `(?i)api[_-]?key\s*[:=]` {
		t.Fatalf("pattern[0] = %q", cfg.Patterns[0])
	}
	if cfg.Replacement != "[REDACTED]" {
		t.Fatalf("replacement = %q", cfg.Replacement)
	}
	if len(cfg.Roles) != 1 || cfg.Roles[0] != "user" {
		t.Fatalf("roles = %v", cfg.Roles)
	}
}

func TestDecodeRegexBlockConfig_ArrayPatternsRequired(t *testing.T) {
	raw := json.RawMessage(`{"patterns": ["pattern1"]}`)
	_, err := decodeRegexBlockDefinitionConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDecodeRegexBlockConfig_StringPatternsFails(t *testing.T) {
	raw := json.RawMessage(`{"patterns": "pattern1\npattern2"}`)
	_, err := decodeRegexBlockDefinitionConfig(raw)
	if err == nil {
		t.Fatal("expected error for string patterns, got nil")
	}
}

func TestDecodeRegexBlockConfig_EmptyPatternsFails(t *testing.T) {
	raw := json.RawMessage(`{"patterns": []}`)
	_, err := decodeRegexBlockDefinitionConfig(raw)
	if err == nil {
		t.Fatal("expected error for empty patterns, got nil")
	}
}

func TestDecodeRegexBlockConfig_UnknownFieldFails(t *testing.T) {
	raw := json.RawMessage(`{"patterns": ["a"], "unknown_field": "val"}`)
	_, err := decodeRegexBlockDefinitionConfig(raw)
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestDecodeRegexBlockConfig_InvalidPatternFails(t *testing.T) {
	raw := json.RawMessage(`{"patterns": ["["]}`)
	_, err := decodeRegexBlockDefinitionConfig(raw)
	if err == nil {
		t.Fatal("expected error for invalid regex pattern, got nil")
	}
}

func TestDecodeRegexBlockConfig_InvalidActionFails(t *testing.T) {
	raw := json.RawMessage(`{"patterns": ["a"], "action": "invalid"}`)
	_, err := decodeRegexBlockDefinitionConfig(raw)
	if err == nil {
		t.Fatal("expected error for invalid action, got nil")
	}
}

func TestDecodeRegexBlockConfig_DefaultReplacementIsEmpty(t *testing.T) {
	raw := json.RawMessage(`{"patterns": ["secret"]}`)
	cfg, err := decodeRegexBlockDefinitionConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Replacement != "" {
		t.Fatalf("replacement = %q, want empty (default is applied at runtime)", cfg.Replacement)
	}
}

func TestDecodeRegexBlockConfig_SanitizeAction(t *testing.T) {
	raw := json.RawMessage(`{"action": "sanitize", "patterns": ["secret"], "replacement": "xxx"}`)
	cfg, err := decodeRegexBlockDefinitionConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Action != "sanitize" {
		t.Fatalf("action = %q, want sanitize", cfg.Action)
	}
}

func TestDecodePIIRedactConfig_ValidFullConfig(t *testing.T) {
	raw := json.RawMessage(`{
		"kinds": ["email", "phone"],
		"roles": ["user"],
		"on_error": "allow"
	}`)
	cfg, err := decodePIIRedactDefinitionConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Kinds) != 2 {
		t.Fatalf("kinds = %v, want 2", cfg.Kinds)
	}
}

func TestDecodePIIRedactConfig_EmptyKindsDefaultsToAll(t *testing.T) {
	raw := json.RawMessage(`{}`)
	cfg, err := decodePIIRedactDefinitionConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Kinds) != 4 {
		t.Fatalf("kinds = %v, want 4 (all)", cfg.Kinds)
	}
}

func TestDecodePIIRedactConfig_UnknownFieldFails(t *testing.T) {
	raw := json.RawMessage(`{"unknown": "val"}`)
	_, err := decodePIIRedactDefinitionConfig(raw)
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestDecodeLengthLimitConfig_Valid(t *testing.T) {
	raw := json.RawMessage(`{"max_chars": 50000, "max_estimated_tokens": 12000}`)
	cfg, err := decodeLengthLimitDefinitionConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxChars != 50000 {
		t.Fatalf("max_chars = %d", cfg.MaxChars)
	}
	if cfg.MaxEstimatedTokens != 12000 {
		t.Fatalf("max_estimated_tokens = %d", cfg.MaxEstimatedTokens)
	}
}

func TestDecodeLengthLimitConfig_UnknownFieldFails(t *testing.T) {
	raw := json.RawMessage(`{"max_chars": 100, "unknown": true}`)
	_, err := decodeLengthLimitDefinitionConfig(raw)
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

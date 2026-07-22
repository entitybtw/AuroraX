package guardrails

import (
	"context"
	"regexp"
	"strings"
)

const (
	PIIKindEmail      = "email"
	PIIKindPhone      = "phone"
	PIIKindSSN        = "ssn"
	PIIKindCreditCard = "credit_card"
)

var piiPatterns = map[string]*regexp.Regexp{
	PIIKindEmail:      regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`),
	PIIKindPhone:      regexp.MustCompile(`(?:\+\d{1,3}[\s.-]?)?(?:\(?\d{3}\)?[\s.-]?)\d{3}[\s.-]?\d{4}\b`),
	PIIKindSSN:        regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	PIIKindCreditCard: regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`),
}

var piiReplacements = map[string]string{
	PIIKindEmail:      "[EMAIL_REDACTED]",
	PIIKindPhone:      "[PHONE_REDACTED]",
	PIIKindSSN:        "[SSN_REDACTED]",
	PIIKindCreditCard: "[CARD_REDACTED]",
}

// PIIRedactConfig configures deterministic PII redaction.
type PIIRedactConfig struct {
	Kinds []string
	Roles []string
}

// PIIRedactGuardrail redacts common PII with deterministic regexes.
type PIIRedactGuardrail struct {
	name  string
	kinds []string
	roles map[string]struct{}
}

// NewPIIRedactGuardrail creates a regex-based PII redactor.
func NewPIIRedactGuardrail(name string, cfg PIIRedactConfig) *PIIRedactGuardrail {
	if name == "" {
		name = "pii_redact"
	}
	kinds := normalizePIIKinds(cfg.Kinds)
	roles := make(map[string]struct{}, len(cfg.Roles))
	for _, role := range cfg.Roles {
		role = strings.ToLower(strings.TrimSpace(role))
		if role != "" {
			roles[role] = struct{}{}
		}
	}
	return &PIIRedactGuardrail{name: name, kinds: kinds, roles: roles}
}

// Name returns the guardrail name.
func (g *PIIRedactGuardrail) Name() string { return g.name }

// Process redacts configured PII kinds without mutating the original slice.
func (g *PIIRedactGuardrail) Process(_ context.Context, msgs []Message) ([]Message, error) {
	if len(msgs) == 0 || len(g.kinds) == 0 {
		return msgs, nil
	}
	out := make([]Message, len(msgs))
	copy(out, msgs)
	for i := range out {
		if !g.roleMatches(out[i].Role) {
			continue
		}
		content := out[i].Content
		for _, kind := range g.kinds {
			content = piiPatterns[kind].ReplaceAllString(content, piiReplacements[kind])
		}
		out[i].Content = content
	}
	return out, nil
}

func (g *PIIRedactGuardrail) roleMatches(role string) bool {
	if len(g.roles) == 0 {
		return true
	}
	_, ok := g.roles[strings.ToLower(strings.TrimSpace(role))]
	return ok
}

func normalizePIIKinds(raw []string) []string {
	if len(raw) == 0 {
		return []string{PIIKindEmail, PIIKindPhone, PIIKindSSN, PIIKindCreditCard}
	}
	seen := make(map[string]struct{}, len(raw))
	kinds := make([]string, 0, len(raw))
	for _, kind := range raw {
		kind = strings.ToLower(strings.TrimSpace(kind))
		if _, ok := piiPatterns[kind]; !ok {
			continue
		}
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		kinds = append(kinds, kind)
	}
	if len(kinds) == 0 {
		return []string{PIIKindEmail, PIIKindPhone, PIIKindSSN, PIIKindCreditCard}
	}
	return kinds
}

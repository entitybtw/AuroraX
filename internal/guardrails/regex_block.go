package guardrails

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"aurora/internal/core"
)

// RegexBlockAction selects the behavior when a pattern matches.
type RegexBlockAction string

const (
	// RegexBlockActionBlock rejects the request with HTTP 400 when any pattern matches.
	RegexBlockActionBlock RegexBlockAction = "block"
	// RegexBlockActionSanitize replaces matched substrings with the configured Replacement.
	RegexBlockActionSanitize RegexBlockAction = "sanitize"
)

// IsValidRegexBlockAction reports whether the given action is recognized.
func IsValidRegexBlockAction(action string) bool {
	switch RegexBlockAction(strings.TrimSpace(action)) {
	case RegexBlockActionBlock, RegexBlockActionSanitize:
		return true
	default:
		return false
	}
}

// EffectiveRegexBlockAction returns action if non-empty, otherwise "block".
func EffectiveRegexBlockAction(action string) string {
	resolved := strings.TrimSpace(action)
	if resolved == "" {
		return string(RegexBlockActionBlock)
	}
	return resolved
}

// DefaultRegexReplacement is used when sanitize mode has no Replacement set.
const DefaultRegexReplacement = "[REDACTED]"

// RegexBlockGuardrail matches Patterns against message content and either blocks
// the request or replaces matches with Replacement. Designed for deterministic
// keyword bans and obvious-secret detection.
type RegexBlockGuardrail struct {
	name        string
	action      RegexBlockAction
	patterns    []*regexp.Regexp
	replacement string
	roles       map[string]struct{}
}

// RegexBlockConfig is the runtime config for RegexBlockGuardrail.
type RegexBlockConfig struct {
	Action      RegexBlockAction
	Patterns    []string
	Replacement string
	// Roles selects which message roles are scanned. Empty = all roles.
	Roles []string
}

// NewRegexBlockGuardrail compiles patterns and returns a ready-to-use guardrail.
func NewRegexBlockGuardrail(name string, cfg RegexBlockConfig) (*RegexBlockGuardrail, error) {
	if name == "" {
		name = "regex_block"
	}
	action := RegexBlockAction(EffectiveRegexBlockAction(string(cfg.Action)))
	if !IsValidRegexBlockAction(string(action)) {
		return nil, fmt.Errorf("invalid regex_block action: %q (must be block or sanitize)", cfg.Action)
	}
	if len(cfg.Patterns) == 0 {
		return nil, fmt.Errorf("regex_block requires at least one pattern")
	}
	patterns := make([]*regexp.Regexp, 0, len(cfg.Patterns))
	for i, raw := range cfg.Patterns {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, fmt.Errorf("regex_block pattern %d is empty", i)
		}
		re, err := regexp.Compile(raw)
		if err != nil {
			return nil, fmt.Errorf("regex_block pattern %d invalid: %w", i, err)
		}
		patterns = append(patterns, re)
	}
	replacement := cfg.Replacement
	if action == RegexBlockActionSanitize && replacement == "" {
		replacement = DefaultRegexReplacement
	}
	roles := make(map[string]struct{}, len(cfg.Roles))
	for _, role := range cfg.Roles {
		role = strings.ToLower(strings.TrimSpace(role))
		if role == "" {
			continue
		}
		roles[role] = struct{}{}
	}
	return &RegexBlockGuardrail{
		name:        name,
		action:      action,
		patterns:    patterns,
		replacement: replacement,
		roles:       roles,
	}, nil
}

// Name returns the guardrail name.
func (g *RegexBlockGuardrail) Name() string { return g.name }

// Process scans Roles for any pattern hit and either rejects or sanitizes.
func (g *RegexBlockGuardrail) Process(_ context.Context, msgs []Message) ([]Message, error) {
	if len(g.patterns) == 0 || len(msgs) == 0 {
		return msgs, nil
	}
	if g.action == RegexBlockActionBlock {
		for _, m := range msgs {
			if !g.roleMatches(m.Role) {
				continue
			}
			if hit := g.firstMatch(m.Content); hit != "" {
				return nil, core.NewInvalidRequestError(
					fmt.Sprintf("guardrail %q blocked request: matched pattern %q", g.name, hit),
					nil,
				)
			}
		}
		return msgs, nil
	}
	out := make([]Message, len(msgs))
	copy(out, msgs)
	for i := range out {
		if !g.roleMatches(out[i].Role) {
			continue
		}
		out[i].Content = g.sanitize(out[i].Content)
	}
	return out, nil
}

func (g *RegexBlockGuardrail) roleMatches(role string) bool {
	if len(g.roles) == 0 {
		return true
	}
	_, ok := g.roles[strings.ToLower(strings.TrimSpace(role))]
	return ok
}

func (g *RegexBlockGuardrail) firstMatch(content string) string {
	for _, re := range g.patterns {
		if re.MatchString(content) {
			return re.String()
		}
	}
	return ""
}

func (g *RegexBlockGuardrail) sanitize(content string) string {
	for _, re := range g.patterns {
		content = re.ReplaceAllString(content, g.replacement)
	}
	return content
}

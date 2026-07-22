package guardrails

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"aurora/internal/core"
)

// LengthLimitConfig configures input length enforcement.
type LengthLimitConfig struct {
	MaxChars           int
	MaxEstimatedTokens int
}

// LengthLimitGuardrail rejects requests above configured text-size limits.
type LengthLimitGuardrail struct {
	name               string
	maxChars           int
	maxEstimatedTokens int
}

// NewLengthLimitGuardrail creates a request-size guardrail.
func NewLengthLimitGuardrail(name string, cfg LengthLimitConfig) (*LengthLimitGuardrail, error) {
	if name == "" {
		name = "length_limit"
	}
	if cfg.MaxChars <= 0 && cfg.MaxEstimatedTokens <= 0 {
		return nil, fmt.Errorf("length_limit requires max_chars or max_estimated_tokens")
	}
	if cfg.MaxChars < 0 {
		return nil, fmt.Errorf("length_limit max_chars cannot be negative")
	}
	if cfg.MaxEstimatedTokens < 0 {
		return nil, fmt.Errorf("length_limit max_estimated_tokens cannot be negative")
	}
	return &LengthLimitGuardrail{name: name, maxChars: cfg.MaxChars, maxEstimatedTokens: cfg.MaxEstimatedTokens}, nil
}

// Name returns the guardrail name.
func (g *LengthLimitGuardrail) Name() string { return g.name }

// Process enforces total message content length without mutating input.
func (g *LengthLimitGuardrail) Process(_ context.Context, msgs []Message) ([]Message, error) {
	chars := 0
	for _, msg := range msgs {
		chars += utf8.RuneCountInString(msg.Content)
	}
	if g.maxChars > 0 && chars > g.maxChars {
		return nil, core.NewInvalidRequestError(
			fmt.Sprintf("guardrail %q blocked request: character count %d exceeds limit %d", g.name, chars, g.maxChars),
			nil,
		)
	}
	estimatedTokens := estimateTokensFromMessages(msgs)
	if g.maxEstimatedTokens > 0 && estimatedTokens > g.maxEstimatedTokens {
		return nil, core.NewInvalidRequestError(
			fmt.Sprintf("guardrail %q blocked request: estimated token count %d exceeds limit %d", g.name, estimatedTokens, g.maxEstimatedTokens),
			nil,
		)
	}
	return msgs, nil
}

func estimateTokensFromMessages(msgs []Message) int {
	chars := 0
	words := 0
	for _, msg := range msgs {
		chars += utf8.RuneCountInString(msg.Content)
		words += len(strings.Fields(msg.Content))
	}
	charEstimate := (chars + 3) / 4
	if words > charEstimate {
		return words
	}
	return charEstimate
}

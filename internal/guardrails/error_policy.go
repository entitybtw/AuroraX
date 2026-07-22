package guardrails

import (
	"context"
	"fmt"
	"strings"
)

const (
	OnErrorBlock = "block"
	OnErrorAllow = "allow"
)

func effectiveOnError(raw, fallback string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		value = fallback
	}
	switch value {
	case OnErrorBlock, OnErrorAllow:
		return value, nil
	default:
		return "", fmt.Errorf("on_error must be block or allow")
	}
}

type errorPolicyGuardrail struct {
	inner   Guardrail
	onError string
}

func wrapErrorPolicy(inner Guardrail, onError string) Guardrail {
	if inner == nil || onError != OnErrorAllow {
		return inner
	}
	return &errorPolicyGuardrail{inner: inner, onError: onError}
}

func (g *errorPolicyGuardrail) Name() string {
	if g == nil || g.inner == nil {
		return ""
	}
	return g.inner.Name()
}

func (g *errorPolicyGuardrail) Process(ctx context.Context, msgs []Message) ([]Message, error) {
	out, err := g.inner.Process(ctx, msgs)
	if err != nil && g.onError == OnErrorAllow {
		return msgs, nil
	}
	return out, err
}

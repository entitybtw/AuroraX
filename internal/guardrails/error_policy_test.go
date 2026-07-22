package guardrails

import (
	"context"
	"errors"
	"testing"
)

type failingGuardrail struct{}

func (f failingGuardrail) Name() string { return "failing" }
func (f failingGuardrail) Process(context.Context, []Message) ([]Message, error) {
	return nil, errors.New("boom")
}

func TestErrorPolicyGuardrail_AllowsOnError(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hello"}}
	wrapped := wrapErrorPolicy(failingGuardrail{}, OnErrorAllow)

	out, err := wrapped.Process(context.Background(), msgs)
	if err != nil {
		t.Fatalf("Process() error = %v, want nil", err)
	}
	if len(out) != 1 || out[0].Content != "hello" {
		t.Fatalf("Process() output = %+v, want original messages", out)
	}
}

func TestErrorPolicyGuardrail_BlocksByDefault(t *testing.T) {
	wrapped := wrapErrorPolicy(failingGuardrail{}, OnErrorBlock)
	_, err := wrapped.Process(context.Background(), []Message{{Role: "user", Content: "hello"}})
	if err == nil {
		t.Fatal("Process() error = nil, want original error")
	}
}

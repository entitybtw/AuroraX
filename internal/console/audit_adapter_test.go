package console

import (
	"testing"
	"time"

	"aurora/internal/audit_logging"
)

func TestFromAuditEntrySanitizesOperationalSummary(t *testing.T) {
	entry := &auditlog.LogEntry{
		ID:             "log-1",
		Timestamp:      time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
		DurationNs:     int64(150 * time.Millisecond),
		RequestedModel: "combo-coding",
		ResolvedModel:  "anthropic/claude-sonnet-4-6",
		Provider:       "anthropic",
		ProviderName:   "anthropic-main",
		StatusCode:     200,
		RequestID:      "req-1",
		Method:         "POST",
		Path:           "/v1/chat/completions",
		Data: &auditlog.LogData{
			RequestHeaders: map[string]string{"Authorization": "secret"},
			RequestBody:    map[string]any{"prompt": "secret prompt"},
		},
	}

	event := FromAuditEntry(entry)
	if event.ID != entry.ID || event.RequestID != entry.RequestID {
		t.Fatalf("unexpected identity fields: %#v", event)
	}
	if event.Level != LevelInfo || event.Kind != KindRequest {
		t.Fatalf("unexpected level/kind: %#v", event)
	}
	if event.Model != entry.ResolvedModel || event.Provider != entry.ProviderName {
		t.Fatalf("unexpected route fields: %#v", event)
	}
	if event.DurationMS != 150 {
		t.Fatalf("expected duration 150ms, got %d", event.DurationMS)
	}
}

func TestFromAuditEntryMarksFallback(t *testing.T) {
	entry := &auditlog.LogEntry{
		ID:         "log-2",
		Timestamp:  time.Now(),
		StatusCode: 503,
		Method:     "POST",
		Path:       "/v1/chat/completions",
		Data: &auditlog.LogData{
			Failover: &auditlog.FailoverSnapshot{TargetModel: "openai/gpt-4o"},
		},
	}

	event := FromAuditEntry(entry)
	if event.Kind != KindFallback {
		t.Fatalf("expected fallback kind, got %q", event.Kind)
	}
	if event.Fallback == nil || event.Fallback.TargetModel != "openai/gpt-4o" {
		t.Fatalf("unexpected fallback data: %#v", event.Fallback)
	}
	if event.Level != LevelError {
		t.Fatalf("expected error level for 503, got %q", event.Level)
	}
}

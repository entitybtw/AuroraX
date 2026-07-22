package console

import (
	"fmt"
	"strings"
	"time"

	"aurora/internal/audit_logging"
)

func FromAuditEntry(entry *auditlog.LogEntry) Event {
	if entry == nil {
		return Event{}
	}
	provider := strings.TrimSpace(entry.ProviderName)
	if provider == "" {
		provider = strings.TrimSpace(entry.Provider)
	}
	event := Event{
		ID:         strings.TrimSpace(entry.ID),
		Time:       entry.Timestamp,
		Level:      levelForStatus(entry.StatusCode),
		Kind:       KindRequest,
		RequestID:  strings.TrimSpace(entry.RequestID),
		Method:     strings.TrimSpace(entry.Method),
		Path:       strings.TrimSpace(entry.Path),
		Status:     entry.StatusCode,
		Model:      modelForEntry(entry),
		Provider:   provider,
		DurationMS: entry.DurationNs / int64(time.Millisecond),
	}
	if entry.Data != nil && entry.Data.Failover != nil {
		target := strings.TrimSpace(entry.Data.Failover.TargetModel)
		if target != "" {
			event.Kind = KindFallback
			event.Fallback = &FallbackEvent{TargetModel: target}
		}
	}
	event.Message = messageForEvent(event, entry)
	return event
}

func levelForStatus(status int) string {
	switch {
	case status >= 500:
		return LevelError
	case status >= 400:
		return LevelWarn
	default:
		return LevelInfo
	}
}

func modelForEntry(entry *auditlog.LogEntry) string {
	if entry == nil {
		return ""
	}
	if model := strings.TrimSpace(entry.ResolvedModel); model != "" {
		return model
	}
	return strings.TrimSpace(entry.RequestedModel)
}

func messageForEvent(event Event, entry *auditlog.LogEntry) string {
	if event.Fallback != nil && event.Fallback.TargetModel != "" {
		return fmt.Sprintf("%s %s failed over to %s", valueOrDash(event.Method), valueOrDash(event.Path), event.Fallback.TargetModel)
	}
	if entry != nil && entry.ErrorType != "" {
		return fmt.Sprintf("%s %s returned %d (%s)", valueOrDash(event.Method), valueOrDash(event.Path), event.Status, entry.ErrorType)
	}
	return fmt.Sprintf("%s %s returned %d", valueOrDash(event.Method), valueOrDash(event.Path), event.Status)
}

func valueOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

package console

import "time"

const (
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"

	KindRequest  = "request"
	KindFallback = "fallback"
)

type FallbackEvent struct {
	TargetModel string `json:"target_model,omitempty"`
}

type Event struct {
	ID         string         `json:"id"`
	Time       time.Time      `json:"time"`
	Level      string         `json:"level"`
	Kind       string         `json:"kind"`
	Message    string         `json:"message"`
	RequestID  string         `json:"request_id,omitempty"`
	Method     string         `json:"method,omitempty"`
	Path       string         `json:"path,omitempty"`
	Status     int            `json:"status,omitempty"`
	Model      string         `json:"model,omitempty"`
	Provider   string         `json:"provider,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	Fallback   *FallbackEvent `json:"fallback,omitempty"`
}

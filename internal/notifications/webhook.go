package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

const (
	DefaultWebhookTimeout = 10 * time.Second
	DefaultMaxRetries     = 2
)

// BudgetEvent describes a budget-threshold-crossing event sent via webhook.
type BudgetEvent struct {
	Event       string    `json:"event"`
	UserPath    string    `json:"user_path"`
	PeriodLabel string    `json:"period_label"`
	Amount      float64   `json:"amount"`
	Spent       float64   `json:"spent"`
	Remaining   float64   `json:"remaining"`
	UsageRatio  float64   `json:"usage_ratio"`
	Timestamp   time.Time `json:"timestamp"`
}

const (
	EventBudgetWarning = "budget.warning"
	EventBudgetBlocked = "budget.blocked"
	EventBudgetReset   = "budget.reset"
)

// WebhookDispatcher dispatches webhook notifications.
type WebhookDispatcher struct {
	client  *http.Client
	timeout time.Duration
}

// NewWebhookDispatcher creates a new webhook dispatcher.
func NewWebhookDispatcher() *WebhookDispatcher {
	return &WebhookDispatcher{
		client: &http.Client{
			Timeout: DefaultWebhookTimeout,
		},
		timeout: DefaultWebhookTimeout,
	}
}

// DispatchBudgetEvent sends a budget notification to the given URL.
// This is a fire-and-forget call; errors are logged but not returned
// to avoid blocking request processing.
func (d *WebhookDispatcher) DispatchBudgetEvent(ctx context.Context, url string, event BudgetEvent) {
	if d == nil || url == "" {
		return
	}
	body, err := json.Marshal(event)
	if err != nil {
		slog.Warn("webhook marshal failed", "url", url, "event", event.Event, "error", err)
		return
	}
	go d.sendWithRetry(url, body)
}

func (d *WebhookDispatcher) sendWithRetry(url string, body []byte) {
	for attempt := 0; attempt <= DefaultMaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			slog.Warn("webhook request create failed", "url", url, "attempt", attempt, "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Aurora-Budget-Webhook/1.0")

		resp, err := d.client.Do(req)
		if err != nil {
			slog.Warn("webhook request failed", "url", url, "attempt", attempt, "error", err)
			if attempt < DefaultMaxRetries {
				time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
			}
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return
		}
		slog.Warn("webhook non-2xx response", "url", url, "status", resp.StatusCode, "attempt", attempt)
		if attempt < DefaultMaxRetries {
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
		}
	}
	slog.Error("webhook delivery failed after retries", "url", url)
}

// FormatBudgetEvent creates a BudgetEvent from threshold info.
func FormatBudgetEvent(eventType string, userPath string, periodLabel string, amount, spent, remaining float64, usageRatio float64) BudgetEvent {
	return BudgetEvent{
		Event:       eventType,
		UserPath:    userPath,
		PeriodLabel: periodLabel,
		Amount:      amount,
		Spent:       spent,
		Remaining:   remaining,
		UsageRatio:  usageRatio,
		Timestamp:   time.Now().UTC(),
	}
}

func init() {
	slog.Debug("notify package loaded (webhook dispatcher available)")
}

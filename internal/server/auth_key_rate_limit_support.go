package server

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v5"

	"aurora/internal/authorization_scope"
	"aurora/internal/core"
)

type AuthKeyRateLimiter interface {
	Allow(ctx context.Context, tenantID string, authKeyID string, limits core.AuthKeyRateLimits, estimatedTokens int, now time.Time) (*AuthKeyRateLimitDecision, error)
}

// AuthKeyRateLimitInspector exposes a non-mutating read of an auth key's current
// rate-limit consumption against its configured limits.
type AuthKeyRateLimitInspector interface {
	Snapshot(tenantID string, authKeyID string, limits core.AuthKeyRateLimits, now time.Time) core.AuthKeyRateLimitSnapshot
}

// gatewayErrorWithResponseHeaders wraps a GatewayError with HTTP response headers.
type gatewayErrorWithResponseHeaders struct {
	*core.GatewayError
	headers http.Header
}

func (e *gatewayErrorWithResponseHeaders) Unwrap() error { return e.GatewayError }

func (e *gatewayErrorWithResponseHeaders) Error() string { return e.GatewayError.Error() }

func (e *gatewayErrorWithResponseHeaders) ResponseHeaders() http.Header { return e.headers }

// AuthKeyRateLimitWindow is re-exported from core for backwards compatibility
// with code that referenced the server-package alias.
type AuthKeyRateLimitWindow = core.AuthKeyRateLimitWindow

// AuthKeyRateLimitSnapshot is re-exported from core for backwards compatibility
// with code that referenced the server-package alias.
type AuthKeyRateLimitSnapshot = core.AuthKeyRateLimitSnapshot

type AuthKeyRateLimitDecision struct {
	Allowed    bool
	Limit      int
	Remaining  int
	ResetAt    time.Time
	RetryAfter time.Duration
	Scope      string
}

type InMemoryAuthKeyRateLimiter struct {
	mu       sync.Mutex
	counters map[string]authKeyRateCounter
}

type authKeyRateCounter struct {
	Count   int
	ResetAt time.Time
}

func NewInMemoryAuthKeyRateLimiter() *InMemoryAuthKeyRateLimiter {
	return &InMemoryAuthKeyRateLimiter{counters: map[string]authKeyRateCounter{}}
}

func (l *InMemoryAuthKeyRateLimiter) Allow(_ context.Context, tenantID string, authKeyID string, limits core.AuthKeyRateLimits, estimatedTokens int, now time.Time) (*AuthKeyRateLimitDecision, error) {
	if l == nil || authKeyID == "" || limits.Empty() {
		return &AuthKeyRateLimitDecision{Allowed: true}, nil
	}
	scopeKey := authKeyRateLimitKey(tenantID, authKeyID)
	if estimatedTokens < 0 {
		estimatedTokens = 0
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	l.mu.Lock()
	defer l.mu.Unlock()

	previousCounters := l.counters
	nextCounters := cloneAuthKeyRateCounters(previousCounters)
	l.counters = nextCounters

	requestMinuteDecision := l.checkWindow(scopeKey, "requests_per_minute", limits.RequestsPerMinute, 1, now, now.Truncate(time.Minute).Add(time.Minute))
	if requestMinuteDecision != nil && !requestMinuteDecision.Allowed {
		l.counters = previousCounters
		return requestMinuteDecision, nil
	}
	requestDayDecision := l.checkWindow(scopeKey, "requests_per_day", limits.RequestsPerDay, 1, now, startOfNextUTCDay(now))
	if requestDayDecision != nil && !requestDayDecision.Allowed {
		l.counters = previousCounters
		return requestDayDecision, nil
	}
	tokenMinuteDecision := l.checkWindow(scopeKey, "tokens_per_minute", limits.TokensPerMinute, estimatedTokens, now, now.Truncate(time.Minute).Add(time.Minute))
	if tokenMinuteDecision != nil && !tokenMinuteDecision.Allowed {
		l.counters = previousCounters
		return tokenMinuteDecision, nil
	}
	tokenDayDecision := l.checkWindow(scopeKey, "tokens_per_day", limits.TokensPerDay, estimatedTokens, now, startOfNextUTCDay(now))
	if tokenDayDecision != nil && !tokenDayDecision.Allowed {
		l.counters = previousCounters
		return tokenDayDecision, nil
	}
	l.counters = nextCounters
	for _, decision := range []*AuthKeyRateLimitDecision{tokenDayDecision, tokenMinuteDecision, requestDayDecision, requestMinuteDecision} {
		if decision != nil {
			return decision, nil
		}
	}
	return &AuthKeyRateLimitDecision{Allowed: true}, nil
}

// Snapshot returns a non-mutating view of the current rate-limit consumption
// for one key. Returns an empty snapshot when no limits are configured.
func (l *InMemoryAuthKeyRateLimiter) Snapshot(tenantID string, authKeyID string, limits core.AuthKeyRateLimits, now time.Time) core.AuthKeyRateLimitSnapshot {
	if l == nil || authKeyID == "" || limits.Empty() {
		return core.AuthKeyRateLimitSnapshot{}
	}
	scopeKey := authKeyRateLimitKey(tenantID, authKeyID)
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	l.mu.Lock()
	defer l.mu.Unlock()

	snap := core.AuthKeyRateLimitSnapshot{}
	if limits.RequestsPerMinute > 0 {
		snap.RequestsPerMinute = l.windowSnapshot(scopeKey, "requests_per_minute", limits.RequestsPerMinute, now, now.Truncate(time.Minute).Add(time.Minute))
	}
	if limits.RequestsPerDay > 0 {
		snap.RequestsPerDay = l.windowSnapshot(scopeKey, "requests_per_day", limits.RequestsPerDay, now, startOfNextUTCDay(now))
	}
	if limits.TokensPerMinute > 0 {
		snap.TokensPerMinute = l.windowSnapshot(scopeKey, "tokens_per_minute", limits.TokensPerMinute, now, now.Truncate(time.Minute).Add(time.Minute))
	}
	if limits.TokensPerDay > 0 {
		snap.TokensPerDay = l.windowSnapshot(scopeKey, "tokens_per_day", limits.TokensPerDay, now, startOfNextUTCDay(now))
	}
	return snap
}

func (l *InMemoryAuthKeyRateLimiter) windowSnapshot(baseKey, scope string, limit int, now, freshResetAt time.Time) *core.AuthKeyRateLimitWindow {
	counterKey := authKeyRateLimitCounterKey(baseKey, scope)
	counter := l.counters[counterKey]
	resetAt := counter.ResetAt
	used := counter.Count
	if resetAt.IsZero() || !now.Before(resetAt) {
		resetAt = freshResetAt
		used = 0
	}
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	return &core.AuthKeyRateLimitWindow{
		Scope:     scope,
		Limit:     limit,
		Used:      used,
		Remaining: remaining,
		ResetAt:   resetAt,
	}
}

func cloneAuthKeyRateCounters(src map[string]authKeyRateCounter) map[string]authKeyRateCounter {
	dst := make(map[string]authKeyRateCounter, len(src))
	for key, counter := range src {
		dst[key] = counter
	}
	return dst
}

func (l *InMemoryAuthKeyRateLimiter) checkWindow(baseKey string, scope string, limit int, increment int, now time.Time, resetAt time.Time) *AuthKeyRateLimitDecision {
	if limit <= 0 || increment <= 0 {
		return nil
	}
	counterKey := authKeyRateLimitCounterKey(baseKey, scope)
	counter := l.counters[counterKey]
	if counter.ResetAt.IsZero() || !now.Before(counter.ResetAt) {
		counter = authKeyRateCounter{ResetAt: resetAt}
	}
	if counter.Count+increment > limit {
		remaining := limit - counter.Count
		if remaining < 0 {
			remaining = 0
		}
		return &AuthKeyRateLimitDecision{
			Allowed:    false,
			Limit:      limit,
			Remaining:  remaining,
			ResetAt:    counter.ResetAt,
			RetryAfter: counter.ResetAt.Sub(now),
			Scope:      scope,
		}
	}
	counter.Count += increment
	l.counters[counterKey] = counter
	remaining := limit - counter.Count
	if remaining < 0 {
		remaining = 0
	}
	return &AuthKeyRateLimitDecision{
		Allowed:   true,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   counter.ResetAt,
		Scope:     scope,
	}
}

func AuthKeyRateLimitMiddleware(limiter AuthKeyRateLimiter) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if err := enforceAuthKeyRateLimit(c, limiter); err != nil {
				return handleError(c, err)
			}
			return next(c)
		}
	}
}

func enforceAuthKeyRateLimit(c *echo.Context, limiter AuthKeyRateLimiter) error {
	if c == nil || c.Request() == nil || limiter == nil {
		return nil
	}
	ctx := c.Request().Context()
	authKeyID := core.GetAuthKeyID(ctx)
	if authKeyID == "" {
		return nil
	}
	limits, ok := core.GetAuthKeyRateLimits(ctx)
	if !ok || limits.Empty() {
		return nil
	}
	tenantID := scope.TenantIDFromContext(ctx)
	decision, err := limiter.Allow(ctx, tenantID, authKeyID, limits, estimateAuthKeyRateLimitTokens(ctx), time.Now().UTC())
	if err != nil {
		return core.NewProviderError("auth_key_rate_limit", http.StatusServiceUnavailable, "API key rate limit check failed", err).
			WithCode("auth_key_rate_limit_check_failed")
	}
	if decision == nil || decision.Allowed {
		return nil
	}
	return authKeyRateLimitError(decision)
}

func authKeyRateLimitError(decision *AuthKeyRateLimitDecision) error {
	gatewayErr := core.NewRateLimitError("auth_key", "API key rate limit exceeded").WithCode("auth_key_rate_limit_exceeded")
	headers := http.Header{}
	if retryAfter := authKeyRetryAfterHeader(decision.RetryAfter); retryAfter != "" {
		headers.Set("Retry-After", retryAfter)
	}
	if decision.Limit > 0 {
		headers.Set("X-RateLimit-Limit", strconv.Itoa(decision.Limit))
		headers.Set("X-RateLimit-Remaining", strconv.Itoa(decision.Remaining))
	}
	if !decision.ResetAt.IsZero() {
		headers.Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.UTC().Unix(), 10))
	}
	if decision.Scope != "" {
		headers.Set("X-RateLimit-Scope", decision.Scope)
	}
	if len(headers) == 0 {
		return gatewayErr
	}
	return &gatewayErrorWithResponseHeaders{GatewayError: gatewayErr, headers: headers}
}

func authKeyRetryAfterHeader(delay time.Duration) string {
	if delay <= 0 {
		return "0"
	}
	return fmt.Sprintf("%d", int64(math.Ceil(delay.Seconds())))
}

func estimateAuthKeyRateLimitTokens(ctx context.Context) int {
	const charsPerToken = 4
	if snapshot := core.GetRequestSnapshot(ctx); snapshot != nil {
		if body := snapshot.CapturedBodyView(); len(body) > 0 {
			return int(math.Ceil(float64(len(body)) / charsPerToken))
		}
	}
	return 1
}

func authKeyRateLimitKey(tenantID, authKeyID string) string {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = scope.DefaultID
	}
	return "tenant:" + tenantID + ":auth_key:" + strings.TrimSpace(authKeyID)
}

func authKeyRateLimitCounterKey(baseKey, scope string) string {
	return baseKey + ":" + scope
}

func startOfNextUTCDay(now time.Time) time.Time {
	year, month, day := now.UTC().Date()
	return time.Date(year, month, day+1, 0, 0, 0, 0, time.UTC)
}

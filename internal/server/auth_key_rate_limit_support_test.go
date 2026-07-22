package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"aurora/internal/authorization_scope"
	"aurora/internal/core"
)

func TestInMemoryAuthKeyRateLimiterRejectsOverLimitRequests(t *testing.T) {
	limiter := NewInMemoryAuthKeyRateLimiter()
	now := time.Date(2026, 5, 3, 10, 15, 30, 0, time.UTC)
	limits := core.AuthKeyRateLimits{RequestsPerMinute: 2}

	for i := 0; i < 2; i++ {
		decision, err := limiter.Allow(context.Background(), "default", "key-1", limits, 1, now)
		require.NoError(t, err)
		require.NotNil(t, decision)
		assert.True(t, decision.Allowed)
	}

	decision, err := limiter.Allow(context.Background(), "default", "key-1", limits, 1, now)
	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.False(t, decision.Allowed)
	assert.Equal(t, "requests_per_minute", decision.Scope)
	assert.Equal(t, 2, decision.Limit)
	assert.Equal(t, 30*time.Second, decision.RetryAfter)
}

func TestInMemoryAuthKeyRateLimiterRejectsOverTokenLimit(t *testing.T) {
	limiter := NewInMemoryAuthKeyRateLimiter()
	now := time.Date(2026, 5, 3, 10, 15, 30, 0, time.UTC)
	limits := core.AuthKeyRateLimits{TokensPerMinute: 10}

	decision, err := limiter.Allow(context.Background(), "default", "key-1", limits, 7, now)
	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.True(t, decision.Allowed)
	assert.Equal(t, 3, decision.Remaining)

	decision, err = limiter.Allow(context.Background(), "default", "key-1", limits, 4, now)
	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.False(t, decision.Allowed)
	assert.Equal(t, "tokens_per_minute", decision.Scope)
	assert.Equal(t, 3, decision.Remaining)
}

func TestInMemoryAuthKeyRateLimiterSeparatesAuthKeysAndWindows(t *testing.T) {
	limiter := NewInMemoryAuthKeyRateLimiter()
	now := time.Date(2026, 5, 3, 10, 15, 30, 0, time.UTC)
	limits := core.AuthKeyRateLimits{RequestsPerMinute: 1}

	first, err := limiter.Allow(context.Background(), "default", "key-1", limits, 1, now)
	require.NoError(t, err)
	assert.True(t, first.Allowed)

	otherKey, err := limiter.Allow(context.Background(), "default", "key-2", limits, 1, now)
	require.NoError(t, err)
	assert.True(t, otherKey.Allowed)

	nextWindow, err := limiter.Allow(context.Background(), "default", "key-1", limits, 1, now.Add(31*time.Second))
	require.NoError(t, err)
	assert.True(t, nextWindow.Allowed)
}

func TestInMemoryAuthKeyRateLimiterSeparatesSameAuthKeyAcrossTenants(t *testing.T) {
	limiter := NewInMemoryAuthKeyRateLimiter()
	now := time.Date(2026, 5, 3, 10, 15, 30, 0, time.UTC)
	limits := core.AuthKeyRateLimits{RequestsPerMinute: 1}

	tenantAFirst, err := limiter.Allow(context.Background(), "tenant-a", "shared-key-id", limits, 1, now)
	require.NoError(t, err)
	assert.True(t, tenantAFirst.Allowed)

	tenantBFirst, err := limiter.Allow(context.Background(), "tenant-b", "shared-key-id", limits, 1, now)
	require.NoError(t, err)
	assert.True(t, tenantBFirst.Allowed)

	tenantASecond, err := limiter.Allow(context.Background(), "tenant-a", "shared-key-id", limits, 1, now)
	require.NoError(t, err)
	assert.False(t, tenantASecond.Allowed)
}

func TestInMemoryAuthKeyRateLimiterConcurrentRequestsDoNotExceedLimit(t *testing.T) {
	limiter := NewInMemoryAuthKeyRateLimiter()
	now := time.Date(2026, 5, 3, 10, 15, 30, 0, time.UTC)
	limits := core.AuthKeyRateLimits{RequestsPerMinute: 10}

	var wg sync.WaitGroup
	allowed := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			decision, err := limiter.Allow(context.Background(), "default", "key-1", limits, 1, now)
			if err != nil {
				allowed <- false
				return
			}
			allowed <- decision != nil && decision.Allowed
		}()
	}
	wg.Wait()
	close(allowed)

	count := 0
	for ok := range allowed {
		if ok {
			count++
		}
	}
	assert.Equal(t, 10, count)
}

func TestAuthKeyRateLimitMiddlewarePassesTenantContextToLimiter(t *testing.T) {
	e := echo.New()
	limiter := &capturingAuthKeyRateLimiter{}
	limits := core.AuthKeyRateLimits{RequestsPerMinute: 1}
	handler := AuthKeyRateLimitMiddleware(limiter)(func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx := scope.WithTenantID(req.Context(), "tenant-a")
	ctx = core.WithAuthKeyID(ctx, "key-1")
	ctx = core.WithAuthKeyRateLimits(ctx, limits)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "tenant-a", limiter.tenantID)
	assert.Equal(t, "key-1", limiter.authKeyID)
	assert.Equal(t, limits, limiter.limits)
}

func TestAuthKeyRateLimitMiddlewareMapsExceededLimitTo429(t *testing.T) {
	e := echo.New()
	limiter := NewInMemoryAuthKeyRateLimiter()
	limits := core.AuthKeyRateLimits{RequestsPerMinute: 1}
	handler := AuthKeyRateLimitMiddleware(limiter)(func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		ctx := core.WithAuthKeyID(req.Context(), "key-1")
		ctx = core.WithAuthKeyRateLimits(ctx, limits)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler(c)
		require.NoError(t, err)
		if i == 0 {
			assert.Equal(t, http.StatusOK, rec.Code)
			continue
		}
		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.Equal(t, "1", rec.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "0", rec.Header().Get("X-RateLimit-Remaining"))
		assert.Equal(t, "requests_per_minute", rec.Header().Get("X-RateLimit-Scope"))
		assert.NotEmpty(t, rec.Header().Get("Retry-After"))
		assert.Contains(t, rec.Body.String(), "auth_key_rate_limit_exceeded")
	}
}

func TestAuthKeyRateLimitMiddlewareSkipsMasterKeyAndUnlimitedRequests(t *testing.T) {
	e := echo.New()
	limiter := NewInMemoryAuthKeyRateLimiter()
	calls := 0
	handler := AuthKeyRateLimitMiddleware(limiter)(func(c *echo.Context) error {
		calls++
		return c.String(http.StatusOK, "ok")
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	}
	assert.Equal(t, 3, calls)
}

func TestServerEnforcesManagedAPIKeyRateLimitBeforeProvider(t *testing.T) {
	auth := mockAuthenticator{
		enabled:     true,
		tokenToID:   map[string]string{"sk_gom_token": "key-1"},
		tokenLimits: map[string]core.AuthKeyRateLimits{"sk_gom_token": {RequestsPerMinute: 1}},
	}
	provider := &countingProvider{}
	server := New(provider, &Config{Authenticator: auth})

	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer sk_gom_token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if i == 0 {
			assert.NotEqual(t, http.StatusTooManyRequests, rec.Code)
			continue
		}
		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.Contains(t, rec.Body.String(), "auth_key_rate_limit_exceeded")
	}

	assert.Equal(t, 1, provider.calls)
}

type capturingAuthKeyRateLimiter struct {
	tenantID  string
	authKeyID string
	limits    core.AuthKeyRateLimits
}

func (l *capturingAuthKeyRateLimiter) Allow(ctx context.Context, tenantID string, authKeyID string, limits core.AuthKeyRateLimits, _ int, _ time.Time) (*AuthKeyRateLimitDecision, error) {
	l.tenantID = scope.TenantIDFromContext(ctx)
	if l.tenantID == "" {
		l.tenantID = tenantID
	}
	l.authKeyID = authKeyID
	l.limits = limits
	return &AuthKeyRateLimitDecision{Allowed: true}, nil
}

type countingProvider struct {
	calls int
}

func (p *countingProvider) ChatCompletion(_ context.Context, _ *core.ChatRequest) (*core.ChatResponse, error) {
	p.calls++
	return &core.ChatResponse{ID: "chatcmpl-test", Object: "chat.completion", Model: "test-model", Choices: []core.Choice{{Index: 0, Message: core.ResponseMessage{Role: "assistant", Content: "ok"}}}}, nil
}

func (p *countingProvider) StreamChatCompletion(_ context.Context, _ *core.ChatRequest) (io.ReadCloser, error) {
	p.calls++
	return io.NopCloser(strings.NewReader("data: [DONE]\n\n")), nil
}

func (p *countingProvider) Responses(_ context.Context, _ *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	return nil, core.NewProviderError("test", http.StatusNotImplemented, "not implemented", nil)
}

func (p *countingProvider) StreamResponses(_ context.Context, _ *core.ResponsesRequest) (io.ReadCloser, error) {
	return nil, core.NewProviderError("test", http.StatusNotImplemented, "not implemented", nil)
}

func (p *countingProvider) Embeddings(_ context.Context, _ *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	return nil, core.NewProviderError("test", http.StatusNotImplemented, "not implemented", nil)
}

func (p *countingProvider) ListModels(_ context.Context) (*core.ModelsResponse, error) {
	return &core.ModelsResponse{Object: "list", Data: []core.Model{}}, nil
}

func (p *countingProvider) Supports(string) bool {
	return true
}

func (p *countingProvider) GetProviderType(string) string {
	return "test"
}

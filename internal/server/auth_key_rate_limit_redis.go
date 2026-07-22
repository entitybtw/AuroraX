package server

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"aurora/internal/core"
)

const redisRateLimiterKeyPrefix = "rate_limit:"

// RedisAuthKeyRateLimiter implements AuthKeyRateLimiter using Redis INCR + EXPIRE
// for atomic, distributed rate-limit counters that survive restarts and work
// across multiple gateway replicas.
type RedisAuthKeyRateLimiter struct {
	client *redis.Client
}

// NewRedisAuthKeyRateLimiter creates a Redis-backed rate limiter.
func NewRedisAuthKeyRateLimiter(client *redis.Client) *RedisAuthKeyRateLimiter {
	return &RedisAuthKeyRateLimiter{client: client}
}

// NewRedisAuthKeyRateLimiterFromURL creates a Redis rate limiter from a URL string.
func NewRedisAuthKeyRateLimiterFromURL(redisURL string) (*RedisAuthKeyRateLimiter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL for rate limiter: %w", err)
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to connect to redis for rate limiter: %w", err)
	}
	return &RedisAuthKeyRateLimiter{client: client}, nil
}

// Close shuts down the Redis client.
func (l *RedisAuthKeyRateLimiter) Close() error {
	if l == nil || l.client == nil {
		return nil
	}
	return l.client.Close()
}

// Allow implements AuthKeyRateLimiter using Redis INCR + EXPIRE for each
// rate-limit window. Each window gets its own key with a TTL equal to the
// window duration so expired counters are cleaned up automatically.
func (l *RedisAuthKeyRateLimiter) Allow(_ context.Context, tenantID string, authKeyID string, limits core.AuthKeyRateLimits, estimatedTokens int, now time.Time) (*AuthKeyRateLimitDecision, error) {
	if l == nil || l.client == nil || authKeyID == "" || limits.Empty() {
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

	ctx := context.Background()

	if limits.RequestsPerMinute > 0 {
		key := redisKey(scopeKey, "rpm", now.Truncate(time.Minute))
		decision, err := l.checkWindow(ctx, key, limits.RequestsPerMinute, 1)
		if err != nil {
			return nil, err
		}
		if !decision.Allowed {
			return decision, nil
		}
	}

	if limits.RequestsPerDay > 0 {
		key := redisKey(scopeKey, "rpd", startOfNextUTCDay(now))
		decision, err := l.checkWindow(ctx, key, limits.RequestsPerDay, 1)
		if err != nil {
			return nil, err
		}
		if !decision.Allowed {
			return decision, nil
		}
	}

	if limits.TokensPerMinute > 0 && estimatedTokens > 0 {
		key := redisKey(scopeKey, "tpm", now.Truncate(time.Minute))
		decision, err := l.checkWindow(ctx, key, limits.TokensPerMinute, estimatedTokens)
		if err != nil {
			return nil, err
		}
		if !decision.Allowed {
			return decision, nil
		}
	}

	if limits.TokensPerDay > 0 && estimatedTokens > 0 {
		key := redisKey(scopeKey, "tpd", startOfNextUTCDay(now))
		decision, err := l.checkWindow(ctx, key, limits.TokensPerDay, estimatedTokens)
		if err != nil {
			return nil, err
		}
		if !decision.Allowed {
			return decision, nil
		}
	}

	return &AuthKeyRateLimitDecision{Allowed: true}, nil
}

func (l *RedisAuthKeyRateLimiter) checkWindow(ctx context.Context, key string, limit, increment int) (*AuthKeyRateLimitDecision, error) {
	pipe := l.client.Pipeline()
	incr := pipe.IncrBy(ctx, key, int64(increment))
	pipe.Expire(ctx, key, 72*time.Hour)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("redis rate limit check: %w", err)
	}

	current := int(incr.Val())
	remaining := limit - current
	if remaining < 0 {
		remaining = 0
	}

	if current > limit {
		return &AuthKeyRateLimitDecision{
			Allowed:    false,
			Limit:      limit,
			Remaining:  remaining,
			ResetAt:    time.Now().UTC().Add(72 * time.Hour),
			RetryAfter: 72 * time.Hour,
			Scope:      key,
		}, nil
	}

	return &AuthKeyRateLimitDecision{
		Allowed:   true,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   time.Now().UTC().Add(72 * time.Hour),
		Scope:     key,
	}, nil
}

// Snapshot implements AuthKeyRateLimitInspector.
func (l *RedisAuthKeyRateLimiter) Snapshot(tenantID string, authKeyID string, limits core.AuthKeyRateLimits, now time.Time) core.AuthKeyRateLimitSnapshot {
	if l == nil || l.client == nil || authKeyID == "" || limits.Empty() {
		return core.AuthKeyRateLimitSnapshot{}
	}
	scopeKey := authKeyRateLimitKey(tenantID, authKeyID)
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	ctx := context.Background()
	snap := core.AuthKeyRateLimitSnapshot{}

	if limits.RequestsPerMinute > 0 {
		key := redisKey(scopeKey, "rpm", now.Truncate(time.Minute))
		snap.RequestsPerMinute = l.snapshotWindow(ctx, key, limits.RequestsPerMinute)
	}
	if limits.RequestsPerDay > 0 {
		key := redisKey(scopeKey, "rpd", startOfNextUTCDay(now))
		snap.RequestsPerDay = l.snapshotWindow(ctx, key, limits.RequestsPerDay)
	}
	if limits.TokensPerMinute > 0 {
		key := redisKey(scopeKey, "tpm", now.Truncate(time.Minute))
		snap.TokensPerMinute = l.snapshotWindow(ctx, key, limits.TokensPerMinute)
	}
	if limits.TokensPerDay > 0 {
		key := redisKey(scopeKey, "tpd", startOfNextUTCDay(now))
		snap.TokensPerDay = l.snapshotWindow(ctx, key, limits.TokensPerDay)
	}
	return snap
}

func (l *RedisAuthKeyRateLimiter) snapshotWindow(ctx context.Context, key string, limit int) *core.AuthKeyRateLimitWindow {
	val, err := l.client.Get(ctx, key).Int()
	if err != nil {
		if err == redis.Nil {
			return &core.AuthKeyRateLimitWindow{
				Scope:     key,
				Limit:     limit,
				Used:      0,
				Remaining: limit,
			}
		}
		return nil
	}
	remaining := limit - val
	if remaining < 0 {
		remaining = 0
	}
	return &core.AuthKeyRateLimitWindow{
		Scope:     key,
		Limit:     limit,
		Used:      val,
		Remaining: remaining,
	}
}

func redisKey(scopeKey, scope string, windowStart time.Time) string {
	return fmt.Sprintf("%s%s:%s:%d", redisRateLimiterKeyPrefix, scopeKey, scope, windowStart.Unix())
}

// compile-time interface checks
var _ AuthKeyRateLimiter = (*RedisAuthKeyRateLimiter)(nil)
var _ AuthKeyRateLimitInspector = (*RedisAuthKeyRateLimiter)(nil)

func init() {
	slog.Debug("redis rate limiter loaded")
}

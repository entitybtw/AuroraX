package responsecache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/tidwall/gjson"

	"aurora/internal/audit_logging"
	"aurora/internal/authorization_scope"
	"aurora/internal/cache"
	"aurora/internal/core"
)

var cacheablePaths = map[string]bool{
	"/v1/chat/completions": true,
	"/v1/responses":        true,
	"/v1/embeddings":       true,
}

const (
	writeWorkerCount = 8
	writeQueueDepth  = 256
)

type writeTask struct {
	key  string
	data []byte
	ttl  time.Duration
}

type simpleCacheMiddleware struct {
	store cache.Store
	ttl   time.Duration
	wg    sync.WaitGroup
	jobs  chan writeTask

	hitRecorder func(*echo.Context, []byte, string)

	workers sync.WaitGroup
	mu      sync.RWMutex
	closed  bool
}

func newSimpleCacheMiddleware(store cache.Store, ttl time.Duration, hitRecorder func(*echo.Context, []byte, string)) *simpleCacheMiddleware {
	m := &simpleCacheMiddleware{
		store:       store,
		ttl:         ttl,
		jobs:        make(chan writeTask, writeQueueDepth),
		hitRecorder: hitRecorder,
	}
	m.startWorkers()
	return m
}

func (m *simpleCacheMiddleware) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if m.store == nil {
				return next(c)
			}
			path := c.Request().URL.Path
			if !cacheablePaths[path] || c.Request().Method != http.MethodPost {
				return next(c)
			}
			if shouldSkipCache(c.Request()) {
				return next(c)
			}
			body, cacheable, err := readRequestBody(c.Request())
			if err != nil {
				return core.NewInvalidRequestError(err.Error(), err)
			}
			if !cacheable {
				return next(c)
			}
			plan := core.GetWorkflow(c.Request().Context())
			if shouldSkipCacheForWorkflow(plan) {
				return next(c)
			}
			hit, err := m.TryHit(c, body)
			if err != nil || hit {
				return err
			}
			return m.StoreAfter(c, body, func() error { return next(c) })
		}
	}
}

func (m *simpleCacheMiddleware) TryHit(c *echo.Context, body []byte) (bool, error) {
	if m == nil || m.store == nil {
		return false, nil
	}
	path := c.Request().URL.Path
	ctx := c.Request().Context()
	plan := core.GetWorkflow(ctx)
	key := hashRequestForContext(ctx, path, body, plan)
	cached, err := m.store.Get(ctx, key)
	if err != nil {
		return false, nil
	}
	if len(cached) > 0 {
		attachDebugHeaders(c, debugMeta{Layer: CacheTypeExact, ExactKey: key})
		if err := writeCachedResponse(c, path, body, cached, CacheTypeExact); err != nil {
			slog.Warn("response cache replay failed", "path", path, "cache_type", CacheTypeExact, "err", err)
			return false, nil
		}
		auditlog.EnrichEntryWithCacheType(c, CacheTypeExact)
		if m.hitRecorder != nil {
			m.hitRecorder(c, cached, CacheTypeExact)
		}
		slog.Info("response cache hit (exact)",
			"path", path,
			"request_id", c.Request().Header.Get("X-Request-ID"),
		)
		return true, nil
	}
	return false, nil
}

func (m *simpleCacheMiddleware) StoreAfter(c *echo.Context, body []byte, next func() error) error {
	if m == nil || m.store == nil {
		return next()
	}
	path := c.Request().URL.Path
	ctx := c.Request().Context()
	plan := core.GetWorkflow(ctx)
	key := hashRequestForContext(ctx, path, body, plan)

	data, ok, err := captureResponseForCache(
		c,
		path,
		"response cache: failed to capture cacheable response body",
		next,
	)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	m.enqueueWrite(writeTask{key: key, data: data, ttl: resolveExactTTL(c.Request(), m.ttl)})
	return nil
}

func (m *simpleCacheMiddleware) close() error {
	m.mu.Lock()
	if !m.closed {
		m.closed = true
		close(m.jobs)
	}
	m.mu.Unlock()
	m.workers.Wait()
	m.wg.Wait()
	return m.store.Close()
}

func (m *simpleCacheMiddleware) startWorkers() {
	for range writeWorkerCount {
		m.workers.Go(func() {
			for job := range m.jobs {
				storeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				ttl := job.ttl
				if ttl <= 0 {
					ttl = m.ttl
				}
				err := m.store.Set(storeCtx, job.key, job.data, ttl)
				cancel()
				if err != nil {
					slog.Warn("response cache write failed", "key", job.key, "err", err)
				}
				m.wg.Done()
			}
		})
	}
}

func resolveExactTTL(req *http.Request, fallback time.Duration) time.Duration {
	if v := headerDuration(req, "X-Cache-TTL"); v > 0 {
		return v
	}
	return fallback
}

func (m *simpleCacheMiddleware) enqueueWrite(job writeTask) {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return
	}
	m.wg.Add(1)
	select {
	case m.jobs <- job:
		m.mu.RUnlock()
	default:
		m.wg.Done()
		m.mu.RUnlock()
		slog.Warn("response cache write queue full", "key", job.key)
	}
}

func shouldSkipCacheForWorkflow(plan *core.Workflow) bool {
	if plan == nil {
		return true
	}
	if !plan.CacheEnabled() {
		return true
	}
	return plan.Mode == core.ExecutionModeTranslated && plan.Resolution == nil
}

func readRequestBody(req *http.Request) ([]byte, bool, error) {
	if snapshot := core.GetRequestSnapshot(req.Context()); snapshot != nil {
		if snapshot.BodyNotCaptured {
			return nil, false, nil
		}
		if body := snapshot.CapturedBodyView(); body != nil {
			return body, true, nil
		}
	}
	if req.Body == nil {
		return []byte{}, true, nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, false, err
	}
	if body == nil {
		body = []byte{}
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, true, nil
}

func shouldSkipCache(req *http.Request) bool {
	cc := req.Header.Get("Cache-Control")
	if cc == "" {
		return false
	}
	directives := strings.SplitSeq(strings.ToLower(cc), ",")
	for d := range directives {
		d = strings.TrimSpace(d)
		if d == "no-cache" || d == "no-store" {
			return true
		}
	}
	return false
}

func isStreamingRequest(path string, body []byte) bool {
	return isStreamingGJSON(path, body)
}

func isStreamingGJSON(path string, body []byte) bool {
	if path == "/v1/embeddings" {
		return false
	}
	result := gjson.GetBytes(body, "stream")
	if !result.Exists() || (result.Type != gjson.True && result.Type != gjson.False) {
		return false
	}
	return result.Bool()
}

func hashRequestForContext(ctx context.Context, path string, body []byte, plan *core.Workflow) string {
	return hashWithTenant(path, body, plan, scope.EffectiveID(ctx))
}

func hashRequest(path string, body []byte, plan *core.Workflow) string {
	return hashWithTenant(path, body, plan, scope.DefaultID)
}

func hashWithTenant(path string, body []byte, plan *core.Workflow, tenantID string) string {
	h := sha256.New()
	h.Write([]byte(strings.TrimSpace(tenantID)))
	h.Write([]byte{0})
	h.Write([]byte(path))
	h.Write([]byte{0})
	if plan != nil {
		h.Write([]byte(plan.Mode))
		h.Write([]byte{0})
		h.Write([]byte(plan.ProviderType))
		h.Write([]byte{0})
		h.Write([]byte(plan.ResolvedQualifiedModel()))
		h.Write([]byte{0})
	}
	h.Write(cacheKeyRequestBody(path, body))
	return hex.EncodeToString(h.Sum(nil))
}

type responseCapture struct {
	http.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (r *responseCapture) cachedBody(contentType string) ([]byte, bool) {
	if r == nil || r.body == nil || r.body.Len() == 0 {
		return nil, false
	}

	raw := bytes.Clone(r.body.Bytes())
	if isEventStreamContentType(contentType) {
		if !validateCacheableSSE(raw) {
			return nil, false
		}
		return raw, true
	}
	if !json.Valid(raw) {
		return nil, false
	}
	return raw, true
}

func (r *responseCapture) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseCapture) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *responseCapture) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func shouldStoreCapture(status int) bool {
	return status == http.StatusOK
}

func captureResponseForCache(c *echo.Context, path, warnMessage string, next func() error) ([]byte, bool, error) {
	capture := &responseCapture{
		ResponseWriter: c.Response(),
		body:           &bytes.Buffer{},
	}
	c.SetResponse(capture)
	if err := next(); err != nil {
		return nil, false, err
	}
	if !shouldStoreCapture(capture.effectiveStatusCode()) || capture.body.Len() == 0 {
		return nil, false, nil
	}
	if core.GetFallbackUsed(c.Request().Context()) {
		return nil, false, nil
	}
	data, ok := capture.cachedBody(c.Response().Header().Get("Content-Type"))
	if !ok {
		slog.Warn(warnMessage, "path", path)
		return nil, false, nil
	}
	return data, true, nil
}

func (r *responseCapture) effectiveStatusCode() int {
	if r == nil {
		return 0
	}
	if r.status != 0 {
		return r.status
	}
	if resp, err := echo.UnwrapResponse(r); err == nil && resp != nil {
		return resp.Status
	}
	return 0
}

func (r *responseCapture) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = r.effectiveStatusCode()
		if r.status == 0 {
			r.status = http.StatusOK
		}
	}
	n, err := r.ResponseWriter.Write(b)
	if n > 0 {
		r.body.Write(b[:n])
	}
	return n, err
}

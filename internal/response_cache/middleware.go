package responsecache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"aurora/configuration"
	"aurora/internal/cache"
	"aurora/internal/core"
	"aurora/internal/embedding"
	"aurora/internal/usage"
)

const cacheKeyPrefix = "aurora:response:"

var passThroughHeaders = map[string]struct{}{
	http.CanonicalHeaderKey("Accept"):                     {},
	http.CanonicalHeaderKey("Baggage"):                    {},
	http.CanonicalHeaderKey("Cache-Control"):              {},
	http.CanonicalHeaderKey("Content-Type"):               {},
	http.CanonicalHeaderKey("Request-Id"):                 {},
	http.CanonicalHeaderKey("Traceparent"):                {},
	http.CanonicalHeaderKey("Tracestate"):                 {},
	http.CanonicalHeaderKey("User-Agent"):                 {},
	http.CanonicalHeaderKey("X-Cache-Control"):            {},
	http.CanonicalHeaderKey("X-Cache-Debug"):              {},
	http.CanonicalHeaderKey("X-Cache-Semantic-Threshold"): {},
	http.CanonicalHeaderKey("X-Cache-TTL"):                {},
	http.CanonicalHeaderKey("X-Cache-Type"):               {},
	http.CanonicalHeaderKey("X-Request-ID"):               {},
}

type ResponseCacheMiddleware struct {
	simple   *simpleCacheMiddleware
	semantic *semanticCacheMiddleware
	echo     *echo.Echo
}

type DebugInfo struct {
	Path                      string  `json:"path"`
	CacheType                 string  `json:"cache_type,omitempty"`
	ExactCacheKey             string  `json:"exact_cache_key,omitempty"`
	SemanticParamsHash        string  `json:"semantic_params_hash,omitempty"`
	SemanticCacheKey          string  `json:"semantic_cache_key,omitempty"`
	SemanticThreshold         float64 `json:"semantic_threshold,omitempty"`
	PromptSimilarityThreshold float64 `json:"prompt_similarity_threshold,omitempty"`
	ExactTTLSeconds           int     `json:"exact_ttl_seconds,omitempty"`
	SemanticTTLSeconds        int     `json:"semantic_ttl_seconds,omitempty"`
	Streaming                 bool    `json:"streaming,omitempty"`
	Cacheable                 bool    `json:"cacheable,omitempty"`
	MissReason                string  `json:"miss_reason,omitempty"`
	GuardrailsHash            string  `json:"guardrails_hash,omitempty"`
	EmbedderIdentity          string  `json:"embedder_identity,omitempty"`
	EffectiveContentType      string  `json:"effective_content_type,omitempty"`
}

type debugMeta struct {
	Layer                     string
	ExactKey                  string
	SemanticKey               string
	SemanticParamsKey         string
	SemanticThreshold         float64
	PromptSimilarityThreshold float64
	PromptSimilarityScore     float64
	SemanticScore             float32
	MissReason                string
}

type InternalHandleResult struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	CacheType  string
}

func NewResponseCacheMiddleware(
	cfg config.ResponseCacheConfig,
	resolvedProviders map[string]config.RawProviderConfig,
	usageLogger usage.LoggerInterface,
	pricingResolver usage.PricingResolver,
) (*ResponseCacheMiddleware, error) {
	m := &ResponseCacheMiddleware{}
	m.echo = echo.New()
	hitRecorder := newUsageHitRecorder(usageLogger, pricingResolver)

	switch {
	case cfg.Simple == nil:
	case !config.SimpleCacheEnabled(cfg.Simple):
		slog.Info("response cache (simple/exact) disabled by config")
	case cfg.Simple.Redis == nil || cfg.Simple.Redis.URL == "":
		slog.Warn("response cache (simple/exact) enabled in config but redis URL is missing; set cache.response.simple.redis.url or REDIS_URL")
	default:
		ttl := time.Duration(cfg.Simple.Redis.TTL) * time.Second
		if ttl == 0 {
			ttl = time.Hour
		}
		prefix := cfg.Simple.Redis.Key
		if prefix == "" {
			prefix = cacheKeyPrefix
		}
		store, err := cache.NewRedisStore(cache.RedisStoreConfig{
			URL:    cfg.Simple.Redis.URL,
			Prefix: prefix,
			TTL:    ttl,
		})
		if err != nil {
			return nil, err
		}
		m.simple = newSimpleCacheMiddleware(store, ttl, hitRecorder)
		slog.Info("response cache (simple/exact) enabled", "ttl_seconds", cfg.Simple.Redis.TTL, "prefix", prefix)
	}

	sem := cfg.Semantic
	if sem != nil && config.SemanticCacheActive(sem) {
		emb, err := embedding.NewEmbedder(sem.Embedder, resolvedProviders)
		if err != nil {
			slog.Warn("response cache (semantic) disabled: embedder initialization failed", "error", err)
		} else {
			vs, err := NewVecStore(sem.VectorStore)
			if err != nil {
				_ = emb.Close()
				if m.simple != nil {
					_ = m.simple.close()
				}
				return nil, err
			}
			m.semantic = newSemanticCacheMiddleware(emb, vs, *sem, hitRecorder)
			ttlLog := 0
			if sem.TTL != nil {
				ttlLog = *sem.TTL
			}
			slog.Debug("response cache (semantic) enabled",
				"threshold", sem.SimilarityThreshold,
				"ttl_seconds", ttlLog,
				"vector_store", sem.VectorStore.Type,
				"embedder", sem.Embedder.Provider,
			)
		}
	}

	return m, nil
}

func (m *ResponseCacheMiddleware) Middleware() echo.MiddlewareFunc {
	if m.simple != nil {
		return m.simple.Middleware()
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error { return next(c) }
	}
}

func (m *ResponseCacheMiddleware) HandleRequest(c *echo.Context, body []byte, next func() error) error {
	if m == nil {
		return next()
	}
	if err := validateOverrideHeaders(c.Request()); err != nil {
		return err
	}
	if debugMode(c.Request()) {
		attachDebugHeaders(c, debugMeta{Layer: "none", MissReason: "miss"})
	}
	if ShouldSkipAllCache(c.Request()) {
		if debugMode(c.Request()) {
			attachDebugHeaders(c, debugMeta{Layer: "none", MissReason: "bypass"})
		}
		return next()
	}

	skipExact := ShouldSkipExactCache(c.Request())
	skipSemantic := m.semantic == nil || strings.EqualFold(c.Request().Header.Get("X-Cache-Type"), CacheTypeExact)

	if !skipExact && m.simple != nil {
		hit, err := m.simple.TryHit(c, body)
		if err != nil || hit {
			return err
		}
	}

	innerNext := next
	if !skipExact && m.simple != nil {
		innerNext = func() error { return m.simple.StoreAfter(c, body, next) }
	}

	if !skipSemantic {
		return m.semantic.Handle(c, body, innerNext)
	}

	return innerNext()
}

func (m *ResponseCacheMiddleware) DebugRequest(ctx context.Context, method, path string, body []byte, headers http.Header) (*DebugInfo, error) {
	if ctx == nil {
		return nil, core.NewInvalidRequestError("context is required", nil)
	}
	if headers == nil {
		headers = make(http.Header)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body)).WithContext(ctx)
	req.Header = headers.Clone()
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("X-Request-ID") == "" {
		if requestID := strings.TrimSpace(core.GetRequestID(ctx)); requestID != "" {
			req.Header.Set("X-Request-ID", requestID)
		}
	}
	info := &DebugInfo{Path: path, Streaming: isStreamingRequest(path, body)}
	if ShouldSkipAllCache(req) {
		info.Cacheable = false
		info.MissReason = "bypass"
		return info, nil
	}
	if !cacheablePaths[path] || req.Method != http.MethodPost {
		info.Cacheable = false
		info.MissReason = "not_cacheable"
		return info, nil
	}
	if m.simple == nil && m.semantic == nil {
		info.Cacheable = false
		info.MissReason = "disabled"
		return info, nil
	}
	info.Cacheable = true
	info.CacheType = strings.TrimSpace(req.Header.Get("X-Cache-Type"))
	info.ExactCacheKey = hashRequestForContext(ctx, path, body, core.GetWorkflow(ctx))
	if sem := m.semantic; sem != nil {
		info.SemanticThreshold = sem.cfg.SimilarityThreshold
		if v := headerFloat64(req, "X-Cache-Semantic-Threshold"); v > 0 {
			info.SemanticThreshold = v
		}
		info.PromptSimilarityThreshold = sem.cfg.PromptSimilarityThreshold
		if info.PromptSimilarityThreshold <= 0 {
			info.PromptSimilarityThreshold = defaultPromptSimilarityThreshold
		}
		if v := headerFloat64(req, "X-Cache-Prompt-Similarity"); v > 0 {
			info.PromptSimilarityThreshold = v
		}
		info.SemanticTTLSeconds = resolveSemanticTTL(req, sem.cfg)
		if embedText, _ := extractEmbedText(body, sem.cfg.ExcludeSystemPrompt); embedText != "" {
			baseParams := computeParamsHashForContext(ctx, body, path, core.GetWorkflow(ctx), core.GetGuardrailsHash(ctx), sem.embedderIdentity)
			msgFp, _ := conversationInvariantFingerprint(body, sem.cfg.ExcludeSystemPrompt)
			info.SemanticParamsHash = sha256HexOf(baseParams + "\x00" + msgFp)
			info.SemanticCacheKey = sha256HexOf(embedText + "\x00" + info.SemanticParamsHash)
			info.EmbedderIdentity = sem.embedderIdentity
		}
	}
	if s := m.simple; s != nil {
		info.ExactTTLSeconds = exactTTLSeconds(req, s.ttl)
	}
	return info, nil
}

func exactTTLSeconds(req *http.Request, fallback time.Duration) int {
	if v := headerDuration(req, "X-Cache-TTL"); v > 0 {
		return int(v.Seconds())
	}
	return int(fallback.Seconds())
}

func resolveSemanticTTL(req *http.Request, cfg config.SemanticCacheConfig) int {
	if v := headerDuration(req, "X-Cache-TTL"); v > 0 {
		return int(v.Seconds())
	}
	if cfg.TTL == nil {
		return 0
	}
	return *cfg.TTL
}

func validateOverrideHeaders(req *http.Request) error {
	if req == nil {
		return nil
	}
	if v := strings.TrimSpace(req.Header.Get("X-Cache-Type")); v != "" {
		switch v {
		case CacheTypeExact, CacheTypeSemantic, CacheTypeBoth:
		default:
			return core.NewInvalidRequestError(fmt.Sprintf("invalid X-Cache-Type %q", v), nil)
		}
	}
	if v := strings.TrimSpace(req.Header.Get("X-Cache-TTL")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return core.NewInvalidRequestError(fmt.Sprintf("invalid X-Cache-TTL %q", v), nil)
		}
	}
	if v := strings.TrimSpace(req.Header.Get("X-Cache-Semantic-Threshold")); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 || f > 1 {
			return core.NewInvalidRequestError(fmt.Sprintf("invalid X-Cache-Semantic-Threshold %q", v), nil)
		}
	}
	return nil
}

func debugMode(req *http.Request) bool {
	if req == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(req.Header.Get("X-Cache-Debug"))) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}

func attachDebugHeaders(c *echo.Context, info debugMeta) {
	if c == nil || c.Request() == nil || !debugMode(c.Request()) {
		return
	}
	h := c.Response().Header()
	h.Set("X-Cache-Debug", "true")
	if info.Layer != "" {
		h.Set("X-Cache-Layer", info.Layer)
	}
	if info.MissReason != "" {
		h.Set("X-Cache-Miss-Reason", info.MissReason)
	}
	if info.ExactKey != "" {
		h.Set("X-Cache-Exact-Key-Hash", info.ExactKey)
	}
	if info.SemanticKey != "" {
		h.Set("X-Cache-Semantic-Key-Hash", info.SemanticKey)
	}
	if info.SemanticParamsKey != "" {
		h.Set("X-Cache-Semantic-Params-Hash", info.SemanticParamsKey)
	}
	if info.SemanticThreshold > 0 {
		h.Set("X-Cache-Semantic-Threshold", strconv.FormatFloat(info.SemanticThreshold, 'f', -1, 64))
	}
	if info.SemanticScore > 0 {
		h.Set("X-Cache-Semantic-Score", strconv.FormatFloat(float64(info.SemanticScore), 'f', 6, 32))
	}
	if info.PromptSimilarityThreshold > 0 {
		h.Set("X-Cache-Prompt-Similarity-Threshold", strconv.FormatFloat(info.PromptSimilarityThreshold, 'f', -1, 64))
	}
	if info.PromptSimilarityScore > 0 {
		h.Set("X-Cache-Prompt-Similarity-Score", strconv.FormatFloat(info.PromptSimilarityScore, 'f', 6, 64))
	}
}

func (m *ResponseCacheMiddleware) HandleInternalRequest(
	ctx context.Context,
	method, path string,
	body []byte,
	next func(*echo.Context) error,
) (*InternalHandleResult, error) {
	if ctx == nil {
		return nil, core.NewInvalidRequestError("context is required", nil)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header = buildInternalHeaders(ctx)
	req = req.WithContext(ctx)

	if m == nil {
		slog.Error("response cache: HandleInternalRequest called on nil middleware")
		return nil, core.NewProviderError("", http.StatusInternalServerError, "response cache middleware is not initialized", nil)
	}
	if m.echo == nil {
		slog.Error("response cache: HandleInternalRequest called with uninitialized Echo instance")
		return nil, core.NewProviderError("", http.StatusInternalServerError, "response cache middleware is not initialized", nil)
	}
	if err := validateOverrideHeaders(req); err != nil {
		return nil, err
	}

	rec := httptest.NewRecorder()
	c := m.echo.NewContext(req, rec)

	err := m.HandleRequest(c, body, func() error { return next(c) })
	if err != nil {
		var gatewayErr *core.GatewayError
		if errors.As(err, &gatewayErr) && gatewayErr != nil {
			return nil, gatewayErr
		}
		return nil, core.NewProviderError("", http.StatusInternalServerError, err.Error(), err)
	}

	return &InternalHandleResult{
		StatusCode: rec.Code,
		Headers:    rec.Header().Clone(),
		Body:       bytes.Clone(rec.Body.Bytes()),
		CacheType:  parseInternalCacheType(rec.Header().Get("X-Cache")),
	}, nil
}

func (m *ResponseCacheMiddleware) Close() error {
	if m == nil {
		return nil
	}
	var simpleErr, semErr error
	if m.simple != nil {
		simpleErr = m.simple.close()
	}
	if m.semantic != nil {
		semErr = m.semantic.close()
	}
	if simpleErr != nil {
		return simpleErr
	}
	return semErr
}

func buildInternalHeaders(ctx context.Context) http.Header {
	headers := make(http.Header)
	if snapshot := core.GetRequestSnapshot(ctx); snapshot != nil {
		for key, values := range snapshot.GetHeaders() {
			key = http.CanonicalHeaderKey(key)
			if _, allowed := passThroughHeaders[key]; !allowed {
				continue
			}
			for _, value := range values {
				headers.Add(key, value)
			}
		}
	}
	if headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", "application/json")
	}
	if requestID := strings.TrimSpace(core.GetRequestID(ctx)); requestID != "" && headers.Get("X-Request-ID") == "" {
		headers.Set("X-Request-ID", requestID)
	}
	return headers
}

func parseInternalCacheType(headerValue string) string {
	headerValue = strings.TrimSpace(headerValue)
	if strings.HasPrefix(headerValue, "HIT (") && strings.HasSuffix(headerValue, ")") {
		headerValue = strings.TrimSpace(headerValue[len("HIT (") : len(headerValue)-1])
	}
	switch headerValue {
	case CacheTypeExact:
		return CacheTypeExact
	case CacheTypeSemantic:
		return CacheTypeSemantic
	default:
		return ""
	}
}

func NewResponseCacheMiddlewareWithStore(store cache.Store, ttl time.Duration) *ResponseCacheMiddleware {
	return &ResponseCacheMiddleware{
		simple: newSimpleCacheMiddleware(store, ttl, nil),
		echo:   echo.New(),
	}
}

package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	httppprof "net/http/pprof"
	"path"
	"strings"
	"time"

	"aurora/configuration"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"aurora/internal/admin"
	"aurora/internal/admin/dashboard"
	"aurora/internal/audit_logging"
	batchstore "aurora/internal/batch"
	"aurora/internal/core"
	"aurora/internal/response_cache"
	"aurora/internal/response_store"
	"aurora/internal/telemetry"
	"aurora/internal/token_saver"
	"aurora/internal/usage"
)

// Server wraps the Echo server
type Server struct {
	echo                    *echo.Echo
	handler                 *Handler
	responseCacheMiddleware *responsecache.ResponseCacheMiddleware
	responseStore           responsestore.Store
}

const (
	inboundServerReadTimeout       = 5 * time.Second
	inboundServerReadHeaderTimeout = 2 * time.Second
	inboundServerWriteTimeout      = 10 * time.Second
	inboundServerIdleTimeout       = 10 * time.Second
)

// Config holds server configuration options
type Config struct {
	BasePath                             string                                 // URL path prefix where the app is mounted (default: /)
	MasterKey                            string                                 // Optional: Master key for authentication
	Authenticator                        BearerTokenAuthenticator               // Optional: managed API key authenticator
	MetricsEnabled                       bool                                   // Whether to expose Prometheus metrics endpoint
	MetricsEndpoint                      string                                 // HTTP path for metrics endpoint (default: /metrics)
	BodySizeLimit                        string                                 // Max request body size (e.g., "10M", "1024K")
	PprofEnabled                         bool                                   // Whether to expose debug profiling routes at /debug/pprof/*
	AuditLogger                          auditlog.LoggerInterface               // Optional: Audit logger for request/response logging
	UsageLogger                          usage.LoggerInterface                  // Optional: Usage logger for token tracking

	AuthKeyRateLimiter                   AuthKeyRateLimiter                     // Optional: per-managed-key request limiter
	PricingResolver                      usage.PricingResolver                  // Optional: Resolves pricing for cost calculation
	ModelResolver                        RequestModelResolver                   // Optional: explicit model resolver used during workflow resolution
	ModelAuthorizer                      RequestModelAuthorizer                 // Optional: request-scoped concrete model access controller
	WorkflowPolicyResolver               RequestWorkflowPolicyResolver          // Optional: persisted workflow resolver used during workflow resolution
	FallbackResolver                     RequestFallbackResolver                // Optional: translated-route fallback resolver
	TranslatedRequestPatcher             TranslatedRequestPatcher               // Optional: request patcher for translated routes after workflow resolution
	BatchRequestPreparer                 BatchRequestPreparer                   // Optional: batch request preparer before native provider submission
	ExposedModelLister                   ExposedModelLister                     // Optional: additional public models to merge into GET /v1/models
	KeepOnlyAliasesAtModelsEndpoint      bool                                   // Whether GET /v1/models should hide concrete provider models
	PassthroughSemanticEnrichers         []core.PassthroughSemanticEnricher     // Optional: provider-owned passthrough semantic enrichers before workflow resolution
	BatchStore                           batchstore.Store                       // Optional: Batch lifecycle persistence store
	ResponseStore                        responsestore.Store                    // Optional: Responses lifecycle persistence store
	LogOnlyModelInteractions             bool                                   // Only log AI model endpoints (default: true)
	DisableRequestLogging                bool                                   // Disable Echo request logging middleware entirely
	DisableRequestBodySnapshot           bool                                   // Disable eager request body snapshot capture/semantic derivation
	DisablePassthroughSemanticEnrichment bool                                   // Disable provider-owned passthrough semantic enrichment middleware
	DisablePassthroughRoutes             bool                                   // Disable /p/{provider}/{endpoint} route registration
	EnabledPassthroughProviders          []string                               // Provider types enabled on /p/{provider}/... passthrough routes
	AllowPassthroughV1Alias              *bool                                  // Allow /p/{provider}/v1/... aliases; nil defaults to true

	AdminEndpointsEnabled                bool                                   // Whether admin API endpoints are enabled
	AdminUIEnabled                       bool                                   // Whether admin dashboard UI is enabled
	AdminHandler                         *admin.Handler                         // Admin API handler (nil if disabled)
	DashboardHandler                *dashboard.Handler                // Dashboard UI handler (Vite-built SPA); nil if disabled
	SwaggerEnabled                       bool                                   // Whether to expose the Swagger UI at /swagger/index.html
	ResponseCacheMiddleware              *responsecache.ResponseCacheMiddleware // Optional: response cache middleware for cacheable endpoints
	GuardrailsHash                       string                                 // Optional: SHA-256 hash of active guardrail rules; stored in context post-patch for semantic cache
	TokenSaver                           config.TokenSaverConfig                // Optional: policy-driven prompt/tool-output compression settings
	PromptCacheConfig                    config.PromptCacheConfig               // Policy for applying prompt cache breakpoints
	Capabilities                         map[string]bool                        // Runtime edition capabilities for route gating
	IPExtractor                          echo.IPExtractor                       // Optional: trusted client IP extraction strategy for proxied deployments
	EnableAnthropicIngress               bool                                   // Enable /v1/messages Anthropic-format endpoint
}

// New creates a new HTTP server
func New(provider core.RoutableProvider, cfg *Config) *Server {
	e := echo.New()
	e.Logger = slog.Default()
	basePath := configuredBasePath(cfg)
	if basePath != "/" {
		e.Pre(stripBasePathMiddleware(basePath))
	}
	// Keep client IP handling explicit after Echo v5.1.0 changed RealIP defaults.
	// Direct extraction is the safe baseline unless a caller opts into trusted
	// proxy header handling via Config.IPExtractor.
	e.IPExtractor = echo.ExtractIPDirect()
	if cfg != nil && cfg.IPExtractor != nil {
		e.IPExtractor = cfg.IPExtractor
	}

	// Get loggers from config (may be nil)
	var auditLogger auditlog.LoggerInterface
	var usageLogger usage.LoggerInterface
	var authKeyRateLimiter AuthKeyRateLimiter
	var pricingResolver usage.PricingResolver
	if cfg != nil {
		auditLogger = cfg.AuditLogger
		usageLogger = cfg.UsageLogger
		authKeyRateLimiter = cfg.AuthKeyRateLimiter
		pricingResolver = cfg.PricingResolver
	}
	if authKeyRateLimiter == nil {
		authKeyRateLimiter = NewInMemoryAuthKeyRateLimiter()
	}

	var modelResolver RequestModelResolver
	var modelAuthorizer RequestModelAuthorizer
	var workflowPolicyResolver RequestWorkflowPolicyResolver
	var fallbackResolver RequestFallbackResolver
	var translatedRequestPatcher TranslatedRequestPatcher
	if cfg != nil {
		modelResolver = cfg.ModelResolver
		modelAuthorizer = cfg.ModelAuthorizer
		workflowPolicyResolver = cfg.WorkflowPolicyResolver
		fallbackResolver = cfg.FallbackResolver
		translatedRequestPatcher = cfg.TranslatedRequestPatcher
	}

	handler := newHandlerWithAuthorizer(provider, auditLogger, usageLogger, pricingResolver, modelResolver, modelAuthorizer, workflowPolicyResolver, fallbackResolver, translatedRequestPatcher)
	if cfg != nil {
		handler.batchRequestPreparer = cfg.BatchRequestPreparer
		handler.exposedModelLister = cfg.ExposedModelLister
		handler.keepOnlyAliasesAtModelsEndpoint = cfg.KeepOnlyAliasesAtModelsEndpoint
		handler.responseCache = cfg.ResponseCacheMiddleware
		handler.guardrailsHash = cfg.GuardrailsHash
		handler.tokenSaver = tokensaver.NewService(cfg.TokenSaver)
		handler.promptCacheConfig = promptCacheConfigFromConfig(cfg.PromptCacheConfig)
	}
	if cfg != nil && cfg.EnabledPassthroughProviders != nil {
		handler.setEnabledPassthroughProviders(cfg.EnabledPassthroughProviders)
	}
	if cfg != nil && !passthroughV1PrefixNormalizationEnabled(cfg) {
		handler.normalizePassthroughV1Prefix = false
	}
	if cfg != nil && cfg.BatchStore != nil {
		handler.SetBatchStore(cfg.BatchStore)
	}
	if cfg != nil && cfg.ResponseStore != nil {
		handler.SetResponseStore(cfg.ResponseStore)
	}

	// Build list of paths that skip authentication
	authSkipPaths := []string{"/health"}

	// Determine metrics path
	metricsPath := "/metrics"
	if cfg != nil && cfg.MetricsEnabled {
		if cfg.MetricsEndpoint != "" {
			// Normalize path to prevent traversal attacks
			metricsPath = path.Clean(cfg.MetricsEndpoint)
		}
		// Prevent metrics endpoint from shadowing API routes.
		if metricsPath == "/v1" || strings.HasPrefix(metricsPath, "/v1/") ||
			metricsPath == "/p" || strings.HasPrefix(metricsPath, "/p/") {
			slog.Warn("metrics endpoint conflicts with API routes, using /metrics instead",
				"configured", cfg.MetricsEndpoint,
				"normalized", metricsPath)
			metricsPath = "/metrics"
		}
	}

	// Admin dashboard pages and static assets skip auth (/* enables prefix matching)
	if cfg != nil && cfg.AdminUIEnabled && cfg.DashboardHandler != nil {
		authSkipPaths = append(authSkipPaths, "/admin/dashboard", "/admin/dashboard/*", "/admin/static/*")
	}

	// Identity auth endpoints (login, OIDC, me) skip auth so unauthenticated
	// users can reach them. /auth/me is a probe that returns:
	//   200 (user info) → already authenticated
	//   401             → identity enabled but not authenticated
	//   503             → identity not configured
	if cfg != nil {
		authSkipPaths = append(authSkipPaths, "/admin/api/v1/auth/me")
	}
	if cfg != nil && cfg.SwaggerEnabled && SwaggerAvailable() {
		// Swagger is registered below but remains behind auth when auth is configured.
	}
	if cfg != nil && cfg.PprofEnabled {
		// pprof is registered below but remains behind auth when auth is configured.
	}

	// Global middleware stack (order matters)
	// Request logger with optional filtering for model-only interactions
	if cfg != nil && cfg.DisableRequestLogging {
		// Benchmark/minimal mode: avoid request logger allocations and slog work.
	} else if cfg != nil && cfg.LogOnlyModelInteractions {
		e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
			Skipper: func(c *echo.Context) bool {
				return !core.IsModelInteractionPath(c.Request().URL.Path)
			},
			LogStatus:        true,
			LogURI:           true,
			LogMethod:        true,
			LogLatency:       true,
			LogProtocol:      true,
			LogRemoteIP:      true,
			LogHost:          true,
			LogURIPath:       true,
			LogUserAgent:     true,
			LogRequestID:     true,
			LogContentLength: true,
			LogResponseSize:  true,
			LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
				slog.Info("REQUEST",
					"method", v.Method,
					"uri", v.URI,
					"status", v.Status,
					"latency", v.Latency.String(),
					"host", v.Host,
					"bytes_in", v.ContentLength,
					"bytes_out", v.ResponseSize,
					"user_agent", v.UserAgent,
					"remote_ip", v.RemoteIP,
					"request_id", v.RequestID,
				)
				return nil
			},
		}))
	} else {
		e.Use(middleware.RequestLogger())
	}
	e.Use(middleware.Recover())

	// Body size limit (default: 10MB)
	bodySizeLimit := "10M"
	if cfg != nil && cfg.BodySizeLimit != "" {
		bodySizeLimit = cfg.BodySizeLimit
	}
	e.Use(middleware.BodyLimit(parseBodySizeLimitBytes(bodySizeLimit)))

	// Request ID middleware (always active — ensures every request has a unique ID
	// for usage tracking, audit logging, and response correlation)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req, id := ensureRequestID(c.Request())
			c.SetRequest(req)
			c.Response().Header().Set("X-Request-ID", id)
			return next(c)
		}
	})

	// Telemetry heartbeat request counter (skipped in bench mode — irrelevant at 10K req/s)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if core.IsModelInteractionPath(c.Request().URL.Path) {
				if minimalBenchModeEnabled() {
					return next(c)
				}
				telemetry.IncrementRequestCount()
			}
			return next(c)
		}
	})

	e.Use(modelInteractionWriteDeadlineMiddleware())

	// Ingress capture (before auth/audit/model validation so they can consume shared raw request state)
	e.Use(RequestSnapshotCaptureWithOptions(RequestSnapshotOptions{
		DisableBodySnapshot: cfg != nil && cfg.DisableRequestBodySnapshot,
	}))

	if cfg != nil && !cfg.DisablePassthroughSemanticEnrichment && len(cfg.PassthroughSemanticEnrichers) > 0 {
		e.Use(PassthroughSemanticEnrichment(provider, cfg.PassthroughSemanticEnrichers, passthroughV1PrefixNormalizationEnabled(cfg)))
	}

	// Audit logging runs before workflow resolution so early workflow resolution/validation
	// failures are still logged. The middleware defers request capture and
	// dynamically gates response capture on the final resolved workflow, so
	// Audit=false still suppresses per-request capture work.
	if cfg != nil && cfg.AuditLogger != nil && cfg.AuditLogger.Config().Enabled {
		e.Use(auditlog.Middleware(cfg.AuditLogger))
	}

	if cfg != nil {
		e.Use(CapabilityGateMiddleware(cfg.Capabilities))
	}

	// Authentication (skips public paths)
	authConfigured := false
	if cfg != nil {
		if cfg.MasterKey != "" {
			authConfigured = true
		}
		if cfg.Authenticator != nil && cfg.Authenticator.Enabled() {
			authConfigured = true
		}
	}
	if authConfigured {
		e.Use(AuthMiddlewareWithFullConfig(cfg.MasterKey, cfg.Authenticator, nil, authSkipPaths))
	}

	// Workflow resolution resolves the request-scoped workflow after auth so
	// managed auth key user-path overrides are visible to policy resolution while
	// still keeping workflow resolution failures loggable through the audit middleware.
	e.Use(WorkflowResolutionWithResolverAndPolicy(provider, modelResolver, workflowPolicyResolver))
	// Auth key rate limit — redundant in bench mode with no managed keys
	if authConfigured && !minimalBenchModeEnabled() {
		e.Use(AuthKeyRateLimitMiddleware(authKeyRateLimiter))
	}

	// Public routes
	e.GET("/health", handler.Health)
	registerSwagger(e, cfg)
	if cfg != nil && cfg.MetricsEnabled {
		e.GET(metricsPath, echo.WrapHandler(promhttp.Handler()))
	}
	if cfg != nil && cfg.PprofEnabled {
		e.GET("/debug/pprof", echo.WrapHandler(http.HandlerFunc(httppprof.Index)))
		e.GET("/debug/pprof/", echo.WrapHandler(http.HandlerFunc(httppprof.Index)))
		e.GET("/debug/pprof/cmdline", echo.WrapHandler(http.HandlerFunc(httppprof.Cmdline)))
		e.GET("/debug/pprof/profile", echo.WrapHandler(http.HandlerFunc(httppprof.Profile)))
		e.GET("/debug/pprof/symbol", echo.WrapHandler(http.HandlerFunc(httppprof.Symbol)))
		e.GET("/debug/pprof/trace", echo.WrapHandler(http.HandlerFunc(httppprof.Trace)))
		e.GET("/debug/pprof/:profile", func(c *echo.Context) error {
			httppprof.Handler(c.Param("profile")).ServeHTTP(c.Response(), c.Request())
			return nil
		})
	}

	// API routes
	if cfg == nil || !cfg.DisablePassthroughRoutes {
		e.GET("/p/:provider/*", handler.ProviderPassthrough)
		e.POST("/p/:provider/*", handler.ProviderPassthrough)
		e.PUT("/p/:provider/*", handler.ProviderPassthrough)
		e.PATCH("/p/:provider/*", handler.ProviderPassthrough)
		e.DELETE("/p/:provider/*", handler.ProviderPassthrough)
		e.HEAD("/p/:provider/*", handler.ProviderPassthrough)
		e.OPTIONS("/p/:provider/*", handler.ProviderPassthrough)
	}
	e.GET("/v1/models", handler.ListModels)
	e.POST("/v1/chat/completions", handler.ChatCompletion)
	if cfg != nil && cfg.EnableAnthropicIngress {
		e.POST("/v1/messages", handler.AnthropicMessages)
		e.POST("/v1/messages/count_tokens", handler.AnthropicCountTokens)
	}
	e.POST("/v1/responses/input_tokens", handler.ResponseInputTokens)
	e.POST("/v1/responses/compact", handler.CompactResponse)
	e.GET("/v1/responses/:id/input_items", handler.ListResponseInputItems)
	e.POST("/v1/responses/:id/cancel", handler.CancelResponse)
	e.GET("/v1/responses/:id", handler.GetResponse)
	e.DELETE("/v1/responses/:id", handler.DeleteResponse)
	e.POST("/v1/responses", handler.Responses)
	e.POST("/v1/embeddings", handler.Embeddings)
	e.POST("/v1/rerank", handler.Rerank)
	e.POST("/v1/files", handler.CreateFile)
	e.GET("/v1/files", handler.ListFiles)
	e.GET("/v1/files/:id", handler.GetFile)
	e.DELETE("/v1/files/:id", handler.DeleteFile)
	e.GET("/v1/files/:id/content", handler.GetFileContent)
	e.POST("/v1/batches", handler.Batches)
	e.GET("/v1/batches", handler.ListBatches)
	e.GET("/v1/batches/:id", handler.GetBatch)
	e.POST("/v1/batches/:id/cancel", handler.CancelBatch)
	e.GET("/v1/batches/:id/results", handler.BatchResults)

	// Admin API routes require the master key or identity login; scoped managed
	// keys are inference credentials only.
	adminAuthConfigured := false
	if cfg != nil {
		if cfg.MasterKey != "" {
			adminAuthConfigured = true
		}
	}
	if cfg != nil && cfg.AdminEndpointsEnabled && cfg.AdminHandler != nil {
		if adminAuthConfigured {
			// Per-IP token-bucket on the admin group regardless of which auth
			// path eventually matches. Closes master-key brute force, throttles
			// audit-log export DoS, and bounds the conversation-walk JSON
			// amplification attack.
			adminLimiter := middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
				Skipper: middleware.DefaultSkipper,
				Store: middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
					Rate:      10,              // sustained 10 req/s per IP
					Burst:     30,              // short bursts up to 30
					ExpiresIn: 5 * time.Minute, // clear idle entries
				}),
				IdentifierExtractor: func(c *echo.Context) (string, error) {
					return c.RealIP(), nil
				},
				ErrorHandler: func(c *echo.Context, _ error) error {
					return c.JSON(http.StatusForbidden, map[string]string{"error": "rate limit identifier error"})
				},
				DenyHandler: func(c *echo.Context, _ string, _ error) error {
					return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "too many requests"})
				},
			})
			adminGroup := e.Group("/admin/api/v1", adminLimiter)
			cfg.AdminHandler.RegisterRoutes(adminGroup)
		} else {
			slog.Warn("admin API disabled because no master key or identity config is set")
		}
	}

	// Admin dashboard UI routes (behind ADMIN_UI_ENABLED flag).
	if cfg != nil && cfg.AdminUIEnabled && cfg.DashboardHandler != nil {
		e.GET("/admin/dashboard", cfg.DashboardHandler.Index)
		e.GET("/admin/dashboard/*", cfg.DashboardHandler.Index)
		e.GET("/admin/static/*", cfg.DashboardHandler.Static)
	}

	var rcm *responsecache.ResponseCacheMiddleware
	if cfg != nil {
		rcm = cfg.ResponseCacheMiddleware
	}
	return &Server{
		echo:                    e,
		handler:                 handler,
		responseCacheMiddleware: rcm,
		responseStore:           handler.currentResponseStore(),
	}
}

func passthroughV1PrefixNormalizationEnabled(cfg *Config) bool {
	if cfg == nil || cfg.AllowPassthroughV1Alias == nil {
		return true
	}
	return *cfg.AllowPassthroughV1Alias
}

// Start starts the HTTP server on the given address and exits when ctx is canceled.
// On Windows, the listener uses SO_CONDITIONAL_ACCEPT to prevent TCP RST storms
// under high connection rates regardless of bench mode. The configureGatewayHTTPServer
// function applies bench-mode-only timeout tightening when AURORA_MINIMAL_BENCH_MODE=true.
func (s *Server) Start(ctx context.Context, addr string) error {
	listener, err := optimizedListener(ctx, addr)
	if err != nil {
		return err
	}
	sc := echo.StartConfig{
		HideBanner: true,
		Listener:   listener,
		BeforeServeFunc: func(server *http.Server) error {
			return configureGatewayHTTPServer(server)
		},
	}
	return sc.Start(ctx, s.echo)
}

// StartWithListener starts the HTTP server using a pre-bound listener.
// This is useful in tests that need an already-reserved loopback port.
func (s *Server) StartWithListener(ctx context.Context, listener net.Listener) error {
	sc := echo.StartConfig{
		HideBanner: true,
		Listener:   listener,
	}
	return sc.Start(ctx, s.echo)
}

// Shutdown releases server resources. The HTTP server itself is stopped by
// cancelling the context passed to Start; this method drains any in-flight
// response cache writes, closes the cache store, and closes the response store.
func (s *Server) Shutdown(_ context.Context) error {
	var firstErr error
	if s.responseCacheMiddleware != nil {
		if err := s.responseCacheMiddleware.Close(); err != nil {
			firstErr = err
		}
	}
	if s.responseStore != nil {
		if err := s.responseStore.Close(); err != nil {
			if firstErr == nil {
				firstErr = err
			} else {
				slog.Warn("response store close failed during shutdown", "error", err)
			}
		}
	}
	return firstErr
}

// ServeHTTP implements the http.Handler interface, allowing Server to be used with httptest
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.echo.ServeHTTP(w, r)
}

func newGatewayStartConfig(addr string) echo.StartConfig {
	return echo.StartConfig{
		Address:    addr,
		HideBanner: true,
		BeforeServeFunc: func(server *http.Server) error {
			return configureGatewayHTTPServer(server)
		},
	}
}

func configureGatewayHTTPServer(server *http.Server) error {
	if server == nil {
		return nil
	}

	// Enable h2c (cleartext HTTP/2) for multiplexing efficiency.
	server.Protocols = new(http.Protocols)
	server.Protocols.SetHTTP1(true)
	server.Protocols.SetUnencryptedHTTP2(true)

	server.ReadTimeout = inboundServerReadTimeout
	server.ReadHeaderTimeout = inboundServerReadHeaderTimeout
	server.WriteTimeout = inboundServerWriteTimeout
	server.IdleTimeout = inboundServerIdleTimeout
	return nil
}

func modelInteractionWriteDeadlineMiddleware() echo.MiddlewareFunc {
	// Cache bench mode at middleware creation time
	isBenchMode := minimalBenchModeEnabled()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if !core.IsModelInteractionPath(c.Request().URL.Path) {
				return next(c)
			}
			// In bench mode with the non-streaming fast path, skip clearing
			// the write deadline — non-streaming responses don't need SSE support.
			if isBenchMode && nonStreamingChatPassthroughFastPathEnabled() &&
				core.DescribeEndpoint(c.Request().Method, c.Request().URL.Path).Operation == core.OperationChatCompletions {
				return next(c)
			}
			if err := http.NewResponseController(c.Response()).SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
				slog.Warn("failed to clear write deadline for model interaction",
					"path", c.Request().URL.Path,
					"request_id", requestIDFromContextOrHeader(c.Request()),
					"error", err,
				)
			}
			return next(c)
		}
	}
}

func parseBodySizeLimitBytes(limit string) int64 {
	limit = strings.TrimSpace(limit)
	if limit == "" {
		return config.DefaultBodySizeLimit
	}

	value, err := config.ParseBodySizeLimitBytes(limit)
	if err != nil {
		slog.Warn("invalid body size limit, falling back to default", "configured", limit)
		return config.DefaultBodySizeLimit
	}

	return value
}

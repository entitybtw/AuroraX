package server

import (
	"bytes"
	"context"
	"errors"
	json "github.com/goccy/go-json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v5"

	"aurora/internal/audit_logging"
	"aurora/internal/core"
	"aurora/internal/gateway"

	"aurora/internal/response_cache"
	"aurora/internal/response_store"
	"aurora/internal/streaming"
	"aurora/internal/telemetry"
	"aurora/internal/token_saver"
	"aurora/internal/usage"
)

// translatedInferenceService adapts Echo requests to the transport-independent
// translated inference orchestrator.
type translatedInferenceService struct {
	provider                 core.RoutableProvider
	modelResolver            RequestModelResolver
	modelAuthorizer          RequestModelAuthorizer
	workflowPolicyResolver   RequestWorkflowPolicyResolver
	fallbackResolver         RequestFallbackResolver
	translatedRequestPatcher TranslatedRequestPatcher
	logger                   auditlog.LoggerInterface
	usageLogger              usage.LoggerInterface

	pricingResolver     usage.PricingResolver
	responseCache       *responsecache.ResponseCacheMiddleware
	guardrailsHash      string
	tokenSaver          *tokensaver.Service
	tokenSaverMu        sync.RWMutex
	responseStore       responsestore.Store
	responseStoreMu     sync.RWMutex
	promptCacheConfig   *core.PromptCacheConfig
	promptCacheConfigMu sync.RWMutex

	orchestrator *gateway.InferenceOrchestrator

	chatCompletionHandler echo.HandlerFunc
	responsesHandler      echo.HandlerFunc
}

func (s *translatedInferenceService) initHandlers() {
	s.orchestrator = s.newInferenceOrchestrator()
	s.chatCompletionHandler = s.handleChatCompletion
	s.responsesHandler = s.handleResponses
}

func (s *translatedInferenceService) inference() *gateway.InferenceOrchestrator {
	return s.orchestrator
}

func (s *translatedInferenceService) newInferenceOrchestrator() *gateway.InferenceOrchestrator {
	return gateway.NewInferenceOrchestrator(gateway.InferenceConfig{
		Provider:                 s.provider,
		ModelResolver:            s.modelResolver,
		ModelAuthorizer:          s.modelAuthorizer,
		WorkflowPolicyResolver:   s.workflowPolicyResolver,
		FallbackResolver:         s.fallbackResolver,
		TranslatedRequestPatcher: s.translatedRequestPatcher,
		UsageLogger:              s.usageLogger,
		PricingResolver:          s.pricingResolver,
		PromptCacheConfig:        s.promptCacheConfig, // nil = auto mode with defaults
	})
}

func (s *translatedInferenceService) ChatCompletion(c *echo.Context) error {
	if handled, err := s.tryEarlyBenchChatPassthrough(c); handled {
		return err
	}
	return s.chatCompletionHandler(c)
}

func (s *translatedInferenceService) tryEarlyBenchChatPassthrough(c *echo.Context) (bool, error) {
	if !minimalBenchModeEnabled() || !nonStreamingChatPassthroughFastPathEnabled() {
		return false, nil
	}
	workflow := core.GetWorkflow(c.Request().Context())
	if workflow == nil || workflow.Resolution == nil {
		return false, nil
	}

	body, err := requestBodyBytes(c)
	if err != nil {
		return true, handleError(c, core.NewInvalidRequestError("failed to read request body", err))
	}
	if bytes.Contains(body, []byte(`"stream":true`)) {
		return false, nil
	}
	if workflow.Resolution.ResolvedSelector.Model == "" {
		return false, nil
	}

	passthroughProvider, ok := s.provider.(core.RoutablePassthrough)
	if !ok {
		return false, nil
	}

	ctx, _ := requestContextWithRequestID(c.Request())
	reqHTTP := c.Request().WithContext(ctx)
	reqHTTP.Body = io.NopCloser(bytes.NewReader(body))
	c.SetRequest(reqHTTP)

	const endpoint = "/chat/completions"
	providerType := strings.TrimSpace(workflow.ProviderType)
	resp, err := passthroughProvider.Passthrough(ctx, providerType, &core.PassthroughRequest{
		Method:   c.Request().Method,
		Endpoint: endpoint,
		Body:     c.Request().Body,
		Headers:  buildPassthroughHeaders(ctx, c.Request().Header),
	})
	if err != nil {
		return true, handleError(c, err)
	}

	passthrough := passthroughService{provider: s.provider, logger: s.logger, usageLogger: s.usageLogger, pricingResolver: s.pricingResolver}
	return true, passthrough.proxyPassthroughResponse(c, providerType, providerNameFromWorkflow(workflow), endpoint, &core.PassthroughRouteInfo{
		Provider:    providerType,
		RawEndpoint: strings.TrimPrefix(endpoint, "/"),
		AuditPath:   c.Request().URL.Path,
		Model:       resolvedModelFromWorkflow(workflow, workflow.Resolution.ResolvedSelector.Model),
	}, resp)
}

func (s *translatedInferenceService) handleChatCompletion(c *echo.Context) error {
	return handleTranslatedJSON(s, c, core.DecodeChatRequest, prepareChatCompletionRequest, s.dispatchChatCompletion)
}

func (s *translatedInferenceService) dispatchChatCompletion(c *echo.Context, req *core.ChatRequest, workflow *core.Workflow) error {
	ctx := c.Request().Context()
	requestID := requestIDFromContextOrHeader(c.Request())
	endpoint := "/v1/chat/completions"
	streamLabel := strconv.FormatBool(req.Stream)

	req, tokenSaverMeta, err := s.applyTokenSaverChat(c, req, workflow)
	if err != nil {
		return handleError(c, err)
	}
	s.emitTokenSaverHeaders(c, tokenSaverMeta)
	if nonStreamingChatPassthroughFastPathEnabled() && !tokenSaverMeta.Enabled &&
		(minimalBenchModeEnabled() || len(s.inference().FallbackSelectors(workflow)) == 0) {
		if handled, err := s.tryFastPathChatPassthrough(c, workflow, req); handled {
			return err
		}
	}

	if req.Stream {
		if len(s.inference().FallbackSelectors(workflow)) == 0 {
			if handled, err := s.tryFastPathStreamingChatPassthrough(c, workflow, req); handled {
				return err
			}
		}
		providerStart := time.Now()
		result, err := s.inference().StreamChatCompletion(ctx, workflow, req)
		recordGatewayPhase(endpoint, "provider_dispatch", errStatus(err), streamLabel, time.Since(providerStart))
		if err != nil {
			return handleError(c, err)
		}
		if result.Meta.UsedFallback {
			markRequestFallbackUsed(c)
		}
		return s.handleStreamingReadCloser(
			c,
			workflow,
			result.Meta.Model,
			result.Meta.ProviderType,
			result.Meta.ProviderName,
			result.Meta.FailoverModel,
			result.Stream,
		)
	}

	providerStart := time.Now()
	result, err := s.inference().ExecuteChatCompletion(ctx, workflow, req, requestID, endpoint)
	recordGatewayPhase(endpoint, "provider_dispatch", errStatus(err), streamLabel, time.Since(providerStart))
	if err != nil {
		return handleError(c, err)
	}
	if result.Meta.UsedFallback {
		markRequestFallbackUsed(c)
		auditlog.EnrichEntryWithFailover(c, result.Meta.FailoverModel)
	}
	if result.Response != nil {
		auditlog.EnrichEntryWithResolvedRoute(
			c,
			qualifyExecutedModel(workflow, result.Response.Model, result.Meta.ProviderName),
			result.Meta.ProviderType,
			result.Meta.ProviderName,
		)
	}

	writeStart := time.Now()
	err = c.JSON(http.StatusOK, result.Response)
	recordGatewayPhase(endpoint, "response_write", errStatus(err), streamLabel, time.Since(writeStart))
	return err
}

func (s *translatedInferenceService) applyTokenSaverChat(c *echo.Context, req *core.ChatRequest, workflow *core.Workflow) (*core.ChatRequest, tokensaver.Metadata, error) {
	tokenSaver := s.currentTokenSaver()
	if tokenSaver == nil {
		return req, tokensaver.Metadata{SkipReason: tokensaver.SkipDisabled}, nil
	}
	meta := tokensaver.ChatMeta{
		Endpoint: tokensaver.EndpointChatCompletions,
		Model:    req.Model,
		Provider: req.Provider,
	}
	if workflow != nil {
		meta.Provider = gateway.ProviderNameFromWorkflow(workflow)
	}
	return tokenSaver.ApplyChat(req, meta)
}

func (s *translatedInferenceService) currentTokenSaver() *tokensaver.Service {
	s.tokenSaverMu.RLock()
	defer s.tokenSaverMu.RUnlock()
	return s.tokenSaver
}

func (s *translatedInferenceService) setTokenSaver(service *tokensaver.Service) {
	s.tokenSaverMu.Lock()
	defer s.tokenSaverMu.Unlock()
	s.tokenSaver = service
}

func (s *translatedInferenceService) setPromptCacheConfig(cfg *core.PromptCacheConfig) {
	s.promptCacheConfigMu.Lock()
	defer s.promptCacheConfigMu.Unlock()
	s.promptCacheConfig = cfg
}

func (s *translatedInferenceService) emitTokenSaverHeaders(c *echo.Context, meta tokensaver.Metadata) {
	if !meta.Enabled || !meta.EmitHeaders {
		return
	}
	header := c.Response().Header()
	if meta.Applied {
		header.Set("X-Aurora-Token-Saver", "applied")
	} else {
		header.Set("X-Aurora-Token-Saver", "skipped")
	}
	if meta.SkipReason != "" {
		header.Set("X-Aurora-Token-Saver-Skip", meta.SkipReason)
	}
	if meta.OutputProfileApplied {
		header.Set("X-Aurora-Token-Saver-Profile", "caveman")
	}
}

func (s *translatedInferenceService) Responses(c *echo.Context) error {
	return s.responsesHandler(c)
}

func (s *translatedInferenceService) handleResponses(c *echo.Context) error {
	return handleTranslatedJSON(s, c, core.DecodeResponsesRequest, prepareResponsesRequest, s.dispatchResponses)
}

func handleTranslatedJSON[Req any](
	s *translatedInferenceService,
	c *echo.Context,
	decode func([]byte, *core.WhiteBoxPrompt) (Req, error),
	prepare func(*translatedInferenceService, context.Context, Req, gateway.RequestMeta) (context.Context, Req, *core.Workflow, error),
	dispatch func(*echo.Context, Req, *core.Workflow) error,
) error {
	totalStart := time.Now()
	endpoint := c.Request().URL.Path
	streamLabel := "unknown"
	status := "success"
	defer func() {
		recordGatewayPhase(endpoint, "total", status, streamLabel, time.Since(totalStart))
	}()

	decodeStart := time.Now()
	req, err := canonicalJSONRequestFromSemantics[Req](c, decode)
	recordGatewayPhase(endpoint, "decode", errStatus(err), streamLabel, time.Since(decodeStart))
	if err != nil {
		status = "error"
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}
	streamLabel = streamLabelForRequest(req)

	prepareStart := time.Now()
	ctx, preparedReq, workflow, err := prepare(s, c.Request().Context(), req, translatedRequestMeta(c))
	recordGatewayPhase(endpoint, "prepare", errStatus(err), streamLabel, time.Since(prepareStart))
	if err != nil {
		status = "error"
		return handleError(c, err)
	}
	attachPreparedWorkflow(c, ctx, workflow)

	err = handleWithCache(s, c, preparedReq, workflow, dispatch)
	if err != nil {
		status = "error"
	}
	return err
}

func errStatus(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

func streamLabelForRequest(req any) string {
	switch typed := req.(type) {
	case *core.ChatRequest:
		return strconv.FormatBool(typed.Stream)
	case *core.ResponsesRequest:
		return strconv.FormatBool(typed.Stream)
	default:
		return "unknown"
	}
}

func recordGatewayPhase(endpoint, phase, status, stream string, duration time.Duration) {
	telemetry.GatewayPhaseDuration.WithLabelValues(endpoint, phase, status, stream).Observe(duration.Seconds())
}

func prepareChatCompletionRequest(
	s *translatedInferenceService,
	ctx context.Context,
	req *core.ChatRequest,
	meta gateway.RequestMeta,
) (context.Context, *core.ChatRequest, *core.Workflow, error) {
	prepared, err := s.inference().PrepareChatRequest(ctx, req, meta)
	return unpackPrepared(ctx, prepared, err, chatPreparedFields)
}

func prepareResponsesRequest(
	s *translatedInferenceService,
	ctx context.Context,
	req *core.ResponsesRequest,
	meta gateway.RequestMeta,
) (context.Context, *core.ResponsesRequest, *core.Workflow, error) {
	prepared, err := s.inference().PrepareResponsesRequest(ctx, req, meta)
	return unpackPrepared(ctx, prepared, err, responsesPreparedFields)
}

func unpackPrepared[Prepared any, Req any](
	fallback context.Context,
	prepared Prepared,
	err error,
	fields func(Prepared) (context.Context, Req, *core.Workflow),
) (context.Context, Req, *core.Workflow, error) {
	if err != nil {
		var zero Req
		return fallback, zero, nil, err
	}
	ctx, req, workflow := fields(prepared)
	return ctx, req, workflow, nil
}

func chatPreparedFields(prepared *gateway.PreparedChatRequest) (context.Context, *core.ChatRequest, *core.Workflow) {
	return prepared.Context, prepared.Request, prepared.Workflow
}

func responsesPreparedFields(prepared *gateway.PreparedResponsesRequest) (context.Context, *core.ResponsesRequest, *core.Workflow) {
	return prepared.Context, prepared.Request, prepared.Workflow
}

// handleWithCache routes translated requests through the response cache when
// enabled. The request has already been resolved and patched by the orchestrator.
// Cache hits intentionally return before dispatch and budget enforcement because
// they do not incur provider spend. Cache misses still run dispatch, where
// dispatchChatCompletion and dispatchResponses call enforceBudget before any
// provider request.
func handleWithCache[R any](
	s *translatedInferenceService,
	c *echo.Context,
	req R,
	workflow *core.Workflow,
	dispatch func(*echo.Context, R, *core.Workflow) error,
) error {
	if s.responseCache != nil && (workflow == nil || workflow.CacheEnabled()) {
		body, marshalErr := marshalRequestBody(req)
		if marshalErr != nil {
			slog.Debug("marshalRequestBody failed", "err", marshalErr)
		} else {
			return s.responseCache.HandleRequest(c, body, func() error {
				return dispatch(c, req, workflow)
			})
		}
	}

	return dispatch(c, req, workflow)
}

func (s *translatedInferenceService) dispatchResponses(c *echo.Context, req *core.ResponsesRequest, workflow *core.Workflow) error {
	ctx := c.Request().Context()
	requestID := requestIDFromContextOrHeader(c.Request())
	endpoint := "/v1/responses"
	streamLabel := strconv.FormatBool(req.Stream)

	if req.Stream {
		providerStart := time.Now()
		result, err := s.inference().StreamResponses(ctx, workflow, req)
		recordGatewayPhase(endpoint, "provider_dispatch", errStatus(err), streamLabel, time.Since(providerStart))
		if err != nil {
			return handleError(c, err)
		}
		if result.Meta.UsedFallback {
			markRequestFallbackUsed(c)
		}
		return s.handleStreamingReadCloser(
			c,
			workflow,
			result.Meta.Model,
			result.Meta.ProviderType,
			result.Meta.ProviderName,
			result.Meta.FailoverModel,
			result.Stream,
		)
	}

	providerStart := time.Now()
	result, err := s.inference().ExecuteResponses(ctx, workflow, req, requestID, endpoint)
	recordGatewayPhase(endpoint, "provider_dispatch", errStatus(err), streamLabel, time.Since(providerStart))
	if err != nil {
		return handleError(c, err)
	}
	if result.Meta.UsedFallback {
		markRequestFallbackUsed(c)
		auditlog.EnrichEntryWithFailover(c, result.Meta.FailoverModel)
	}
	if result.Response != nil {
		auditlog.EnrichEntryWithResolvedRoute(
			c,
			qualifyExecutedModel(workflow, result.Response.Model, result.Meta.ProviderName),
			result.Meta.ProviderType,
			result.Meta.ProviderName,
		)
	}

	if err := s.storeResponseSnapshot(ctx, workflow, req, result.Response, result.Meta.ProviderType, result.Meta.ProviderName, requestID); err != nil {
		s.recordResponseSnapshotStoreFailure(workflow, result.Response, result.Meta.ProviderType, result.Meta.ProviderName, requestID, err)
	}

	writeStart := time.Now()
	err = c.JSON(http.StatusOK, result.Response)
	recordGatewayPhase(endpoint, "response_write", errStatus(err), streamLabel, time.Since(writeStart))
	return err
}

func (s *translatedInferenceService) storeResponseSnapshot(ctx context.Context, workflow *core.Workflow, req *core.ResponsesRequest, resp *core.ResponsesResponse, providerType, providerName, requestID string) error {
	store := s.currentResponseStore()
	if store == nil || resp == nil || resp.ID == "" {
		return nil
	}

	stored := &responsestore.StoredResponse{
		Response:           resp,
		InputItems:         normalizedResponseInputItems(resp.ID, req),
		Provider:           strings.TrimSpace(providerType),
		ProviderName:       strings.TrimSpace(providerName),
		ProviderResponseID: resp.ID,
		RequestID:          requestID,
		UserPath:           accountingUserPath(ctx),
		WorkflowVersionID:  workflow.WorkflowVersionID(),
	}
	if createErr := store.Create(ctx, stored); createErr != nil {
		updateErr := store.Update(ctx, stored)
		if updateErr == nil {
			return nil
		}
		return core.NewProviderError("response_store", http.StatusInternalServerError, "failed to persist response", errors.Join(createErr, updateErr))
	}
	return nil
}

func (s *translatedInferenceService) currentResponseStore() responsestore.Store {
	s.responseStoreMu.RLock()
	defer s.responseStoreMu.RUnlock()
	return s.responseStore
}

func (s *translatedInferenceService) setResponseStore(store responsestore.Store) {
	s.responseStoreMu.Lock()
	defer s.responseStoreMu.Unlock()
	s.responseStore = store
}

func (s *translatedInferenceService) recordResponseSnapshotStoreFailure(workflow *core.Workflow, resp *core.ResponsesResponse, providerType, providerName, requestID string, err error) {
	telemetry.ResponseSnapshotStoreFailures.WithLabelValues(
		strings.TrimSpace(providerType),
		strings.TrimSpace(providerName),
		"store",
	).Inc()

	slog.Warn("response snapshot store failed",
		"request_id", requestID,
		"provider_type", providerType,
		"provider_name", providerName,
		"workflow_version_id", workflow.WorkflowVersionID(),
		"response_id", responseIDForLog(resp),
		"error", err,
	)
}

func responseIDForLog(resp *core.ResponsesResponse) string {
	if resp == nil {
		return ""
	}
	return strings.TrimSpace(resp.ID)
}

func (s *translatedInferenceService) tryFastPathStreamingChatPassthrough(c *echo.Context, workflow *core.Workflow, req *core.ChatRequest) (bool, error) {
	if !s.inference().CanFastPathStreamingChatPassthrough(workflow, req) {
		return false, nil
	}

	passthroughProvider, ok := s.provider.(core.RoutablePassthrough)
	if !ok {
		return false, nil
	}

	ctx, _ := requestContextWithRequestID(c.Request())
	c.SetRequest(c.Request().WithContext(ctx))

	const endpoint = "/chat/completions"
	providerType := strings.TrimSpace(workflow.ProviderType)
	resp, err := passthroughProvider.Passthrough(ctx, providerType, &core.PassthroughRequest{
		Method:   c.Request().Method,
		Endpoint: endpoint,
		Body:     c.Request().Body,
		Headers:  buildPassthroughHeaders(ctx, c.Request().Header),
	})
	if err != nil {
		return true, handleError(c, err)
	}

	info := &core.PassthroughRouteInfo{
		Provider:    providerType,
		RawEndpoint: strings.TrimPrefix(endpoint, "/"),
		AuditPath:   c.Request().URL.Path,
		Model:       resolvedModelFromWorkflow(workflow, req.Model),
	}
	passthrough := passthroughService{
		provider:        s.provider,
		logger:          s.logger,
		usageLogger:     s.usageLogger,
		pricingResolver: s.pricingResolver,
	}
	return true, passthrough.proxyPassthroughResponse(c, providerType, providerNameFromWorkflow(workflow), endpoint, info, resp)
}

func (s *translatedInferenceService) tryFastPathChatPassthrough(c *echo.Context, workflow *core.Workflow, req *core.ChatRequest) (bool, error) {
	if !s.inference().CanFastPathChatPassthrough(workflow, req, false) {
		return false, nil
	}

	passthroughProvider, ok := s.provider.(core.RoutablePassthrough)
	if !ok {
		return false, nil
	}

	ctx, _ := requestContextWithRequestID(c.Request())
	c.SetRequest(c.Request().WithContext(ctx))

	const endpoint = "/chat/completions"
	providerType := strings.TrimSpace(workflow.ProviderType)
	resp, err := passthroughProvider.Passthrough(ctx, providerType, &core.PassthroughRequest{
		Method:   c.Request().Method,
		Endpoint: endpoint,
		Body:     c.Request().Body,
		Headers:  buildPassthroughHeaders(ctx, c.Request().Header),
	})
	if err != nil {
		return true, handleError(c, err)
	}

	info := &core.PassthroughRouteInfo{
		Provider:    providerType,
		RawEndpoint: strings.TrimPrefix(endpoint, "/"),
		AuditPath:   c.Request().URL.Path,
		Model:       resolvedModelFromWorkflow(workflow, req.Model),
	}
	passthrough := passthroughService{
		provider:        s.provider,
		logger:          s.logger,
		usageLogger:     s.usageLogger,
		pricingResolver: s.pricingResolver,
	}
	return true, passthrough.proxyPassthroughResponse(c, providerType, providerNameFromWorkflow(workflow), endpoint, info, resp)
}

func nonStreamingChatPassthroughFastPathEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("AURORA_CHAT_FAST_PATH_PASSTHROUGH")))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func minimalBenchModeEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("AURORA_MINIMAL_BENCH_MODE")))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func (s *translatedInferenceService) Embeddings(c *echo.Context) error {
	req, err := canonicalJSONRequestFromSemantics[*core.EmbeddingRequest](c, core.DecodeEmbeddingRequest)
	if err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}

	prepared, err := s.inference().PrepareEmbeddingRequest(c.Request().Context(), req, translatedRequestMeta(c))
	if err != nil {
		return handleError(c, err)
	}
	attachPreparedWorkflow(c, prepared.Context, prepared.Workflow)

	requestID := requestIDFromContextOrHeader(c.Request())
	result, err := s.inference().ExecuteEmbeddings(c.Request().Context(), prepared.Workflow, prepared.Request, requestID, "/v1/embeddings")
	if err != nil {
		return handleError(c, err)
	}
	auditlog.EnrichEntryWithResolvedRoute(
		c,
		qualifyExecutedModel(prepared.Workflow, result.Response.Model, result.Meta.ProviderName),
		result.Meta.ProviderType,
		result.Meta.ProviderName,
	)

	return c.JSON(http.StatusOK, result.Response)
}

func (s *translatedInferenceService) Rerank(c *echo.Context) error {
	req, err := canonicalJSONRequestFromSemantics[*core.RerankRequest](c, core.DecodeRerankRequest)
	if err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}

	prepared, err := s.inference().PrepareRerankRequest(c.Request().Context(), req, translatedRequestMeta(c))
	if err != nil {
		return handleError(c, err)
	}
	attachPreparedWorkflow(c, prepared.Context, prepared.Workflow)

	requestID := requestIDFromContextOrHeader(c.Request())
	result, err := s.inference().ExecuteRerank(c.Request().Context(), prepared.Workflow, prepared.Request, requestID, "/v1/rerank")
	if err != nil {
		return handleError(c, err)
	}
	auditlog.EnrichEntryWithResolvedRoute(
		c,
		qualifyExecutedModel(prepared.Workflow, result.Response.Model, result.Meta.ProviderName),
		result.Meta.ProviderType,
		result.Meta.ProviderName,
	)

	return c.JSON(http.StatusOK, result.Response)
}

func translatedRequestMeta(c *echo.Context) gateway.RequestMeta {
	return gateway.RequestMeta{
		RequestID: requestIDFromContextOrHeader(c.Request()),
		Endpoint:  core.DescribeEndpoint(c.Request().Method, c.Request().URL.Path),
		Workflow:  core.GetWorkflow(c.Request().Context()),
	}
}

func attachPreparedWorkflow(c *echo.Context, ctx context.Context, workflow *core.Workflow) {
	if ctx != nil {
		c.SetRequest(c.Request().WithContext(ctx))
	}
	cacheWorkflowResolutionHints(c, workflow)
	storeWorkflow(c, workflow)
}

func cacheWorkflowResolutionHints(c *echo.Context, workflow *core.Workflow) {
	if c == nil || workflow == nil || workflow.Resolution == nil {
		return
	}
	if env := core.GetWhiteBoxPrompt(c.Request().Context()); env != nil {
		env.RouteHints.Model = workflow.Resolution.ResolvedSelector.Model
		env.RouteHints.Provider = workflow.Resolution.ResolvedSelector.Provider
	}
}

func (s *translatedInferenceService) handleStreamingReadCloser(
	c *echo.Context,
	workflow *core.Workflow,
	model, provider, providerName string,
	failoverModel string,
	stream io.ReadCloser,
) error {
	auditlog.MarkEntryAsStreaming(c, true)
	auditlog.EnrichEntryWithStream(c, true)
	auditlog.EnrichEntryWithFailover(c, failoverModel)
	auditlog.EnrichEntryWithResolvedRoute(c, qualifyExecutedModel(workflow, model, providerName), provider, providerName)

	entry := auditlog.GetStreamEntryFromContext(c)
	auditEnabled := s.logger != nil && s.logger.Config().Enabled && (workflow == nil || workflow.AuditEnabled())
	if auditEnabled && entry != nil {
		auditlog.PopulateRequestData(entry, c.Request(), s.logger.Config())
	}
	streamEntry := auditlog.CreateStreamEntry(entry)
	if streamEntry != nil {
		streamEntry.StatusCode = http.StatusOK
	}

	requestID := requestIDFromContextOrHeader(c.Request())
	endpoint := c.Request().URL.Path
	observers := make([]streaming.Observer, 0, 2)
	if auditEnabled && streamEntry != nil {
		observers = append(observers, auditlog.NewStreamLogObserver(s.logger, streamEntry, endpoint))
	}
	if s.usageLogger != nil && s.usageLogger.Config().Enabled && (workflow == nil || workflow.UsageEnabled()) {
		usageObserver := usage.NewStreamUsageObserver(s.usageLogger, model, provider, requestID, endpoint, s.pricingResolver, accountingUserPath(c.Request().Context()))
		if usageObserver != nil {
			usageObserver.SetProviderName(providerName)
			observers = append(observers, usageObserver)
		}
	}
	wrappedStream := streaming.NewObservedSSEStream(stream, observers...)

	defer func() {
		_ = wrappedStream.Close() //nolint:errcheck
	}()

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")

	if auditEnabled && streamEntry != nil && s.logger.Config().LogHeaders {
		auditlog.PopulateResponseHeaders(streamEntry, c.Response().Header())
	}

	c.Response().WriteHeader(http.StatusOK)
	if err := flushStream(c.Response(), wrappedStream); err != nil {
		recordStreamingError(streamEntry, model, provider, c.Request().URL.Path, requestID, err)
	}
	return nil
}

func (s *translatedInferenceService) handleStreamingResponse(
	c *echo.Context,
	workflow *core.Workflow,
	model, provider, providerName string,
	streamFn func() (io.ReadCloser, error),
) error {
	stream, err := streamFn()
	if err != nil {
		return handleError(c, err)
	}
	return s.handleStreamingReadCloser(c, workflow, model, provider, providerName, "", stream)
}

func recordStreamingError(streamEntry *auditlog.LogEntry, model, provider, path, requestID string, err error) {
	if streamEntry != nil {
		streamEntry.ErrorType = "stream_error"
		if streamEntry.Data == nil {
			streamEntry.Data = &auditlog.LogData{}
		}
		streamEntry.Data.ErrorMessage = err.Error()
	}

	slog.Warn("stream terminated abnormally",
		"error", err,
		"model", model,
		"provider", provider,
		"path", path,
		"request_id", requestID,
	)
}

func providerNameFromWorkflow(workflow *core.Workflow) string {
	return gateway.ProviderNameFromWorkflow(workflow)
}

func qualifyExecutedModel(workflow *core.Workflow, model, providerName string) string {
	return gateway.QualifyExecutedModel(workflow, model, providerName)
}

func markRequestFallbackUsed(c *echo.Context) {
	if c == nil || c.Request() == nil {
		return
	}
	c.SetRequest(c.Request().WithContext(core.WithFallbackUsed(c.Request().Context())))
}

func resolvedModelFromWorkflow(workflow *core.Workflow, fallback string) string {
	return gateway.ResolvedModelFromWorkflow(workflow, fallback)
}

func marshalRequestBody(req any) ([]byte, error) {
	return json.Marshal(req)
}

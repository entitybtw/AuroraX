package server

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/labstack/echo/v5"
	"github.com/pkoukk/tiktoken-go"

	"aurora/internal/audit_logging"
	"aurora/internal/core"
	"aurora/internal/gateway"

	"aurora/internal/response_cache"
	"aurora/internal/streaming"
	"aurora/internal/usage"
)

type anthropicIngressService struct {
	provider                 core.RoutableProvider
	modelResolver            RequestModelResolver
	modelAuthorizer          RequestModelAuthorizer
	workflowPolicyResolver   RequestWorkflowPolicyResolver
	fallbackResolver         RequestFallbackResolver
	translatedRequestPatcher TranslatedRequestPatcher
	logger                   auditlog.LoggerInterface
	usageLogger              usage.LoggerInterface

	pricingResolver          usage.PricingResolver
	responseCache            *responsecache.ResponseCacheMiddleware
	guardrailsHash           string
	promptCacheConfig        *core.PromptCacheConfig

	orchestrator *gateway.InferenceOrchestrator

	handlerFn echo.HandlerFunc
	initOnce  sync.Once
}

func newAnthropicIngressService(
	provider core.RoutableProvider,
	modelResolver RequestModelResolver,
	modelAuthorizer RequestModelAuthorizer,
	workflowPolicyResolver RequestWorkflowPolicyResolver,
	fallbackResolver RequestFallbackResolver,
	translatedRequestPatcher TranslatedRequestPatcher,
	logger auditlog.LoggerInterface,
	usageLogger usage.LoggerInterface,
	pricingResolver usage.PricingResolver,
	responseCache *responsecache.ResponseCacheMiddleware,
	guardrailsHash string,
) *anthropicIngressService {
	s := &anthropicIngressService{
		provider:                 provider,
		modelResolver:            modelResolver,
		modelAuthorizer:          modelAuthorizer,
		workflowPolicyResolver:   workflowPolicyResolver,
		fallbackResolver:         fallbackResolver,
		translatedRequestPatcher: translatedRequestPatcher,
		logger:                   logger,
		usageLogger:              usageLogger,

		pricingResolver:          pricingResolver,
		responseCache:            responseCache,
		guardrailsHash:           guardrailsHash,
	}
	return s
}

func (s *anthropicIngressService) ensureInit() {
	s.initOnce.Do(func() {
		s.orchestrator = s.newInferenceOrchestrator()
		s.handlerFn = s.handleMessages
	})
}

func (s *anthropicIngressService) Messages(c *echo.Context) error {
	s.ensureInit()
	return s.handlerFn(c)
}

func (s *anthropicIngressService) CountTokens(c *echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return handleError(c, core.NewInvalidRequestError("failed to read request body: "+err.Error(), err))
	}

	inputTokens := estimateAnthropicInputTokens(body)
	return c.JSON(http.StatusOK, anthropicCountTokensResponse{
		InputTokens: inputTokens,
	})
}

func (s *anthropicIngressService) inference() *gateway.InferenceOrchestrator {
	return s.orchestrator
}

func (s *anthropicIngressService) newInferenceOrchestrator() *gateway.InferenceOrchestrator {
	return gateway.NewInferenceOrchestrator(gateway.InferenceConfig{
		Provider:                 s.provider,
		ModelResolver:            s.modelResolver,
		ModelAuthorizer:          s.modelAuthorizer,
		WorkflowPolicyResolver:   s.workflowPolicyResolver,
		FallbackResolver:         s.fallbackResolver,
		TranslatedRequestPatcher: s.translatedRequestPatcher,
		UsageLogger:              s.usageLogger,
		PricingResolver:          s.pricingResolver,
		GuardrailsHash:           s.guardrailsHash,
			PromptCacheConfig:        s.promptCacheConfig, // nil = auto mode with defaults
	})
}

func (s *anthropicIngressService) handleMessages(c *echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return handleError(c, core.NewInvalidRequestError("failed to read request body: "+err.Error(), err))
	}

	chatReq, err := decodeAnthropicRequest(body)
	if err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid anthropic request: "+err.Error(), err))
	}

	meta := gateway.RequestMeta{
		RequestID: requestIDFromContextOrHeader(c.Request()),
		Endpoint:  core.DescribeEndpoint(c.Request().Method, c.Request().URL.Path),
		Workflow:  core.GetWorkflow(c.Request().Context()),
	}

	prepared, err := s.inference().PrepareChatRequest(c.Request().Context(), chatReq, meta)
	if err != nil {
		return handleError(c, err)
	}
	workflow := prepared.Workflow
	ctx := prepared.Context

	attachPreparedWorkflow(c, ctx, workflow)

	if chatReq.Stream {
		return s.handleStreamingMessages(c, workflow, chatReq)
	}

	return s.handleNonStreamingMessages(c, workflow, chatReq)
}

func (s *anthropicIngressService) handleNonStreamingMessages(c *echo.Context, workflow *core.Workflow, chatReq *core.ChatRequest) error {
	requestID := requestIDFromContextOrHeader(c.Request())

	result, err := s.inference().ExecuteChatCompletion(c.Request().Context(), workflow, chatReq, requestID, "/v1/messages")
	if err != nil {
		return handleError(c, err)
	}

	if result.Meta.UsedFallback {
		markRequestFallbackUsed(c)
		auditlog.EnrichEntryWithFailover(c, result.Meta.FailoverModel)
	}
	auditlog.EnrichEntryWithResolvedRoute(
		c,
		qualifyExecutedModel(workflow, result.Response.Model, result.Meta.ProviderName),
		result.Meta.ProviderType,
		result.Meta.ProviderName,
	)

	model := result.Response.Model
	if workflow != nil && workflow.Resolution != nil {
		model = workflow.Resolution.ResolvedSelector.Model
	}

	anthropicResp := formatAnthropicResponse(result.Response, model)
	if len(anthropicResp.Content) <= 1 && (len(anthropicResp.Content) == 0 || anthropicResp.Content[0].Text == "") {
		slog.Warn("anthropic ingress: returning minimal/empty response",
			"request_id", requestID,
			"model", model,
			"upstream_model", result.Response.Model,
			"content_blocks", len(anthropicResp.Content),
			"finish_reason", result.Response.Choices[0].FinishReason,
		)
	}
	return c.JSON(http.StatusOK, anthropicResp)
}

func (s *anthropicIngressService) handleStreamingMessages(c *echo.Context, workflow *core.Workflow, chatReq *core.ChatRequest) error {
	streamReq := chatReq.WithStreaming()

	result, err := s.inference().StreamChatCompletion(c.Request().Context(), workflow, streamReq)
	if err != nil {
		return handleError(c, err)
	}

	model := result.Meta.Model
	if workflow != nil && workflow.Resolution != nil {
		model = workflow.Resolution.ResolvedSelector.Model
	}

	auditlog.MarkEntryAsStreaming(c, true)
	auditlog.EnrichEntryWithStream(c, true)
	auditlog.EnrichEntryWithFailover(c, result.Meta.FailoverModel)
	auditlog.EnrichEntryWithResolvedRoute(c, qualifyExecutedModel(workflow, model, result.Meta.ProviderName), result.Meta.ProviderType, result.Meta.ProviderName)

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
	endpoint := "/v1/messages"
	observers := make([]streaming.Observer, 0, 2)
	if auditEnabled && streamEntry != nil {
		observers = append(observers, auditlog.NewStreamLogObserver(s.logger, streamEntry, endpoint))
	}
	if s.usageLogger != nil && s.usageLogger.Config().Enabled && (workflow == nil || workflow.UsageEnabled()) {
		usageObserver := usage.NewStreamUsageObserver(s.usageLogger, result.Meta.Model, result.Meta.ProviderType, requestID, endpoint, s.pricingResolver, accountingUserPath(c.Request().Context()))
		if usageObserver != nil {
			usageObserver.SetProviderName(result.Meta.ProviderName)
			observers = append(observers, usageObserver)
		}
	}

	convertedStream := newOpenAIToAnthropicStream(result.Stream, model)
	wrappedStream := streaming.NewObservedSSEStream(convertedStream, observers...)

	defer func() {
		_ = wrappedStream.Close()
	}()

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")

	if auditEnabled && streamEntry != nil && s.logger.Config().LogHeaders {
		auditlog.PopulateResponseHeaders(streamEntry, c.Response().Header())
	}

	c.Response().WriteHeader(http.StatusOK)
	if err := flushStream(c.Response(), wrappedStream); err != nil {
		slog.Warn("anthropic stream terminated abnormally",
			"error", err,
			"model", result.Meta.Model,
			"provider", result.Meta.ProviderType,
			"path", endpoint,
			"request_id", requestID,
		)
	}
	return nil
}

func (s *anthropicIngressService) handleStreamingReadCloser(
	c *echo.Context,
	workflow *core.Workflow,
	model, provider, providerName string,
	failoverModel string,
	openAIStream io.ReadCloser,
) error {
	anthropicStream := newOpenAIToAnthropicStream(openAIStream, model)

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
	endpoint := "/v1/messages"
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

	wrappedStream := streaming.NewObservedSSEStream(anthropicStream, observers...)

	defer func() {
		_ = wrappedStream.Close()
	}()

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")

	if auditEnabled && streamEntry != nil && s.logger.Config().LogHeaders {
		auditlog.PopulateResponseHeaders(streamEntry, c.Response().Header())
	}

	c.Response().WriteHeader(http.StatusOK)
	if err := flushStream(c.Response(), wrappedStream); err != nil {
		recordStreamingError(streamEntry, model, provider, endpoint, requestID, err)
	}
	return nil
}

// anthropicCountTokensResponse is the JSON response for POST /v1/messages/count_tokens.
type anthropicCountTokensResponse struct {
	InputTokens int `json:"input_tokens"`
}

// estimateAnthropicInputTokens tokenizes the request body and returns an
// accurate input token count using tiktoken (the same tokenizer used by
// OpenAI). The model name in the request determines which encoding to use:
//   - o200k_base for GPT-4o / o1 models
//   - cl100k_base for GPT-4 / GPT-3.5 / text-embedding models
//   - falls back to cl100k_base for unrecognised models
func estimateAnthropicInputTokens(body []byte) int {
	if len(body) == 0 {
		return 0
	}

	// Attempt to decode the request to extract the model name.
	// If decoding fails, fall back to cl100k_base (the most common encoding).
	model := ""
	if req, err := decodeAnthropicRequest(body); err == nil && req != nil {
		model = strings.TrimSpace(req.Model)
	}

	encoding := selectTokenEncoding(model)
	tkm, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		return len(body) / 4
	}

	tokens := tkm.Encode(string(body), nil, nil)
	return len(tokens)
}

func selectTokenEncoding(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	switch {
	case model == "":
		return "cl100k_base"
	case strings.HasPrefix(model, "gpt-4o"),
		strings.HasPrefix(model, "o1"),
		strings.HasPrefix(model, "o3"),
		strings.HasPrefix(model, "chatgpt-4o"):
		return "o200k_base"
	default:
		// cl100k_base covers gpt-4, gpt-3.5, text-embedding-*, and most
		// open-source models. It is the safest general-purpose fallback.
		return "cl100k_base"
	}
}

func anthropicIngressRequestID(c *echo.Context) string {
	id := c.Request().Header.Get("X-Request-ID")
	if id != "" {
		return id
	}
	return c.Response().Header().Get("X-Request-ID")
}

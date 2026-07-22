package server

import (
	"bytes"
	"context"
	json "github.com/goccy/go-json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"aurora/internal/audit_logging"
	"aurora/internal/authorization_scope"
	"aurora/internal/core"
	"aurora/internal/streaming"
	"aurora/internal/usage"
)

var defaultEnabledPassthroughProviders = []string{"openai", "anthropic", "openrouter", "zai", "vllm"}

func (h *Handler) setEnabledPassthroughProviders(providerTypes []string) {
	h.enabledPassthroughProviders = normalizeEnabledPassthroughProviders(providerTypes)
}

func isEnabledPassthroughProvider(providerType string, enabledPassthroughProviders map[string]struct{}) bool {
	providerType = strings.TrimSpace(providerType)
	if providerType == "" {
		return false
	}
	_, ok := enabledPassthroughProviders[providerType]
	return ok
}

func validatePassthroughProviderAccess(ctx context.Context, providerType string) error {
	policy, ok := core.GetAuthKeyAccessPolicy(ctx)
	if !ok || len(policy.AllowedProviders) == 0 {
		return nil
	}
	providerType = strings.TrimSpace(providerType)
	for _, allowed := range policy.AllowedProviders {
		if strings.EqualFold(strings.TrimSpace(allowed), providerType) {
			return nil
		}
	}
	return core.NewInvalidRequestError(fmt.Sprintf("provider passthrough for %q is not allowed by this tenant/auth key policy", providerType), nil)
}

func normalizeEnabledPassthroughProviders(providerTypes []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(providerTypes))
	for _, providerType := range providerTypes {
		providerType = strings.TrimSpace(providerType)
		if providerType == "" {
			continue
		}
		allowed[providerType] = struct{}{}
	}
	return allowed
}

func (s *passthroughService) enabledPassthroughProviderNames() []string {
	providers := make([]string, 0, len(s.enabledPassthroughProviders))
	for providerType := range s.enabledPassthroughProviders {
		providers = append(providers, providerType)
	}
	sort.Strings(providers)
	return providers
}

func (s *passthroughService) unsupportedPassthroughProviderError(providerType string) error {
	providers := s.enabledPassthroughProviderNames()
	if len(providers) == 0 {
		return core.NewInvalidRequestError("provider passthrough is not enabled for any providers", nil)
	}
	return core.NewInvalidRequestError(
		fmt.Sprintf("provider passthrough for %q is not enabled; currently enabled providers: %s", strings.TrimSpace(providerType), strings.Join(providers, ", ")),
		nil,
	)
}

func normalizePassthroughEndpoint(endpoint string, enabled bool) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	switch {
	case endpoint == "v1":
		if !enabled {
			return "", core.NewInvalidRequestError("provider passthrough v1 alias is disabled; use /p/{provider}/... without the v1 prefix", nil)
		}
		return "", nil
	case strings.HasPrefix(endpoint, "v1/"):
		if !enabled {
			return "", core.NewInvalidRequestError("provider passthrough v1 alias is disabled; use /p/{provider}/... without the v1 prefix", nil)
		}
		return strings.TrimPrefix(endpoint, "v1/"), nil
	default:
		return endpoint, nil
	}
}

func buildPassthroughHeaders(ctx context.Context, src http.Header) http.Header {
	if minimalBenchModeEnabled() {
		return src
	}

	connectionHeaders := passthroughConnectionHeaders(src)
	dst := make(http.Header)
	for key, values := range src {
		canonicalKey := http.CanonicalHeaderKey(strings.TrimSpace(key))
		if skipPassthroughRequestHeader(canonicalKey) || len(values) == 0 {
			continue
		}
		if _, hopByHop := connectionHeaders[canonicalKey]; hopByHop {
			continue
		}
		clonedValues := make([]string, len(values))
		copy(clonedValues, values)
		dst[canonicalKey] = clonedValues
	}
	requestID := strings.TrimSpace(src.Get("X-Request-ID"))
	if requestID == "" {
		requestID = strings.TrimSpace(core.GetRequestID(ctx))
	}
	if requestID != "" && strings.TrimSpace(dst.Get("X-Request-ID")) == "" {
		dst.Set("X-Request-ID", requestID)
	}
	if len(dst) == 0 {
		return nil
	}
	return dst
}

func skipPassthroughHeader(key string) bool {
	canonicalKey := http.CanonicalHeaderKey(strings.TrimSpace(key))
	switch canonicalKey {
	case "Authorization", "X-Api-Key", "Host", "Content-Length", "Connection", "Keep-Alive",
		"Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
		"Cookie", "Forwarded", "Set-Cookie":
		return true
	default:
		return strings.HasPrefix(canonicalKey, "X-Forwarded-")
	}
}

func skipPassthroughRequestHeader(key string) bool {
	if http.CanonicalHeaderKey(strings.TrimSpace(key)) == http.CanonicalHeaderKey(core.UserPathHeader) {
		return true
	}
	return skipPassthroughHeader(key)
}

const maxPassthroughUsageBodyBytes = 256 * 1024

func (s *passthroughService) logPassthroughUsage(c *echo.Context, body []byte, providerType, providerName, endpoint string, info *core.PassthroughRouteInfo) {
	if s.usageLogger == nil || !s.usageLogger.Config().Enabled {
		return
	}
	workflow := core.GetWorkflow(c.Request().Context())
	if workflow != nil && !workflow.UsageEnabled() {
		return
	}
	if len(body) == 0 || len(body) > maxPassthroughUsageBodyBytes {
		return
	}

	model := ""
	if info != nil {
		model = strings.TrimSpace(info.Model)
	}
	model = resolvedModelFromWorkflow(workflow, model)

	usagePath := passthroughAuditPath(c, providerType, endpoint, info)
	entry := extractNonStreamingPassthroughUsage(body, model, providerType, requestIDFromContextOrHeader(c.Request()), usagePath)
	if entry == nil {
		return
	}

	if s.pricingResolver != nil {
		if pricingResult := s.pricingResolver.ResolvePricing(model, providerType); pricingResult != nil {
			costResult := usage.CalculateUsageCost(entry.InputTokens, entry.OutputTokens, entry.RawData, providerType, pricingResult.Pricing)
			entry.InputCost = costResult.InputCost
			entry.OutputCost = costResult.OutputCost
			entry.TotalCost = costResult.TotalCost
			entry.CostSource = costResult.Source
			entry.CostsCalculationCaveat = costResult.Caveat
			if p := pricingResult.Provenance; p != nil {
				entry.PricingSource = p.Source
				entry.PricingVersion = p.Version
				if !p.ResolvedAt.IsZero() {
					entry.PricingResolvedAt = &p.ResolvedAt
				}
			}
		}
	}
	entry.ProviderName = strings.TrimSpace(providerName)
	entry.TenantID = scope.TenantIDFromContext(c.Request().Context())
	entry.UserPath = accountingUserPath(c.Request().Context())
	s.usageLogger.Write(entry)
}

func extractNonStreamingPassthroughUsage(body []byte, model, provider, requestID, endpoint string) *usage.UsageEntry {
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}

	var payload struct {
		Usage map[string]any `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Usage == nil {
		return nil
	}

	var inputTokens, outputTokens, totalTokens int
	rawData := make(map[string]any)

	if v, ok := numericValue(payload.Usage["prompt_tokens"]); ok {
		inputTokens = v
	}
	if v, ok := numericValue(payload.Usage["input_tokens"]); ok {
		inputTokens = v
	}
	if v, ok := numericValue(payload.Usage["completion_tokens"]); ok {
		outputTokens = v
	}
	if v, ok := numericValue(payload.Usage["output_tokens"]); ok {
		outputTokens = v
	}
	if v, ok := numericValue(payload.Usage["total_tokens"]); ok {
		totalTokens = v
	}

	for key, value := range payload.Usage {
		switch key {
		case "prompt_tokens", "completion_tokens", "total_tokens",
			"input_tokens", "output_tokens":
			continue
		}
		if nv, ok := numericValue(value); ok && nv > 0 {
			rawData[key] = nv
		}
	}

	if inputTokens == 0 && outputTokens == 0 && totalTokens == 0 {
		return nil
	}
	if len(rawData) == 0 {
		rawData = nil
	}

	return &usage.UsageEntry{
		ID:           uuid.New().String(),
		RequestID:    requestID,
		Timestamp:    time.Now().UTC(),
		Model:        model,
		Provider:     provider,
		Endpoint:     endpoint,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  totalTokens,
		RawData:      rawData,
	}
}

func isJSONResponse(body []byte) bool {
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func numericValue(v any) (int, bool) {
	switch typed := v.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case json.Number:
		n, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(n), true
	default:
		return 0, false
	}
}

func passthroughConnectionHeaders(headers http.Header) map[string]struct{} {
	var tokens map[string]struct{}
	for key, values := range headers {
		if http.CanonicalHeaderKey(strings.TrimSpace(key)) != "Connection" {
			continue
		}
		for _, value := range values {
			for token := range strings.SplitSeq(value, ",") {
				canonicalKey := http.CanonicalHeaderKey(strings.TrimSpace(token))
				if canonicalKey == "" {
					continue
				}
				if tokens == nil {
					tokens = make(map[string]struct{})
				}
				tokens[canonicalKey] = struct{}{}
			}
		}
	}
	return tokens
}

func copyPassthroughResponseHeaders(dst, src http.Header) {
	connectionHeaders := passthroughConnectionHeaders(src)
	for key, values := range src {
		canonicalKey := http.CanonicalHeaderKey(strings.TrimSpace(key))
		if skipPassthroughHeader(canonicalKey) || len(values) == 0 {
			continue
		}
		if _, hopByHop := connectionHeaders[canonicalKey]; hopByHop {
			continue
		}
		dst.Del(canonicalKey)
		for _, value := range values {
			dst.Add(canonicalKey, value)
		}
	}
}

func isSSEContentType(headers map[string][]string) bool {
	for key, values := range headers {
		if !strings.EqualFold(key, "Content-Type") {
			continue
		}
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), "text/event-stream") {
				return true
			}
		}
	}
	return false
}

func passthroughStreamAuditPath(requestPath, providerType, endpoint string) string {
	normalized := "/" + strings.TrimLeft(strings.SplitN(endpoint, "?", 2)[0], "/")
	switch providerType {
	case "openai":
		switch normalized {
		case "/chat/completions":
			return "/v1/chat/completions"
		case "/responses":
			return "/v1/responses"
		}
	case "anthropic":
		switch normalized {
		case "/messages":
			return "/v1/messages"
		}
	}
	return requestPath
}

func passthroughAuditPath(c *echo.Context, providerType, endpoint string, info *core.PassthroughRouteInfo) string {
	if info != nil {
		if auditPath := strings.TrimSpace(info.AuditPath); auditPath != "" {
			return auditPath
		}
	}
	if c != nil {
		if workflow := core.GetWorkflow(c.Request().Context()); workflow != nil && workflow.Passthrough != nil {
			if auditPath := strings.TrimSpace(workflow.Passthrough.AuditPath); auditPath != "" {
				return auditPath
			}
		}
		if env := core.GetWhiteBoxPrompt(c.Request().Context()); env != nil {
			if info := env.CachedPassthroughRouteInfo(); info != nil {
				if auditPath := strings.TrimSpace(info.AuditPath); auditPath != "" {
					return auditPath
				}
			}
		}
		if requestPath := strings.TrimSpace(c.Request().URL.Path); requestPath != "" {
			return passthroughStreamAuditPath(requestPath, providerType, endpoint)
		}
	}
	return passthroughStreamAuditPath("", providerType, endpoint)
}

func (s *passthroughService) proxyPassthroughResponse(c *echo.Context, providerType, providerName, endpoint string, info *core.PassthroughRouteInfo, resp *core.PassthroughResponse) error {
	if resp == nil || resp.Body == nil {
		return handleError(c, core.NewProviderError(providerType, http.StatusBadGateway, "provider returned empty passthrough response", nil))
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= http.StatusBadRequest {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return handleError(c, core.NewProviderError(providerType, http.StatusBadGateway, "failed to read provider passthrough error response", err))
		}
		return handleError(c, core.ParseProviderError(providerType, resp.StatusCode, body, nil))
	}

	copyPassthroughResponseHeaders(c.Response().Header(), http.Header(resp.Headers))

	if minimalBenchModeEnabled() && !isSSEContentType(resp.Headers) {
		c.Response().WriteHeader(resp.StatusCode)
		if _, err := io.Copy(c.Response(), resp.Body); err != nil {
			return err
		}
		return nil
	}

	if isSSEContentType(resp.Headers) {
		auditlog.MarkEntryAsStreaming(c, true)
		auditlog.EnrichEntryWithStream(c, true)
		workflow := core.GetWorkflow(c.Request().Context())
		auditEnabled := s.logger != nil && s.logger.Config().Enabled && (workflow == nil || workflow.AuditEnabled())

		entry := auditlog.GetStreamEntryFromContext(c)
		if auditEnabled && entry != nil {
			auditlog.PopulateRequestData(entry, c.Request(), s.logger.Config())
		}
		streamEntry := auditlog.CreateStreamEntry(entry)
		if streamEntry != nil {
			streamEntry.StatusCode = resp.StatusCode
		}
		if auditEnabled && streamEntry != nil && s.logger.Config().LogHeaders {
			auditlog.PopulateResponseHeaders(streamEntry, c.Response().Header())
		}

		requestID := requestIDFromContextOrHeader(c.Request())
		auditPath := passthroughAuditPath(c, providerType, endpoint, info)
		requestPath := c.Request().URL.Path
		model := ""
		if info != nil {
			model = strings.TrimSpace(info.Model)
		}
		model = resolvedModelFromWorkflow(workflow, model)

		observers := make([]streaming.Observer, 0, 2)
		if auditEnabled && streamEntry != nil {
			if observer := auditlog.NewStreamLogObserver(s.logger, streamEntry, auditPath); observer != nil {
				observers = append(observers, observer)
			}
		}
		if s.usageLogger != nil && s.usageLogger.Config().Enabled && (workflow == nil || workflow.UsageEnabled()) {
			if observer := usage.NewStreamUsageObserver(s.usageLogger, model, providerType, requestID, requestPath, s.pricingResolver, accountingUserPath(c.Request().Context())); observer != nil {
				observer.SetProviderName(providerName)
				observers = append(observers, observer)
			}
		}
		wrappedStream := streaming.NewObservedSSEStream(resp.Body, observers...)
		if len(observers) > 0 {
			defer func() {
				_ = wrappedStream.Close()
			}()
		}

		c.Response().WriteHeader(resp.StatusCode)
		if err := flushStream(c.Response(), wrappedStream); err != nil {
			recordStreamingError(streamEntry, model, providerType, c.Request().URL.Path, requestID, err)
			return err
		}
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return handleError(c, core.NewProviderError(providerType, http.StatusBadGateway, "failed to read passthrough response body", err))
	}

	c.Response().WriteHeader(resp.StatusCode)
	if _, err := c.Response().Write(body); err != nil {
		return err
	}
	if f, ok := c.Response().(http.Flusher); ok {
		f.Flush()
	}

	s.logPassthroughUsage(c, body, providerType, providerName, endpoint, info)
	return nil
}

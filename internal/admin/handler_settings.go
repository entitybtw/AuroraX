package admin

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/labstack/echo/v5"

	"aurora/internal/audit_logging"
	"aurora/internal/core"
)

type DashboardSettingsUpdateRequest struct {
	Client               DashboardSettingsUpdateClient               `json:"client"`
	Caching              DashboardSettingsUpdateCaching              `json:"caching"`
	Logging              DashboardSettingsUpdateLogging              `json:"logging"`
	Observability        DashboardSettingsUpdateObservability        `json:"observability"`

	Performance          DashboardSettingsUpdatePerformance          `json:"performance"`
	Security             DashboardSettingsUpdateSecurity             `json:"security"`
	Pricing              DashboardSettingsUpdatePricing              `json:"pricing"`
	TokenSaver           DashboardSettingsUpdateTokenSaver           `json:"token_saver"`
	Proxy                DashboardSettingsUpdateProxy                `json:"proxy"`
}

type DashboardSettingsUpdateClient struct {
	Port                            string   `json:"port"`
	BasePath                        string   `json:"base_path"`
	BodySizeLimit                   string   `json:"body_size_limit"`
	SwaggerEnabled                  bool     `json:"swagger_enabled"`
	PprofEnabled                    bool     `json:"pprof_enabled"`
	ConfiguredProviderModelsMode    string   `json:"configured_provider_models_mode"`
	KeepOnlyAliasesAtModelsEndpoint bool     `json:"keep_only_aliases_at_models_endpoint"`
	AllowPassthroughV1Alias         bool     `json:"allow_passthrough_v1_alias"`
	EnabledPassthroughProviders     []string `json:"enabled_passthrough_providers"`
	AdminEndpointsEnabled           *bool    `json:"admin_endpoints_enabled,omitempty"`
	AdminUIEnabled                  *bool    `json:"admin_ui_enabled,omitempty"`
	EnableAnthropicIngress          *bool    `json:"enable_anthropic_ingress,omitempty"`
}

type DashboardSettingsUpdateCachingPromptCache struct {
	Mode              string `json:"prompt_cache_mode"`
	SystemPromptCache bool   `json:"prompt_cache_system_prompt"`
	FirstMessageCache bool   `json:"prompt_cache_first_message"`
	ToolsCache        bool   `json:"prompt_cache_tools"`
	MinTokens         int    `json:"prompt_cache_min_tokens"`
}

type DashboardSettingsUpdateCaching struct {
	ModelRefreshIntervalSeconds     int                                      `json:"model_refresh_interval_seconds"`
	ModelListURL                    string                                   `json:"model_list_url"`
	ExactCacheEnabled               *bool                                    `json:"exact_cache_enabled,omitempty"`
	ExactCacheTTLSeconds            *int                                     `json:"exact_cache_ttl_seconds,omitempty"`
	ExactCacheRedisKey              string                                   `json:"exact_cache_redis_key,omitempty"`
	SemanticCacheEnabled            *bool                                    `json:"semantic_cache_enabled,omitempty"`
	SemanticSimilarityThreshold     *float64                                 `json:"semantic_similarity_threshold,omitempty"`
	SemanticPromptSimilarityMin     *float64                                 `json:"semantic_prompt_similarity_min,omitempty"`
	SemanticTTLSeconds              *int                                     `json:"semantic_ttl_seconds,omitempty"`
	SemanticMaxConversationMessages *int                                     `json:"semantic_max_conversation_messages,omitempty"`
	SemanticExcludeSystemPrompt     *bool                                    `json:"semantic_exclude_system_prompt,omitempty"`
	SemanticEmbedderProvider        string                                   `json:"semantic_embedder_provider,omitempty"`
	SemanticEmbedderModel           string                                   `json:"semantic_embedder_model,omitempty"`
	SemanticVectorStoreType         string                                   `json:"semantic_vector_store_type,omitempty"`
	PromptCache                     DashboardSettingsUpdateCachingPromptCache `json:"prompt_cache"`
}

type DashboardSettingsUpdateLogging struct {
	Enabled               bool `json:"enabled"`
	LogBodies             bool `json:"log_bodies"`
	LogHeaders            bool `json:"log_headers"`
	BufferSize            int  `json:"buffer_size"`
	FlushIntervalSeconds  int  `json:"flush_interval_seconds"`
	RetentionDays         int  `json:"retention_days"`
	OnlyModelInteractions bool `json:"only_model_interactions"`
}

type DashboardSettingsUpdateObservability struct {
	MetricsEnabled  bool   `json:"metrics_enabled"`
	MetricsEndpoint string `json:"metrics_endpoint"`
}

type DashboardSettingsUpdatePerformance struct {
	HTTPTimeoutSeconds                int     `json:"http_timeout_seconds"`
	HTTPResponseHeaderTimeoutSeconds  int     `json:"http_response_header_timeout_seconds"`
	WorkflowRefreshIntervalSeconds    int     `json:"workflow_refresh_interval_seconds"`
	RetryMaxRetries                   int     `json:"retry_max_retries"`
	RetryInitialBackoffMilliseconds   int64   `json:"retry_initial_backoff_milliseconds"`
	RetryMaxBackoffMilliseconds       int64   `json:"retry_max_backoff_milliseconds"`
	RetryBackoffFactor                float64 `json:"retry_backoff_factor"`
	RetryJitterFactor                 float64 `json:"retry_jitter_factor"`
	CircuitBreakerFailureThreshold    int     `json:"circuit_breaker_failure_threshold"`
	CircuitBreakerSuccessThreshold    int     `json:"circuit_breaker_success_threshold"`
	CircuitBreakerTimeoutMilliseconds int64   `json:"circuit_breaker_timeout_milliseconds"`
}

type DashboardSettingsUpdateSecurity struct {
	GuardrailsEnabled bool `json:"guardrails_enabled"`
	BatchGuardrails   bool `json:"batch_guardrails"`
}

type DashboardSettingsUpdatePricing struct {
	EnforceReturningUsageData   bool `json:"enforce_returning_usage_data"`
	PricingRecalculationEnabled bool `json:"pricing_recalculation_enabled"`
	UsageRetentionDays          int  `json:"usage_retention_days"`
}

type DashboardSettingsUpdateTokenSaver struct {
	Enabled         bool     `json:"enabled"`
	ApplyStreaming  bool     `json:"apply_streaming"`
	Endpoints       []string `json:"endpoints"`
	OutputEnabled   bool     `json:"output_enabled"`
	OutputProfile   string   `json:"output_profile"`
	OutputLevel     string   `json:"output_level"`
	EmitHeaders     bool     `json:"emit_headers"`
	OnError         string   `json:"on_error"`
	ModelInclude    []string `json:"model_include"`
	ModelExclude    []string `json:"model_exclude"`
	ProviderInclude []string `json:"provider_include"`
	ProviderExclude []string `json:"provider_exclude"`
	AuditEnabled    bool     `json:"audit_enabled"`
}

type DashboardSettingsUpdateProxy struct {
	HTTPProxy        string `json:"http_proxy"`
	HTTPSProxy       string `json:"https_proxy"`
	NoProxy          string `json:"no_proxy"`
	ProxyAuthEnabled bool   `json:"proxy_auth_enabled"`
	CACertPEM        string `json:"ca_cert_pem"`
}

type DashboardSettingsUpdateResponse struct {
	Message          string                  `json:"message"`
	RefreshSuggested bool                    `json:"refresh_suggested"`
	RequiresRestart  bool                    `json:"requires_restart"`
	RestartReasons   []string                `json:"restart_reasons,omitempty"`
	DashboardConfig  DashboardConfigResponse `json:"dashboard_config"`
}

// UpdateDashboardSettings handles PUT /admin/api/v1/dashboard/settings.
func (h *Handler) UpdateDashboardSettings(c *echo.Context) error {
	if h.settingsManager == nil {
		code := "service_unavailable"
		return c.JSON(http.StatusServiceUnavailable, core.GatewayError{
			Code:    &code,
			Type:    "feature_unavailable",
			Message: "dashboard settings updates are unavailable",
		})
	}

	var req DashboardSettingsUpdateRequest
	if err := c.Bind(&req); err != nil {
		code := "bad_request"
		return c.JSON(http.StatusBadRequest, core.GatewayError{
			Code:    &code,
			Type:    "invalid_request_error",
			Message: "invalid dashboard settings payload: " + err.Error(),
		})
	}

	resp, err := h.settingsManager.UpdateDashboardSettings(c.Request().Context(), req)
	if err != nil {
		message := strings.TrimSpace(err.Error())
		if message == "" {
			message = "failed to update dashboard settings"
		}
		code := "bad_request"
		return c.JSON(http.StatusBadRequest, core.GatewayError{
			Code:    &code,
			Type:    "invalid_request_error",
			Message: message,
		})
	}

	h.mutationMu.Lock()
	h.runtimeConfig = normalizeDashboardRuntimeConfig(resp.DashboardConfig)
	h.mutationMu.Unlock()
	h.auditDashboardSettingsChange(c, req)

	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) auditDashboardSettingsChange(c *echo.Context, req DashboardSettingsUpdateRequest) {
	if h == nil || h.auditLogger == nil {
		return
	}
	entry := &auditlog.LogEntry{
		ID:         uuid.NewString(),
		Timestamp:  time.Now().UTC(),
		StatusCode: http.StatusOK,
		Method:     "PUT",
		Path:       "/admin/api/v1/dashboard/settings",
		ClientIP:   c.RealIP(),
		Data: &auditlog.LogData{
			RequestBody: dashboardSettingsAuditSummary(req),
		},
	}
	if request := c.Request(); request != nil {
		entry.RequestID = request.Header.Get("X-Request-ID")
		entry.Data.UserAgent = request.UserAgent()
	}
	h.auditLogger.Write(entry)
}

func dashboardSettingsAuditSummary(req DashboardSettingsUpdateRequest) map[string]any {
	return map[string]any{

	}
}

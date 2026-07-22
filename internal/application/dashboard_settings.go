package app

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"aurora/configuration"
	"aurora/internal/admin"
)

type dashboardSettingsOverlay struct {
	Server               *dashboardServerOverlay               `yaml:"server,omitempty"`
	Models               *dashboardModelsOverlay               `yaml:"models,omitempty"`
	Cache                *dashboardCacheOverlay                `yaml:"cache,omitempty"`
	Logging              *dashboardLoggingOverlay              `yaml:"logging,omitempty"`
	Usage                *dashboardUsageOverlay                `yaml:"usage,omitempty"`
	Metrics              *dashboardMetricsOverlay              `yaml:"metrics,omitempty"`

	HTTP                 *dashboardHTTPOverlay                 `yaml:"http,omitempty"`
	Proxy                *dashboardProxyOverlay                `yaml:"proxy,omitempty"`
	Resilience           *dashboardResilienceOverlay           `yaml:"resilience,omitempty"`
	Guardrails           *dashboardGuardrailsOverlay           `yaml:"guardrails,omitempty"`
	Workflows            *dashboardWorkflowsOverlay            `yaml:"workflows,omitempty"`
	TokenSaver           *tokenSaverOverlay                    `yaml:"token_saver,omitempty"`
}

type dashboardServerOverlay struct {
	Port                        *string  `yaml:"port,omitempty"`
	BasePath                    *string  `yaml:"base_path,omitempty"`
	BodySizeLimit               *string  `yaml:"body_size_limit,omitempty"`
	SwaggerEnabled              *bool    `yaml:"swagger_enabled,omitempty"`
	PprofEnabled                *bool    `yaml:"pprof_enabled,omitempty"`
	AllowPassthroughV1Alias     *bool    `yaml:"allow_passthrough_v1_alias,omitempty"`
	EnabledPassthroughProviders []string `yaml:"enabled_passthrough_providers,omitempty"`
	AdminEndpointsEnabled       *bool    `yaml:"admin_endpoints_enabled,omitempty"`
	AdminUIEnabled              *bool    `yaml:"admin_ui_enabled,omitempty"`
	EnableAnthropicIngress      *bool    `yaml:"enable_anthropic_ingress,omitempty"`
}

type dashboardModelsOverlay struct {
	ConfiguredProviderModelsMode    *string `yaml:"configured_provider_models_mode,omitempty"`
	KeepOnlyAliasesAtModelsEndpoint *bool   `yaml:"keep_only_aliases_at_models_endpoint,omitempty"`
}

type dashboardCacheOverlay struct {
	Model    *dashboardCacheModelOverlay    `yaml:"model,omitempty"`
	Response *dashboardCacheResponseOverlay `yaml:"response,omitempty"`
	Prompt   *dashboardCachePromptOverlay   `yaml:"prompt,omitempty"`
}

type dashboardCachePromptOverlay struct {
	Mode              *string `yaml:"mode,omitempty"`
	SystemPromptCache *bool   `yaml:"system_prompt,omitempty"`
	FirstMessageCache *bool   `yaml:"first_message,omitempty"`
	ToolsCache        *bool   `yaml:"tools,omitempty"`
	MinTokens         *int    `yaml:"min_tokens,omitempty"`
}

type dashboardCacheModelOverlay struct {
	RefreshInterval *int                       `yaml:"refresh_interval,omitempty"`
	ModelList       *dashboardModelListOverlay `yaml:"model_list,omitempty"`
}

type dashboardModelListOverlay struct {
	URL *string `yaml:"url,omitempty"`
}

type dashboardCacheResponseOverlay struct {
	Simple   *dashboardCacheSimpleOverlay   `yaml:"simple,omitempty"`
	Semantic *dashboardCacheSemanticOverlay `yaml:"semantic,omitempty"`
}

type dashboardCacheSimpleOverlay struct {
	Enabled *bool                             `yaml:"enabled,omitempty"`
	Redis   *dashboardCacheSimpleRedisOverlay `yaml:"redis,omitempty"`
}

type dashboardCacheSimpleRedisOverlay struct {
	Key *string `yaml:"key,omitempty"`
	TTL *int    `yaml:"ttl,omitempty"`
}

type dashboardCacheSemanticOverlay struct {
	Enabled                   *bool    `yaml:"enabled,omitempty"`
	SimilarityThreshold       *float64 `yaml:"similarity_threshold,omitempty"`
	PromptSimilarityThreshold *float64 `yaml:"prompt_similarity_threshold,omitempty"`
	TTL                       *int     `yaml:"ttl,omitempty"`
	MaxConversationMessages   *int     `yaml:"max_conversation_messages,omitempty"`
	ExcludeSystemPrompt       *bool    `yaml:"exclude_system_prompt,omitempty"`
	EmbedderProvider          *string  `yaml:"embedder_provider,omitempty"`
	EmbedderModel             *string  `yaml:"embedder_model,omitempty"`
	VectorStoreType           *string  `yaml:"vector_store_type,omitempty"`
}

type dashboardLoggingOverlay struct {
	Enabled               *bool `yaml:"enabled,omitempty"`
	LogBodies             *bool `yaml:"log_bodies,omitempty"`
	LogHeaders            *bool `yaml:"log_headers,omitempty"`
	BufferSize            *int  `yaml:"buffer_size,omitempty"`
	FlushInterval         *int  `yaml:"flush_interval,omitempty"`
	RetentionDays         *int  `yaml:"retention_days,omitempty"`
	OnlyModelInteractions *bool `yaml:"only_model_interactions,omitempty"`
}

type dashboardUsageOverlay struct {
	EnforceReturningUsageData   *bool `yaml:"enforce_returning_usage_data,omitempty"`
	PricingRecalculationEnabled *bool `yaml:"pricing_recalculation_enabled,omitempty"`
	RetentionDays               *int  `yaml:"retention_days,omitempty"`
}

type dashboardMetricsOverlay struct {
	Enabled  *bool   `yaml:"enabled,omitempty"`
	Endpoint *string `yaml:"endpoint,omitempty"`
}

type dashboardHTTPOverlay struct {
	Timeout               *int `yaml:"timeout,omitempty"`
	ResponseHeaderTimeout *int `yaml:"response_header_timeout,omitempty"`
}

type dashboardProxyOverlay struct {
	HTTPProxy        *string `yaml:"http_proxy,omitempty"`
	HTTPSProxy       *string `yaml:"https_proxy,omitempty"`
	NoProxy          *string `yaml:"no_proxy,omitempty"`
	ProxyAuthEnabled *bool   `yaml:"proxy_auth_enabled,omitempty"`
	CACertPEM        *string `yaml:"ca_cert_pem,omitempty"`
}

type dashboardResilienceOverlay struct {
	Retry          *dashboardRetryOverlay          `yaml:"retry,omitempty"`
	CircuitBreaker *dashboardCircuitBreakerOverlay `yaml:"circuit_breaker,omitempty"`
}

type dashboardRetryOverlay struct {
	MaxRetries     *int     `yaml:"max_retries,omitempty"`
	InitialBackoff *string  `yaml:"initial_backoff,omitempty"`
	MaxBackoff     *string  `yaml:"max_backoff,omitempty"`
	BackoffFactor  *float64 `yaml:"backoff_factor,omitempty"`
	JitterFactor   *float64 `yaml:"jitter_factor,omitempty"`
}

type dashboardCircuitBreakerOverlay struct {
	FailureThreshold *int    `yaml:"failure_threshold,omitempty"`
	SuccessThreshold *int    `yaml:"success_threshold,omitempty"`
	Timeout          *string `yaml:"timeout,omitempty"`
}

type dashboardGuardrailsOverlay struct {
	Enabled                  *bool `yaml:"enabled,omitempty"`
	EnableForBatchProcessing *bool `yaml:"enable_for_batch_processing,omitempty"`
}

type dashboardWorkflowsOverlay struct {
	RefreshInterval *string `yaml:"refresh_interval,omitempty"`
}

type tokenSaverOverlay struct {
	Enabled        *bool                    `yaml:"enabled,omitempty"`
	Endpoints      []string                 `yaml:"endpoints,omitempty"`
	ApplyStreaming *bool                    `yaml:"apply_streaming,omitempty"`
	Output         *tokenSaverOutputOverlay `yaml:"output,omitempty"`
	Models         *tokenSaverScopeOverlay  `yaml:"models,omitempty"`
	Providers      *tokenSaverScopeOverlay  `yaml:"providers,omitempty"`
	OnError        *string                  `yaml:"on_error,omitempty"`
	EmitHeaders    *bool                    `yaml:"emit_headers,omitempty"`
	AuditEnabled   *bool                    `yaml:"audit_enabled,omitempty"`
}

type tokenSaverOutputOverlay struct {
	Enabled *bool   `yaml:"enabled,omitempty"`
	Profile *string `yaml:"profile,omitempty"`
	Level   *string `yaml:"level,omitempty"`
}

type tokenSaverScopeOverlay struct {
	Include []string `yaml:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

func (a *App) UpdateDashboardSettings(_ context.Context, req admin.DashboardSettingsUpdateRequest) (admin.DashboardSettingsUpdateResponse, error) {
	if a == nil || a.config == nil {
		return admin.DashboardSettingsUpdateResponse{}, fmt.Errorf("application config is unavailable")
	}
	if err := validateDashboardSettingsUpdate(a.config, req); err != nil {
		return admin.DashboardSettingsUpdateResponse{}, err
	}

	overlayPath := config.DashboardOverridesPath()
	overlay, err := loadDashboardSettingsOverlay(overlayPath)
	if err != nil {
		return admin.DashboardSettingsUpdateResponse{}, err
	}

	oldCfg := *a.config
	applyDashboardSettingsToConfig(a.config, req)
	applyHTTPClientConfig(a.config.HTTP)
	applyDashboardSettingsToOverlay(&overlay, req)
	if err := saveDashboardSettingsOverlay(overlayPath, overlay); err != nil {
		*a.config = oldCfg
		return admin.DashboardSettingsUpdateResponse{}, err
	}

	if registry := a.modelRegistry(); registry != nil {
		registry.SetConfiguredProviderModelsMode(a.config.Models.ConfiguredProviderModelsMode)
	}
	if a.server != nil {
		a.server.SetKeepOnlyAliasesAtModelsEndpoint(a.config.Models.KeepOnlyAliasesAtModelsEndpoint)
		a.server.SetEnabledPassthroughProviders(a.config.Server.EnabledPassthroughProviders)
		a.server.SetAllowPassthroughV1Alias(a.config.Server.AllowPassthroughV1Alias)
		a.server.SetTokenSaver(a.config.TokenSaver)
		a.server.SetPromptCacheConfig(a.config.Cache.Prompt)
	}

	restartReasons := collectDashboardRestartReasons(&oldCfg, a.config)
	return admin.DashboardSettingsUpdateResponse{
		Message:          "dashboard settings saved",
		RefreshSuggested: true,
		RequiresRestart:  len(restartReasons) > 0,
		RestartReasons:   restartReasons,
		DashboardConfig:  dashboardRuntimeConfig(a.config, a.config.Usage.Enabled),
	}, nil
}

func validateDashboardSettingsUpdate(cfg *config.Config, req admin.DashboardSettingsUpdateRequest) error {
	if err := config.ValidateBodySizeLimit(strings.TrimSpace(req.Client.BodySizeLimit)); err != nil {
		return fmt.Errorf("body size limit: %w", err)
	}
	mode := config.NormalizeConfiguredProviderModelsMode(config.ConfiguredProviderModelsMode(req.Client.ConfiguredProviderModelsMode))
	if mode == "" || !mode.Valid() {
		return fmt.Errorf("configured provider models mode must be one of: fallback, allowlist")
	}
	if req.Caching.ModelRefreshIntervalSeconds <= 0 {
		return fmt.Errorf("model refresh interval must be greater than 0 seconds")
	}
	if req.Logging.RetentionDays < 0 {
		return fmt.Errorf("logging retention days must be >= 0")
	}
	if req.Pricing.UsageRetentionDays < 0 {
		return fmt.Errorf("usage retention days must be >= 0")
	}
	if req.Performance.HTTPTimeoutSeconds <= 0 {
		return fmt.Errorf("HTTP timeout must be greater than 0 seconds")
	}
	if req.Performance.HTTPResponseHeaderTimeoutSeconds <= 0 {
		return fmt.Errorf("HTTP response header timeout must be greater than 0 seconds")
	}
	if req.Performance.RetryMaxRetries < 0 {
		return fmt.Errorf("retry max retries must be >= 0")
	}
	if req.Performance.RetryInitialBackoffMilliseconds <= 0 {
		return fmt.Errorf("retry initial backoff must be > 0")
	}
	if req.Performance.RetryMaxBackoffMilliseconds <= 0 {
		return fmt.Errorf("retry max backoff must be > 0")
	}
	if req.Performance.CircuitBreakerFailureThreshold <= 0 {
		return fmt.Errorf("circuit breaker failure threshold must be > 0")
	}
	if req.Performance.CircuitBreakerSuccessThreshold <= 0 {
		return fmt.Errorf("circuit breaker success threshold must be > 0")
	}
	if req.Performance.CircuitBreakerTimeoutMilliseconds <= 0 {
		return fmt.Errorf("circuit breaker timeout must be > 0")
	}
	if req.Performance.WorkflowRefreshIntervalSeconds <= 0 {
		return fmt.Errorf("workflow refresh interval must be greater than 0 seconds")
	}
	if err := validateDashboardProxyURL("HTTP proxy", req.Proxy.HTTPProxy); err != nil {
		return err
	}
	if err := validateDashboardProxyURL("HTTPS proxy", req.Proxy.HTTPSProxy); err != nil {
		return err
	}
	if endpoint := strings.TrimSpace(req.Observability.MetricsEndpoint); endpoint != "" {
		normalized := path.Clean(endpoint)
		if !strings.HasPrefix(normalized, "/") {
			return fmt.Errorf("metrics endpoint must start with /")
		}
	}
	tokenSaver := config.TokenSaverConfig{
		Enabled:        req.TokenSaver.Enabled,
		Endpoints:      req.TokenSaver.Endpoints,
		ApplyStreaming: req.TokenSaver.ApplyStreaming,
		Output: config.TokenSaverOutputConfig{
			Enabled: req.TokenSaver.OutputEnabled,
			Profile: req.TokenSaver.OutputProfile,
			Level:   req.TokenSaver.OutputLevel,
		},
		Models: config.TokenSaverModelScopeConfig{
			Include: req.TokenSaver.ModelInclude,
			Exclude: req.TokenSaver.ModelExclude,
		},
		Providers: config.TokenSaverProviderScopeConfig{
			Include: req.TokenSaver.ProviderInclude,
			Exclude: req.TokenSaver.ProviderExclude,
		},
		OnError:     req.TokenSaver.OnError,
		EmitHeaders: req.TokenSaver.EmitHeaders,
	}
	if err := config.ValidateTokenSaverConfig(&tokenSaver); err != nil {
		return err
	}
	return nil
}

func applyDashboardSettingsToConfig(cfg *config.Config, req admin.DashboardSettingsUpdateRequest) {
	if cfg == nil {
		return
	}
	cfg.Server.Port = strings.TrimSpace(req.Client.Port)
	cfg.Server.BasePath = config.NormalizeBasePath(req.Client.BasePath)
	cfg.Server.BodySizeLimit = strings.TrimSpace(req.Client.BodySizeLimit)
	cfg.Server.SwaggerEnabled = req.Client.SwaggerEnabled
	cfg.Server.PprofEnabled = req.Client.PprofEnabled
	cfg.Server.AllowPassthroughV1Alias = req.Client.AllowPassthroughV1Alias
	cfg.Server.EnabledPassthroughProviders = normalizeDashboardStringSlice(req.Client.EnabledPassthroughProviders)
	if req.Client.AdminEndpointsEnabled != nil {
		cfg.Admin.EndpointsEnabled = *req.Client.AdminEndpointsEnabled
	}
	if req.Client.AdminUIEnabled != nil {
		cfg.Admin.UIEnabled = *req.Client.AdminUIEnabled
	}
	if req.Client.EnableAnthropicIngress != nil {
		cfg.Server.EnableAnthropicIngress = *req.Client.EnableAnthropicIngress
	}
	cfg.Models.ConfiguredProviderModelsMode = config.ResolveConfiguredProviderModelsMode(config.ConfiguredProviderModelsMode(req.Client.ConfiguredProviderModelsMode))
	cfg.Models.KeepOnlyAliasesAtModelsEndpoint = req.Client.KeepOnlyAliasesAtModelsEndpoint
	cfg.Cache.Model.RefreshInterval = req.Caching.ModelRefreshIntervalSeconds
	cfg.Cache.Model.ModelList.URL = strings.TrimSpace(req.Caching.ModelListURL)
	if req.Caching.ExactCacheEnabled != nil {
		if cfg.Cache.Response.Simple == nil {
			cfg.Cache.Response.Simple = &config.SimpleCacheConfig{}
		}
		cfg.Cache.Response.Simple.Enabled = req.Caching.ExactCacheEnabled
	}
	if req.Caching.ExactCacheTTLSeconds != nil {
		if cfg.Cache.Response.Simple == nil {
			cfg.Cache.Response.Simple = &config.SimpleCacheConfig{}
		}
		if cfg.Cache.Response.Simple.Redis == nil {
			cfg.Cache.Response.Simple.Redis = &config.RedisResponseConfig{}
		}
		cfg.Cache.Response.Simple.Redis.TTL = *req.Caching.ExactCacheTTLSeconds
	}
	if key := strings.TrimSpace(req.Caching.ExactCacheRedisKey); key != "" {
		if cfg.Cache.Response.Simple == nil {
			cfg.Cache.Response.Simple = &config.SimpleCacheConfig{}
		}
		if cfg.Cache.Response.Simple.Redis == nil {
			cfg.Cache.Response.Simple.Redis = &config.RedisResponseConfig{}
		}
		cfg.Cache.Response.Simple.Redis.Key = key
	}
	if req.Caching.SemanticCacheEnabled != nil {
		if cfg.Cache.Response.Semantic == nil {
			cfg.Cache.Response.Semantic = &config.SemanticCacheConfig{}
		}
		cfg.Cache.Response.Semantic.Enabled = req.Caching.SemanticCacheEnabled
	}
	if req.Caching.SemanticSimilarityThreshold != nil {
		if cfg.Cache.Response.Semantic == nil {
			cfg.Cache.Response.Semantic = &config.SemanticCacheConfig{}
		}
		cfg.Cache.Response.Semantic.SimilarityThreshold = *req.Caching.SemanticSimilarityThreshold
	}
	if req.Caching.SemanticPromptSimilarityMin != nil {
		if cfg.Cache.Response.Semantic == nil {
			cfg.Cache.Response.Semantic = &config.SemanticCacheConfig{}
		}
		cfg.Cache.Response.Semantic.PromptSimilarityThreshold = *req.Caching.SemanticPromptSimilarityMin
	}
	if req.Caching.SemanticTTLSeconds != nil {
		if cfg.Cache.Response.Semantic == nil {
			cfg.Cache.Response.Semantic = &config.SemanticCacheConfig{}
		}
		cfg.Cache.Response.Semantic.TTL = req.Caching.SemanticTTLSeconds
	}
	if req.Caching.SemanticMaxConversationMessages != nil {
		if cfg.Cache.Response.Semantic == nil {
			cfg.Cache.Response.Semantic = &config.SemanticCacheConfig{}
		}
		cfg.Cache.Response.Semantic.MaxConversationMessages = req.Caching.SemanticMaxConversationMessages
	}
	if req.Caching.SemanticExcludeSystemPrompt != nil {
		if cfg.Cache.Response.Semantic == nil {
			cfg.Cache.Response.Semantic = &config.SemanticCacheConfig{}
		}
		cfg.Cache.Response.Semantic.ExcludeSystemPrompt = *req.Caching.SemanticExcludeSystemPrompt
	}
	if provider := strings.TrimSpace(req.Caching.SemanticEmbedderProvider); provider != "" {
		if cfg.Cache.Response.Semantic == nil {
			cfg.Cache.Response.Semantic = &config.SemanticCacheConfig{}
		}
		cfg.Cache.Response.Semantic.Embedder.Provider = provider
	}
	if model := strings.TrimSpace(req.Caching.SemanticEmbedderModel); model != "" {
		if cfg.Cache.Response.Semantic == nil {
			cfg.Cache.Response.Semantic = &config.SemanticCacheConfig{}
		}
		cfg.Cache.Response.Semantic.Embedder.Model = model
	}
	if vsType := strings.TrimSpace(req.Caching.SemanticVectorStoreType); vsType != "" {
		if cfg.Cache.Response.Semantic == nil {
			cfg.Cache.Response.Semantic = &config.SemanticCacheConfig{}
		}
		cfg.Cache.Response.Semantic.VectorStore.Type = vsType
	}
	cfg.Logging.Enabled = req.Logging.Enabled
	cfg.Logging.LogBodies = req.Logging.LogBodies
	cfg.Logging.LogHeaders = req.Logging.LogHeaders
	cfg.Logging.BufferSize = req.Logging.BufferSize
	cfg.Logging.FlushInterval = req.Logging.FlushIntervalSeconds
	cfg.Logging.RetentionDays = req.Logging.RetentionDays
	cfg.Logging.OnlyModelInteractions = req.Logging.OnlyModelInteractions
	cfg.Metrics.Enabled = req.Observability.MetricsEnabled
	cfg.Metrics.Endpoint = normalizeMetricsEndpoint(req.Observability.MetricsEndpoint)

	cfg.HTTP.Timeout = req.Performance.HTTPTimeoutSeconds
	cfg.HTTP.ResponseHeaderTimeout = req.Performance.HTTPResponseHeaderTimeoutSeconds
	cfg.HTTP.Proxy.HTTPProxy = strings.TrimSpace(req.Proxy.HTTPProxy)
	cfg.HTTP.Proxy.HTTPSProxy = strings.TrimSpace(req.Proxy.HTTPSProxy)
	cfg.HTTP.Proxy.NoProxy = strings.TrimSpace(req.Proxy.NoProxy)
	cfg.HTTP.Proxy.ProxyAuthEnabled = req.Proxy.ProxyAuthEnabled
	cfg.HTTP.Proxy.CACertPEM = strings.TrimSpace(req.Proxy.CACertPEM)
	cfg.Resilience.Retry.MaxRetries = req.Performance.RetryMaxRetries
	cfg.Resilience.Retry.InitialBackoff = time.Duration(req.Performance.RetryInitialBackoffMilliseconds) * time.Millisecond
	cfg.Resilience.Retry.MaxBackoff = time.Duration(req.Performance.RetryMaxBackoffMilliseconds) * time.Millisecond
	cfg.Resilience.Retry.BackoffFactor = req.Performance.RetryBackoffFactor
	cfg.Resilience.Retry.JitterFactor = req.Performance.RetryJitterFactor
	cfg.Resilience.CircuitBreaker.FailureThreshold = req.Performance.CircuitBreakerFailureThreshold
	cfg.Resilience.CircuitBreaker.SuccessThreshold = req.Performance.CircuitBreakerSuccessThreshold
	cfg.Resilience.CircuitBreaker.Timeout = time.Duration(req.Performance.CircuitBreakerTimeoutMilliseconds) * time.Millisecond
	cfg.Guardrails.Enabled = req.Security.GuardrailsEnabled
	cfg.Guardrails.EnableForBatchProcessing = req.Security.BatchGuardrails
	cfg.Workflows.RefreshInterval = time.Duration(req.Performance.WorkflowRefreshIntervalSeconds) * time.Second
	cfg.Usage.EnforceReturningUsageData = req.Pricing.EnforceReturningUsageData
	cfg.Usage.PricingRecalculationEnabled = req.Pricing.PricingRecalculationEnabled
	cfg.Usage.RetentionDays = req.Pricing.UsageRetentionDays
	cfg.Cache.Prompt.Mode = req.Caching.PromptCache.Mode
	cfg.Cache.Prompt.SystemPromptCache = req.Caching.PromptCache.SystemPromptCache
	cfg.Cache.Prompt.FirstMessageCache = req.Caching.PromptCache.FirstMessageCache
	cfg.Cache.Prompt.ToolsCache = req.Caching.PromptCache.ToolsCache
	cfg.Cache.Prompt.MinTokensBeforeCache = req.Caching.PromptCache.MinTokens
	applyDashboardTokenSaverToConfig(cfg, req.TokenSaver)
}

func applyDashboardTokenSaverToConfig(cfg *config.Config, values admin.DashboardSettingsUpdateTokenSaver) {
	cfg.TokenSaver.Enabled = values.Enabled
	cfg.TokenSaver.Endpoints = normalizeDashboardStringSlice(values.Endpoints)
	cfg.TokenSaver.ApplyStreaming = values.ApplyStreaming
	cfg.TokenSaver.Output.Enabled = values.OutputEnabled
	cfg.TokenSaver.Output.Profile = strings.TrimSpace(values.OutputProfile)
	cfg.TokenSaver.Output.Level = strings.TrimSpace(values.OutputLevel)
	cfg.TokenSaver.EmitHeaders = values.EmitHeaders
	cfg.TokenSaver.OnError = strings.TrimSpace(values.OnError)
	cfg.TokenSaver.Models.Include = normalizeDashboardStringSlice(values.ModelInclude)
	cfg.TokenSaver.Models.Exclude = normalizeDashboardStringSlice(values.ModelExclude)
	cfg.TokenSaver.Providers.Include = normalizeDashboardStringSlice(values.ProviderInclude)
	cfg.TokenSaver.Providers.Exclude = normalizeDashboardStringSlice(values.ProviderExclude)
	cfg.TokenSaver.Audit.Enabled = values.AuditEnabled
	_ = config.ValidateTokenSaverConfig(&cfg.TokenSaver)
}

func applyDashboardSettingsToOverlay(overlay *dashboardSettingsOverlay, req admin.DashboardSettingsUpdateRequest) {
	if overlay.Server == nil {
		overlay.Server = &dashboardServerOverlay{}
	}
	if overlay.Models == nil {
		overlay.Models = &dashboardModelsOverlay{}
	}
	if overlay.Cache == nil {
		overlay.Cache = &dashboardCacheOverlay{}
	}
	if overlay.Cache.Model == nil {
		overlay.Cache.Model = &dashboardCacheModelOverlay{}
	}
	if overlay.Cache.Model.ModelList == nil {
		overlay.Cache.Model.ModelList = &dashboardModelListOverlay{}
	}
	if overlay.Logging == nil {
		overlay.Logging = &dashboardLoggingOverlay{}
	}
	if overlay.Usage == nil {
		overlay.Usage = &dashboardUsageOverlay{}
	}
	if overlay.Metrics == nil {
		overlay.Metrics = &dashboardMetricsOverlay{}
	}
	if overlay.HTTP == nil {
		overlay.HTTP = &dashboardHTTPOverlay{}
	}
	if overlay.Proxy == nil {
		overlay.Proxy = &dashboardProxyOverlay{}
	}
	if overlay.Resilience == nil {
		overlay.Resilience = &dashboardResilienceOverlay{}
	}
	if overlay.Resilience.Retry == nil {
		overlay.Resilience.Retry = &dashboardRetryOverlay{}
	}
	if overlay.Resilience.CircuitBreaker == nil {
		overlay.Resilience.CircuitBreaker = &dashboardCircuitBreakerOverlay{}
	}
	if overlay.Guardrails == nil {
		overlay.Guardrails = &dashboardGuardrailsOverlay{}
	}
	if overlay.Workflows == nil {
		overlay.Workflows = &dashboardWorkflowsOverlay{}
	}
	if overlay.TokenSaver == nil {
		overlay.TokenSaver = &tokenSaverOverlay{}
	}
	if overlay.TokenSaver.Output == nil {
		overlay.TokenSaver.Output = &tokenSaverOutputOverlay{}
	}
	if overlay.TokenSaver.Models == nil {
		overlay.TokenSaver.Models = &tokenSaverScopeOverlay{}
	}
	if overlay.TokenSaver.Providers == nil {
		overlay.TokenSaver.Providers = &tokenSaverScopeOverlay{}
	}

	overlay.Server.Port = stringPtr(strings.TrimSpace(req.Client.Port))
	overlay.Server.BasePath = stringPtr(config.NormalizeBasePath(req.Client.BasePath))
	overlay.Server.BodySizeLimit = stringPtr(strings.TrimSpace(req.Client.BodySizeLimit))
	overlay.Server.SwaggerEnabled = boolPtr(req.Client.SwaggerEnabled)
	overlay.Server.PprofEnabled = boolPtr(req.Client.PprofEnabled)
	overlay.Server.AllowPassthroughV1Alias = boolPtr(req.Client.AllowPassthroughV1Alias)
	overlay.Server.EnabledPassthroughProviders = normalizeDashboardStringSlice(req.Client.EnabledPassthroughProviders)
	if req.Client.AdminEndpointsEnabled != nil {
		overlay.Server.AdminEndpointsEnabled = req.Client.AdminEndpointsEnabled
	}
	if req.Client.AdminUIEnabled != nil {
		overlay.Server.AdminUIEnabled = req.Client.AdminUIEnabled
	}
	if req.Client.EnableAnthropicIngress != nil {
		overlay.Server.EnableAnthropicIngress = req.Client.EnableAnthropicIngress
	}
	overlay.Models.ConfiguredProviderModelsMode = stringPtr(strings.TrimSpace(req.Client.ConfiguredProviderModelsMode))
	overlay.Models.KeepOnlyAliasesAtModelsEndpoint = boolPtr(req.Client.KeepOnlyAliasesAtModelsEndpoint)
	overlay.Cache.Model.RefreshInterval = intPtr(req.Caching.ModelRefreshIntervalSeconds)
	overlay.Cache.Model.ModelList.URL = stringPtr(strings.TrimSpace(req.Caching.ModelListURL))
	if overlay.Cache.Response == nil {
		overlay.Cache.Response = &dashboardCacheResponseOverlay{}
	}
	if req.Caching.ExactCacheEnabled != nil {
		if overlay.Cache.Response.Simple == nil {
			overlay.Cache.Response.Simple = &dashboardCacheSimpleOverlay{}
		}
		overlay.Cache.Response.Simple.Enabled = req.Caching.ExactCacheEnabled
	}
	if req.Caching.ExactCacheTTLSeconds != nil {
		if overlay.Cache.Response.Simple == nil {
			overlay.Cache.Response.Simple = &dashboardCacheSimpleOverlay{}
		}
		if overlay.Cache.Response.Simple.Redis == nil {
			overlay.Cache.Response.Simple.Redis = &dashboardCacheSimpleRedisOverlay{}
		}
		overlay.Cache.Response.Simple.Redis.TTL = req.Caching.ExactCacheTTLSeconds
	}
	if key := strings.TrimSpace(req.Caching.ExactCacheRedisKey); key != "" {
		if overlay.Cache.Response.Simple == nil {
			overlay.Cache.Response.Simple = &dashboardCacheSimpleOverlay{}
		}
		if overlay.Cache.Response.Simple.Redis == nil {
			overlay.Cache.Response.Simple.Redis = &dashboardCacheSimpleRedisOverlay{}
		}
		overlay.Cache.Response.Simple.Redis.Key = stringPtr(key)
	}
	if req.Caching.SemanticCacheEnabled != nil {
		if overlay.Cache.Response.Semantic == nil {
			overlay.Cache.Response.Semantic = &dashboardCacheSemanticOverlay{}
		}
		overlay.Cache.Response.Semantic.Enabled = req.Caching.SemanticCacheEnabled
	}
	if req.Caching.SemanticSimilarityThreshold != nil {
		if overlay.Cache.Response.Semantic == nil {
			overlay.Cache.Response.Semantic = &dashboardCacheSemanticOverlay{}
		}
		overlay.Cache.Response.Semantic.SimilarityThreshold = req.Caching.SemanticSimilarityThreshold
	}
	if req.Caching.SemanticPromptSimilarityMin != nil {
		if overlay.Cache.Response.Semantic == nil {
			overlay.Cache.Response.Semantic = &dashboardCacheSemanticOverlay{}
		}
		overlay.Cache.Response.Semantic.PromptSimilarityThreshold = req.Caching.SemanticPromptSimilarityMin
	}
	if req.Caching.SemanticTTLSeconds != nil {
		if overlay.Cache.Response.Semantic == nil {
			overlay.Cache.Response.Semantic = &dashboardCacheSemanticOverlay{}
		}
		overlay.Cache.Response.Semantic.TTL = req.Caching.SemanticTTLSeconds
	}
	if req.Caching.SemanticMaxConversationMessages != nil {
		if overlay.Cache.Response.Semantic == nil {
			overlay.Cache.Response.Semantic = &dashboardCacheSemanticOverlay{}
		}
		overlay.Cache.Response.Semantic.MaxConversationMessages = req.Caching.SemanticMaxConversationMessages
	}
	if req.Caching.SemanticExcludeSystemPrompt != nil {
		if overlay.Cache.Response.Semantic == nil {
			overlay.Cache.Response.Semantic = &dashboardCacheSemanticOverlay{}
		}
		overlay.Cache.Response.Semantic.ExcludeSystemPrompt = req.Caching.SemanticExcludeSystemPrompt
	}
	if provider := strings.TrimSpace(req.Caching.SemanticEmbedderProvider); provider != "" {
		if overlay.Cache.Response.Semantic == nil {
			overlay.Cache.Response.Semantic = &dashboardCacheSemanticOverlay{}
		}
		overlay.Cache.Response.Semantic.EmbedderProvider = stringPtr(provider)
	}
	if model := strings.TrimSpace(req.Caching.SemanticEmbedderModel); model != "" {
		if overlay.Cache.Response.Semantic == nil {
			overlay.Cache.Response.Semantic = &dashboardCacheSemanticOverlay{}
		}
		overlay.Cache.Response.Semantic.EmbedderModel = stringPtr(model)
	}
	if vsType := strings.TrimSpace(req.Caching.SemanticVectorStoreType); vsType != "" {
		if overlay.Cache.Response.Semantic == nil {
			overlay.Cache.Response.Semantic = &dashboardCacheSemanticOverlay{}
		}
		overlay.Cache.Response.Semantic.VectorStoreType = stringPtr(vsType)
	}
	overlay.Logging.Enabled = boolPtr(req.Logging.Enabled)
	overlay.Logging.LogBodies = boolPtr(req.Logging.LogBodies)
	overlay.Logging.LogHeaders = boolPtr(req.Logging.LogHeaders)
	overlay.Logging.BufferSize = intPtr(req.Logging.BufferSize)
	overlay.Logging.FlushInterval = intPtr(req.Logging.FlushIntervalSeconds)
	overlay.Logging.RetentionDays = intPtr(req.Logging.RetentionDays)
	overlay.Logging.OnlyModelInteractions = boolPtr(req.Logging.OnlyModelInteractions)
	overlay.Metrics.Enabled = boolPtr(req.Observability.MetricsEnabled)
	overlay.Metrics.Endpoint = stringPtr(normalizeMetricsEndpoint(req.Observability.MetricsEndpoint))
	overlay.HTTP.Timeout = intPtr(req.Performance.HTTPTimeoutSeconds)
	overlay.HTTP.ResponseHeaderTimeout = intPtr(req.Performance.HTTPResponseHeaderTimeoutSeconds)
	overlay.Proxy.HTTPProxy = stringPtr(strings.TrimSpace(req.Proxy.HTTPProxy))
	overlay.Proxy.HTTPSProxy = stringPtr(strings.TrimSpace(req.Proxy.HTTPSProxy))
	overlay.Proxy.NoProxy = stringPtr(strings.TrimSpace(req.Proxy.NoProxy))
	overlay.Proxy.ProxyAuthEnabled = boolPtr(req.Proxy.ProxyAuthEnabled)
	overlay.Proxy.CACertPEM = stringPtr(strings.TrimSpace(req.Proxy.CACertPEM))

	initialBackoff := (time.Duration(req.Performance.RetryInitialBackoffMilliseconds) * time.Millisecond).String()
	maxBackoff := (time.Duration(req.Performance.RetryMaxBackoffMilliseconds) * time.Millisecond).String()
	cbTimeout := (time.Duration(req.Performance.CircuitBreakerTimeoutMilliseconds) * time.Millisecond).String()

	overlay.Resilience.Retry.MaxRetries = intPtr(req.Performance.RetryMaxRetries)
	overlay.Resilience.Retry.InitialBackoff = stringPtr(initialBackoff)
	overlay.Resilience.Retry.MaxBackoff = stringPtr(maxBackoff)
	overlay.Resilience.Retry.BackoffFactor = &req.Performance.RetryBackoffFactor
	overlay.Resilience.Retry.JitterFactor = &req.Performance.RetryJitterFactor
	overlay.Resilience.CircuitBreaker.FailureThreshold = intPtr(req.Performance.CircuitBreakerFailureThreshold)
	overlay.Resilience.CircuitBreaker.SuccessThreshold = intPtr(req.Performance.CircuitBreakerSuccessThreshold)
	overlay.Resilience.CircuitBreaker.Timeout = stringPtr(cbTimeout)

	overlay.Guardrails.Enabled = boolPtr(req.Security.GuardrailsEnabled)
	overlay.Guardrails.EnableForBatchProcessing = boolPtr(req.Security.BatchGuardrails)

	refreshInterval := (time.Duration(req.Performance.WorkflowRefreshIntervalSeconds) * time.Second).String()
	overlay.Workflows.RefreshInterval = stringPtr(refreshInterval)
	overlay.Usage.EnforceReturningUsageData = boolPtr(req.Pricing.EnforceReturningUsageData)
	overlay.Usage.PricingRecalculationEnabled = boolPtr(req.Pricing.PricingRecalculationEnabled)
	overlay.Usage.RetentionDays = intPtr(req.Pricing.UsageRetentionDays)

	if overlay.Cache.Prompt == nil {
		overlay.Cache.Prompt = &dashboardCachePromptOverlay{}
	}
	overlay.Cache.Prompt.Mode = stringPtr(req.Caching.PromptCache.Mode)
	overlay.Cache.Prompt.SystemPromptCache = boolPtr(req.Caching.PromptCache.SystemPromptCache)
	overlay.Cache.Prompt.FirstMessageCache = boolPtr(req.Caching.PromptCache.FirstMessageCache)
	overlay.Cache.Prompt.ToolsCache = boolPtr(req.Caching.PromptCache.ToolsCache)
	overlay.Cache.Prompt.MinTokens = intPtr(req.Caching.PromptCache.MinTokens)

	applyDashboardTokenSaverToOverlay(overlay.TokenSaver, req.TokenSaver)
}

func applyDashboardTokenSaverToOverlay(overlay *tokenSaverOverlay, values admin.DashboardSettingsUpdateTokenSaver) {
	overlay.Enabled = boolPtr(values.Enabled)
	overlay.Endpoints = normalizeDashboardStringSlice(values.Endpoints)
	overlay.ApplyStreaming = boolPtr(values.ApplyStreaming)
	overlay.Output.Enabled = boolPtr(values.OutputEnabled)
	overlay.Output.Profile = stringPtr(strings.TrimSpace(values.OutputProfile))
	overlay.Output.Level = stringPtr(strings.TrimSpace(values.OutputLevel))
	overlay.EmitHeaders = boolPtr(values.EmitHeaders)
	overlay.OnError = stringPtr(strings.TrimSpace(values.OnError))
	overlay.Models.Include = normalizeDashboardStringSlice(values.ModelInclude)
	overlay.Models.Exclude = normalizeDashboardStringSlice(values.ModelExclude)
	overlay.Providers.Include = normalizeDashboardStringSlice(values.ProviderInclude)
	overlay.Providers.Exclude = normalizeDashboardStringSlice(values.ProviderExclude)
	overlay.AuditEnabled = boolPtr(values.AuditEnabled)
}

func loadDashboardSettingsOverlay(path string) (dashboardSettingsOverlay, error) {
	var overlay dashboardSettingsOverlay
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return overlay, nil
		}
		return overlay, fmt.Errorf("read dashboard overrides: %w", err)
	}
	if err := yaml.Unmarshal(raw, &overlay); err != nil {
		return overlay, fmt.Errorf("parse dashboard overrides: %w", err)
	}
	return overlay, nil
}

func saveDashboardSettingsOverlay(path string, overlay dashboardSettingsOverlay) error {
	if err := os.MkdirAll(filepathDir(path), 0o755); err != nil {
		return fmt.Errorf("create dashboard override directory: %w", err)
	}
	raw, err := yaml.Marshal(overlay)
	if err != nil {
		return fmt.Errorf("marshal dashboard overrides: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write dashboard overrides: %w", err)
	}
	return nil
}

func collectDashboardRestartReasons(oldCfg, newCfg *config.Config) []string {
	if oldCfg == nil || newCfg == nil {
		return []string{"server config changed"}
	}
	reasons := make([]string, 0, 4)
	if oldCfg.Server.BodySizeLimit != newCfg.Server.BodySizeLimit {
		reasons = append(reasons, "HTTP body size limit changes require a server restart")
	}
	if oldCfg.Admin != newCfg.Admin {
		reasons = append(reasons, "admin API/UI toggle changes require a server restart")
	}
	if oldCfg.Cache.Response != newCfg.Cache.Response {
		reasons = append(reasons, "response cache config changes require a server restart")
	}
	if oldCfg.Logging != newCfg.Logging {
		reasons = append(reasons, "logging middleware changes require a server restart")
	}
	if oldCfg.Metrics.Enabled != newCfg.Metrics.Enabled || oldCfg.Metrics.Endpoint != newCfg.Metrics.Endpoint {
		reasons = append(reasons, "metrics endpoint changes require a server restart")
	}
	if oldCfg.HTTP != newCfg.HTTP || oldCfg.Workflows.RefreshInterval != newCfg.Workflows.RefreshInterval || oldCfg.Usage.RetentionDays != newCfg.Usage.RetentionDays {
		reasons = append(reasons, "HTTP timeout, outbound proxy, workflow refresh, or usage retention changes apply fully after restart")
	}
	return reasons
}

func validateDashboardProxyURL(label, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid proxy URL", label)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "socks5" {
		return fmt.Errorf("%s must use http, https, or socks5 scheme", label)
	}
	return nil
}

func normalizeDashboardStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func normalizeDashboardStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeMetricsEndpoint(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "/metrics"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return path.Clean(trimmed)
}

func filepathDir(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "."
	}
	if dir := strings.TrimSpace(filepath.Dir(path)); dir != "" {
		return dir
	}
	return "."
}

func boolPtr(value bool) *bool          { return &value }
func intPtr(value int) *int             { return &value }
func float64Ptr(value float64) *float64 { return &value }
func stringPtr(value string) *string    { return &value }

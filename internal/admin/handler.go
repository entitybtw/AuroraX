// Package admin provides the admin REST API and dashboard for aurora.
package admin

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v5"

	"aurora/internal/audit_logging"
	"aurora/internal/authentication_keys"
	"aurora/internal/command_line_tools"
	"aurora/internal/console"
	"aurora/internal/core"
	"aurora/internal/guardrails"
	"aurora/internal/model_aliases"
	"aurora/internal/model_combinations"
	"aurora/internal/model_overrides"
	"aurora/internal/providers"
	"aurora/internal/providers/pool"
	"aurora/internal/response_cache"
	"aurora/internal/usage"
	"aurora/internal/workflow"
)

// Handler serves admin API endpoints.
type Handler struct {
	usageReader          usage.UsageReader
	usageRecalculator    usage.PricingRecalculator
	auditReader          auditlog.Reader
	auditLogger          auditlog.LoggerInterface
	console              *console.Service
	registry             *providers.ModelRegistry
	pools                *pool.Registry
	authKeys             *authkeys.Service
	authKeyRateInspector AuthKeyRateLimitInspector
	aliases              *aliases.Service
	combos               *combos.Service
	cliTools             *clitools.Service
	modelOverrides       *modeloverrides.Service
	workflows            *workflow.Service
	guardrails           guardrails.Catalog
	guardrailDefs        *guardrails.Service
	responseCache        *responsecache.ResponseCacheMiddleware
	runtimeConfig        DashboardConfigResponse
	runtimeRefresher     RuntimeRefresher
	settingsManager      DashboardSettingsManager
	configuredProviders  []providers.SanitizedProviderConfig
	providerOverrides    *ProviderOverrideStore
	poolWeights          *PoolOverrideStore

	mutationMu sync.Mutex
	pricingMu  sync.Mutex

	masterKey string
}

// AuthKeyRateLimitInspector returns a live snapshot of an auth key's
// per-window rate-limit consumption. Implemented by the server's
// InMemoryAuthKeyRateLimiter and any future distributed variant.
type AuthKeyRateLimitInspector interface {
	Snapshot(tenantID string, authKeyID string, limits core.AuthKeyRateLimits, now time.Time) core.AuthKeyRateLimitSnapshot
}

// Option configures the admin API handler.
type Option func(*Handler)

const (
	DashboardConfigFeatureFallbackMode  = "FEATURE_FALLBACK_MODE"
	DashboardConfigLoggingEnabled       = "LOGGING_ENABLED"
	DashboardConfigUsageEnabled         = "USAGE_ENABLED"
	DashboardConfigBudgetsEnabled       = "BUDGETS_ENABLED"
	DashboardConfigGuardrailsEnabled    = "GUARDRAILS_ENABLED"
	DashboardConfigCacheEnabled         = "CACHE_ENABLED"
	DashboardConfigRedisURL             = "REDIS_URL"
	DashboardConfigSemanticCacheEnabled = "SEMANTIC_CACHE_ENABLED"
	DashboardConfigPricingRecalculation = "USAGE_PRICING_RECALCULATION_ENABLED"
	DashboardConfigIdentityEnabled      = "IDENTITY_ENABLED"
	DashboardConfigIdentityOIDCEnabled  = "IDENTITY_OIDC_ENABLED"
)

// statusClientClosedRequest is the de facto status used by proxies for client-aborted requests.
const statusClientClosedRequest = 499

// DashboardConfigResponse is the allowlisted runtime config contract exposed to the dashboard UI.
type DashboardConfigResponse struct {
	Edition               string                    `json:"EDITION,omitempty"`
	Capabilities          []string                  `json:"CAPABILITIES,omitempty"`
	CapabilityMap         map[string]bool           `json:"capabilities,omitempty"`
	FeatureFallbackMode   string                    `json:"FEATURE_FALLBACK_MODE,omitempty"`
	LoggingEnabled        string                    `json:"LOGGING_ENABLED,omitempty"`
	UsageEnabled          string                    `json:"USAGE_ENABLED,omitempty"`
	BudgetsEnabled        string                    `json:"BUDGETS_ENABLED,omitempty"`
	GuardrailsEnabled     string                    `json:"GUARDRAILS_ENABLED,omitempty"`
	CacheEnabled          string                    `json:"CACHE_ENABLED,omitempty"`
	RedisURL              string                    `json:"REDIS_URL,omitempty"`
	SemanticCacheEnabled  string                    `json:"SEMANTIC_CACHE_ENABLED,omitempty"`
	PricingRecalculation  string                    `json:"USAGE_PRICING_RECALCULATION_ENABLED,omitempty"`
	Fallback              FallbackConfigSnapshot    `json:"fallback"`
	Settings              DashboardSettingsSnapshot `json:"settings,omitempty"`
	RuntimeFeatures       []RuntimeFeatureSnapshot  `json:"runtime_features,omitempty"`
}

type DashboardSettingsSnapshot struct {
	Client               DashboardClientSettingsSnapshot               `json:"client"`
	Caching              DashboardCachingSettingsSnapshot              `json:"caching"`
	Logging              DashboardLoggingSettingsSnapshot              `json:"logging"`
	Observability        DashboardObservabilitySettingsSnapshot        `json:"observability"`
	Storage              DashboardStorageSettingsSnapshot              `json:"storage"`
	Performance          DashboardPerformanceSettingsSnapshot          `json:"performance"`
	Security             DashboardSecuritySettingsSnapshot             `json:"security"`
	Pricing              DashboardPricingSettingsSnapshot              `json:"pricing"`
	TokenSaver           DashboardTokenSaverSettingsSnapshot           `json:"token_saver"`
	Proxy                DashboardProxySettingsSnapshot                `json:"proxy"`
}

type DashboardStorageSettingsSnapshot struct {
	Type               string `json:"type,omitempty"`
	SQLitePath         string `json:"sqlite_path,omitempty"`
	PostgreSQLURL      string `json:"postgresql_url,omitempty"`
	PostgreSQLMaxConns int    `json:"postgresql_max_conns"`
	MongoDBURL         string `json:"mongodb_url,omitempty"`
	MongoDBDatabase    string `json:"mongodb_database,omitempty"`
}

type DashboardClientSettingsSnapshot struct {
	Port                            string   `json:"port,omitempty"`
	BasePath                        string   `json:"base_path,omitempty"`
	BodySizeLimit                   string   `json:"body_size_limit,omitempty"`
	SwaggerEnabled                  bool     `json:"swagger_enabled"`
	PprofEnabled                    bool     `json:"pprof_enabled"`
	AdminEndpointsEnabled           bool     `json:"admin_endpoints_enabled"`
	AdminUIEnabled                  bool     `json:"admin_ui_enabled"`

	EnableAnthropicIngress          bool     `json:"enable_anthropic_ingress"`
	EnablePassthroughRoutes         bool     `json:"enable_passthrough_routes"`
	AllowPassthroughV1Alias         bool     `json:"allow_passthrough_v1_alias"`
	EnabledPassthroughProviders     []string `json:"enabled_passthrough_providers,omitempty"`
	ModelsEnabledByDefault          bool     `json:"models_enabled_by_default"`
	ModelOverridesEnabled           bool     `json:"model_overrides_enabled"`
	KeepOnlyAliasesAtModelsEndpoint bool     `json:"keep_only_aliases_at_models_endpoint"`
	ConfiguredProviderModelsMode    string   `json:"configured_provider_models_mode,omitempty"`
}

type DashboardCachingSettingsSnapshot struct {
	ModelCacheBackend               string   `json:"model_cache_backend,omitempty"`
	ModelCacheLocalDir              string   `json:"model_cache_local_dir,omitempty"`
	ModelCacheRedisURL              string   `json:"model_cache_redis_url,omitempty"`
	ModelCacheRedisKey              string   `json:"model_cache_redis_key,omitempty"`
	ModelCacheRedisTTLSeconds       int      `json:"model_cache_redis_ttl_seconds"`
	ModelRefreshIntervalSeconds     int      `json:"model_refresh_interval_seconds"`
	ModelListURL                    string   `json:"model_list_url,omitempty"`
	ModelListLocalPath              string   `json:"model_list_local_path,omitempty"`
	ModelListUserOverridesPath      string   `json:"model_list_user_overrides_path,omitempty"`
	ExactCacheEnabled               bool     `json:"exact_cache_enabled"`
	ExactCacheRedisURL              string   `json:"exact_cache_redis_url,omitempty"`
	ExactCacheRedisKey              string   `json:"exact_cache_redis_key,omitempty"`
	ExactCacheTTLSeconds            int      `json:"exact_cache_ttl_seconds"`
	SemanticCacheEnabled            bool     `json:"semantic_cache_enabled"`
	SemanticSimilarityThreshold     float64  `json:"semantic_similarity_threshold"`
	SemanticPromptSimilarityMin     float64  `json:"semantic_prompt_similarity_min"`
	SemanticTTLSeconds              int      `json:"semantic_ttl_seconds"`
	SemanticMaxConversationMessages int      `json:"semantic_max_conversation_messages"`
	SemanticExcludeSystemPrompt     bool     `json:"semantic_exclude_system_prompt"`
	SemanticEmbedderProvider        string   `json:"semantic_embedder_provider,omitempty"`
	SemanticEmbedderModel           string   `json:"semantic_embedder_model,omitempty"`
	SemanticVectorStoreType         string   `json:"semantic_vector_store_type,omitempty"`
	SemanticVectorStoreHints        []string `json:"semantic_vector_store_hints,omitempty"`
	SemanticVectorStoreURL          string   `json:"semantic_vector_store_url,omitempty"`
	SemanticVectorStoreCollection   string   `json:"semantic_vector_store_collection,omitempty"`
	SemanticVectorStoreTable        string   `json:"semantic_vector_store_table,omitempty"`
	SemanticVectorStoreNamespace    string   `json:"semantic_vector_store_namespace,omitempty"`
	SemanticVectorStoreClass        string   `json:"semantic_vector_store_class,omitempty"`
	SemanticVectorStoreDimension    int      `json:"semantic_vector_store_dimension"`
	SemanticVectorStoreAPIKeySet    bool     `json:"semantic_vector_store_api_key_set"`
	PromptCacheMode                 string   `json:"prompt_cache_mode"`
	PromptCacheSystemPrompt         bool     `json:"prompt_cache_system_prompt"`
	PromptCacheFirstMessage         bool     `json:"prompt_cache_first_message"`
	PromptCacheTools                bool     `json:"prompt_cache_tools"`
	PromptCacheMinTokens            int      `json:"prompt_cache_min_tokens"`
}

type DashboardLoggingSettingsSnapshot struct {
	Enabled               bool `json:"enabled"`
	LogBodies             bool `json:"log_bodies"`
	LogHeaders            bool `json:"log_headers"`
	BufferSize            int  `json:"buffer_size"`
	FlushIntervalSeconds  int  `json:"flush_interval_seconds"`
	RetentionDays         int  `json:"retention_days"`
	OnlyModelInteractions bool `json:"only_model_interactions"`
}

type DashboardObservabilitySettingsSnapshot struct {
	MetricsEnabled  bool   `json:"metrics_enabled"`
	MetricsEndpoint string `json:"metrics_endpoint,omitempty"`
	StorageType     string `json:"storage_type,omitempty"`
}

type DashboardPerformanceSettingsSnapshot struct {
	HTTPTimeoutSeconds                int     `json:"http_timeout_seconds"`
	HTTPResponseHeaderTimeoutSeconds  int     `json:"http_response_header_timeout_seconds"`
	WorkflowRefreshIntervalSeconds    int64   `json:"workflow_refresh_interval_seconds"`
	RetryMaxRetries                   int     `json:"retry_max_retries"`
	RetryInitialBackoffMilliseconds   int64   `json:"retry_initial_backoff_milliseconds"`
	RetryMaxBackoffMilliseconds       int64   `json:"retry_max_backoff_milliseconds"`
	RetryBackoffFactor                float64 `json:"retry_backoff_factor"`
	RetryJitterFactor                 float64 `json:"retry_jitter_factor"`
	CircuitBreakerFailureThreshold    int     `json:"circuit_breaker_failure_threshold"`
	CircuitBreakerSuccessThreshold    int     `json:"circuit_breaker_success_threshold"`
	CircuitBreakerTimeoutMilliseconds int64   `json:"circuit_breaker_timeout_milliseconds"`
}

type DashboardProxySettingsSnapshot struct {
	HTTPProxy        string `json:"http_proxy"`
	HTTPSProxy       string `json:"https_proxy"`
	NoProxy          string `json:"no_proxy"`
	ProxyAuthEnabled bool   `json:"proxy_auth_enabled"`
	CACertPEM        string `json:"ca_cert_pem,omitempty"`
}

type DashboardSecuritySettingsSnapshot struct {
	MasterKeyConfigured bool `json:"master_key_configured"`
	GuardrailsEnabled   bool `json:"guardrails_enabled"`
	BatchGuardrails     bool `json:"batch_guardrails"`
}

type DashboardPricingSettingsSnapshot struct {
	UsageEnabled                  bool `json:"usage_enabled"`
	EnforceReturningUsageData     bool `json:"enforce_returning_usage_data"`
	PricingRecalculationEnabled   bool `json:"pricing_recalculation_enabled"`
	UsageBufferSize               int  `json:"usage_buffer_size"`
	UsageFlushIntervalSeconds     int  `json:"usage_flush_interval_seconds"`
	UsageRetentionDays            int  `json:"usage_retention_days"`
	BudgetsEnabled                bool `json:"budgets_enabled"`
	ConfiguredBudgetUserPathCount int  `json:"configured_budget_user_path_count"`
}

type DashboardTokenSaverSettingsSnapshot struct {
	Enabled         bool     `json:"enabled"`
	ApplyStreaming  bool     `json:"apply_streaming"`
	Endpoints       []string `json:"endpoints,omitempty"`
	OutputEnabled   bool     `json:"output_enabled"`
	OutputProfile   string   `json:"output_profile,omitempty"`
	OutputLevel     string   `json:"output_level,omitempty"`
	EmitHeaders     bool     `json:"emit_headers"`
	OnError         string   `json:"on_error,omitempty"`
	ModelInclude    []string `json:"model_include,omitempty"`
	ModelExclude    []string `json:"model_exclude,omitempty"`
	ProviderInclude []string `json:"provider_include,omitempty"`
	ProviderExclude []string `json:"provider_exclude,omitempty"`
	AuditEnabled    bool     `json:"audit_enabled"`
}

type RuntimeFeatureSnapshot struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	Configured  bool   `json:"configured"`
	Description string `json:"description"`
	Usage       string `json:"usage"`
	Dependency  string `json:"dependency,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
}

type FeatureStatusResponse struct {
	Edition      string                  `json:"edition"`
	Capabilities map[string]bool         `json:"capabilities"`
	Features     []FeatureStatusSnapshot `json:"features"`
	ServerTime   time.Time               `json:"server_time"`
}

type FeatureStatusSnapshot struct {
	Key               string   `json:"key"`
	Label             string   `json:"label"`
	Category          string   `json:"category"`
	Configured        bool     `json:"configured"`
	Available         bool     `json:"available"`
	Effective         bool     `json:"effective"`
	Status            string   `json:"status"`
	Capability        string   `json:"capability,omitempty"`
	Dependencies      []string `json:"dependencies,omitempty"`
	Endpoint          string   `json:"endpoint,omitempty"`
	OffBehavior       string   `json:"off_behavior"`
	Conflict          string   `json:"conflict,omitempty"`
	Recommendation    string   `json:"recommendation,omitempty"`
	RuntimeStatus     string   `json:"runtime_status,omitempty"`
	RuntimeDependency string   `json:"runtime_dependency,omitempty"`
}

type FallbackConfigSnapshot struct {
	Mode                  string                     `json:"mode"`
	ManualRulesConfigured bool                       `json:"manual_rules_configured"`
	ManualRuleCount       int                        `json:"manual_rule_count"`
	ManualRules           []FallbackRuleSnapshot     `json:"manual_rules,omitempty"`
	Overrides             []FallbackOverrideSnapshot `json:"overrides,omitempty"`
}

type FallbackRuleSnapshot struct {
	Source  string   `json:"source"`
	Targets []string `json:"targets"`
}

type FallbackOverrideSnapshot struct {
	Model string `json:"model"`
	Mode  string `json:"mode"`
}

type providerStatusSummaryResponse struct {
	Total         int    `json:"total"`
	Healthy       int    `json:"healthy"`
	Degraded      int    `json:"degraded"`
	Unhealthy     int    `json:"unhealthy"`
	OverallStatus string `json:"overall_status"`
}

type providerStatusItemResponse struct {
	Name         string                            `json:"name"`
	Type         string                            `json:"type"`
	Status       string                            `json:"status"`
	StatusLabel  string                            `json:"status_label"`
	StatusReason string                            `json:"status_reason"`
	LastError    string                            `json:"last_error,omitempty"`
	Config       providers.SanitizedProviderConfig `json:"config"`
	Runtime      providers.ProviderRuntimeSnapshot `json:"runtime"`
	ConfigSource string                            `json:"config_source,omitempty"`
}

type providerStatusResponse struct {
	Summary   providerStatusSummaryResponse `json:"summary"`
	Providers []providerStatusItemResponse  `json:"providers"`
}

type auditLogEntryResponse struct {
	auditlog.LogEntry
	Usage *usage.RequestUsageSummary `json:"usage,omitempty"`
}

type auditLogListResponse struct {
	Entries []auditLogEntryResponse `json:"entries"`
	Total   int                     `json:"total"`
	Limit   int                     `json:"limit"`
	Offset  int                     `json:"offset"`
	Sort    string                  `json:"sort,omitempty"`
	Stats   *auditlog.LogStats      `json:"stats,omitempty"`
}

type auditConversationResponse struct {
	AnchorID string                  `json:"anchor_id"`
	Entries  []auditLogEntryResponse `json:"entries"`
}

const (
	RuntimeRefreshStatusOK      = "ok"
	RuntimeRefreshStatusPartial = "partial"
	RuntimeRefreshStatusFailed  = "failed"
	RuntimeRefreshStatusSkipped = "skipped"
)

// RuntimeRefreshStep describes the result of one manual runtime refresh step.
type RuntimeRefreshStep struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

// RuntimeRefreshReport is returned by the manual runtime refresh endpoint.
type RuntimeRefreshReport struct {
	Status        string               `json:"status"`
	StartedAt     time.Time            `json:"started_at"`
	FinishedAt    time.Time            `json:"finished_at"`
	DurationMS    int64                `json:"duration_ms"`
	ModelCount    int                  `json:"model_count"`
	ProviderCount int                  `json:"provider_count"`
	Steps         []RuntimeRefreshStep `json:"steps"`
}

// RuntimeRefresher refreshes application runtime snapshots on demand.
type RuntimeRefresher interface {
	RefreshRuntime(ctx context.Context) (RuntimeRefreshReport, error)
}

// DashboardSettingsManager updates the admin-managed editable config subset.
type DashboardSettingsManager interface {
	UpdateDashboardSettings(ctx context.Context, req DashboardSettingsUpdateRequest) (DashboardSettingsUpdateResponse, error)
}

// WithAuditReader enables audit log read endpoints.
func WithAuditReader(reader auditlog.Reader) Option {
	return func(h *Handler) {
		h.auditReader = reader
	}
}

// WithAuditLogger enables live audit log streaming.
func WithAuditLogger(logger auditlog.LoggerInterface) Option {
	return func(h *Handler) {
		h.auditLogger = logger
	}
}

// WithConsole enables sanitized live console endpoints.
func WithConsole(service *console.Service) Option {
	return func(h *Handler) {
		h.console = service
	}
}

// WithUsagePricingRecalculator enables persisted usage pricing recalculation.
func WithUsagePricingRecalculator(recalculator usage.PricingRecalculator) Option {
	return func(h *Handler) {
		h.usageRecalculator = recalculator
	}
}

// WithAliases enables alias administration endpoints.
func WithAliases(service *aliases.Service) Option {
	return func(h *Handler) {
		h.aliases = service
	}
}

// WithCombos enables combo administration endpoints.
func WithCombos(service *combos.Service) Option {
	return func(h *Handler) {
		h.combos = service
	}
}

// WithCLITools enables CLI tool helper endpoints.
func WithCLITools(service *clitools.Service) Option {
	return func(h *Handler) {
		h.cliTools = service
	}
}

// WithAuthKeys enables managed auth key administration endpoints.
func WithAuthKeys(service *authkeys.Service) Option {
	return func(h *Handler) {
		h.authKeys = service
	}
}

// WithAuthKeyRateInspector wires a live rate-limit inspector so the per-key
// stats endpoint can return current consumption alongside historical usage.
func WithAuthKeyRateInspector(inspector AuthKeyRateLimitInspector) Option {
	return func(h *Handler) {
		h.authKeyRateInspector = inspector
	}
}

// WithModelOverrides enables model override administration endpoints.
func WithModelOverrides(service *modeloverrides.Service) Option {
	return func(h *Handler) {
		h.modelOverrides = service
	}
}

// WithWorkflows enables workflow administration endpoints.
func WithWorkflows(service *workflow.Service) Option {
	return func(h *Handler) {
		h.workflows = service
	}
}

// WithGuardrailsRegistry enables listing valid guardrail references for workflow authoring.
func WithGuardrailsRegistry(registry guardrails.Catalog) Option {
	return func(h *Handler) {
		h.guardrails = registry
	}
}

// WithGuardrailService enables full guardrail definition administration endpoints.
func WithGuardrailService(service *guardrails.Service) Option {
	return func(h *Handler) {
		h.guardrails = service
		h.guardrailDefs = service
	}
}

// WithResponseCache enables cache debug and overview helper endpoints.
func WithResponseCache(cacheMiddleware *responsecache.ResponseCacheMiddleware) Option {
	return func(h *Handler) {
		h.responseCache = cacheMiddleware
	}
}

// SetResponseCache wires the response cache after admin handler construction.
func (h *Handler) SetResponseCache(cacheMiddleware *responsecache.ResponseCacheMiddleware) {
	if h == nil {
		return
	}
	h.responseCache = cacheMiddleware
}

// WithDashboardRuntimeConfig enables the allowlisted dashboard runtime config endpoint.
func WithDashboardRuntimeConfig(values DashboardConfigResponse) Option {
	return func(h *Handler) {
		h.runtimeConfig = normalizeDashboardRuntimeConfig(values)
	}
}

// WithRuntimeRefresher enables manual runtime refresh from the admin API.
func WithRuntimeRefresher(refresher RuntimeRefresher) Option {
	return func(h *Handler) {
		h.runtimeRefresher = refresher
	}
}

// WithDashboardSettingsManager enables editable dashboard settings backed by the server config.
func WithDashboardSettingsManager(manager DashboardSettingsManager) Option {
	return func(h *Handler) {
		h.settingsManager = manager
	}
}

// WithConfiguredProviders enables the admin-safe provider inventory endpoint.
func WithConfiguredProviders(configs []providers.SanitizedProviderConfig) Option {
	return func(h *Handler) {
		h.configuredProviders = cloneConfiguredProviders(configs)
	}
}

// WithProviderOverrides enables provider CRUD via the admin API.
func WithProviderOverrides(store ...*ProviderOverrideStore) Option {
	return func(h *Handler) {
		if len(store) > 0 && store[0] != nil {
			h.providerOverrides = store[0]
			return
		}
		h.providerOverrides = NewProviderOverrideStore()
	}
}

// WithPoolWeights enables pool weight overrides via the admin API.
func WithPoolWeights(store ...*PoolOverrideStore) Option {
	return func(h *Handler) {
		if len(store) > 0 && store[0] != nil {
			h.poolWeights = store[0]
			return
		}
		h.poolWeights = NewPoolOverrideStore()
	}
}

// WithPools attaches the pool registry so the admin API can expose pool
// inventory + per-member load/health for the dashboard.
func WithPools(reg *pool.Registry) Option {
	return func(h *Handler) {
		h.pools = reg
	}
}

func WithMasterKey(key string) Option {
	return func(h *Handler) {
		h.masterKey = key
	}
}

// NewHandler creates a new admin API handler.
// usageReader may be nil if usage tracking is not available.
func NewHandler(reader usage.UsageReader, registry *providers.ModelRegistry, options ...Option) *Handler {
	h := &Handler{
		usageReader:   reader,
		registry:      registry,
		runtimeConfig: DashboardConfigResponse{},
	}

	for _, opt := range options {
		if opt != nil {
			opt(h)
		}
	}

	return h
}

func normalizeDashboardRuntimeConfig(values DashboardConfigResponse) DashboardConfigResponse {
	return DashboardConfigResponse{
		Edition:               strings.TrimSpace(values.Edition),
		Capabilities:          normalizeStringSlice(values.Capabilities),
		CapabilityMap:         normalizeCapabilityMap(values.CapabilityMap),
		FeatureFallbackMode:   strings.TrimSpace(values.FeatureFallbackMode),
		LoggingEnabled:        strings.TrimSpace(values.LoggingEnabled),
		UsageEnabled:          strings.TrimSpace(values.UsageEnabled),
		BudgetsEnabled:        strings.TrimSpace(values.BudgetsEnabled),
		GuardrailsEnabled:     strings.TrimSpace(values.GuardrailsEnabled),
		CacheEnabled:          strings.TrimSpace(values.CacheEnabled),
		RedisURL:              strings.TrimSpace(values.RedisURL),
		SemanticCacheEnabled:  strings.TrimSpace(values.SemanticCacheEnabled),
		PricingRecalculation:  strings.TrimSpace(values.PricingRecalculation),
		Fallback:              normalizeFallbackConfigSnapshot(values.Fallback),
		Settings:              normalizeDashboardSettingsSnapshot(values.Settings),
		RuntimeFeatures:       normalizeRuntimeFeatureSnapshots(values.RuntimeFeatures),
	}
}

func normalizeDashboardSettingsSnapshot(values DashboardSettingsSnapshot) DashboardSettingsSnapshot {
	return DashboardSettingsSnapshot{
		Client: DashboardClientSettingsSnapshot{
			Port:                            strings.TrimSpace(values.Client.Port),
			BasePath:                        strings.TrimSpace(values.Client.BasePath),
			BodySizeLimit:                   strings.TrimSpace(values.Client.BodySizeLimit),
			SwaggerEnabled:                  values.Client.SwaggerEnabled,
			PprofEnabled:                    values.Client.PprofEnabled,
			AdminEndpointsEnabled:           values.Client.AdminEndpointsEnabled,
			AdminUIEnabled:                  values.Client.AdminUIEnabled,

			EnableAnthropicIngress:          values.Client.EnableAnthropicIngress,
			EnablePassthroughRoutes:         values.Client.EnablePassthroughRoutes,
			AllowPassthroughV1Alias:         values.Client.AllowPassthroughV1Alias,
			EnabledPassthroughProviders:     normalizeStringSlice(values.Client.EnabledPassthroughProviders),
			ModelsEnabledByDefault:          values.Client.ModelsEnabledByDefault,
			ModelOverridesEnabled:           values.Client.ModelOverridesEnabled,
			KeepOnlyAliasesAtModelsEndpoint: values.Client.KeepOnlyAliasesAtModelsEndpoint,
			ConfiguredProviderModelsMode:    strings.TrimSpace(values.Client.ConfiguredProviderModelsMode),
		},
		Caching: DashboardCachingSettingsSnapshot{
			ModelCacheBackend:               strings.TrimSpace(values.Caching.ModelCacheBackend),
			ModelCacheLocalDir:              strings.TrimSpace(values.Caching.ModelCacheLocalDir),
			ModelCacheRedisURL:              strings.TrimSpace(values.Caching.ModelCacheRedisURL),
			ModelCacheRedisKey:              strings.TrimSpace(values.Caching.ModelCacheRedisKey),
			ModelCacheRedisTTLSeconds:       values.Caching.ModelCacheRedisTTLSeconds,
			ModelRefreshIntervalSeconds:     values.Caching.ModelRefreshIntervalSeconds,
			ModelListURL:                    strings.TrimSpace(values.Caching.ModelListURL),
			ModelListLocalPath:              strings.TrimSpace(values.Caching.ModelListLocalPath),
			ModelListUserOverridesPath:      strings.TrimSpace(values.Caching.ModelListUserOverridesPath),
			ExactCacheEnabled:               values.Caching.ExactCacheEnabled,
			ExactCacheRedisURL:              strings.TrimSpace(values.Caching.ExactCacheRedisURL),
			ExactCacheRedisKey:              strings.TrimSpace(values.Caching.ExactCacheRedisKey),
			ExactCacheTTLSeconds:            values.Caching.ExactCacheTTLSeconds,
			SemanticCacheEnabled:            values.Caching.SemanticCacheEnabled,
			SemanticSimilarityThreshold:     values.Caching.SemanticSimilarityThreshold,
			SemanticPromptSimilarityMin:     values.Caching.SemanticPromptSimilarityMin,
			SemanticTTLSeconds:              values.Caching.SemanticTTLSeconds,
			SemanticMaxConversationMessages: values.Caching.SemanticMaxConversationMessages,
			SemanticExcludeSystemPrompt:     values.Caching.SemanticExcludeSystemPrompt,
			SemanticEmbedderProvider:        strings.TrimSpace(values.Caching.SemanticEmbedderProvider),
			SemanticEmbedderModel:           strings.TrimSpace(values.Caching.SemanticEmbedderModel),
			SemanticVectorStoreType:         strings.TrimSpace(values.Caching.SemanticVectorStoreType),
			SemanticVectorStoreHints:        normalizeStringSlice(values.Caching.SemanticVectorStoreHints),
			SemanticVectorStoreURL:          strings.TrimSpace(values.Caching.SemanticVectorStoreURL),
			SemanticVectorStoreCollection:   strings.TrimSpace(values.Caching.SemanticVectorStoreCollection),
			SemanticVectorStoreTable:        strings.TrimSpace(values.Caching.SemanticVectorStoreTable),
			SemanticVectorStoreNamespace:    strings.TrimSpace(values.Caching.SemanticVectorStoreNamespace),
			SemanticVectorStoreClass:        strings.TrimSpace(values.Caching.SemanticVectorStoreClass),
			SemanticVectorStoreDimension:    values.Caching.SemanticVectorStoreDimension,
			SemanticVectorStoreAPIKeySet:    values.Caching.SemanticVectorStoreAPIKeySet,
			PromptCacheMode:                 strings.TrimSpace(values.Caching.PromptCacheMode),
			PromptCacheSystemPrompt:         values.Caching.PromptCacheSystemPrompt,
			PromptCacheFirstMessage:         values.Caching.PromptCacheFirstMessage,
			PromptCacheTools:                values.Caching.PromptCacheTools,
			PromptCacheMinTokens:            values.Caching.PromptCacheMinTokens,
		},
		Logging: DashboardLoggingSettingsSnapshot{
			Enabled:               values.Logging.Enabled,
			LogBodies:             values.Logging.LogBodies,
			LogHeaders:            values.Logging.LogHeaders,
			BufferSize:            values.Logging.BufferSize,
			FlushIntervalSeconds:  values.Logging.FlushIntervalSeconds,
			RetentionDays:         values.Logging.RetentionDays,
			OnlyModelInteractions: values.Logging.OnlyModelInteractions,
		},
		Observability: DashboardObservabilitySettingsSnapshot{
			MetricsEnabled:  values.Observability.MetricsEnabled,
			MetricsEndpoint: strings.TrimSpace(values.Observability.MetricsEndpoint),
			StorageType:     strings.TrimSpace(values.Observability.StorageType),
		},

		Storage: DashboardStorageSettingsSnapshot{
			Type:               strings.TrimSpace(values.Storage.Type),
			SQLitePath:         strings.TrimSpace(values.Storage.SQLitePath),
			PostgreSQLURL:      strings.TrimSpace(values.Storage.PostgreSQLURL),
			PostgreSQLMaxConns: values.Storage.PostgreSQLMaxConns,
			MongoDBURL:         strings.TrimSpace(values.Storage.MongoDBURL),
			MongoDBDatabase:    strings.TrimSpace(values.Storage.MongoDBDatabase),
		},
		Performance: DashboardPerformanceSettingsSnapshot{
			HTTPTimeoutSeconds:                values.Performance.HTTPTimeoutSeconds,
			HTTPResponseHeaderTimeoutSeconds:  values.Performance.HTTPResponseHeaderTimeoutSeconds,
			WorkflowRefreshIntervalSeconds:    values.Performance.WorkflowRefreshIntervalSeconds,
			RetryMaxRetries:                   values.Performance.RetryMaxRetries,
			RetryInitialBackoffMilliseconds:   values.Performance.RetryInitialBackoffMilliseconds,
			RetryMaxBackoffMilliseconds:       values.Performance.RetryMaxBackoffMilliseconds,
			RetryBackoffFactor:                values.Performance.RetryBackoffFactor,
			RetryJitterFactor:                 values.Performance.RetryJitterFactor,
			CircuitBreakerFailureThreshold:    values.Performance.CircuitBreakerFailureThreshold,
			CircuitBreakerSuccessThreshold:    values.Performance.CircuitBreakerSuccessThreshold,
			CircuitBreakerTimeoutMilliseconds: values.Performance.CircuitBreakerTimeoutMilliseconds,
		},
		Proxy: DashboardProxySettingsSnapshot{
			HTTPProxy:        strings.TrimSpace(values.Proxy.HTTPProxy),
			HTTPSProxy:       strings.TrimSpace(values.Proxy.HTTPSProxy),
			NoProxy:          strings.TrimSpace(values.Proxy.NoProxy),
			ProxyAuthEnabled: values.Proxy.ProxyAuthEnabled,
			CACertPEM:        strings.TrimSpace(values.Proxy.CACertPEM),
		},
		Security: DashboardSecuritySettingsSnapshot{
			MasterKeyConfigured: values.Security.MasterKeyConfigured,
			GuardrailsEnabled:   values.Security.GuardrailsEnabled,
			BatchGuardrails:     values.Security.BatchGuardrails,
		},
		Pricing: DashboardPricingSettingsSnapshot{
			UsageEnabled:                  values.Pricing.UsageEnabled,
			EnforceReturningUsageData:     values.Pricing.EnforceReturningUsageData,
			PricingRecalculationEnabled:   values.Pricing.PricingRecalculationEnabled,
			UsageBufferSize:               values.Pricing.UsageBufferSize,
			UsageFlushIntervalSeconds:     values.Pricing.UsageFlushIntervalSeconds,
			UsageRetentionDays:            values.Pricing.UsageRetentionDays,
			BudgetsEnabled:                values.Pricing.BudgetsEnabled,
			ConfiguredBudgetUserPathCount: values.Pricing.ConfiguredBudgetUserPathCount,
		},
		TokenSaver: normalizeTokenSaverSettingsSnapshot(values.TokenSaver),
	}
}

func normalizeTokenSaverSettingsSnapshot(values DashboardTokenSaverSettingsSnapshot) DashboardTokenSaverSettingsSnapshot {
	return DashboardTokenSaverSettingsSnapshot{
		Enabled:         values.Enabled,
		ApplyStreaming:  values.ApplyStreaming,
		Endpoints:       normalizeStringSlice(values.Endpoints),
		OutputEnabled:   values.OutputEnabled,
		OutputProfile:   strings.TrimSpace(values.OutputProfile),
		OutputLevel:     strings.TrimSpace(values.OutputLevel),
		EmitHeaders:     values.EmitHeaders,
		OnError:         strings.TrimSpace(values.OnError),
		ModelInclude:    normalizeStringSlice(values.ModelInclude),
		ModelExclude:    normalizeStringSlice(values.ModelExclude),
		ProviderInclude:       normalizeStringSlice(values.ProviderInclude),
		ProviderExclude:       normalizeStringSlice(values.ProviderExclude),
		AuditEnabled:    values.AuditEnabled,
	}
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeCapabilityMap(values map[string]bool) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]bool, len(values))
	for key, enabled := range values {
		key = strings.TrimSpace(key)
		if key != "" && enabled {
			out[key] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeRuntimeFeatureSnapshots(values []RuntimeFeatureSnapshot) []RuntimeFeatureSnapshot {
	features := make([]RuntimeFeatureSnapshot, 0, len(values))
	for _, value := range values {
		key := strings.TrimSpace(value.Key)
		label := strings.TrimSpace(value.Label)
		if key == "" || label == "" {
			continue
		}
		features = append(features, RuntimeFeatureSnapshot{
			Key:         key,
			Label:       label,
			Status:      strings.TrimSpace(value.Status),
			Configured:  value.Configured,
			Description: strings.TrimSpace(value.Description),
			Usage:       strings.TrimSpace(value.Usage),
			Dependency:  strings.TrimSpace(value.Dependency),
			Endpoint:    strings.TrimSpace(value.Endpoint),
		})
	}
	return features
}

func normalizeFallbackConfigSnapshot(values FallbackConfigSnapshot) FallbackConfigSnapshot {
	rules := make([]FallbackRuleSnapshot, 0, len(values.ManualRules))
	for _, rule := range values.ManualRules {
		source := strings.TrimSpace(rule.Source)
		if source == "" {
			continue
		}
		targets := make([]string, 0, len(rule.Targets))
		for _, target := range rule.Targets {
			if target = strings.TrimSpace(target); target != "" {
				targets = append(targets, target)
			}
		}
		rules = append(rules, FallbackRuleSnapshot{Source: source, Targets: targets})
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].Source < rules[j].Source })

	overrides := make([]FallbackOverrideSnapshot, 0, len(values.Overrides))
	for _, override := range values.Overrides {
		model := strings.TrimSpace(override.Model)
		mode := strings.TrimSpace(override.Mode)
		if model == "" || mode == "" {
			continue
		}
		overrides = append(overrides, FallbackOverrideSnapshot{Model: model, Mode: mode})
	}
	sort.Slice(overrides, func(i, j int) bool { return overrides[i].Model < overrides[j].Model })

	return FallbackConfigSnapshot{
		Mode:                  strings.TrimSpace(values.Mode),
		ManualRulesConfigured: values.ManualRulesConfigured || len(rules) > 0 || values.ManualRuleCount > 0,
		ManualRuleCount:       max(values.ManualRuleCount, len(rules)),
		ManualRules:           rules,
		Overrides:             overrides,
	}
}

func cloneDashboardRuntimeConfig(values DashboardConfigResponse) DashboardConfigResponse {
	return normalizeDashboardRuntimeConfig(values)
}

func cloneConfiguredProviders(configs []providers.SanitizedProviderConfig) []providers.SanitizedProviderConfig {
	if len(configs) == 0 {
		return nil
	}
	cloned := make([]providers.SanitizedProviderConfig, len(configs))
	for i := range configs {
		cloned[i] = configs[i]
		if len(configs[i].Models) > 0 {
			cloned[i].Models = append([]string(nil), configs[i].Models...)
		}
	}
	return cloned
}

var validIntervals = map[string]bool{
	"daily":   true,
	"weekly":  true,
	"monthly": true,
	"yearly":  true,
}

const (
	dashboardTimeZoneHeader = "X-aurora-Timezone"
	defaultDashboardTZ      = "UTC"
	defaultDateRangeDays    = 30
	maxDateRangeDays        = 365
)

var timeNow = time.Now

// parseUsageParams extracts UsageQueryParams from the request query string.
// Returns an error if date parameters are provided but malformed.
func parseUsageParams(c *echo.Context) (usage.UsageQueryParams, error) {
	params, err := parseDateRangeParams(c)
	if err != nil {
		return params, err
	}

	// Parse interval
	params.Interval = c.QueryParam("interval")
	if !validIntervals[params.Interval] {
		params.Interval = "daily"
	}
	params.CacheMode = c.QueryParam("cache_mode")

	userPath, err := normalizeUserPathQueryParam("user_path", c.QueryParam("user_path"))
	if err != nil {
		return params, err
	}
	params.UserPath = userPath

	return params, nil
}

func normalizeUserPathQueryParam(fieldName, raw string) (string, error) {
	userPath, err := core.NormalizeUserPath(raw)
	if err != nil {
		return "", core.NewInvalidRequestError("invalid "+fieldName+": "+err.Error(), err)
	}
	return userPath, nil
}

// parseDateRangeParams extracts common date range query params.
// Returns an error if date parameters are provided but malformed.
func parseDateRangeParams(c *echo.Context) (usage.UsageQueryParams, error) {
	var params usage.UsageQueryParams

	timeZone, location := dashboardTimeZone(c)
	params.TimeZone = timeZone

	now := timeNow().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)

	days := defaultDateRangeDays
	if d := c.QueryParam("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = min(parsed, maxDateRangeDays)
		}
	}

	start, end, err := buildDateRange(strings.TrimSpace(c.QueryParam("start_date")), strings.TrimSpace(c.QueryParam("end_date")), days, location, today)
	if err != nil {
		return params, err
	}
	params.StartDate = start
	params.EndDate = end
	return params, nil
}

func buildDateRange(startStr, endStr string, days int, location *time.Location, today time.Time) (time.Time, time.Time, error) {
	var start, end time.Time
	var startParsed, endParsed bool

	if startStr != "" {
		t, err := time.ParseInLocation("2006-01-02", startStr, location)
		if err != nil {
			return time.Time{}, time.Time{}, core.NewInvalidRequestError("invalid start_date format, expected YYYY-MM-DD", nil)
		}
		start = t
		startParsed = true
	}
	if endStr != "" {
		t, err := time.ParseInLocation("2006-01-02", endStr, location)
		if err != nil {
			return time.Time{}, time.Time{}, core.NewInvalidRequestError("invalid end_date format, expected YYYY-MM-DD", nil)
		}
		end = t
		endParsed = true
	}

	if startParsed || endParsed {
		if !startParsed {
			start = end.AddDate(0, 0, -29)
		}
		if !endParsed {
			end = today
		}
	} else {
		days = normalizeDateRangeDays(days)
		end = today
		start = today.AddDate(0, 0, -(days - 1))
	}

	if start.After(end) {
		return time.Time{}, time.Time{}, core.NewInvalidRequestError("start_date must be on or before end_date", nil)
	}
	return start, end, nil
}

func normalizeDateRangeDays(days int) int {
	if days <= 0 {
		return defaultDateRangeDays
	}
	return min(days, maxDateRangeDays)
}

func dashboardTimeZone(c *echo.Context) (string, *time.Location) {
	value := strings.TrimSpace(c.Request().Header.Get(dashboardTimeZoneHeader))
	if value == "" {
		return defaultDashboardTZ, time.UTC
	}

	location, err := time.LoadLocation(value)
	if err != nil {
		return defaultDashboardTZ, time.UTC
	}

	return location.String(), location
}

// handleError converts errors to appropriate HTTP responses, matching the
// format used by the main API handlers in the server package.
func handleError(c *echo.Context, err error) error {
	if gatewayErr, ok := errors.AsType[*core.GatewayError](err); ok {
		logHandledAdminError(c, gatewayErr)
		return c.JSON(gatewayErr.HTTPStatusCode(), gatewayErr.ToJSON())
	}

	if errors.Is(err, context.Canceled) {
		gatewayErr := core.NewInvalidRequestErrorWithStatus(statusClientClosedRequest, "request canceled", err).
			WithCode("request_canceled")
		logHandledAdminError(c, gatewayErr)
		return c.JSON(gatewayErr.HTTPStatusCode(), gatewayErr.ToJSON())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		gatewayErr := core.NewInvalidRequestErrorWithStatus(http.StatusGatewayTimeout, "request timed out", err).
			WithCode("request_timeout")
		logHandledAdminError(c, gatewayErr)
		return c.JSON(gatewayErr.HTTPStatusCode(), gatewayErr.ToJSON())
	}

	fallback := &core.GatewayError{
		Type:       "internal_error",
		Message:    "an unexpected error occurred",
		StatusCode: http.StatusInternalServerError,
		Err:        err,
	}
	logHandledAdminError(c, fallback)
	return c.JSON(fallback.HTTPStatusCode(), fallback.ToJSON())
}

func logHandledAdminError(c *echo.Context, gatewayErr *core.GatewayError) {
	if gatewayErr == nil {
		return
	}

	attrs := []any{
		"type", gatewayErr.Type,
		"status", gatewayErr.HTTPStatusCode(),
		"message", gatewayErr.Message,
	}
	if gatewayErr.Provider != "" {
		attrs = append(attrs, "provider", gatewayErr.Provider)
	}
	if gatewayErr.Param != nil {
		attrs = append(attrs, "param", *gatewayErr.Param)
	}
	if gatewayErr.Code != nil {
		attrs = append(attrs, "code", *gatewayErr.Code)
	}
	if gatewayErr.Err != nil {
		attrs = append(attrs, "error", gatewayErr.Err)
	}
	if c != nil && c.Request() != nil {
		req := c.Request()
		attrs = append(attrs,
			"method", req.Method,
			"path", req.URL.Path,
		)
		if requestID := requestIDFromAdminContextOrHeader(req); requestID != "" {
			attrs = append(attrs, "request_id", requestID)
		}
	}

	status := gatewayErr.HTTPStatusCode()
	if status == statusClientClosedRequest {
		slog.Debug("admin request canceled", attrs...)
		return
	}
	if status >= http.StatusInternalServerError {
		slog.Error("admin request failed", attrs...)
		return
	}
	slog.Warn("admin request failed", attrs...)
}

func requestIDFromAdminContextOrHeader(req *http.Request) string {
	if req == nil {
		return ""
	}
	if requestID := strings.TrimSpace(core.GetRequestID(req.Context())); requestID != "" {
		return requestID
	}
	return strings.TrimSpace(req.Header.Get("X-Request-ID"))
}

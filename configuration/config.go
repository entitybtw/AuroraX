// Package config provides configuration management for the application.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"aurora/internal/storage"
)

// Body size limit constants
const (
	DefaultBodySizeLimit int64 = 10 * 1024 * 1024  // 10MB
	MinBodySizeLimit     int64 = 1 * 1024          // 1KB
	MaxBodySizeLimit     int64 = 100 * 1024 * 1024 // 100MB
)

var bodySizeLimitRegex = regexp.MustCompile(`(?i)^(\d+)([KMG])?B?$`)

// Config holds the application configuration.
type Config struct {
	Server               ServerConfig               `yaml:"server"`
	Models               ModelsConfig               `yaml:"models"`
	Cache                CacheConfig                `yaml:"cache"`
	Storage              StorageConfig              `yaml:"storage"`
	Logging              LogConfig                  `yaml:"logging"`
	Usage                UsageConfig                `yaml:"usage"`
	Metrics              MetricsConfig              `yaml:"metrics"`
	HTTP                 HTTPConfig                 `yaml:"http"`
	Admin                AdminConfig                `yaml:"admin"`
	Guardrails           GuardrailsConfig           `yaml:"guardrails"`
	Fallback             FallbackConfig             `yaml:"fallback"`
	Combos               CombosConfig               `yaml:"combos"`
	CLITools             CLIToolsConfig             `yaml:"cli_tools"`
	Workflows            WorkflowsConfig            `yaml:"workflows"`
	Resilience           ResilienceConfig           `yaml:"resilience"`
	TokenSaver           TokenSaverConfig           `yaml:"token_saver"`
	Edition              EditionConfig              `yaml:"edition"`
}

// LoadResult is returned by Load and bundles the application config with the raw
// provider map parsed from YAML. Provider env vars and resolution are handled by
// the providers package.
type LoadResult struct {
	Config       *Config
	RawProviders map[string]RawProviderConfig
	RawPools     map[string]RawPoolConfig
}

// RawProviderConfig is the YAML-sourced provider configuration before env var
// overrides, credential filtering, or resilience merging. Exported so the
// providers package can resolve it into a fully-configured ProviderConfig.
type RawProviderConfig struct {
	Type       string               `yaml:"type"`
	APIKey     string               `yaml:"api_key"`
	BaseURL    string               `yaml:"base_url"`
	APIVersion string               `yaml:"api_version"`
	Models     []RawProviderModel   `yaml:"models"`
	Resilience *RawResilienceConfig `yaml:"resilience"`
}

// RawPoolConfig is the YAML-sourced provider pool definition. Pools group
// configured providers of the same upstream type into a logical load-balanced
// unit. The pool name shares the configured-provider namespace; the providers
// package validates that no pool name collides with a real provider instance.
type RawPoolConfig struct {
	// Members is the list of configured provider instance names that belong
	// to this pool. All members must share the same provider type and expose
	// the same model IDs (the gateway does not validate that strictly, but
	// mismatched members will surface as 4xx/5xx routing errors).
	Members []string `yaml:"members"`
	// Strategy selects the load-balancing algorithm. Empty / unset defaults
	// to round_robin. Allowed: round_robin.
	Strategy string `yaml:"strategy"`
	// Weights is required for strategy=weighted and ignored otherwise. Map
	// keys are member provider names; values are positive integers.
	Weights map[string]int `yaml:"weights"`
	// HealthAware enables circuit-breaker-aware member skipping. Defaults
	// to true; set to false to route to unhealthy members regardless.
	HealthAware *bool `yaml:"health_aware"`
}

// RawResilienceConfig holds optional per-provider resilience overrides from YAML.
// Nil fields inherit from the global ResilienceConfig.
type RawResilienceConfig struct {
	Retry          *RawRetryConfig          `yaml:"retry"`
	CircuitBreaker *RawCircuitBreakerConfig `yaml:"circuit_breaker"`
}

// RawCircuitBreakerConfig holds optional per-provider circuit breaker overrides from YAML.
// Nil fields inherit from the global CircuitBreakerConfig.
type RawCircuitBreakerConfig struct {
	FailureThreshold *int           `yaml:"failure_threshold"`
	SuccessThreshold *int           `yaml:"success_threshold"`
	Timeout          *time.Duration `yaml:"timeout"`
}

// RawRetryConfig holds optional per-provider retry overrides from YAML.
// Nil fields inherit from the global RetryConfig.
type RawRetryConfig struct {
	MaxRetries     *int           `yaml:"max_retries"`
	InitialBackoff *time.Duration `yaml:"initial_backoff"`
	MaxBackoff     *time.Duration `yaml:"max_backoff"`
	BackoffFactor  *float64       `yaml:"backoff_factor"`
	JitterFactor   *float64       `yaml:"jitter_factor"`
}

// ModelsConfig holds global model access defaults.
type ModelsConfig struct {
	// EnabledByDefault controls whether provider models are available
	// when no persisted user-path override exists and model overrides are enabled.
	// Default: true.
	EnabledByDefault bool `yaml:"enabled_by_default" env:"MODELS_ENABLED_BY_DEFAULT"`

	// OverridesEnabled controls whether persisted model access overrides are
	// loaded, enforced, and exposed through the admin dashboard/API.
	// Default: true.
	OverridesEnabled bool `yaml:"overrides_enabled" env:"MODEL_OVERRIDES_ENABLED"`

	// KeepOnlyAliasesAtModelsEndpoint controls whether GET /v1/models hides
	// provider models and returns only alias-projected model entries.
	// Default: false.
	KeepOnlyAliasesAtModelsEndpoint bool `yaml:"keep_only_aliases_at_models_endpoint" env:"KEEP_ONLY_ALIASES_AT_MODELS_ENDPOINT"`

	// ConfiguredProviderModelsMode controls how providers.<name>.models and
	// provider *_MODELS env vars affect the provider model inventory.
	// Supported values: "fallback", "allowlist". Default: "fallback".
	ConfiguredProviderModelsMode ConfiguredProviderModelsMode `yaml:"configured_provider_models_mode" env:"CONFIGURED_PROVIDER_MODELS_MODE"`
}

// ConfiguredProviderModelsMode controls how explicitly configured provider
// model lists are applied to the discovered model inventory.
type ConfiguredProviderModelsMode string

type EditionName string

type CapabilityKey string

const (
	EditionOSS EditionName = "oss"
)

type EditionConfig struct {
	Name         EditionName `yaml:"name"`
	Capabilities []string    `yaml:"capabilities"`
}

func NormalizeEditionName(name EditionName) EditionName {
	switch EditionName(strings.ToLower(strings.TrimSpace(string(name)))) {
	default:
		return EditionOSS
	}
}

func ResolveCapabilities(edition EditionConfig) map[string]bool {
	return map[string]bool{}
}

func HasCapability(edition EditionConfig, key CapabilityKey) bool {
	return ResolveCapabilities(edition)[string(key)]
}

func CapabilityList(capabilities map[string]bool) []string {
	out := make([]string, 0, len(capabilities))
	for key, enabled := range capabilities {
		if enabled {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

const (
	ConfiguredProviderModelsModeFallback  ConfiguredProviderModelsMode = "fallback"
	ConfiguredProviderModelsModeAllowlist ConfiguredProviderModelsMode = "allowlist"
)

// Valid reports whether mode is one of the supported configured-provider-models modes.
func (m ConfiguredProviderModelsMode) Valid() bool {
	switch NormalizeConfiguredProviderModelsMode(m) {
	case ConfiguredProviderModelsModeFallback, ConfiguredProviderModelsModeAllowlist:
		return true
	default:
		return false
	}
}

// NormalizeConfiguredProviderModelsMode canonicalizes a configured provider models mode.
func NormalizeConfiguredProviderModelsMode(mode ConfiguredProviderModelsMode) ConfiguredProviderModelsMode {
	return ConfiguredProviderModelsMode(strings.ToLower(strings.TrimSpace(string(mode))))
}

// ResolveConfiguredProviderModelsMode canonicalizes mode and applies the process default.
func ResolveConfiguredProviderModelsMode(mode ConfiguredProviderModelsMode) ConfiguredProviderModelsMode {
	mode = NormalizeConfiguredProviderModelsMode(mode)
	if mode == "" {
		return ConfiguredProviderModelsModeFallback
	}
	return mode
}

// AdminConfig holds configuration for the admin API and dashboard UI.
type AdminConfig struct {
	// EndpointsEnabled controls whether the admin REST API is active
	// Default: true
	EndpointsEnabled bool `yaml:"endpoints_enabled" env:"ADMIN_ENDPOINTS_ENABLED"`

	// UIEnabled controls whether the admin dashboard UI is active
	// Requires EndpointsEnabled â€” if endpoints are disabled and UI is enabled,
	// a warning is logged and UI is forced to false.
	// Default: true
	UIEnabled bool `yaml:"ui_enabled" env:"ADMIN_UI_ENABLED"`


}

// GuardrailsConfig holds configuration for the request guardrails pipeline.
type GuardrailsConfig struct {
	// Enabled controls whether guardrails are active
	// Default: false
	Enabled bool `yaml:"enabled" env:"GUARDRAILS_ENABLED"`

	// EnableForBatchProcessing controls whether guardrails are applied to inline
	// batch items for /v1/batches requests.
	// Default: false
	EnableForBatchProcessing bool `yaml:"enable_for_batch_processing" env:"ENABLE_GUARDRAILS_FOR_BATCH_PROCESSING"`

	// Rules is a list of guardrail instances. Each entry defines one guardrail
	// with its own name, type, order, and type-specific settings. Multiple
	// instances of the same type are allowed (e.g. two system_prompt guardrails
	// with different content).
	Rules []GuardrailRuleConfig `yaml:"rules"`
}

// GuardrailRuleConfig defines a single guardrail instance.
type GuardrailRuleConfig struct {
	// Name is a unique identifier for this guardrail instance (used in logs and errors)
	Name string `yaml:"name"`

	// Type selects the guardrail implementation: "system_prompt", "regex_block", "pii_redact", or "length_limit"
	Type string `yaml:"type"`

	// UserPath scopes internal auxiliary guardrail requests for workflow
	// selection and audit logging. When empty, the caller user path is used.
	UserPath string `yaml:"user_path"`

	// Direction controls whether this guardrail runs before provider dispatch,
	// after non-streaming provider responses, or both. Default: "input".
	Direction string `yaml:"direction"`

	// Order controls execution ordering relative to other guardrails.
	// Guardrails with the same order run in parallel; different orders run sequentially.
	// Default: 0
	Order int `yaml:"order"`

	// SystemPrompt holds settings when Type is "system_prompt"
	SystemPrompt SystemPromptSettings `yaml:"system_prompt"`

	// LLMBasedAltering holds settings when Type is "llm_based_altering"
	LLMBasedAltering LLMBasedAlteringSettings `yaml:"llm_based_altering"`

	// RegexBlock holds settings when Type is "regex_block"
	RegexBlock RegexBlockSettings `yaml:"regex_block"`

	// PIIRedact holds settings when Type is "pii_redact"
	PIIRedact PIIRedactSettings `yaml:"pii_redact"`

	// LengthLimit holds settings when Type is "length_limit"
	LengthLimit LengthLimitSettings `yaml:"length_limit"`
}

// SystemPromptSettings holds the type-specific settings for a system_prompt guardrail.
type SystemPromptSettings struct {
	// Mode controls how the system prompt is applied: "inject", "override", or "decorator"
	//   - inject: adds a system message only if none exists
	//   - override: replaces all existing system messages
	//   - decorator: prepends to the first existing system message
	// Default: "inject"
	Mode string `yaml:"mode"`

	// Content is the system prompt text to apply
	Content string `yaml:"content"`
	// OnError controls guardrail failure behavior: "block" or "allow".
	OnError string `yaml:"on_error"`
}

// LLMBasedAlteringSettings holds the type-specific settings for an llm_based_altering guardrail.
type LLMBasedAlteringSettings struct {
	// Model is the model selector used for the auxiliary rewrite call.
	// This can be a concrete model name, provider-qualified selector, or alias.
	Model string `yaml:"model"`

	// Provider is an optional routing hint for Model.
	Provider string `yaml:"provider"`

	// Prompt is the system prompt used to rewrite targeted messages.
	// When empty, the built-in LiteLLM-derived anonymization prompt is used.
	Prompt string `yaml:"prompt"`

	// Roles selects which message roles are rewritten.
	// Default: ["user"]
	Roles []string `yaml:"roles"`

	// SkipContentPrefix skips rewriting for messages whose trimmed text begins with this prefix.
	SkipContentPrefix string `yaml:"skip_content_prefix"`

	// MaxTokens limits the auxiliary rewrite completion.
	// Default: 4096
	MaxTokens int `yaml:"max_tokens"`
	// OnError controls guardrail failure behavior: "block" or "allow".
	OnError string `yaml:"on_error"`
}

// RegexBlockSettings holds the type-specific settings for a regex_block guardrail.
type RegexBlockSettings struct {
	// Action is "block" or "sanitize". Default: "block".
	Action string `yaml:"action"`
	// Patterns are Go regular expressions matched against selected message roles.
	Patterns []string `yaml:"patterns"`
	// Replacement is used when Action is "sanitize". Default: "[REDACTED]".
	Replacement string `yaml:"replacement"`
	// Roles selects which roles are scanned. Empty means all roles.
	Roles []string `yaml:"roles"`
	// OnError controls guardrail failure behavior: "block" or "allow".
	OnError string `yaml:"on_error"`
}

// PIIRedactSettings holds the type-specific settings for a pii_redact guardrail.
type PIIRedactSettings struct {
	// Kinds selects PII categories: email, phone, ssn, credit_card. Empty means all.
	Kinds []string `yaml:"kinds"`
	// Roles selects which roles are redacted. Empty means all roles.
	Roles []string `yaml:"roles"`
	// OnError controls guardrail failure behavior: "block" or "allow".
	OnError string `yaml:"on_error"`
}

// LengthLimitSettings holds the type-specific settings for a length_limit guardrail.
type LengthLimitSettings struct {
	// MaxChars rejects requests whose combined message content exceeds this character count.
	MaxChars int `yaml:"max_chars"`
	// MaxEstimatedTokens rejects requests whose estimated token count exceeds this value.
	MaxEstimatedTokens int `yaml:"max_estimated_tokens"`
	// OnError controls guardrail failure behavior: "block" or "allow".
	OnError string `yaml:"on_error"`
}

// HTTPConfig holds HTTP client configuration for upstream API requests.
// These values are also readable via the HTTP_TIMEOUT and HTTP_RESPONSE_HEADER_TIMEOUT
// environment variables in internal/httpclient/client.go.
type HTTPConfig struct {
	// Timeout is the overall HTTP request timeout in seconds (default: 600)
	Timeout int `yaml:"timeout" env:"HTTP_TIMEOUT"`

	// ResponseHeaderTimeout is the time to wait for response headers in seconds (default: 600)
	ResponseHeaderTimeout int `yaml:"response_header_timeout" env:"HTTP_RESPONSE_HEADER_TIMEOUT"`

	// Proxy configures outbound upstream HTTP(S) requests. Empty values use environment proxy settings.
	Proxy HTTPProxyConfig `yaml:"proxy"`
}

// HTTPProxyConfig holds explicit outbound proxy settings for upstream API requests.
type HTTPProxyConfig struct {
	HTTPProxy        string `yaml:"http_proxy" env:"HTTP_PROXY"`
	HTTPSProxy       string `yaml:"https_proxy" env:"HTTPS_PROXY"`
	NoProxy          string `yaml:"no_proxy" env:"NO_PROXY"`
	ProxyAuthEnabled bool   `yaml:"proxy_auth_enabled"`
	CACertPEM        string `yaml:"ca_cert_pem"`
}

// WorkflowsConfig holds runtime refresh behavior for persisted workflows.
type WorkflowsConfig struct {
	// RefreshInterval controls how often the in-memory workflow snapshot
	// is refreshed from storage. Default: 1m.
	RefreshInterval time.Duration `yaml:"refresh_interval" env:"WORKFLOW_REFRESH_INTERVAL"`
}

// LogConfig holds audit logging configuration
type LogConfig struct {
	// Enabled controls whether audit logging is active
	// Default: false
	Enabled bool `yaml:"enabled" env:"LOGGING_ENABLED"`

	// LogBodies enables logging of full request/response bodies
	// WARNING: May contain sensitive data (PII, API keys in prompts)
	// Default: true
	LogBodies bool `yaml:"log_bodies" env:"LOGGING_LOG_BODIES"`

	// LogHeaders enables logging of request/response headers
	// Sensitive headers (Authorization, Cookie, etc.) are auto-redacted
	// Default: true
	LogHeaders bool `yaml:"log_headers" env:"LOGGING_LOG_HEADERS"`

	// BufferSize is the number of log entries to buffer before flushing
	// Default: 1000
	BufferSize int `yaml:"buffer_size" env:"LOGGING_BUFFER_SIZE"`

	// FlushInterval is how often to flush buffered logs (in seconds)
	// Default: 5
	FlushInterval int `yaml:"flush_interval" env:"LOGGING_FLUSH_INTERVAL"`

	// RetentionDays is how long to keep logs (0 = forever)
	// Default: 30
	RetentionDays int `yaml:"retention_days" env:"LOGGING_RETENTION_DAYS"`

	// OnlyModelInteractions limits audit logging to AI model endpoints only
	// When true, only /v1/chat/completions, /v1/responses, /v1/embeddings, /v1/files, and /v1/batches are logged
	// Endpoints like /health, /metrics, /admin, /v1/models are skipped
	// Default: true
	OnlyModelInteractions bool `yaml:"only_model_interactions" env:"LOGGING_ONLY_MODEL_INTERACTIONS"`
}

// UsageConfig holds token usage tracking configuration
type UsageConfig struct {
	// Enabled controls whether usage tracking is active
	// Default: true
	Enabled bool `yaml:"enabled" env:"USAGE_ENABLED"`

	// EnforceReturningUsageData controls whether to ask streaming providers to return usage data when possible.
	// When true, stream_options: {"include_usage": true} is added for provider paths that support it.
	// Default: true
	EnforceReturningUsageData bool `yaml:"enforce_returning_usage_data" env:"ENFORCE_RETURNING_USAGE_DATA"`

	// PricingRecalculationEnabled controls whether the admin pricing recalculation action is available.
	// Storage and pricing metadata support are still required; false always disables the feature.
	// Default: true
	PricingRecalculationEnabled bool `yaml:"pricing_recalculation_enabled" env:"USAGE_PRICING_RECALCULATION_ENABLED"`

	// BufferSize is the number of usage entries to buffer before flushing
	// Default: 1000
	BufferSize int `yaml:"buffer_size" env:"USAGE_BUFFER_SIZE"`

	// FlushInterval is how often to flush buffered usage entries (in seconds)
	// Default: 5
	FlushInterval int `yaml:"flush_interval" env:"USAGE_FLUSH_INTERVAL"`

	// RetentionDays is how long to keep usage data (0 = forever)
	// Default: 90
	RetentionDays int `yaml:"retention_days" env:"USAGE_RETENTION_DAYS"`
}

// StorageConfig holds database storage configuration (used by audit logging, usage tracking, future IAM, etc.)
type StorageConfig struct {
	// Type specifies the storage backend: "sqlite" (default), "postgresql", or "mongodb"
	Type string `yaml:"type" env:"STORAGE_TYPE"`

	// SQLite configuration
	SQLite SQLiteStorageConfig `yaml:"sqlite"`

	// PostgreSQL configuration
	PostgreSQL PostgreSQLStorageConfig `yaml:"postgresql"`

	// MongoDB configuration
	MongoDB MongoDBStorageConfig `yaml:"mongodb"`
}

// SQLiteStorageConfig holds SQLite-specific storage configuration
type SQLiteStorageConfig struct {
	// Path is the database file path (default: data/aurora.db)
	Path string `yaml:"path" env:"SQLITE_PATH"`
}

// PostgreSQLStorageConfig holds PostgreSQL-specific storage configuration
type PostgreSQLStorageConfig struct {
	// URL is the connection string (e.g., postgres://user:pass@localhost/dbname)
	URL string `yaml:"url" env:"POSTGRES_URL"`
	// MaxConns is the maximum connection pool size (default: 10)
	MaxConns int `yaml:"max_conns" env:"POSTGRES_MAX_CONNS"`
}

// MongoDBStorageConfig holds MongoDB-specific storage configuration
type MongoDBStorageConfig struct {
	// URL is the connection string (e.g., mongodb://localhost:27017)
	URL string `yaml:"url" env:"MONGODB_URL"`
	// Database is the database name (default: aurora)
	Database string `yaml:"database" env:"MONGODB_DATABASE"`
}

// BackendConfig converts the application storage config into the internal storage config.
func (c StorageConfig) BackendConfig() storage.Config {
	cfg := storage.Config{
		Type: c.Type,
		SQLite: storage.SQLiteConfig{
			Path: c.SQLite.Path,
		},
		PostgreSQL: storage.PostgreSQLConfig{
			URL:      c.PostgreSQL.URL,
			MaxConns: c.PostgreSQL.MaxConns,
		},
		MongoDB: storage.MongoDBConfig{
			URL:      c.MongoDB.URL,
			Database: c.MongoDB.Database,
		},
	}
	if cfg.Type == "" {
		cfg.Type = storage.TypeSQLite
	}
	if cfg.SQLite.Path == "" {
		cfg.SQLite.Path = storage.DefaultSQLitePath
	}
	if cfg.MongoDB.Database == "" {
		cfg.MongoDB.Database = "aurora"
	}
	return cfg
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port           string `yaml:"port" env:"PORT"`
	BasePath       string `yaml:"base_path" env:"BASE_PATH"`             // URL path prefix where the app is mounted (e.g., "/g")
	MasterKey      string `yaml:"master_key" env:"AURORA_MASTER_KEY"`    // Optional: Master key for authentication
	BodySizeLimit  string `yaml:"body_size_limit" env:"BODY_SIZE_LIMIT"` // Max request body size (e.g., "10M", "1024K")
	SwaggerEnabled bool   `yaml:"swagger_enabled" env:"SWAGGER_ENABLED"` // Whether to expose the Swagger UI at /swagger/index.html
	PprofEnabled   bool   `yaml:"pprof_enabled" env:"PPROF_ENABLED"`     // Whether to expose debug profiling routes at /debug/pprof/*
	// DisableRequestLogging removes Echo request logging middleware. Useful for
	// high-RPS benchmarks where audit logging is already disabled and logger cost
	// would otherwise hide gateway/provider latency.
	DisableRequestLogging bool `yaml:"disable_request_logging" env:"DISABLE_REQUEST_LOGGING"`
	// DisableRequestBodySnapshot skips eager model-route body capture and
	// white-box prompt derivation. Request IDs and context carriers are still
	// created; handlers read the body on demand. Useful for high-RPS benchmark
	// profiles where audit/cache features are disabled.
	DisableRequestBodySnapshot bool `yaml:"disable_request_body_snapshot" env:"DISABLE_REQUEST_BODY_SNAPSHOT"`
	// DisablePassthroughSemanticEnrichment skips provider-owned passthrough
	// metadata enrichment middleware. Only use when passthrough route semantics
	// are not needed for the deployment or benchmark scenario.
	DisablePassthroughSemanticEnrichment bool `yaml:"disable_passthrough_semantic_enrichment" env:"DISABLE_PASSTHROUGH_SEMANTIC_ENRICHMENT"`
	// MinimalBenchMode enables the safe benchmark hot-path reductions above while
	// keeping auth, routing, and provider behavior explicit and configurable.
	MinimalBenchMode bool `yaml:"minimal_bench_mode" env:"AURORA_MINIMAL_BENCH_MODE"`
	// EnablePassthroughRoutes exposes provider-native passthrough endpoints under
	// /p/{provider}/{endpoint}. Default: true.
	EnablePassthroughRoutes bool `yaml:"enable_passthrough_routes" env:"ENABLE_PASSTHROUGH_ROUTES"`
	// AllowPassthroughV1Alias allows /p/{provider}/v1/... style passthrough routes
	// while keeping /p/{provider}/... as the canonical form. Default: true.
	AllowPassthroughV1Alias bool `yaml:"allow_passthrough_v1_alias" env:"ALLOW_PASSTHROUGH_V1_ALIAS"`
	// EnabledPassthroughProviders lists the provider types enabled on
	// /p/{provider}/... passthrough routes. Default:
	// ["openai", "anthropic", "openrouter", "zai", "vllm"].
	EnabledPassthroughProviders []string `yaml:"enabled_passthrough_providers" env:"ENABLED_PASSTHROUGH_PROVIDERS"`
	// EnableAnthropicIngress exposes the /v1/messages endpoint for native
	// Anthropic-format chat completions, allowing Anthropic SDK clients and
	// Claude Code CLI to route through the gateway.
	EnableAnthropicIngress bool `yaml:"enable_anthropic_ingress" env:"ENABLE_ANTHROPIC_INGRESS"`
}

// NormalizeBasePath canonicalizes the public mount path for the HTTP server.
// Empty, whitespace-only, and "/" all resolve to root.
func NormalizeBasePath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "/" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	normalized := path.Clean(trimmed)
	if normalized == "." || normalized == "/" {
		return "/"
	}
	return normalized
}

// JoinBasePath prefixes urlPath with the normalized public mount path.
func JoinBasePath(basePath, urlPath string) string {
	basePath = NormalizeBasePath(basePath)
	trimmedPath := strings.TrimSpace(urlPath)
	if trimmedPath == "" || trimmedPath == "/" {
		if basePath == "/" {
			return "/"
		}
		return basePath
	}
	if !strings.HasPrefix(trimmedPath, "/") {
		trimmedPath = "/" + trimmedPath
	}
	if basePath == "/" {
		return trimmedPath
	}
	return basePath + trimmedPath
}

// MetricsConfig holds observability configuration for Prometheus metrics
type MetricsConfig struct {
	// Enabled controls whether Prometheus metrics are collected and exposed
	// Default: false
	Enabled bool `yaml:"enabled" env:"METRICS_ENABLED"`

	// Endpoint is the HTTP path where metrics are exposed
	// Default: "/metrics"
	Endpoint string `yaml:"endpoint" env:"METRICS_ENDPOINT"`
}



// RetryConfig holds resolved retry settings for an LLM client.
// This is the canonical type shared between config and llmclient.
type RetryConfig struct {
	MaxRetries     int           `yaml:"max_retries"     env:"RETRY_MAX_RETRIES"`
	InitialBackoff time.Duration `yaml:"initial_backoff" env:"RETRY_INITIAL_BACKOFF"`
	MaxBackoff     time.Duration `yaml:"max_backoff"     env:"RETRY_MAX_BACKOFF"`
	BackoffFactor  float64       `yaml:"backoff_factor"  env:"RETRY_BACKOFF_FACTOR"`
	JitterFactor   float64       `yaml:"jitter_factor"   env:"RETRY_JITTER_FACTOR"`
}

// DefaultRetryConfig returns the default retry settings.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
		JitterFactor:   0.1,
	}
}

// CircuitBreakerConfig holds resolved circuit breaker settings.
// This is the canonical type shared between config and llmclient.
type CircuitBreakerConfig struct {
	FailureThreshold int           `yaml:"failure_threshold" env:"CIRCUIT_BREAKER_FAILURE_THRESHOLD"`
	SuccessThreshold int           `yaml:"success_threshold" env:"CIRCUIT_BREAKER_SUCCESS_THRESHOLD"`
	Timeout          time.Duration `yaml:"timeout"           env:"CIRCUIT_BREAKER_TIMEOUT"`
}

// DefaultCircuitBreakerConfig returns the default circuit breaker settings.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

// ResilienceConfig holds resolved resilience settings (retry and circuit breaker).
type ResilienceConfig struct {
	Retry          RetryConfig          `yaml:"retry"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
}

// buildDefaultConfig returns the single source of truth for all configuration defaults.
func buildDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:                    "8080",
			BasePath:                "/",
			SwaggerEnabled:          false,
			PprofEnabled:            false,
			DisableRequestLogging: true,
			EnablePassthroughRoutes: true,
			EnableAnthropicIngress:  false,
			AllowPassthroughV1Alias: true,
			EnabledPassthroughProviders: []string{
				"openai",
				"anthropic",
				"openrouter",
				"zai",
				"vllm",
			},
		},
		Models: ModelsConfig{
			EnabledByDefault:                true,
			OverridesEnabled:                true,
			KeepOnlyAliasesAtModelsEndpoint: false,
			ConfiguredProviderModelsMode:    ConfiguredProviderModelsModeFallback,
		},
		Cache: CacheConfig{
			Prompt: defaultPromptCacheConfig(),
			Model: ModelCacheConfig{
				RefreshInterval: 3600,
				ModelList: ModelListConfig{
					URL:               "",
					LocalPath:         "data/models.local.json",
					UserOverridesPath: "data/user_pricing.yaml",
				},
				Local: nil,
				Redis: nil,
			},
			Response: ResponseCacheConfig{},
		},
		Storage: StorageConfig{
			Type: "sqlite",
			SQLite: SQLiteStorageConfig{
				Path: storage.DefaultSQLitePath,
			},
			PostgreSQL: PostgreSQLStorageConfig{
				MaxConns: 10,
			},
			MongoDB: MongoDBStorageConfig{
				Database: "aurora",
			},
		},
		Logging: LogConfig{
			LogBodies:             true,
			LogHeaders:            true,
			BufferSize:            1000,
			FlushInterval:         5,
			RetentionDays:         30,
			OnlyModelInteractions: true,
		},
		Usage: UsageConfig{
			EnforceReturningUsageData:   true,
			PricingRecalculationEnabled: true,
			BufferSize:                  1000,
			FlushInterval:               5,
			RetentionDays:               90,
		},
		Metrics: MetricsConfig{
			Endpoint: "/metrics",
		},
		HTTP: HTTPConfig{
			Timeout:               600,
			ResponseHeaderTimeout: 600,
		},
		Fallback: FallbackConfig{
			DefaultMode: FallbackModeManual,
		},
		Combos: CombosConfig{
			Enabled: false,
		},
		CLITools: CLIToolsConfig{
			Enabled: false,
		},
		Workflows: WorkflowsConfig{
			RefreshInterval: time.Minute,
		},
		Resilience: ResilienceConfig{
			Retry:          DefaultRetryConfig(),
			CircuitBreaker: DefaultCircuitBreakerConfig(),
		},
		Admin:      AdminConfig{EndpointsEnabled: false, UIEnabled: false},
		Guardrails: GuardrailsConfig{},
		TokenSaver: defaultTokenSaverConfig(),
		Edition: EditionConfig{
			Name: EditionOSS,
		},
	}
}

// Load reads configuration from file and environment using a three-layer pipeline:
//
//	defaults (code) -> configs/config.yaml (optional overlay) -> env vars (always win)
//
// The returned LoadResult contains the resolved application Config and the raw
// provider map parsed from YAML. Provider env var discovery, credential filtering,
// and resilience merging are handled by the providers package.
func Load() (*LoadResult, error) {
	cfg := buildDefaultConfig()

	rawProviders, rawPools, err := applyYAML(cfg)
	if err != nil {
		return nil, err
	}

	if err := applyResponseSimpleEnv(&cfg.Cache.Response); err != nil {
		return nil, err
	}
	if err := applyResponseSemanticEnv(&cfg.Cache.Response); err != nil {
		return nil, err
	}
	mergeSemanticResponseDefaults(cfg.Cache.Response.Semantic)

	if err := applyEnvOverrides(cfg); err != nil {
		return nil, err
	}
	cfg.Edition.Name = NormalizeEditionName(cfg.Edition.Name)
	cfg.Server.BasePath = NormalizeBasePath(cfg.Server.BasePath)
	cfg.Models.ConfiguredProviderModelsMode = ResolveConfiguredProviderModelsMode(cfg.Models.ConfiguredProviderModelsMode)
	if !cfg.Models.ConfiguredProviderModelsMode.Valid() {
		return nil, fmt.Errorf("models.configured_provider_models_mode must be one of: fallback, allowlist")
	}

	if err := loadFallbackConfig(&cfg.Fallback); err != nil {
		return nil, err
	}

	// When no model cache backend was specified at all, default to local.
	if cfg.Cache.Model.Local == nil && cfg.Cache.Model.Redis == nil {
		cfg.Cache.Model.Local = &LocalCacheConfig{}
	}

	if cfg.Server.BodySizeLimit != "" {
		if err := ValidateBodySizeLimit(cfg.Server.BodySizeLimit); err != nil {
			return nil, fmt.Errorf("invalid BODY_SIZE_LIMIT: %w", err)
		}
	}

	if err := ValidateCacheConfig(&cfg.Cache); err != nil {
		return nil, err
	}
	if err := ValidateTokenSaverConfig(&cfg.TokenSaver); err != nil {
		return nil, err
	}

	return &LoadResult{
		Config:       cfg,
		RawProviders: rawProviders,
		RawPools:     rawPools,
	}, nil
}

// applyYAML reads an optional runtime config and overlays it onto cfg.
// Returns the raw provider map and the raw pool map parsed from the
// providers: and pools: YAML sections respectively. If no config file is
// found, this is a no-op (not an error).
func applyYAML(cfg *Config) (map[string]RawProviderConfig, map[string]RawPoolConfig, error) {
	paths := []string{
		"configs/config.yaml",
		"config.yaml",
	}
	customPath := strings.TrimSpace(os.Getenv("AURORA_CONFIG_PATH"))
	if customPath != "" {
		paths = []string{customPath}
	}

	var data []byte
	var configPath string
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err == nil {
			data = raw
			configPath = p
			break
		}
		if customPath != "" {
			return nil, nil, fmt.Errorf("failed to read config file %s: %w", p, err)
		}
	}

	rawProviders := make(map[string]RawProviderConfig)
	rawPools := make(map[string]RawPoolConfig)

	if data == nil {
		return rawProviders, rawPools, nil
	}

	expanded := expandString(string(data))

	// yamlTarget is a local struct that mirrors Config for YAML unmarshaling,
	// using RawProviderConfig for providers so nullable resilience overrides are preserved.
	type yamlTarget struct {
		*Config      `yaml:",inline"`
		RawProviders map[string]RawProviderConfig `yaml:"providers"`
		RawPools     map[string]RawPoolConfig     `yaml:"pools"`
	}

	target := yamlTarget{Config: cfg}
	if err := yaml.Unmarshal([]byte(expanded), &target); err != nil {
		return nil, nil, fmt.Errorf("failed to parse %s: %w", configPath, err)
	}

	if target.RawProviders != nil {
		rawProviders = target.RawProviders
	}
	if target.RawPools != nil {
		rawPools = target.RawPools
	}

	for _, overlayPath := range dashboardOverridePaths(customPath) {
		overlayRaw, err := os.ReadFile(overlayPath)
		if err != nil {
			continue
		}
		overlayExpanded := expandString(string(overlayRaw))
		overlayTarget := yamlTarget{Config: cfg}
		if err := yaml.Unmarshal([]byte(overlayExpanded), &overlayTarget); err != nil {
			return nil, nil, fmt.Errorf("failed to parse %s: %w", overlayPath, err)
		}
		if overlayTarget.RawProviders != nil {
			rawProviders = overlayTarget.RawProviders
		}
		if overlayTarget.RawPools != nil {
			rawPools = overlayTarget.RawPools
		}
		break
	}

	return rawProviders, rawPools, nil
}

// DashboardOverridesPath returns the admin-managed config overlay path.
// When AURORA_CONFIG_PATH is set, the overlay lives next to that file.
// Otherwise it defaults to configs/dashboard-overrides.yaml.
func DashboardOverridesPath() string {
	customPath := strings.TrimSpace(os.Getenv("AURORA_CONFIG_PATH"))
	if customPath != "" {
		return filepath.Join(filepath.Dir(customPath), "dashboard-overrides.yaml")
	}
	return filepath.Join("configs", "dashboard-overrides.yaml")
}

func dashboardOverridePaths(customPath string) []string {
	customPath = strings.TrimSpace(customPath)
	if customPath != "" {
		return []string{filepath.Join(filepath.Dir(customPath), "dashboard-overrides.yaml")}
	}
	return []string{
		filepath.Join("configs", "dashboard-overrides.yaml"),
		"dashboard-overrides.yaml",
	}
}



func validatePublicHTTPURL(raw, field string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", field)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("%s must use http or https", field)
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return fmt.Errorf("%s host is required", field)
	}
	if ip := net.ParseIP(host); ip != nil && !isPublicIP(ip) {
		return fmt.Errorf("%s must not target private or loopback addresses", field)
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified())
}

// ValidateBodySizeLimit validates a body size limit string.
// Accepts formats like: "10M", "10MB", "1024K", "1024KB", "104857600"
// Returns an error if the format is invalid or value is outside bounds (1KB - 100MB).
func ValidateBodySizeLimit(s string) error {
	_, err := ParseBodySizeLimitBytes(s)
	return err
}

// ParseBodySizeLimitBytes parses a configured body size limit into bytes.
// Accepts formats like: "10M", "10MB", "1024K", "1024KB", "104857600".
// Returns an error if the format is invalid or value is outside bounds (1KB - 100MB).
func ParseBodySizeLimitBytes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	matches := bodySizeLimitRegex.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid format %q: expected pattern like '10M', '1024K', or '104857600'", s)
	}

	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number in %q: %w", s, err)
	}

	switch strings.ToUpper(matches[2]) {
	case "K":
		value *= 1024
	case "M":
		value *= 1024 * 1024
	case "G":
		value *= 1024 * 1024 * 1024
	}

	if value < MinBodySizeLimit {
		return 0, fmt.Errorf("value %d bytes is below minimum of %d bytes (1KB)", value, MinBodySizeLimit)
	}
	if value > MaxBodySizeLimit {
		return 0, fmt.Errorf("value %d bytes exceeds maximum of %d bytes (100MB)", value, MaxBodySizeLimit)
	}

	return value, nil
}

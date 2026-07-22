package config

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"
)

type EnvVarSchema struct {
	Name        string `json:"name"`
	Default     string `json:"default,omitempty"`
	Type        string `json:"type,omitempty"`
	Section     string `json:"section,omitempty"`
	Description string `json:"description,omitempty"`
}

var sectionNames = map[string]string{
	"Server":               "Server",
	"Models":               "Models",
	"Model":                "Model Cache",
	"Cache":                "Cache",
	"Storage":              "Storage",
	"Logging":              "Audit Logging",
	"Usage":                "Usage Tracking",
	"Budgets":              "Budgets",
	"Metrics":              "Metrics",
	"ObservabilityExports": "Observability",
	"Cluster":              "Cluster",
	"Compliance":           "Compliance",
	"HTTP":                 "HTTP Client",
	"Admin":                "Admin API",
	"Guardrails":           "Guardrails",
	"TokenSaver":           "Token Saver",
	"Fallback":             "Fallback",
	"Combos":               "Combos",
	"CLITools":             "CLI Tools",
	"Workflows":            "Workflows",
	"Resilience":           "Resilience",
	"Identity":             "Identity",
	"ModelCache":           "Model Cache",
	"ModelList":            "Model Registry",
	"RedisModel":           "Redis Model Cache",
	"PostgreSQL":           "PostgreSQL Storage",
	"MongoDB":              "MongoDB Storage",
	"SQLite":               "SQLite Storage",
	"Retry":                "Retry",
	"CircuitBreaker":       "Circuit Breaker",
	"Output":               "Token Saver Output",
	"Audit":                "Token Saver Audit",
	"ModelsScope":          "Token Saver Models",
	"ProvidersScope":       "Token Saver Providers",
	"HTTPProxy":            "HTTP Proxy",
}

func sectionName(fieldName string) string {
	if s, ok := sectionNames[fieldName]; ok {
		return s
	}
	return fieldName
}

var knownDescriptions = map[string]string{
	"PORT":                              "HTTP listen port",
	"BASE_PATH":                         "URL path prefix where the gateway is mounted",
	"AURORA_MASTER_KEY":                 "Gateway & admin API security key (set before exposing outside localhost)",
	"BODY_SIZE_LIMIT":                   "Maximum request body size (e.g. 10M, 1G, 500K)",
	"SWAGGER_ENABLED":                   "Expose Swagger UI at /swagger/index.html",
	"PPROF_ENABLED":                     "Expose pprof profiling routes at /debug/pprof/*",
	"ENABLE_PASSTHROUGH_ROUTES":         "Expose provider-native passthrough routes under /p/{provider}/{endpoint}",
	"ALLOW_PASSTHROUGH_V1_ALIAS":        "Allow /p/{provider}/v1/... aliases alongside canonical /p/{provider}/... routes",
	"ENABLED_PASSTHROUGH_PROVIDERS":     "Comma-separated provider types enabled for passthrough routes",
	"ENABLE_ANTHROPIC_INGRESS":          "Expose /v1/messages for native Anthropic-format clients",
	"DISABLE_REQUEST_LOGGING":           "Disable Echo request logging middleware",
	"DISABLE_REQUEST_BODY_SNAPSHOT":     "Skip eager model-route body capture",
	"DISABLE_PASSTHROUGH_SEMANTIC_ENRICHMENT": "Skip provider-owned passthrough metadata enrichment",
	"AURORA_MINIMAL_BENCH_MODE":         "Enable safe benchmark hot-path reductions",
	"MODELS_ENABLED_BY_DEFAULT":         "Process-wide default for provider model availability",
	"MODEL_OVERRIDES_ENABLED":           "Enable persisted model overrides and dashboard editing",
	"KEEP_ONLY_ALIASES_AT_MODELS_ENDPOINT": "Expose only enabled aliases at GET /v1/models",
	"CONFIGURED_PROVIDER_MODELS_MODE":   "How configured provider model lists affect inventory: fallback or allowlist",
	"ADMIN_ENDPOINTS_ENABLED":           "Enable /admin/api/v1/* REST endpoints",
	"ADMIN_UI_ENABLED":                  "Enable /admin/dashboard UI",

	"GUARDRAILS_ENABLED":                "Enable configured guardrails globally",
	"ENABLE_GUARDRAILS_FOR_BATCH_PROCESSING": "Apply guardrails to inline /v1/batches request items",
	"LOGGING_ENABLED":                   "Enable audit logging to configured storage",
	"LOGGING_LOG_BODIES":                "Log full request/response bodies (may contain PII)",
	"LOGGING_LOG_HEADERS":               "Log request/response headers (sensitive headers redacted)",
	"LOGGING_BUFFER_SIZE":               "In-memory audit log queue capacity",
	"LOGGING_FLUSH_INTERVAL":            "Audit log flush interval in seconds",
	"LOGGING_RETENTION_DAYS":            "Auto-delete audit logs older than N days (0 = forever)",
	"LOGGING_ONLY_MODEL_INTERACTIONS":   "Log only model interactions, skipping /health, /metrics, /admin",
	"USAGE_ENABLED":                     "Enable token usage tracking",
	"ENFORCE_RETURNING_USAGE_DATA":      "Add stream_options.include_usage=true to streaming requests",
	"USAGE_PRICING_RECALCULATION_ENABLED": "Enable admin usage pricing recalculation action",
	"USAGE_BUFFER_SIZE":                 "In-memory usage queue capacity",
	"USAGE_FLUSH_INTERVAL":              "Usage flush interval in seconds",
	"USAGE_RETENTION_DAYS":              "Auto-delete usage data older than N days (0 = forever)",
	"STORAGE_TYPE":                      "Storage backend: sqlite, postgresql, or mongodb",
	"SQLITE_PATH":                       "SQLite database file path",
	"POSTGRES_URL":                      "PostgreSQL connection string",
	"POSTGRES_MAX_CONNS":                "PostgreSQL max connection pool size",
	"MONGODB_URL":                       "MongoDB connection string",
	"MONGODB_DATABASE":                  "MongoDB database name",
	"METRICS_ENABLED":                   "Enable Prometheus metrics collection and /metrics endpoint",
	"METRICS_ENDPOINT":                  "Metrics endpoint path",
	"HTTP_TIMEOUT":                      "Overall upstream request timeout in seconds",
	"HTTP_RESPONSE_HEADER_TIMEOUT":      "Time to wait for upstream response headers in seconds",
	"HTTP_PROXY":                        "HTTP proxy for outbound upstream requests",
	"HTTPS_PROXY":                       "HTTPS proxy for outbound upstream requests",
	"NO_PROXY":                          "Comma-separated hosts to bypass proxy",
	"WORKFLOW_REFRESH_INTERVAL":         "How often to refresh persisted workflows from storage",
	"FEATURE_FALLBACK_MODE":             "Default translated-route fallback mode: auto, manual, or off",
	"FALLBACK_MANUAL_RULES_PATH":        "Path to manual fallback rules JSON file",
	"TOKEN_SAVER_ENABLED":               "Enable concise-output/token-saving transforms",
	"TOKEN_SAVER_ENDPOINTS":             "Endpoints to apply token saver to",
	"TOKEN_SAVER_APPLY_STREAMING":       "Apply token saver to streaming responses",
	"TOKEN_SAVER_ON_ERROR":              "Token saver behavior on error: allow or block",
	"TOKEN_SAVER_EMIT_HEADERS":          "Emit token saver headers in responses",
	"TOKEN_SAVER_OUTPUT_ENABLED":        "Enable token saver output style",
	"TOKEN_SAVER_OUTPUT_PROFILE":        "Token saver output profile",
	"TOKEN_SAVER_OUTPUT_LEVEL":          "Token saver output level: lite, full, ultra, wenyan",
	"TOKEN_SAVER_MODELS_INCLUDE":        "Token saver model include filter",
	"TOKEN_SAVER_MODELS_EXCLUDE":        "Token saver model exclude filter",
	"TOKEN_SAVER_PROVIDERS_INCLUDE":     "Token saver provider include filter",
	"TOKEN_SAVER_PROVIDERS_EXCLUDE":     "Token saver provider exclude filter",
	"TOKEN_SAVER_AUDIT_ENABLED":         "Enable token saver audit logging",
	"RETRY_MAX_RETRIES":                 "Retry attempts for upstream provider calls",
	"RETRY_INITIAL_BACKOFF":             "Initial retry backoff duration",
	"RETRY_MAX_BACKOFF":                 "Maximum retry backoff duration",
	"RETRY_BACKOFF_FACTOR":              "Retry backoff multiplier",
	"RETRY_JITTER_FACTOR":               "Retry jitter factor",
	"CIRCUIT_BREAKER_FAILURE_THRESHOLD": "Circuit breaker failure count threshold",
	"CIRCUIT_BREAKER_SUCCESS_THRESHOLD": "Circuit breaker success count threshold to half-open",
	"CIRCUIT_BREAKER_TIMEOUT":           "Circuit breaker open state timeout",
	"CACHE_REFRESH_INTERVAL":            "How often to refresh model registry cache in seconds",
	"AURORA_CACHE_DIR":                  "Local filesystem cache directory for model metadata",
	"MODEL_LIST_URL":                    "External model metadata registry URL",
	"MODEL_LIST_LOCAL_PATH":             "Local model registry snapshot path",
	"MODEL_LIST_USER_OVERRIDES_PATH":    "User pricing/model overrides YAML path",
	"REDIS_URL":                         "Redis connection string",
	"REDIS_KEY_MODELS":                  "Redis key prefix for model cache",
	"REDIS_TTL_MODELS":                  "Redis TTL for model cache in seconds",
	"RESPONSE_CACHE_SIMPLE_ENABLED":     "Enable Redis exact response cache",
	"REDIS_KEY_RESPONSES":               "Redis key prefix for response cache",
	"REDIS_TTL_RESPONSES":               "Redis TTL for response cache in seconds",
	"SEMANTIC_CACHE_ENABLED":            "Enable embedding + vector-store semantic response cache",
	"SEMANTIC_CACHE_THRESHOLD":          "Semantic cache similarity threshold (0-1)",
	"SEMANTIC_CACHE_PROMPT_SIMILARITY":  "Semantic cache prompt similarity threshold (0-1)",
	"SEMANTIC_CACHE_TTL":                "Semantic cache entry TTL in seconds",
	"SEMANTIC_CACHE_MAX_CONV_MESSAGES":  "Recent conversation messages to include in semantic cache key",
	"SEMANTIC_CACHE_EXCLUDE_SYSTEM_PROMPT": "Exclude system prompt from semantic cache keys",
	"SEMANTIC_CACHE_EMBEDDER_PROVIDER":  "Embedder provider for semantic cache",
	"SEMANTIC_CACHE_EMBEDDER_MODEL":     "Embedder model for semantic cache",
	"SEMANTIC_CACHE_VECTOR_STORE_TYPE":  "Vector store backend: qdrant, pgvector, pinecone, weaviate",
	"SEMANTIC_CACHE_QDRANT_URL":         "Qdrant URL for semantic cache",
	"SEMANTIC_CACHE_QDRANT_COLLECTION":  "Qdrant collection for semantic cache",
	"SEMANTIC_CACHE_QDRANT_API_KEY":     "Qdrant API key for semantic cache",
	"SEMANTIC_CACHE_PGVECTOR_URL":       "pgvector connection string for semantic cache",
	"SEMANTIC_CACHE_PGVECTOR_TABLE":     "pgvector table for semantic cache",
	"SEMANTIC_CACHE_PGVECTOR_DIMENSION": "pgvector embedding dimension",
	"SEMANTIC_CACHE_PINECONE_HOST":      "Pinecone host URL",
	"SEMANTIC_CACHE_PINECONE_API_KEY":   "Pinecone API key",
	"SEMANTIC_CACHE_PINECONE_NAMESPACE": "Pinecone namespace",
	"SEMANTIC_CACHE_PINECONE_DIMENSION": "Pinecone embedding dimension",
	"SEMANTIC_CACHE_WEAVIATE_URL":       "Weaviate URL",
	"SEMANTIC_CACHE_WEAVIATE_CLASS":     "Weaviate class name",
	"SEMANTIC_CACHE_WEAVIATE_API_KEY":   "Weaviate API key",
	"COMBOS_ENABLED":                    "Enable combo model calls/workflows",
	"CLI_TOOLS_ENABLED":                 "Enable CLI tools integration",
	"CLI_TOOLS_APPLY_ENABLED":           "Allow admin/API to apply CLI tool changes",
	"BUDGETS_ENABLED":                   "Enable budget enforcement",
}

var manualCacheEnvVars = []EnvVarSchema{
	{Name: "RESPONSE_CACHE_SIMPLE_ENABLED", Type: "bool", Default: "false", Section: "Response Cache", Description: "Enable Redis exact response cache"},
	{Name: "REDIS_KEY_RESPONSES", Type: "string", Default: "aurora:response:", Section: "Response Cache", Description: "Redis key prefix for response cache"},
	{Name: "REDIS_TTL_RESPONSES", Type: "int", Default: "3600", Section: "Response Cache", Description: "Redis TTL for response cache in seconds"},
	{Name: "SEMANTIC_CACHE_ENABLED", Type: "bool", Default: "false", Section: "Semantic Cache", Description: "Enable embedding + vector-store semantic response cache"},
	{Name: "SEMANTIC_CACHE_THRESHOLD", Type: "float", Default: "0.92", Section: "Semantic Cache", Description: "Semantic cache similarity threshold (0-1)"},
	{Name: "SEMANTIC_CACHE_PROMPT_SIMILARITY", Type: "float", Default: "0.72", Section: "Semantic Cache", Description: "Semantic cache prompt similarity threshold (0-1)"},
	{Name: "SEMANTIC_CACHE_TTL", Type: "int", Default: "3600", Section: "Semantic Cache", Description: "Semantic cache entry TTL in seconds"},
	{Name: "SEMANTIC_CACHE_MAX_CONV_MESSAGES", Type: "int", Default: "3", Section: "Semantic Cache", Description: "Recent conversation messages to include in semantic cache key"},
	{Name: "SEMANTIC_CACHE_EXCLUDE_SYSTEM_PROMPT", Type: "bool", Default: "false", Section: "Semantic Cache", Description: "Exclude system prompt from semantic cache keys"},
	{Name: "SEMANTIC_CACHE_EMBEDDER_PROVIDER", Type: "string", Default: "openai", Section: "Semantic Cache", Description: "Embedder provider for semantic cache"},
	{Name: "SEMANTIC_CACHE_EMBEDDER_MODEL", Type: "string", Default: "text-embedding-3-small", Section: "Semantic Cache", Description: "Embedder model for semantic cache"},
	{Name: "SEMANTIC_CACHE_VECTOR_STORE_TYPE", Type: "string", Default: "qdrant", Section: "Semantic Cache", Description: "Vector store backend: qdrant, pgvector, pinecone, weaviate"},
	{Name: "SEMANTIC_CACHE_QDRANT_URL", Type: "string", Default: "http://localhost:6333", Section: "Semantic Cache", Description: "Qdrant URL for semantic cache"},
	{Name: "SEMANTIC_CACHE_QDRANT_COLLECTION", Type: "string", Default: "aurora_semantic", Section: "Semantic Cache", Description: "Qdrant collection for semantic cache"},
	{Name: "SEMANTIC_CACHE_QDRANT_API_KEY", Type: "string", Section: "Semantic Cache", Description: "Qdrant API key for semantic cache"},
	{Name: "SEMANTIC_CACHE_PGVECTOR_URL", Type: "string", Section: "Semantic Cache", Description: "pgvector connection string for semantic cache"},
	{Name: "SEMANTIC_CACHE_PGVECTOR_TABLE", Type: "string", Default: "aurora_semantic_cache", Section: "Semantic Cache", Description: "pgvector table for semantic cache"},
	{Name: "SEMANTIC_CACHE_PGVECTOR_DIMENSION", Type: "int", Default: "1536", Section: "Semantic Cache", Description: "pgvector embedding dimension"},
	{Name: "SEMANTIC_CACHE_PINECONE_HOST", Type: "string", Section: "Semantic Cache", Description: "Pinecone host URL"},
	{Name: "SEMANTIC_CACHE_PINECONE_API_KEY", Type: "string", Section: "Semantic Cache", Description: "Pinecone API key"},
	{Name: "SEMANTIC_CACHE_PINECONE_NAMESPACE", Type: "string", Section: "Semantic Cache", Description: "Pinecone namespace"},
	{Name: "SEMANTIC_CACHE_PINECONE_DIMENSION", Type: "int", Default: "1536", Section: "Semantic Cache", Description: "Pinecone embedding dimension"},
	{Name: "SEMANTIC_CACHE_WEAVIATE_URL", Type: "string", Section: "Semantic Cache", Description: "Weaviate URL"},
	{Name: "SEMANTIC_CACHE_WEAVIATE_CLASS", Type: "string", Default: "AuroraSemanticCache", Section: "Semantic Cache", Description: "Weaviate class name"},
	{Name: "SEMANTIC_CACHE_WEAVIATE_API_KEY", Type: "string", Section: "Semantic Cache", Description: "Weaviate API key"},
}

type providerInfoExtended struct {
	Type            string
	HasVersion      bool
	KeyOptional     bool
	RequiresBaseURL bool
	Desc            string
}

var knownProvidersExtended = []providerInfoExtended{
	{Type: "openai", Desc: "OpenAI API"},
	{Type: "anthropic", Desc: "Anthropic Claude API"},
	{Type: "gemini", Desc: "Google Gemini API (OpenAI-compatible endpoint)"},
	{Type: "deepseek", Desc: "DeepSeek API"},
	{Type: "groq", Desc: "Groq API"},
	{Type: "openrouter", Desc: "OpenRouter API"},
	{Type: "zai", Desc: "Z.ai API"},
	{Type: "xai", Desc: "xAI (Grok) API"},
	{Type: "minimax", Desc: "MiniMax API"},
	{Type: "azure", Desc: "Azure OpenAI API", HasVersion: true},
	{Type: "oracle", Desc: "Oracle OCI Generative AI", RequiresBaseURL: true},
	{Type: "ollama", Desc: "Ollama local LLM server", KeyOptional: true},
	{Type: "vllm", Desc: "vLLM OpenAI-compatible server", KeyOptional: true},
}

var providerDefaultURLs = map[string]string{
	"openai":    "https://api.openai.com/v1",
	"anthropic": "https://api.anthropic.com/v1",
	"gemini":    "https://generativelanguage.googleapis.com/v1beta/openai",
	"deepseek":  "https://api.deepseek.com",
	"groq":      "https://api.groq.com/openai/v1",
	"openrouter": "https://openrouter.ai/api/v1",
	"zai":       "https://api.z.ai/api/paas/v4",
	"xai":       "https://api.x.ai/v1",
	"minimax":   "https://api.minimax.io/v1",
	"ollama":    "http://localhost:11434/v1",
	"vllm":      "http://localhost:8000/v1",
}

func providerEnvPrefix(providerType string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range providerType {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToUpper(r))
			lastUnderscore = false
		case !lastUnderscore:
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func providerEnvVars() []EnvVarSchema {
	var vars []EnvVarSchema
	for _, p := range knownProvidersExtended {
		prefix := providerEnvPrefix(p.Type)
		section := "Provider: " + p.Desc

		vars = append(vars, EnvVarSchema{
			Name:    prefix + "_API_KEY",
			Type:    "string",
			Section: section,
		})
		vars = append(vars, EnvVarSchema{
			Name:    prefix + "_BASE_URL",
			Type:    "string",
			Default: providerDefaultURLs[p.Type],
			Section: section,
		})
		if p.HasVersion {
			vars = append(vars, EnvVarSchema{
				Name:    prefix + "_API_VERSION",
				Type:    "string",
				Default: "2024-10-21",
				Section: section,
			})
		}
		vars = append(vars, EnvVarSchema{
			Name:    prefix + "_MODELS",
			Type:    "string",
			Section: section,
		})
	}
	return vars
}

func GetEnvSchema() []EnvVarSchema {
	defaults := buildDefaultConfig()
	defVal := reflect.ValueOf(defaults).Elem()

	var vars []EnvVarSchema
	walkStructForEnv(defVal, &vars, "")

	seen := make(map[string]bool)
	for _, v := range vars {
		seen[v.Name] = true
	}
	for _, v := range manualCacheEnvVars {
		if !seen[v.Name] {
			vars = append(vars, v)
			seen[v.Name] = true
		}
	}
	for _, v := range providerEnvVars() {
		if !seen[v.Name] {
			vars = append(vars, v)
			seen[v.Name] = true
		}
	}

	sort.Slice(vars, func(i, j int) bool {
		if vars[i].Section != vars[j].Section {
			if vars[i].Section == "" {
				return false
			}
			if vars[j].Section == "" {
				return true
			}
			return vars[i].Section < vars[j].Section
		}
		return vars[i].Name < vars[j].Name
	})

	return vars
}

func walkStructForEnv(v reflect.Value, vars *[]EnvVarSchema, parentSection string) {
	t := v.Type()
	if t.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		walkStructForEnv(v.Elem(), vars, parentSection)
		return
	}
	if t.Kind() != reflect.Struct {
		return
	}

	section := parentSection

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldVal := v.Field(i)
		envKey := field.Tag.Get("env")

		currentSection := section
		if currentSection == "" {
			currentSection = sectionName(field.Name)
		} else if sectionName(field.Name) != field.Name {
			currentSection = sectionName(field.Name)
		}

		if envKey != "" {
			envVar := EnvVarSchema{
				Name:    envKey,
				Type:    field.Type.String(),
				Section: section,
			}
			if d, ok := knownDescriptions[envKey]; ok {
				envVar.Description = d
			}

			switch field.Type.Kind() {
			case reflect.String:
				envVar.Default = fieldVal.String()
			case reflect.Bool:
				envVar.Default = fmt.Sprintf("%v", fieldVal.Bool())
			case reflect.Int, reflect.Int64:
				if field.Type == reflect.TypeFor[time.Duration]() {
					d := time.Duration(fieldVal.Int())
					envVar.Default = d.String()
				} else {
					envVar.Default = fmt.Sprintf("%d", fieldVal.Int())
				}
			case reflect.Float64:
				envVar.Default = fmt.Sprintf("%v", fieldVal.Float())
			case reflect.Slice:
				if field.Type.Elem().Kind() == reflect.String {
					if fieldVal.Len() > 0 {
						items := make([]string, fieldVal.Len())
						for j := 0; j < fieldVal.Len(); j++ {
							items[j] = fieldVal.Index(j).String()
						}
						envVar.Default = strings.Join(items, ",")
					} else {
						envVar.Default = "[]"
					}
				}
			}
			*vars = append(*vars, envVar)
		}

		ft := field.Type
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			fv := fieldVal
			if fv.Kind() == reflect.Ptr {
				if fv.IsNil() {
					continue
				}
				fv = fv.Elem()
			}
			subSection := currentSection
			if envKey == "" {
				mapped := sectionName(field.Name)
				if mapped != field.Name {
					subSection = mapped
				}
			}
			walkStructForEnv(fv, vars, subSection)
		}
	}
}

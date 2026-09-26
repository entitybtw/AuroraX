# Environment variables

Complete reference of every environment variable the gateway reads.

Load priority (later wins):

```
code defaults → config.yaml → .env / environment variables
```

Generate a documented `config.yaml` with `aurora init`. Secrets belong in your
`.env` file (gitignored) or your deployment secrets manager — never commit them.

---

<details>
<summary>Server & Security</summary>

| Env var | Default | Description |
|---------|---------|-------------|
| `PORT` | `8080` | HTTP listening port |
| `BASE_PATH` | `/` | URL path prefix to mount under |
| `AURORA_MASTER_KEY` | `""` | Master API key for auth |
| `BODY_SIZE_LIMIT` | `10M` | Max request body size |
| `SWAGGER_ENABLED` | `false` | Enable Swagger UI at `/swagger/index.html` |
| `PPROF_ENABLED` | `false` | Enable pprof at `/debug/pprof/` |
| `ENABLE_PASSTHROUGH_ROUTES` | `true` | Provider-native passthrough at `/p/{provider}` |
| `ALLOW_PASSTHROUGH_V1_ALIAS` | `true` | Allow `/p/{provider}/v1/...` alias routes |
| `ENABLED_PASSTHROUGH_PROVIDERS` | `openai,anthropic,openrouter,zai,vllm` | Provider types for passthrough |
| `ENABLE_ANTHROPIC_INGRESS` | `false` | Expose `/v1/messages` for native Anthropic clients |
| `DISABLE_REQUEST_LOGGING` | `false` | Turn off request logging |
| `DISABLE_REQUEST_BODY_SNAPSHOT` | `false` | Don't snapshot request bodies |
| `DISABLE_PASSTHROUGH_SEMANTIC_ENRICHMENT` | `false` | Disable semantic enrichment on passthrough |
</details>

<details>
<summary>HTTP Client & Proxy</summary>

| Env var | Default | Description |
|---------|---------|-------------|
| `HTTP_TIMEOUT` | `600` | Upstream request timeout (seconds) |
| `HTTP_RESPONSE_HEADER_TIMEOUT` | `600` | Timeout for upstream response headers |
| `HTTP_PROXY` | — | HTTP proxy URL for upstream calls |
| `HTTPS_PROXY` | — | HTTPS proxy URL |
| `NO_PROXY` | — | Hosts to exclude from proxy |
</details>

<details>
<summary>Storage</summary>

| Env var | Default | Description |
|---------|---------|-------------|
| `STORAGE_TYPE` | `sqlite` | Backend: `sqlite`, `postgresql`, or `mongodb` |
| `SQLITE_PATH` | `data/aurora.db` | SQLite database file path |
| `POSTGRES_URL` | — | PostgreSQL connection string |
| `POSTGRES_MAX_CONNS` | `10` | PostgreSQL connection pool max |
| `MONGODB_URL` | — | MongoDB connection string |
| `MONGODB_DATABASE` | `aurora` | MongoDB database name |
</details>

<details>
<summary>Model Registry</summary>

| Env var | Default | Description |
|---------|---------|-------------|
| `MODEL_LIST_URL` | `https://raw.githubusercontent.com/aurorallm/aurora/refs/heads/main/docs-assets/assets/models.json` | External model metadata registry |
| `MODEL_LIST_LOCAL_PATH` | `data/models.local.json` | Local model registry snapshot path |
| `MODEL_LIST_USER_OVERRIDES_PATH` | `data/user_pricing.yaml` | User pricing override file |
| `MODELS_ENABLED_BY_DEFAULT` | `true` | Default enabled state for provider models |
| `MODEL_OVERRIDES_ENABLED` | `true` | Allow per-model overrides |
| `KEEP_ONLY_ALIASES_AT_MODELS_ENDPOINT` | `false` | Hide provider models, show only aliases |
| `CONFIGURED_PROVIDER_MODELS_MODE` | `fallback` | `fallback` or `allowlist` |
</details>

<details>
<summary>Caching</summary>

**Model cache:**

| Env var | Default | Description |
|---------|---------|-------------|
| `CACHE_REFRESH_INTERVAL` | `3600` | Model registry cache refresh (seconds) |
| `AURORA_CACHE_DIR` | `.cache` | Local filesystem cache directory |
| `REDIS_URL` | — | Redis connection URL (enables Redis-backed model cache) |
| `REDIS_KEY_MODELS` | `aurora:models` | Redis key for model cache |
| `REDIS_TTL_MODELS` | `86400` | Redis model cache TTL (seconds) |

**Response cache (exact match):**

| Env var | Default | Description |
|---------|---------|-------------|
| `RESPONSE_CACHE_SIMPLE_ENABLED` | `false` | Enable Redis exact-response cache |
| `REDIS_KEY_RESPONSES` | `aurora:response:` | Redis key prefix for responses |
| `REDIS_TTL_RESPONSES` | `3600` | Response cache TTL (seconds) |

**Semantic cache (vector similarity):**

| Env var | Default | Description |
|---------|---------|-------------|
| `SEMANTIC_CACHE_ENABLED` | `false` | Enable semantic cache |
| `SEMANTIC_CACHE_THRESHOLD` | `0.92` | Similarity threshold (0-1) |
| `SEMANTIC_CACHE_PROMPT_SIMILARITY` | `0.90` | Prompt similarity threshold |
| `SEMANTIC_CACHE_TTL` | `3600` | Entry TTL (seconds) |
| `SEMANTIC_CACHE_MAX_CONV_MESSAGES` | `3` | Recent conversation messages to embed |
| `SEMANTIC_CACHE_EXCLUDE_SYSTEM_PROMPT` | `false` | Exclude system prompt from cache key |
| `SEMANTIC_CACHE_EMBEDDER_PROVIDER` | `openai` | Embedder provider name |
| `SEMANTIC_CACHE_EMBEDDER_MODEL` | `text-embedding-3-small` | Embedder model |
| `SEMANTIC_CACHE_VECTOR_STORE_TYPE` | `qdrant` | Backend: `qdrant`, `pgvector`, `pinecone`, `weaviate` |
| `SEMANTIC_CACHE_QDRANT_URL` | `http://localhost:6333` | Qdrant URL |
| `SEMANTIC_CACHE_QDRANT_COLLECTION` | `aurora_semantic` | Qdrant collection name |
| `SEMANTIC_CACHE_QDRANT_API_KEY` | — | Qdrant API key |
| `SEMANTIC_CACHE_PGVECTOR_URL` | — | pgvector connection string |
| `SEMANTIC_CACHE_PGVECTOR_TABLE` | `aurora_semantic_cache` | pgvector table name |
| `SEMANTIC_CACHE_PGVECTOR_DIMENSION` | `1536` | pgvector embedding dimension |
| `SEMANTIC_CACHE_PINECONE_HOST` | — | Pinecone host URL |
| `SEMANTIC_CACHE_PINECONE_API_KEY` | — | Pinecone API key |
| `SEMANTIC_CACHE_PINECONE_NAMESPACE` | — | Pinecone namespace |
| `SEMANTIC_CACHE_PINECONE_DIMENSION` | `1536` | Pinecone embedding dimension |
| `SEMANTIC_CACHE_WEAVIATE_URL` | — | Weaviate URL |
| `SEMANTIC_CACHE_WEAVIATE_CLASS` | `AuroraSemanticCache` | Weaviate class name |
| `SEMANTIC_CACHE_WEAVIATE_API_KEY` | — | Weaviate API key |
</details>

<details>
<summary>Audit Logging</summary>

| Env var | Default | Description |
|---------|---------|-------------|
| `LOGGING_ENABLED` | `false` | Enable audit log to storage |
| `LOGGING_LOG_BODIES` | `true` | Log request/response bodies |
| `LOGGING_LOG_HEADERS` | `true` | Log headers (sensitive headers redacted) |
| `LOGGING_ONLY_MODEL_INTERACTIONS` | `true` | Skip health/metrics/admin endpoints |
| `LOGGING_BUFFER_SIZE` | `1000` | In-memory queue capacity |
| `LOGGING_FLUSH_INTERVAL` | `5` | Flush interval (seconds) |
| `LOGGING_RETENTION_DAYS` | `30` | Auto-delete after N days (0 = forever) |
</details>

<details>
<summary>Usage Tracking</summary>

| Env var | Default | Description |
|---------|---------|-------------|
| `USAGE_ENABLED` | `true` | Enable token usage tracking |
| `USAGE_PRICING_RECALCULATION_ENABLED` | `true` | Allow admin pricing recalculation |
| `ENFORCE_RETURNING_USAGE_DATA` | `true` | Add `stream_options.include_usage=true` to streaming requests |
| `USAGE_BUFFER_SIZE` | `1000` | In-memory queue capacity |
| `USAGE_FLUSH_INTERVAL` | `5` | Flush interval (seconds) |
| `USAGE_RETENTION_DAYS` | `90` | Auto-delete after N days (0 = forever) |
</details>

<details>
<summary>Guardrails</summary>

| Env var | Default | Description |
|---------|---------|-------------|
| `GUARDRAILS_ENABLED` | `false` | Enable content safety filters globally |
| `ENABLE_GUARDRAILS_FOR_BATCH_PROCESSING` | `false` | Apply guardrails to `/v1/batches` items |
</details>

<details>
<summary>Metrics</summary>

| Env var | Default | Description |
|---------|---------|-------------|
| `METRICS_ENABLED` | `false` | Enable Prometheus `/metrics` endpoint |
| `METRICS_ENDPOINT` | `/metrics` | Metrics endpoint path |
</details>

<details>
<summary>Token Saver</summary>

| Env var | Default | Description |
|---------|---------|-------------|
| `TOKEN_SAVER_ENABLED` | `false` | Enable output compression |
| `TOKEN_SAVER_ENDPOINTS` | `chat_completions` | Endpoints to apply it to |
| `TOKEN_SAVER_APPLY_STREAMING` | `true` | Apply to streaming responses |
| `TOKEN_SAVER_OUTPUT_ENABLED` | `false` | Enable output style/profile |
| `TOKEN_SAVER_OUTPUT_PROFILE` | `concise` | Profile: `concise`, `caveman`, `ultra`, `wenyan` |
| `TOKEN_SAVER_MODELS_INCLUDE` | — | Models to include (comma-separated) |
| `TOKEN_SAVER_MODELS_EXCLUDE` | — | Models to exclude |
| `TOKEN_SAVER_PROVIDERS_INCLUDE` | — | Providers to include |
| `TOKEN_SAVER_PROVIDERS_EXCLUDE` | — | Providers to exclude |
| `TOKEN_SAVER_ON_ERROR` | `allow` | Behavior on error: `allow` or `block` |
| `TOKEN_SAVER_EMIT_HEADERS` | `true` | Emit token-saver headers in response |
| `TOKEN_SAVER_AUDIT_ENABLED` | `true` | Log token-saver actions |
</details>

<details>
<summary>Resilience</summary>

| Env var | Default | Description |
|---------|---------|-------------|
| `RETRY_MAX_RETRIES` | `3` | Upstream retry count |
| `RETRY_INITIAL_BACKOFF` | `1s` | Initial backoff duration |
| `RETRY_MAX_BACKOFF` | `30s` | Maximum backoff duration |
| `RETRY_BACKOFF_FACTOR` | `2.0` | Exponential backoff multiplier |
| `RETRY_JITTER_FACTOR` | `0.1` | Random jitter fraction |
| `CIRCUIT_BREAKER_FAILURE_THRESHOLD` | `5` | Failures before circuit opens |
| `CIRCUIT_BREAKER_SUCCESS_THRESHOLD` | `2` | Successes before circuit closes |
| `CIRCUIT_BREAKER_TIMEOUT` | `30s` | Time before half-open retry |
</details>

<details>
<summary>Fallback</summary>

| Env var | Default | Description |
|---------|---------|-------------|
| `FEATURE_FALLBACK_MODE` | `manual` | Fallback mode: `auto`, `manual`, or `off` |
| `FALLBACK_MANUAL_RULES_PATH` | — | Path to manual fallback rules JSON |
</details>

<details>
<summary>Admin & Features</summary>

| Env var | Default | Description |
|---------|---------|-------------|
| `ADMIN_ENDPOINTS_ENABLED` | `true` | Enable `/admin/api/v1/*` REST endpoints |
| `ADMIN_UI_ENABLED` | `true` | Enable `/admin/dashboard` UI |
| `COMBOS_ENABLED` | `true` | Enable combo model calls |
| `CLI_TOOLS_ENABLED` | `true` | Enable CLI tools integration |
| `CLI_TOOLS_APPLY_ENABLED` | `false` | Allow admin/API to apply tool changes |
| `WORKFLOW_REFRESH_INTERVAL` | `1m` | Workflow refresh interval from storage |
| `EDITION` | — | Edition identifier (Enterprise use) |
</details>

<details>
<summary>Config file path</summary>

| Env var | Default | Description |
|---------|---------|-------------|
| `AURORA_CONFIG_PATH` | `configs/config.yaml` | Override path to config YAML |
</details>

---


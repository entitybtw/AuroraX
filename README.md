<p align="center">
  <img src="https://raw.githubusercontent.com/aurorallm/aurora/main/docs-assets/assets/aurora-logo-animated.svg" width="96" height="96" alt="Aurora Logo">
</p>

<h1 align="center"> Aurora - The Fastest AI Gateway </h1>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/github/license/aurorallm/aurora" alt="License" height="20"></a>
  <a href="https://www.npmjs.com/package/iaurora"><img src="https://img.shields.io/npm/v/iaurora" alt="npm" height="20"></a>
  <a href="https://www.npmjs.com/package/iaurora"><img src="https://img.shields.io/npm/dw/iaurora" alt="npm weekly downloads" height="20"></a>
  <a href="https://github.com/aurorallm/aurora"><img src="https://img.shields.io/github/stars/aurorallm/aurora" alt="GitHub Stars" height="20"></a>
  <a href="https://discord.gg/AfaFBSU2km"><img src="https://dcbadge.limes.pink/api/server/https://discord.gg/AfaFBSU2km?style=flat" alt="Discord" height="20"></a>
  <img src="https://img.shields.io/docker/pulls/aurorahq/aurora" alt="Docker Pulls" height="20">
  <a href="https://artifacthub.io/packages/search?repo=aurora-gateway"><img src="https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/aurora-gateway" alt="Artifact Hub" height="20"></a>
</p>

<p align="center"><b>One API for every AI provider. Self-hosted. No vendor lock-in.</b></p>

<p align="center">14 provider types &bull; OpenAI &amp; Anthropic compatible &bull; Go &bull; Apache 2.0  &bull; Built for raw speed </p>

<a href="https://raw.githubusercontent.com/aurorallm/aurora/main/docs-assets/assets/dashboard-overview.png">
  <img src="https://raw.githubusercontent.com/aurorallm/aurora/main/docs-assets/assets/dashboard-overview.png" alt="Aurora admin dashboard showing provider stats and usage metrics" width="100%">
</a>

---

## What Aurora Does

Aurora sits between your app and LLM providers. Your app sends requests using the standard OpenAI or Anthropic SDK — Aurora routes them to whichever provider you've configured. One format handles everything — you dont need to worry about provider-specific formats.


```python
# Before: hardcoded provider
client = OpenAI(base_url="https://api.openai.com/v1", api_key="sk-...")

# After: Aurora Gateway
client = OpenAI(base_url="http://localhost:8080/v1", api_key="your-aurora-key")
```

No SDK changes. No format changes. Just swap the `base_url`.

---

## Features

### Routing & Providers

- **14 provider types** — OpenAI, Anthropic, Gemini, Groq, DeepSeek, OpenRouter, xAI, Z.ai, MiniMax, Azure OpenAI, Oracle, Ollama, vLLM, Jina
- **Auto-discovery** — set an API key as an env var, restart, provider + all its models appear automatically
- **Provider pools** — group multiple keys/endpoints, load-balance with round-robin or weighted distribution, health-aware failover
- **Model aliases** — rename/remap any model to a custom identifier across the entire gateway
- **Model overrides** — enable or disable specific models per user path, persisted via dashboard or `user_pricing.yaml`
- **Fallback** — automatic failover on 5xx/429, or manual rules (from config or external JSON) mapping failed provider+model to backups
- **Resilience** — exponential backoff with jitter, circuit breaker per provider (closed → open → half-open), per-provider override of global retry/circuit-breaker settings
- **Multiple instances** — run `OPENAI_EAST_API_KEY` and `OPENAI_WEST_API_KEY` as separate providers
- **Custom base URLs** — override any provider's endpoint (corporate proxies, regional endpoints)
- **Passthrough** — `/p/{provider}/*` for full upstream API access (not just chat completions); filter which provider types get passthrough routes
- **Config-driven workflows** — per-request routing, caching, guardrail, audit, usage, budget, and fallback behavior controlled by persisted workflow documents

### API Surface

- **OpenAI-compatible** — `/v1/chat/completions`, `/v1/embeddings`, `/v1/rerank`, `/v1/models`, `/v1/files`, `/v1/batches`
- **Responses API** — `/v1/responses` with full CRUD, cancel, input items, compact
- **Anthropic-compatible** — `/v1/messages`, `/v1/messages/count_tokens` (native Anthropic wire format); optional dedicated ingress at `/v1/messages`
- **Streaming** — SSE streaming for all endpoints, preserved end-to-end
- **Keep-only-aliases mode** — hide raw provider models from `/v1/models` and expose only aliased names
- **Configured provider models mode** — `fallback` (add listed models to auto-discovered) or `allowlist` (only serve explicitly listed models)

### Caching

- **Exact cache** — SHA-256 hash match on request, Redis-backed, async writes
- **Semantic cache** — vector similarity with configurable threshold, supports Qdrant, pgvector, Pinecone, Weaviate
- **Prompt cache** — forwards `cache_control` to Anthropic/OpenAI/Gemini native prompt caching; configurable modes (`auto`, `manual`, `off`), component toggles, and minimum token threshold
- **Model registry cache** — local filesystem + Redis, offline-safe; supports vendored JSON snapshots with per-field user pricing overrides

### Security & Guardrails

- **Master key** — top-level gateway auth
- **Managed API keys** — scoped, rate-limited, per-key model authorization, usage stats
- **Rate limiting** — per-key rate limiting backed by in-memory or Redis
- **PII redaction** — email, phone, SSN, credit card detection and masking
- **Prompt injection blocking** — detects and blocks injection attempts
- **System prompt protection** — inject, override, or decorate system prompts
- **Regex blocking** — custom pattern matching with block or sanitize actions
- **Length limits** — character/token count enforcement on requests
- **LLM-based altering** — guardrail that rewrites message content via an auxiliary LLM call (anonymization, custom prompts)
- **Guardrail direction & ordering** — run before provider dispatch (`input`), after response (`output`), or both; same-order guardrails run in parallel
- **Batch guardrails** — apply configured guardrails to inline items in `/v1/batches` requests

### Observability

- **Audit logging** — full request/response capture, buffered writes, configurable retention (body/header logging, buffer size, flush interval), live SSE stream
- **Usage analytics** — per-model token counting, cost tracking, daily aggregation by model/user-path, pricing recalculation action
- **Prometheus metrics** — `aurora_requests_total`, `aurora_request_duration_seconds`, `aurora_requests_in_flight`, plus gateway phase timing
- **Admin dashboard** — React SPA built into the Go binary: providers, pools, models, aliases, guardrails, cache, usage, audit, auth keys, workflows, console, playground
- **pprof endpoints** — Go runtime profiling at `/debug/pprof/*` (heap, goroutine, mutex, block, threadcreate)
- **Structured logging** — configurable format (JSON/text), level (debug/info/warn/error), source info, service metadata

### Cost Control

- **Token saver** — policy-driven output compression (profiles: concise, caveman, ultra, wenyan); scoped to specific models/providers via include/exclude filters; configurable on-error behavior (allow/block)
- **Pricing management** — per-model pricing overrides, recalculation, import/export
- **Usage budgets** — per-key usage tracking and limits, per-request budget enforcement via workflow feature flags

### Developer Experience

- **Single binary** — `npm install -g iaurora` or `docker pull aurorahq/aurora`
- **CLI** — `aurora init`, `aurora models sync/diff/show`, `aurora update`, `aurora uninstall`
- **CLI tools API** — admin REST endpoints for CLI configuration sync, gated separately
- **Swagger docs** — `/swagger/index.html` (build-tag gated)
- **Config profiles** — pre-built configs for local, local-power, and team deployments
- **3-layer config** — code defaults → config.yaml → env vars (env vars win)
- **Helm chart** — deploy on Kubernetes with pre-built Helm chart
- **Docker Compose** — full infrastructure stack: Redis, PostgreSQL, Qdrant, Prometheus, Grafana
- **Grafana dashboard** — pre-configured panels for request rate, errors, latency, in-flight requests, per-model breakdown

---

## Quick Start

Start routing AI traffic in 60 seconds.

### Option A — CLI (npm)

```bash
npm install -g iaurora
mkdir my-gateway && cd my-gateway
aurora init        # creates config.yaml, .env, data/
```

Set your provider keys in `.env`:

```env
# ── REQUIRED ──────────────────────────────────────────────
AURORA_MASTER_KEY="your-secure-key"

# ── PROVIDER API KEYS (at least one) ─────────────────────
OPENAI_API_KEY="sk-..."
ANTHROPIC_API_KEY="sk-ant-..."
GEMINI_API_KEY="..."
GROQ_API_KEY="gsk_..."
DEEPSEEK_API_KEY="..."
OPENROUTER_API_KEY="..."
XAI_API_KEY="..."
ZAI_API_KEY="..."
MINIMAX_API_KEY="..."
AZURE_API_KEY="..."
ORACLE_API_KEY="..."
OLLAMA_API_KEY="..."
VLLM_API_KEY="..."
JINA_API_KEY="..."

# ── OPTIONAL FEATURE TOGGLES (set true to enable) ────────
LOGGING_ENABLED=true                  # Audit logging to storage
METRICS_ENABLED=true                  # Prometheus /metrics endpoint
GUARDRAILS_ENABLED=true               # Content safety filters
TOKEN_SAVER_ENABLED=true              # Output compression to cut token use

# ── PRODUCTION STORAGE ───────────────────────────────────
# STORAGE_TYPE=postgresql
# POSTGRES_URL=postgres://user:pass@localhost:5432/aurora

# ── REDIS CACHE (model cache + response cache) ──────────
# REDIS_URL=redis://localhost:6379
# RESPONSE_CACHE_SIMPLE_ENABLED=true
```

```bash
aurora
```

### Option B — inline env vars (no `.env` needed)

<details>
<summary>Linux / macOS</summary>

```bash
AURORA_MASTER_KEY=your-secure-key \
  OPENAI_API_KEY=sk-... \
  ANTHROPIC_API_KEY=sk-ant-... \
  GEMINI_API_KEY=... \
  GROQ_API_KEY=gsk_... \
  DEEPSEEK_API_KEY=... \
  OPENROUTER_API_KEY=... \
  XAI_API_KEY=... \
  ZAI_API_KEY=... \
  MINIMAX_API_KEY=... \
  AZURE_API_KEY=... \
  ORACLE_API_KEY=... \
  OLLAMA_API_KEY=... \
  VLLM_API_KEY=... \
  JINA_API_KEY=... \
  LOGGING_ENABLED=true \
  METRICS_ENABLED=true \
  GUARDRAILS_ENABLED=true \
  TOKEN_SAVER_ENABLED=true \
  aurora
```
</details>

<details>
<summary>Windows PowerShell</summary>

```powershell
$env:AURORA_MASTER_KEY="your-secure-key"; `
$env:OPENAI_API_KEY="sk-..."; `
$env:ANTHROPIC_API_KEY="sk-ant-..."; `
$env:GEMINI_API_KEY="..."; `
$env:GROQ_API_KEY="gsk_..."; `
$env:DEEPSEEK_API_KEY="..."; `
$env:OPENROUTER_API_KEY="..."; `
$env:XAI_API_KEY="..."; `
$env:ZAI_API_KEY="..."; `
$env:MINIMAX_API_KEY="..."; `
$env:AZURE_API_KEY="..."; `
$env:ORACLE_API_KEY="..."; `
$env:OLLAMA_API_KEY="..."; `
$env:VLLM_API_KEY="..."; `
$env:JINA_API_KEY="..."; `
$env:LOGGING_ENABLED="true"; `
$env:METRICS_ENABLED="true"; `
$env:GUARDRAILS_ENABLED="true"; `
$env:TOKEN_SAVER_ENABLED="true"; `
aurora
```
</details>

<details>
<summary>Windows CMD</summary>

```cmd
set AURORA_MASTER_KEY=your-secure-key ^
  && set OPENAI_API_KEY=sk-... ^
  && set ANTHROPIC_API_KEY=sk-ant-... ^
  && set GEMINI_API_KEY=... ^
  && set GROQ_API_KEY=gsk_... ^
  && set DEEPSEEK_API_KEY=... ^
  && set OPENROUTER_API_KEY=... ^
  && set XAI_API_KEY=... ^
  && set ZAI_API_KEY=... ^
  && set MINIMAX_API_KEY=... ^
  && set AZURE_API_KEY=... ^
  && set ORACLE_API_KEY=... ^
  && set OLLAMA_API_KEY=... ^
  && set VLLM_API_KEY=... ^
  && set JINA_API_KEY=... ^
  && set LOGGING_ENABLED=true ^
  && set METRICS_ENABLED=true ^
  && set GUARDRAILS_ENABLED=true ^
  && set TOKEN_SAVER_ENABLED=true ^
  && aurora
```
</details>

### Option C — Docker

```bash
docker run -d --name aurora -p 8080:8080 \
  -e AURORA_MASTER_KEY="your-secure-key" \
  -e OPENAI_API_KEY="sk-..." \
  -e ANTHROPIC_API_KEY="sk-ant-..." \
  -e GEMINI_API_KEY="..." \
  -e GROQ_API_KEY="gsk_..." \
  -e DEEPSEEK_API_KEY="..." \
  -e OPENROUTER_API_KEY="..." \
  -e XAI_API_KEY="..." \
  -e ZAI_API_KEY="..." \
  -e MINIMAX_API_KEY="..." \
  -e AZURE_API_KEY="..." \
  -e ORACLE_API_KEY="..." \
  -e OLLAMA_API_KEY="..." \
  -e VLLM_API_KEY="..." \
  -e JINA_API_KEY="..." \
  -e LOGGING_ENABLED=true \
  -e METRICS_ENABLED=true \
  -e GUARDRAILS_ENABLED=true \
  -e TOKEN_SAVER_ENABLED=true \
  aurorahq/aurora
```

### Option D — Kubernetes (Helm)

```bash
# Quick dev — Groq, no Redis, no auth
helm install aurora ./helm \
  --namespace aurora --create-namespace \
  --set image.repository=aurorahq/aurora \
  --set image.tag=latest \
  --set providers.groq.apiKey="gsk_your_key_here" \
  --set providers.groq.enabled=true \
  --set redis.enabled=false \
  --set auth.masterKey=""
```

```bash
# Production — multiple providers, auth, Redis
helm upgrade --install aurora ./helm \
  --namespace aurora --create-namespace \
  --set image.repository=aurorahq/aurora \
  --set image.tag=latest \
  --set auth.masterKey="your-secure-key" \
  --set providers.openai.apiKey="sk-..." \
  --set providers.openai.enabled=true \
  --set providers.anthropic.apiKey="sk-ant-..." \
  --set providers.anthropic.enabled=true \
  --set providers.gemini.apiKey="..." \
  --set providers.gemini.enabled=true \
  --set providers.groq.apiKey="gsk_..." \
  --set providers.groq.enabled=true \
  --set providers.deepseek.apiKey="..." \
  --set providers.deepseek.enabled=true \
  --set redis.enabled=true
```

**Full Helm docs:** [helm/README.md](./helm/README.md)

### Test your gateway

```bash
# OpenAI format
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-master-key" \
  -d '{"model":"groq/llama-4-scout-17b-16e-instruct","messages":[{"role":"user","content":"Hello!"}]}'

# Anthropic format with streaming
curl http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-master-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "anthropic/claude-sonnet-5-20260630",
    "max_tokens": 1024,
    "stream": true,
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Embeddings
curl http://localhost:8080/v1/embeddings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-master-key" \
  -d '{"model":"openai/text-embedding-3-small","input":"Hello world"}'

# Reranking (Jina)
curl http://localhost:8080/v1/rerank \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-master-key" \
  -d '{"model":"jina/jina-reranker-v2-base-multilingual","query":"test","documents":["doc1","doc2"]}'
```

Dashboard: `http://localhost:8080/admin/dashboard`

**Docs:** [aurorallm.online/docs](https://aurorallm.online/docs) · [Website](https://aurorallm.online) · [npm](https://www.npmjs.com/package/iaurora) · [Docker](https://hub.docker.com/r/aurorahq/aurora) · [GitHub](https://github.com/aurorallm/aurora)

---

## Providers

Providers are **auto-discovered from environment variables**. Set any provider's `_API_KEY` and restart — the provider and its default models appear automatically.

> **Security note:** The env var names below are documentation references. Actual secrets go into your **`.env` file** (in `.gitignore`) or your **deployment secrets manager** — never commit them.

| Provider | Env var | Default base URL | Requires base URL | API key required | Default models |
|----------|---------|-----------------|-------------------|-----------------|----------------|
| OpenAI | `OPENAI_API_KEY` | `https://api.openai.com/v1` | No | Yes | `gpt-5.6-sol`, `gpt-5.6-luna` |
| Anthropic | `ANTHROPIC_API_KEY` | `https://api.anthropic.com/v1` | No | Yes | `claude-sonnet-5`, `claude-fable-5` |
| Google Gemini | `GEMINI_API_KEY` | `https://generativelanguage.googleapis.com/v1beta/openai` | No | Yes | `gemini-3.1-pro`, `gemini-3.5-flash` |
| Groq | `GROQ_API_KEY` | `https://api.groq.com/openai/v1` | No | Yes | `llama-4-scout-17b`, `llama-4-maverick-17b`, `qwen3-32b` |
| DeepSeek | `DEEPSEEK_API_KEY` | `https://api.deepseek.com` | No | Yes | `deepseek-v4-pro`, `deepseek-v4-flash` |
| OpenRouter | `OPENROUTER_API_KEY` | `https://openrouter.ai/api/v1` | No | Yes | 300+ models |
| xAI (Grok) | `XAI_API_KEY` | `https://api.x.ai/v1` | No | Yes | `grok-4.5`, `grok-4.3` |
| Z.ai | `ZAI_API_KEY` | `https://api.z.ai/api/paas/v4` | No | Yes | `glm-5.2` |
| MiniMax | `MINIMAX_API_KEY` | `https://api.minimax.io/v1` | No | Yes | `minimax-m3` |
| Azure OpenAI | `AZURE_API_KEY` | — | **Yes** | Yes | Your deployments |
| Oracle | `ORACLE_API_KEY` | — | **Yes** | Yes | `cohere.command-r-plus` |
| Ollama | `OLLAMA_API_KEY` | `http://localhost:11434/v1` | No | **No** (optional) | Any local model |
| vLLM | `VLLM_API_KEY` | `http://localhost:8000/v1` | No | **No** (optional) | Any served model |
| Jina (reranker) | `JINA_API_KEY` | — | **Yes** | Yes | `jina-embeddings-v3` |

### Per-provider configuration

Every provider supports `*_MODELS` to override auto-discovered models:

```env
OPENAI_MODELS=gpt-5.6-sol,gpt-5.6-terra,gpt-5.6-luna
```

Custom base URL:

```env
OPENAI_BASE_URL=https://my-corp-openai-proxy.example.com/v1
```

Multiple instances of the same provider (underscores become hyphens in the provider name):

```env
OPENAI_EAST_API_KEY=sk-...     # → provider: openai-east
OPENAI_WEST_API_KEY=sk-...     # → provider: openai-west
```

Azure requires API version:

```env
AZURE_API_VERSION=2024-10-21
```

OpenRouter extras:

```env
OPENROUTER_SITE_URL=https://github.com/aurorallm/aurora
OPENROUTER_APP_NAME=Aurora Gateway
```

---

---

## Configuration

The gateway loads settings in this priority order (later wins):

```
code defaults → config.yaml → .env / environment variables
```

Generated by `aurora init`, every section of `config.yaml` is documented inline:

| Section | What it controls |
|---------|-----------------|
| `server` | Port, base path, master key, passthrough, Anthropic ingress |
| `admin` | Dashboard API and UI |
| `models` | Discovery, overrides, allowlisting |
| `storage` | SQLite (default), PostgreSQL, or MongoDB |
| `logging` | Audit logging of requests/responses |
| `usage` | Token tracking, pricing, retention |
| `metrics` | Prometheus endpoint |
| `guardrails` | Content safety filters |
| `cache` | Model cache, response cache (exact + semantic) |
| `combos` | Multi-model combo definitions |
| `token_saver` | Output compression |
| `fallback` | Provider failover rules |
| `resilience` | Retry + circuit breaker |
| `workflows` | Policy-based request routing |

### Config profiles

Pre-built configs in `configs/editions/`:

| Profile | File | Use case |
|---------|------|----------|
| OSS | `oss.env.example` | Minimal local — SQLite, no Redis |
| OSS Local Power | `oss.local-power.env.example` | SQLite + Redis exact cache |
| OSS Team | `oss.team.env.example` | Postgres + Redis + Qdrant — full team deployment |

```bash
export AURORA_CONFIG_PATH=configs/editions/oss.team.example.yaml
```

### Complete env var reference

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

## CLI Reference

Installed via `npm install -g iaurora`.

| Command | Description |
|---------|-------------|
| `aurora` | Start the gateway server (default port 8080) |
| `aurora init` | Scaffold `config.yaml`, `.env`, `data/` in current directory |
| `aurora update` | Self-update via `npm install -g iaurora@latest` |
| `aurora uninstall` | Remove via `npm uninstall -g iaurora` |
| `aurora models sync` | Download upstream model registry to local file |
| `aurora models diff` | Show pricing diff between upstream and local snapshot |
| `aurora models show` | Print effective pricing for a model after merging overrides |
| `aurora -version` | Print version information |
| `aurora -help` | Show all CLI options and config reference |
| `aurora -help-json` | Dump env var schema as JSON |

---

## Repository Structure

```text
aurora/
├── apps/              # Application entrypoints
├── internal/          # Core packages (providers, gateway, storage, guardrails, etc.)
├── dashboard-ui/      # React admin dashboard (Vite)
├── configs/           # Configuration profiles and examples
├── docs-assets/       # Images, models.json, assets
├── helm/              # Kubernetes Helm charts
├── monitoring/        # Prometheus + Grafana configs
├── npm/               # npm CLI wrapper
├── bench-results/     # Benchmark data
├── release/           # Release scripts
└── scripts/           # Build and utility scripts
```

---

## Enterprise Deployments

Aurora supports enterprise-grade deployments for teams running production AI systems at scale.
In addition to private networking, custom security controls, and governance, **Aurora Enterprise** unlocks advanced capabilities including SSO, RBAC, tenant isolation, budget enforcement, compliance workflows, and production support.

The Enterprise edition is a separate distribution with a signed license.

<a href="https://raw.githubusercontent.com/aurorallm/aurora/main/docs-assets/assets/comparison.png">
  <img src="https://raw.githubusercontent.com/aurorallm/aurora/main/docs-assets/assets/comparison.png" alt="OSS vs Enterprise comparison" width="100%" style="border-radius:12px;margin:16px 0;">
</a>

<div align="center">
  <a href="mailto:team.auroragate@gmail.com?subject=Aurora%20Enterprise%20Inquiry" style="display:block;margin-top:5px;">
    <img src="https://img.shields.io/badge/Email%20Us-Enterprise-5865F2?style=for-the-badge&logo=gmail&logoColor=white" alt="Email Aurora Enterprise" width="200"/>
  </a>
</div>

---

## Documentation

- [aurorallm.online/docs](https://aurorallm.online/docs) — full documentation
- [aurorallm.online/docs/getting-started/quickstart](https://aurorallm.online/docs/getting-started/quickstart) — quickstart guide
- [aurorallm.online/docs/guides](https://aurorallm.online/docs/guides) — provider and integration guides
- [aurorallm.online/docs/api/overview](https://aurorallm.online/docs/api/overview) — API reference
- [aurorallm.online/benchmarks](https://aurorallm.online/benchmarks) — performance benchmarks

---

## Need Help?

[Join our Discord](https://discord.gg/AfaFBSU2km) for community support, setup help, and discussions.


---

## License

This project is licensed under the Apache 2.0 License — see the [LICENSE](LICENSE) file for details.

Built by the Aurora team.

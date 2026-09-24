<p align="center">
  <img src="docs-assets/assets/aurora-logo-animated.svg" width="96" height="96" alt="Aurora Logo">
</p>

<h1 align="center">AuroraX - The Fastest AI Gateway </h1>
<h2 align="center">A fork focused on multi-IP setups & API integration</h2>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/github/license/entitybtw/aurora" alt="License" height="20"></a>
  <a href="https://github.com/entitybtw/aurora"><img src="https://img.shields.io/github/stars/entitybtw/aurora?style=social" alt="GitHub Stars" height="20"></a>
  <a href="https://github.com/entitybtw/aurora/fork"><img src="https://img.shields.io/github/forks/entitybtw/aurora?style=social" alt="GitHub Forks" height="20"></a>
  <a href="https://hub.docker.com/r/entbtw/aurora"><img src="https://img.shields.io/docker/pulls/entbtw/aurora" alt="Docker Pulls" height="20"></a>
  <a href="https://hub.docker.com/r/entbtw/aurora"><img src="https://img.shields.io/docker/stars/entbtw/aurora" alt="Docker Stars" height="20"></a>
  <a href="https://hub.docker.com/r/entbtw/aurora"><img src="https://img.shields.io/docker/image-size/entbtw/aurora" alt="Docker Image Size" height="20"></a>
  <a href="https://hub.docker.com/r/entbtw/aurora"><img src="https://img.shields.io/docker/v/entbtw/aurora?sort=semver" alt="Docker Version" height="20"></a>
</p>

> [!WARNING]
> **Disclaimer:** This project is a research proof-of-concept. Some features (OAuth device flow, session hub header mapping) interact with third-party APIs in ways that may violate their Terms of Service. The author is not responsible for any account suspensions, bans, or other consequences that may result from using these features. Use at your own risk.

<p align="center"><b>One API for every AI provider. Self-hosted. No vendor lock-in.</b></p>

<p align="center">14 provider types &bull; OpenAI &amp; Anthropic compatible &bull; Go &bull; Apache 2.0  &bull; Built for raw speed </p>

<a href="docs-assets/assets/dashboard-overview.png">
  <img src="docs-assets/assets/dashboard-overview.png" alt="AuroraX admin dashboard showing provider stats and usage metrics" width="100%">
</a>

## Documentation

Full guides, written for this fork.

| Guide | What it covers |
|-------|----------------|
| [Getting Started](documentation/GETTING_STARTED.md) | first run, build, config, basic usage, OpenAI-compatible client |
| [Deployment](documentation/DEPLOYMENT.md) | Docker / Docker Compose, persistent state files, multi-IP host networking |
| [Multi-account pools](documentation/MULTI_ACCOUNT.md) | end-to-end: load-balanced accounts with distinct, stable client identities |
| [Session Hub](documentation/SESSION_HUB.md) | header transformation & session mapping engine, header modes, API reference, dashboard |
| [Docker image](documentation/DOCKER_PUSH.md) | published image `entbtw/aurora`, tags, how to build & publish |

**Quick deploy:**

```bash
docker pull entbtw/aurora:latest
docker run -d --name aurora -p 8080:8080 -e AURORA_MASTER_KEY="your-secure-key" entbtw/aurora:latest
```

See [Deployment](documentation/DEPLOYMENT.md) for production (persistent config & state, `network_mode: host` for multi-IP).

---

## What's new in this fork

Dashboard-driven operations — no more `.env`-only workflows for the things you change most. Everything below is managed from the UI and **persists across restarts**.

> **Warning:** This fork contains custom features not present in the original [aurorallm/aurora](https://github.com/aurorallm/aurora). Some features (dashboard redesign, session hub, UI enhancements) were vibecoded and may contain rough edges. Designed for advanced API integration workflows — use at your own discretion.

- **Redesigned dashboard** — full **Catppuccin** theme, mobile-responsive, compact/touch-friendly layout, clean auth/logo/sidebar, shared `SearchInput` fix in audit logs & usage.
- **Provider CRUD** — manage providers from the UI (base URL, API key, models, type). Per-provider `bind_ip`, `pool_only`, runtime enable/disable, live rename, duplicate protection. Status shows if a key is set **without exposing it**. OpenRouter list is now an **allowlist**; **vLLM** type added to the dashboard (was `.env`-only). **Bulk actions** — select multiple providers with checkboxes, then enable/disable, toggle auto-fetch, or delete in one click. Auto-fetch filter shown as pill on each card.
- **Custom User-Agent** — set a custom `User-Agent` header per provider for upstream attribution (e.g. OpenRouter recommends this for credits).
- **Auto-fetch models toggle** — disable automatic `/models` discovery per provider to use only explicitly configured model lists.
- **Auto-fetch model filter** — narrow automatic discovery with per-provider conditions (substring / regex on the model ID, price ceilings). Keep only matching models; everything else is dropped and cannot be routed to.
- **Fallback chains** — edit rules in the UI, applied at **runtime**; callable by name, exposed in `/v1/models`, order preserved on toggle/edit/delete.
- **Provider pools** — create/edit/delete with member selection and **weighted / round-robin** strategies; health-aware members, `pool_only` models, live registry rebuild.
- **Response headers** — configurable `X-Actual-Provider` / `X-Actual-Model` / `X-Requested` / `X-Fallback-Chain`, per-header toggles, custom headers, success/error/always modes, emitted on `429`/`401`.
- **Persistence** — state saved to `configs/provider-overrides.json`, `configs/pool-overrides.json`, `configs/fallback.json` (env-overridable); Docker volumes keep it across recreation.
- **Session Hub** — header transformation engine with per-provider/pool session mapping, inbound→outbound unique ID generation, disk persistence with live toggle, and pool-aware binding via UI (see [Session Hub](#session-hub) below).
- **Sidecar + extensions** — bundled Bun sidecar for extension-driven TLS fingerprinting (headers, tools, retries), **multi-IP egress** (per-IP Go CONNECT proxies), a dedicated **Settings → Sidecar** tab (extensions, browse store, enable, port, tools injection, auth, retry, bind IPs), a one-click **Session Hub rule** dialog, and graceful fallback for paid/OAuth traffic (see [Sidecar](#sidecar--extension-driven-tls-fingerprint-proxy) below).

---

## What AuroraX Does

AuroraX sits between your app and LLM providers. Your app sends requests using the standard OpenAI or Anthropic SDK — Aurora routes them to whichever provider you've configured. One format handles everything — you dont need to worry about provider-specific formats.


```python
# Before: hardcoded provider
client = OpenAI(base_url="https://api.openai.com/v1", api_key="sk-...")

# After: AuroraX Gateway
client = OpenAI(base_url="http://localhost:8080/v1", api_key="your-aurorax-key")
```

No SDK changes. No format changes. Just swap the `base_url`.

---

## Features

### Routing & Providers

- **14 provider types** — OpenAI, Anthropic, Gemini, Groq, DeepSeek, OpenRouter, xAI, Z.ai, MiniMax, Azure OpenAI, Oracle, Ollama, vLLM, Jina
- **Auto-discovery** — set an API key as an env var, restart, provider + all its models appear automatically
- **Auto-fetch toggle** — disable per-provider model auto-discovery to use only explicitly configured model lists
- **Auto-fetch filter** — restrict discovery to models matching declared conditions (e.g. `contains: free`, `max_price: 0`); filtered models are never registered
- **Custom User-Agent** — set a custom `User-Agent` header per provider for upstream attribution or branding
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
- **Admin dashboard** — React SPA built into the Go binary (Catppuccin, fully mobile-responsive): full provider CRUD, fallback chains, provider pools, response-header config, plus models, aliases, guardrails, cache, usage, audit, auth keys, workflows, console, playground
- **pprof endpoints** — Go runtime profiling at `/debug/pprof/*` (heap, goroutine, mutex, block, threadcreate)
- **Structured logging** — configurable format (JSON/text), level (debug/info/warn/error), source info, service metadata

### Cost Control

- **Token saver** — policy-driven output compression (profiles: concise, caveman, ultra, wenyan); scoped to specific models/providers via include/exclude filters; configurable on-error behavior (allow/block)
- **Pricing management** — per-model pricing overrides, recalculation, import/export
- **Usage budgets** — per-key usage tracking and limits, per-request budget enforcement via workflow feature flags

### Developer Experience

- **Single binary** — `docker pull entbtw/aurora` (this fork) or run from source with Go
- **CLI** — run from source, or drive via config files + the dashboard
- **CLI tools API** — admin REST endpoints for CLI configuration sync, gated separately
- **Swagger docs** — `/swagger/index.html` (build-tag gated)
- **Config profiles** — pre-built configs for local, local-power, and team deployments
- **3-layer config** — code defaults → config.yaml → env vars (env vars win)

### Session Hub

Header transformation engine for API integration workflows where upstream services require unique client identifiers per account.

> **⚠️ Use at your own risk.** The Session Hub rewrites upstream headers to impersonate official clients. This may violate a provider's terms of service, and upstream fingerprint hardening can break it at any time. A warning banner is shown on the dashboard tab.

- **Per-provider/pool binding** — attach transformation rules to specific providers, pools, fallbacks, or all targets (`*`)
- **7 header modes** — `map` (stable inbound→outbound per provider), `map_or_generate` (map when present, generate fresh when absent), `generate` (fresh ID each request), `passthrough`, `static`, `random_from_list`, `remove`
- **Configurable charset** — generated values use `alphanumeric` (default), `hex`, or `digits`; some upstreams require a specific alphabet (e.g. identity-prefix + hex)
- **Pool-aware** — rules bound to a pool automatically apply to all member providers
- **Inbound header forwarding** — client session headers are forwarded through the translation layer so `map` mode works even when the provider path drops arbitrary inbound headers
- **Lock-free hot path** — `Apply()` is a single atomic map read; benchmarked at ~495 ns/op (negligible)
- **Extensions (import only)** — portable JSON bundles that configure sidecar + Session Hub headers (+ optional provider types). Applied idempotently via `POST /admin/api/v1/sessionhub/providers/:name/ensure-headers`; existing rules are preserved, only missing ones are added, and non-default values are flagged. Install from a store or by URL (see **aurorax-store**).
- **Editable in place** — expand any rule in the Session Hub tab to toggle it, add/remove headers, and save; changes apply live without a restart
- **Persistent or in-memory** — toggled live via API or dashboard (`PUT /admin/api/v1/sessionhub/storage {"mode":"disk"}`)
- **Dashboard UI** — Settings → Session Hub: status cards, binding overview from live server targets (pools/providers), inline rule editing, live mapping viewer, storage toggle, mobile-responsive grids and touch-friendly inputs

#### How it works

1. Client sends request to AuroraX (e.g. with an inbound identity header)
2. Gateway intercepts the inbound session header and stores it in request context
3. Request is routed to a pool member (e.g. `pool-a` → `acc-backup`)
4. Provider's outbound `headerSetter` fires: session hub applies rules for that provider/pool
5. `map` mode: inbound value → unique outbound value per provider (stable, deduplicated)
6. `map_or_generate` mode: same as `map` when inbound is present; generates a fresh prefixed value when absent (for clients that only sometimes send a session header)
7. `generate` mode: fresh random value per request (always unique)
8. Additional static headers are injected per rule
9. Outbound request goes to upstream with transformed headers

#### Config

Rules are persisted in `configs/session-hub-rules.yaml` (gitignored). Live edits via API or dashboard are auto-saved.

```yaml
enabled: true
mapping_storage: disk          # "memory" or "disk"
providers:
  my-provider:                 # matches pool name or provider name
    enabled: true
    headers:
      - name: x-session-id
        mode: map_or_generate  # recommended: map when present, generate when absent
        prefix: "ses_"
        length: 26
        charset: hex
      - name: x-client-id
        mode: static
        value: cli
```

#### API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/admin/api/v1/sessionhub/status` | Stats + `storage_mode` |
| `GET` | `/admin/api/v1/sessionhub/providers` | List bound rules |
| `POST` | `/admin/api/v1/sessionhub/providers` | Create rule |
| `PUT` | `/admin/api/v1/sessionhub/providers/:name` | Update rule |
| `DELETE` | `/admin/api/v1/sessionhub/providers/:name` | Delete rule |
| `GET` | `/admin/api/v1/sessionhub/mappings` | List live mappings |
| `DELETE` | `/admin/api/v1/sessionhub/mappings` | Clear all mappings |
| `PUT` | `/admin/api/v1/sessionhub/storage` | Toggle `memory`/`disk` |
| `POST` | `/admin/api/v1/sessionhub/apply` | Test transform |

#### Header modes

| Mode | Behavior |
|------|----------|
| `map` | First request generates unique outbound value per provider; subsequent requests with same inbound reuse it |
| `map_or_generate` | **(default)** Like `map` but falls back to `generate` when the inbound header is absent — ideal for clients that only sometimes send a session header |
| `generate` | Fresh random value every request |
| `passthrough` | Original value forwarded unchanged |
| `static` | Fixed value (set `value:`) |
| `random_from_list` | Random pick from `values:` list |
| `remove` | Strip header entirely |

---

## Sidecar — Extension-driven TLS fingerprint proxy

AuroraX can route upstream traffic through a bundled **Bun sidecar** when an extension supplies the base URL, User-Agent, auth and tool scope. The sidecar reproduces a client fingerprint Go cannot (BoringSSL ClientHello). It runs inside the `runtime-sidecar` image variant and is managed from **Settings → Sidecar**.

> **⚠️ Use at your own risk.** The sidecar emulates an upstream client. Upstream hardening can break it at any time, it may violate the provider's terms of service, and it relies on an actively maintained fingerprint. Review this section before enabling, and prefer a paid/API-key path for production workloads. Review third-party extensions before installing them from a store.

Upstream-specific free-tier details live in a **store extension** (not built into the gateway). Install via **Settings → Sidecar → Browse store** or import by URL.

### Why a sidecar exists

Some upstream edges apply **layered** client verification. A request must pass **all** of these simultaneously:

| Layer | Check | Why Go alone fails |
|-------|-------|--------------------|
| 1. TLS fingerprint | Ja3/Ja4 ClientHello must match the extension's expected profile (e.g. Bun BoringSSL) | Go `net/http` and even Chrome-impersonating uTLS may **not** match |
| 2. HTTP headers | User-Agent + identity headers declared by the extension | Header shape must match exactly |
| 3. Tool schema | Body must carry the extension's full tool schema + `tool_choice: auto` (if required) | Partial tool sets can be rejected |
| 4. Streaming | Some free tiers only answer `stream: true` | Non-streaming requests must be re-aggregated |
| 5. Auth | Extension-supplied default auth or a real OAuth token | Dashboard API keys may be rejected |
| 6. Behaviour | Fresh connection per request; retry transient 403/429 | Connection reuse can trigger rejection |

Go cannot always produce layer 1, and one layer without the others is useless — hence the sidecar. **Bun** performs the end-to-end TLS handshake while Aurora still owns routing, auth, and pooling.

### Architecture

```
Aurora (Go)
   │  provider.BindIP → header x-aurora-bind-ip
   ▼
Bun sidecar (adapter.js, :8090)
   │  picks the per-IP CONNECT proxy by bind_ip
   ▼
Go bind-proxy (aurora bindproxy, :8981+)
   │  binds the TCP source IP; does NOT touch TLS
   ▼
Bun one-shot (fresh process per request)
   │  Bun TLS handshake (BoringSSL)      ← layer 1
   │  extension tool schema              ← layer 3
   │  stream: true (if required)         ← layer 4
   │  UA + identity headers              ← layer 2
   │  default auth / OAuth token         ← layer 5
   ▼
upstream /v1/chat/completions  → 200
```

The sidecar always streams upstream; when the caller asked for a non-streaming response it aggregates the SSE chunks back into a single `chat.completion` object (falling back to `reasoning_content` when a reasoning model emits no visible `content`).

### What works

- **Extension-driven fingerprints** — install a free-tier extension from the store to supply base URL, UA, headers, tools and retries
- **Paid/OAuth traffic** — the sidecar forwards the incoming `Authorization` untouched, so OAuth tokens and API keys keep working through the same path
- **Multi-IP egress** — one Go CONNECT proxy per configured IP (ports `8981+`), selected per provider via `bind_ip`, so rate limits can spread across IPs
- **Streaming and non-streaming** — both, with SSE aggregation on the non-streaming path
- **Runtime tuning** — every knob is editable from **Settings → Sidecar** and persisted to `configs/sidecar-overrides.json`

### Enabling it

Build (or pull) the sidecar image variant and set the environment:

```bash
docker build --target runtime-sidecar -t entbtw/aurora:sidecar .

docker run -d --name aurora \
  -p 8080:8080 \
  -v $PWD/configs:/app/configs \
  -v $PWD/data:/app/data \
  -e AURORA_SIDECAR_ENABLED=true \
  -e AURORA_SIDECAR_BIND_IPS=203.0.113.10,203.0.113.11 \
  -e AURORA_SIDECAR_BASE_URL=http://127.0.0.1:8090/v1 \
  entbtw/aurora:sidecar
```

Then configure a provider (`type: vllm`, or the extension-provided type if the extension declares one) with:

```yaml
providers:
  sidecar-upstream:
    type: vllm
    base_url: https://upstream.example.com/v1   # real upstream, rewritten to the sidecar at runtime
    sidecar_url: http://127.0.0.1:8090/v1       # route this provider through the sidecar
    bind_ip: 203.0.113.10                       # egress IP → matching CONNECT proxy
    pool_only: true
```

Import the matching extension (headers + settings) from the store, then apply it in **Settings → Sidecar**.

### Session Hub integration

The sidecar and Session Hub work together: the sidecar supplies the *fingerprint*, the extension supplies the *headers*. **Settings → Sidecar** lets you pick an **extension**, choose a target pool/provider, and apply it — enabling the sidecar with the right settings and installing the matching Session Hub rules in one step. Rules are merged idempotently (existing rules are **never overwritten**, only missing headers are added) via `POST /admin/api/v1/sessionhub/providers/:name/ensure-headers` and can be edited any time in the **Session Hub** tab.

### Sidecar configuration reference

All settings are editable in **Settings → Sidecar** (persisted to `configs/sidecar-overrides.json`) and/or via the admin API `GET`/`PUT /admin/api/v1/sidecar`.

| Setting | Env var | Default | Purpose |
|---------|---------|---------|---------|
| Enabled | `AURORA_SIDECAR_ENABLED` | `true` | Start/stop the sidecar |
| Port | `AURORA_SIDECAR_PORT` | `8090` | Loopback HTTP port |
| Inject tools | `AURORA_SIDECAR_INJECT_TOOLS` | `true` | Add tool schemas when absent |
| Default auth | `AURORA_SIDECAR_DEFAULT_AUTH` | — | Fallback Authorization |
| User-Agent | `AURORA_SIDECAR_USER_AGENT` | — | Upstream UA (set by extension) |
| Max attempts | `AURORA_SIDECAR_MAX_ATTEMPTS` | `4` | Retries on 403/429 |
| Retry delay | `AURORA_SIDECAR_RETRY_DELAY_MS` | `750` | Base backoff between retries |
| Bind IPs | `AURORA_SIDECAR_BIND_IPS` | — | Comma-separated egress IPs |
| Upstream | `AURORA_SIDECAR_BASE_URL` | — | Provider routing target (Go side) |
| Sidecar upstream | `AURORA_SIDECAR_UPSTREAM_URL` | — | Sidecar's own upstream (Bun side) |


> **Reserved namespace:** env vars whose provider-suffix is `SIDECAR` or contains `BIND` are ignored by provider auto-discovery, so they never materialise as phantom providers.

### Hardening resilience

The fingerprint is emulated, not inherent, so upstream changes can break it. To recover quickly:

- **Update the profile** — bump `BUN_VERSION` in `Dockerfile.builder` and the extension's `User-Agent` / tool schema (see the store extension) to match the current client.
- **Watch for degradation** — a rising share of `403` in the logs signals a server-side change. The sidecar retries transient 403/429 automatically.
- **Tune, don't fork** — headers, auth, user-agent, tool injection, retries, and IPs are all runtime settings, so most adjustments need no rebuild.
- **Fail over gracefully** — keep paid/API-key providers in the same pool so a broken fingerprint doesn't take down routing.

### Known limitations

- The sidecar variant is **debian-slim + Bun** (~100 MB larger than the distroless default). Use `runtime` when the sidecar is not needed.
- Free-tier availability depends on the upstream's current policy and can change without notice.
- Full tool schemas add request-body size on every upstream call.

---

## Extensions & store

AuroraX loads portable **extensions** (JSON) that configure the sidecar and Session Hub in one apply. Extensions ship tool schemas, identity headers, UA/auth defaults, and optional provider types (`provides.provider_types`).

- Install from **aurorax-store** (**Settings → Sidecar → Browse store**) or import by URL (`POST /sidecar/extensions/import`).
- Apply activates sidecar settings + optional provider types; headers are installed via `ensure-headers`.
- The gateway does **not** bundle third-party free-tier policy; use the matching store extension if you need it.

> **⚠️ Use at your own risk.** Review third-party extensions before installing them from a store.

---

## Quick Start

Start routing AI traffic in 60 seconds.

> **Recommended — Docker (published image):**
> ```bash
> docker pull entbtw/aurora:latest
> docker run -d --name aurora -p 8080:8080 -e AURORA_MASTER_KEY="your-secure-key" entbtw/aurora:latest
> ```
> Full examples below. For production (persistent state, multi-IP) see the [Deployment guide](documentation/DEPLOYMENT.md).

The quickest way to configure providers from scratch is the dashboard: **http://localhost:8080/admin/dashboard → Providers → Add provider**. For env-var driven setups:

### Option A — inline env vars

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

### Option B — Docker

> Published image: **`entbtw/aurora`** · tags `latest`, `v1.1.2`.
> ```bash
> docker pull entbtw/aurora:latest
> ```

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
  entbtw/aurora:latest
```

For production setups (persistent config/state, multi-IP host networking) see the [Deployment guide](documentation/DEPLOYMENT.md).

### Verify it's alive

After starting, confirm the gateway is up and the dashboard loads:

```bash
# Health check
curl -s http://localhost:8080/health

# Dashboard
open http://localhost:8080/admin/dashboard

# Session Hub status (should show storage_mode: disk or memory)
curl -s http://localhost:8080/admin/api/v1/sessionhub/status \
  -H "Authorization: Bearer your-master-key"
```

If health returns `{"status":"ok"}` — the gateway is running. Now add a provider via the dashboard or env vars, then test a model call:

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

**Docs (this fork):** [Getting Started](documentation/GETTING_STARTED.md) · [Deployment](documentation/DEPLOYMENT.md) · [Session Hub](documentation/SESSION_HUB.md) · [Docker image](documentation/DOCKER_PUSH.md)

**Source:** [github.com/entitybtw/aurora](https://github.com/entitybtw/aurora) · **Image:** [hub.docker.com/r/entbtw/aurora](https://hub.docker.com/r/entbtw/aurora)

Base project (upstream): [aurorallm/aurora](https://github.com/aurorallm/aurora) · [aurorallm.online/docs](https://aurorallm.online/docs)

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

YAML provider config supports additional options:

```yaml
providers:
  openai:
    type: openai
    api_key: "${OPENAI_API_KEY}"
    base_url: "https://api.openai.com/v1"
    # Custom User-Agent header for upstream attribution
    user_agent: "MyApp/1.0"
    # Disable auto-fetching models from /models endpoint (use only configured list)
    auto_fetch_models: false
    models:
      - gpt-4o
      - gpt-4o-mini
    # Optional: narrow auto-discovered models to those matching these conditions.
    # Filtered-out models are not registered and cannot be routed to.
    autofetch_filter:
      mode: all            # all (AND, default) | any (OR)
      conditions:
        - contains: "free"     # keep only IDs containing this substring
```

#### Auto-fetch filter

`autofetch_filter` narrows the model list discovered from a provider's `/models`
endpoint. Models that do not match the conditions are **dropped before
registration** — they never appear in `/v1/models` and cannot be called.

```yaml
providers:
  openrouter:
    type: openrouter
    api_key: "${OPENROUTER_API_KEY}"
    autofetch_filter:
      mode: all
      conditions:
        - contains: "free"          # substring match on the model ID
        - not_contains: "preview"   # drop these
        # - regex: "^[a-z]+/.*:free$"
        # - max_price: 0            # free models only (input + output)
        # - max_prompt_price: 0
        # - max_completion_price: 0
```

Common recipes:

| Goal | Filter |
|------|--------|
| Only OpenRouter free tier | `conditions: [{contains: ":free"}]` |
| Only zero-price models | `conditions: [{max_price: 0}]` |
| Any of several families | `mode: any` + one `contains` per family |

Notes:

- Substring conditions are case-insensitive. Conditions inside a single entry are
  ANDed; entries combine according to `mode`.
- Price conditions require pricing metadata. A model **without** pricing is
  rejected rather than assumed free — `max_price: 0` can never silently expose a
  paid model.
- Pools can override member filters with a pool-level `autofetch_filter`.
- Invalid configuration (bad regex, unknown mode, empty condition) is reported in
  the logs with the provider name and leaves that provider unfiltered.
- The same setting is available in the dashboard: **Providers → Edit → Auto-fetch
  filter** (comma-separated substrings, e.g. `free, flash`).

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
OPENROUTER_SITE_URL=https://github.com/entitybtw/aurora
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

Run the built binary directly (from source: `go build -o aurora ./apps/aurora`, then `./aurora`). The npm `iaurora` wrapper is the upstream package and isn't republished by this fork.

| Command | Description |
|---------|-------------|
| `aurora` | Start the gateway server (default port 8080) |
| `aurora init` | Scaffold `config.yaml`, `.env`, `data/` in current directory |
| `aurora bindproxy` | Run a single-IP HTTP CONNECT proxy (uses `BIND_IP` + `PROXY_LISTEN`) — started automatically per egress IP by the sidecar entrypoint |
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
├── apps/              # Application entrypoints (gateway, aurora bindproxy subcommand)
├── internal/          # Core packages (providers, gateway, storage, guardrails, sessionhub, admin)
│   └── providers/sidecarclient/sidecar/  # Bun sidecar: adapter.js, one-shot.js, default-tools.json
├── dashboard-ui/      # React admin dashboard (Vite)
├── configs/           # Configuration profiles and examples
├── documentation/     # Markdown docs (Getting Started, Deployment, Session Hub, Docker)
├── docs-assets/       # Images, models.json, assets
├── monitoring/        # Prometheus + Grafana configs
├── bench-results/     # Benchmark data
├── release/           # Release scripts
├── scripts/           # Build and utility scripts
├── build/             # docker-entrypoint.sh (starts bind proxies + sidecar)
├── Dockerfile         # Multi-stage: runtime / runtime-sidecar targets
├── Dockerfile.runtime # Local-artifact build (build/aurora + dashboard dist)
└── Dockerfile.builder # Combined Go+Bun builder image
```

## License

This project is licensed under the Apache 2.0 License — see the [LICENSE](LICENSE) file for details.

Community fork of [Aurora](https://github.com/aurorallm/aurora). Session Hub features and multi-account integration built by [entitybtw](https://github.com/entitybtw/aurora). The upstream project is built by the Aurora team.

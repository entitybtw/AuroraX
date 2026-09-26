# Sidecar — extension-driven TLS fingerprint proxy

The sidecar routes upstream traffic through a bundled **Bun** process when an
extension supplies the base URL, User-Agent, auth and tool scope. It reproduces
a client fingerprint Go cannot (BoringSSL ClientHello), runs inside the
`runtime-sidecar` image variant, and is managed from **Settings → Sidecar**.



Upstream-specific free-tier details live in a **store extension** (not built into the gateway). Install via **Settings → Sidecar → Browse store** or import by URL.

### Why a sidecar exists

Some upstream edges apply **layered** client verification. A request must pass **all** of these simultaneously:

| Layer | Check | Why Go alone fails |
|-------|-------|--------------------|
| 1. TLS fingerprint | Ja3/Ja4 ClientHello must match the extension's expected profile (e.g. Bun BoringSSL) | Go `net/http` and even Chrome-impersonating uTLS may **not** match |
| 2. HTTP headers | User-Agent + identity headers declared by the extension | Header shape must match exactly |
| 3. Tool schema | Body must carry the extension's full tool schema + `tool_choice: auto` (if required) | Partial tool sets can be rejected |
| 4. Streaming | Some free tiers only answer `stream: true` | Non-streaming requests must be re-aggregated |
| 5. Auth | Extension-supplied default auth or a real bearer token | Dashboard API keys may be rejected |
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
   │  default auth / bearer token         ← layer 5
   ▼
upstream /v1/chat/completions  → 200
```

The sidecar always streams upstream; when the caller asked for a non-streaming response it aggregates the SSE chunks back into a single `chat.completion` object (falling back to `reasoning_content` when a reasoning model emits no visible `content`).

### What works

- **Extension-driven fingerprints** — install a free-tier extension from the store to supply base URL, UA, headers, tools and retries
- **Paid / API-key traffic** — the sidecar forwards the incoming `Authorization` untouched, so bearer tokens and API keys keep working through the same path
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


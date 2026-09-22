# Deploying AuroraX (this fork)

Aurora is a single static Go binary, served inside a distroless Docker image. This fork is published as **`entbtw/aurora`** on Docker Hub. The dashboard UI and the Session Hub are compiled into the binary, so a fresh pull gives you everything.

## Quick start (Docker Hub image)

```bash
docker pull entbtw/aurora:latest
```

Minimal container (one-off, in-memory state):

```bash
docker run -d --name aurora -p 8080:8080 \
  -e AURORA_MASTER_KEY="your-secure-key" \
  entbtw/aurora:latest
```

Open:
- API base: `http://localhost:8080/v1`
- Dashboard: `http://localhost:8080/admin/dashboard`
- Health: `http://localhost:8080/health`

> The image **Exposes port 8080**. The container itself stores short-lived data under `/app/data` by default — mount volumes (below) for anything you want to keep.

## Production with persistent config & state

This fork keeps everything you manage in files under `configs/`, and runtime/DB state under a data dir. Mount both with volumes. Use `AURORA_CONFIG_PATH` to tell Aurora where the main config is; override files land next to it.

Example `docker-compose.yml`:

```yaml
services:
  aurora:
    image: entbtw/aurora:latest
    container_name: aurora-gateway
    restart: unless-stopped
    network_mode: host     # <-- required for per-provider bind_ip (multi-IP outbound)
    working_dir: /app      # <-- REQUIRED: distroless image defaults to /home/nonroot
    volumes:
      - ./aurora-data:/app/data
      - ./configs:/app/configs
    environment:
      PORT: 7841           # host networking listens directly on this port
      ADMIN_ENDPOINTS_ENABLED: "true"
      ADMIN_UI_ENABLED: "true"
    env_file:
      - .env
```

> **Important:** `working_dir: /app` is required. The distroless runtime image defaults the working directory to `/home/nonroot`, but the binary reads config files (`configs/provider-overrides.json`, `configs/pool-overrides.json`, etc.) relative to `/app`. Without this line, provider overrides silently fail to load on startup.

```bash
docker compose up -d
```

## Environment variables (common)

| Env | Purpose |
|-----|---------|
| `AURORA_MASTER_KEY` | Master key for admin API + dashboard auth. **Required.** |
| `AURORA_CONFIG_PATH` | Path to main config; overrides live next to it. |
| `PORT` | HTTP listen port. (Useful with host networking.) |
| `ADMIN_ENDPOINTS_ENABLED` | Enable `/admin/api/v1`. |
| `ADMIN_UI_ENABLED` | Serve the dashboard UI. |
| `STORAGE_TYPE` | `sqlite` (default) or `postgresql`. |
| `SQLITE_PATH` | sqlite file (default `data/aurora.db`). |
| `POSTGRES_URL` | PostgreSQL DSN when `STORAGE_TYPE=postgresql`. |
| `REDIS_URL` | Optional, for model/response cache. |
| `METRICS_ENABLED` | Expose Prometheus on `/metrics`. |

Provider keys use per-type env vars (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `VLLM_API_KEY`, …) — see the README or `.env` examples.

## What lives where

| Path | Contents |
|------|----------|
| `/app/configs/config.yaml` | Main config (providers, pools, resilience…). |
| `/app/configs/dashboard-overrides.yaml` | Settings edited in the dashboard. |
| `/app/configs/provider-overrides.json` | Providers created via the UI. |
| `/app/configs/pool-overrides.json` | Pools created via the UI. |
| `/app/configs/fallback.json` | Manual fallback rules (optional). |
| `/app/configs/session-hub-rules.yaml` | Session Hub rules. |
| `/app/configs/session-hub-mappings.json` | Persisted session mappings (storage mode = `disk`). |
| `/app/configs/sidecar-overrides.json` | Sidecar runtime settings (opencode image only). |
| `/app/data/` | SQLite DB, model list cache, pool counters, instance id. |

Mount `./configs` and `./data` (or custom dirs) so none of this is lost across container recreation.

## Multi-IP outbound (per-provider `bind_ip`)

Providers can pin their outbound source IP with `bind_ip`. This requires Aurora to bind a specific local source address on the host — which only works with **`network_mode: host`** (the default bridge can't bind arbitrary host source IPs).

```yaml
# provider-overrides.json (or config.yaml)
{
  "name": "vllm-acc-1",
  "type": "vllm",
  "base_url": "https://opencode.ai/zen/v1",
  "api_key": "sk-...",
  "bind_ip": "193.23.210.27",     # residential/egress IP on the host
  "pool_only": true,
  "models": "mimo-v2.5-free"
}
```

Pair with host networking so those IPs route correctly, and a pool that load-balances the accounts round-robin:

```json
{
  "pools": {
    "opencode-zen": {
      "Members": ["vllm-acc-1", "vllm-acc-2", "vllm-acc-3"],
      "Strategy": "round_robin",
      "HealthAware": true
    }
  }
}
```

## Sanity checks after deploy

1. `curl http://localhost:8080/health` → healthy.
2. Dashboard loads and shows your providers/pools.
3. A chat to a model returns 200 (see GETTING_STARTED for the Python example).
4. Session Hub tab loads; `GET /admin/api/v1/sessionhub/status` reports your `storage_mode`.
5. Sidecar tab loads (opencode image only); `GET /admin/api/v1/sidecar` reports enabled/bind proxies.

If the Session Hub isn't transforming headers the way you expect, re-check the rule Name matches the provider/pool name and that storage mode matches your intent (see SESSION_HUB.md).

## OpenCode sidecar (free-tier image variant)

The default image is distroless and has no sidecar. For OpenCode free-tier routing, use the `runtime-sidecar` image variant, which ships the Bun sidecar + per-IP CONNECT proxies.

> **⚠️ Use at your own risk.** The sidecar emulates the OpenCode client fingerprint. Upstream hardening can break it at any time and it may violate the provider's terms of service.

```bash
docker pull entbtw/aurora:sidecar   # if published; otherwise build --target runtime-sidecar
```

```yaml
services:
  aurora:
    image: entbtw/aurora:sidecar
    container_name: aurora-gateway
    restart: unless-stopped
    network_mode: host     # required: bind proxies must bind host egress IPs
    working_dir: /app
    volumes:
      - ./aurora-data:/app/data
      - ./configs:/app/configs
    environment:
      PORT: 7841
      ADMIN_ENDPOINTS_ENABLED: "true"
      ADMIN_UI_ENABLED: "true"
      AURORA_SIDECAR_ENABLED: "true"
      # Comma-separated egress IPs; one CONNECT proxy (8981+) is started per IP
      AURORA_SIDECAR_BIND_IPS: "203.0.113.10,203.0.113.11"
      # Provider-side routing target (Go); sidecar reads AURORA_SIDECAR_UPSTREAM_URL
      AURORA_SIDECAR_BASE_URL: "http://127.0.0.1:8090/v1"
    env_file:
      - .env
```

- Entrypoint starts `aurora bindproxy` once per `AURORA_SIDECAR_BIND_IPS` entry (ports `8981+`), then the Bun sidecar on `AURORA_SIDECAR_PORT` (default `8090`).
- The provider sets `bind_ip`; the request carries `x-aurora-bind-ip` so the sidecar picks the matching proxy. Bun does end-to-end TLS inside the tunnel — the proxy only binds the source IP.
- Sidecar runtime settings persist to `configs/sidecar-overrides.json` and are editable from **Settings → Sidecar** (or `GET`/`PUT /admin/api/v1/sidecar`).
- Env vars with suffix `SIDECAR`/`BIND` are reserved and ignored by provider auto-discovery (they cannot become phantom providers).

Full reference: see the **OpenCode Sidecar** section of the README.

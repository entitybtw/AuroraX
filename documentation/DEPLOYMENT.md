# Deployment

Aurora is a single binary that you run behind Docker, systemd, or bare. This fork adds persistent provider/pool state and a Session Hub, all of it managed from the dashboard and config files.

## Pre-built Docker image

```bash
docker pull entbtw/aurora:latest
```

Images are published for `linux/amd64`, `linux/arm64`, and `linux/arm/v7`.

| Tag | Meaning |
|-----|---------|
| `entbtw/aurora:latest` | latest stable |
| `entbtw/aurora:v1.0.0` | pinned full release |

## Docker run (minimal)

```bash
docker run -d --name aurora -p 7841:8080 \
  -e AURORA_MASTER_KEY="your-secure-key" \
  -e AURORA_CONFIG_PATH="/app/configs/config.yaml" \
  -v "$(pwd)/configs:/app/configs" \
  -v "$(pwd)/data:/app/data" \
  entbtw/aurora:latest
```

> **Networking note:** the gateway binds HTTP on the container port (Expose 8080). Map it to your host port as needed. For multi-IP outbound routing (per-provider `bind_ip`) run with `network_mode: host` so the outbound connections can bind the host's source addresses — see below.

## Docker Compose (host network, recommended for multi-IP)

```yaml
services:
  aurora:
    image: entbtw/aurora:latest
    container_name: aurora-gateway
    restart: unless-stopped
    network_mode: host          # lets providers bind host source IPs
    volumes:
      - ./aurora-data:/app/data
      - ./configs:/app/configs
    environment:
      PORT: 7841                # host network => listens directly on this port
    env_file:
      - .env
```

## Key environment variables

| Env | Purpose |
|-----|---------|
| `AURORA_MASTER_KEY` | Master key for admin API + auth. **Required.** |
| `ADMIN_ENDPOINTS_ENABLED` | Enable the `/admin/api/v1` API (`true`). |
| `ADMIN_UI_ENABLED` | Serve the dashboard UI (`true`). |
| `STORAGE_TYPE` | `sqlite` or `postgresql`. |
| `SQLITE_PATH` | sqlite db path (default `data/aurora.db`). |
| `AURORA_CONFIG_PATH` | Path to `config.yaml`; when set, runtime override files (`dashboard-overrides.yaml`, `session-hub-rules.yaml`, `session-hub-mappings.json`) are placed next to it. |

## Where state lives

| File | Purpose |
|------|---------|
| `configs/config.yaml` | Main config (providers, pools, resilience…). |
| `configs/dashboard-overrides.yaml` | Settings edited from the dashboard. |
| `configs/provider-overrides.json` | Providers created via the UI. |
| `configs/pool-overrides.json` | Pools created via the UI. |
| `configs/fallback.json` | Manual fallback rules (optional). |
| `configs/session-hub-rules.yaml` | Session Hub rules (edited in UI or API). |
| `configs/session-hub-mappings.json` | Persisted session mappings when storage mode is `disk`. |

All of these are gitignored — they are runtime/operator state, not source.

## Completeness check after a fresh clone+deploy

1. Gateway responds on its port.
2. Dashboard loads at `/admin/dashboard`.
3. Providers and pools appear and are routable.
4. The Session Hub storage toggle reflects the mode you choose.

See `GETTING_STARTED.md` for a from-scratch local run and `SESSION_HUB.md` for configuration the session mapping engine.

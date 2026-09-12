# Session Hub

The Session Hub is a header-transformation engine used in API-integration setups where an upstream service expects each logical client to present a consistent, unique session identifier — especially across multiple accounts in a load-balanced pool.

It is built to be fast: the hot path is a single lock-free map read (microns). Configuration persists, and session mappings can live in memory or on disk (toggleable).

> For a complete end-to-end walkthrough (accounts in a pool, each with its own stable client session), see [MULTI_ACCOUNT.md](MULTI_ACCOUNT.md).

## Concepts

**Rules** bind a set of header transformations to a *target* — a provider name, a pool name, a fallback, or the wildcard `*`. A pool-bound rule automatically applies to every member provider because the rule index is pre-expanded at write time.

**Mappings** record `inbound → outbound` session overrides produced on the fly. For a single inbound client session, each provider in a pool gets its own stable outbound session, and repeated use reuses the stored value instead of generating a new one.

## Header modes

| Mode | Behavior |
|------|----------|
| `map` | Request1 → generate a unique outbound value per provider, remember it. Later requests with the same inbound reuse the stored value. |
| `map_or_generate` | Like `map`, but when the inbound header is absent a fresh value is generated instead of being skipped. Ideal for clients like the OpenCode CLI that only sometimes send a session header. |
| `generate` | Fresh random value on every request. |
| `passthrough` | Forward the original value unchanged. |
| `static` | Fixed value (config via `value`). |
| `random_from_list` | Pick randomly among `values`. |
| `remove` | Strip the header. |

## Configuration file

Rules live in `configs/session-hub-rules.yaml` and are rewritten automatically when you edit rules (via API or dashboard).

```yaml
enabled: true
mapping_storage: disk          # "memory" or "disk"
providers:
  opencode-zen:                # bind to a pool or provider name
    enabled: true
    headers:
      - name: x-opencode-session
        mode: map              # stable inbound→outbound per provider
        prefix: "ses_"
        length: 28
      - name: x-opencode-client
        mode: static
        value: cli
      - name: user-agent
        mode: static
        value: "opencode/1.18.26 ai-sdk/openai/2.0.0 runtime/bun/1.0.0"
```

### How a request flows

1. Client sends a request (e.g. with `x-opencode-session: ses_abc`).
2. Aurora snapshots the inbound session-scoped header onto the request context.
3. The request routes to a pool member (e.g. `opencode-zen` → `vllm-zen-backup`).
4. That provider's outbound `headerSetter` fires.
5. Any inbound session header is copied onto the outbound request so the hub can act on it.
6. The rule for that provider/pool runs:
   - `map` → `ses_abc` becomes a unique `ses_R4f8k2m…` for this provider and is remembered.
   - extra headers (`x-opencode-client`, `user-agent`, …) are injected/rewritten.
7. The transformed request is sent upstream.

## Mapping storage (memory vs disk)

Mappings can be kept only in memory (reset on restart) or persisted to `configs/session-hub-mappings.json` and restored after a restart. Persistence keeps each account on the same outbound session across gateway restarts.

Toggle from the dashboard (**Settings → Session Hub → Live Mappings → "Persist to disk"**) or via the API:

```bash
curl -X PUT http://localhost:8080/admin/api/v1/sessionhub/storage \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"mode":"disk"}'
```

`mode` accepts `memory` or `disk`. The dashboard shows the active mode and explains the current policy for mappings.

## Admin API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/admin/api/v1/sessionhub/status` | Stats incl. `storage_mode` |
| `GET` | `/admin/api/v1/sessionhub/providers` | List bound rules |
| `POST` | `/admin/api/v1/sessionhub/providers` | Create rule `{name, rule}` |
| `GET` | `/admin/api/v1/sessionhub/providers/:name` | Get one rule |
| `PUT` | `/admin/api/v1/sessionhub/providers/:name` | Update rule |
| `DELETE` | `/admin/api/v1/sessionhub/providers/:name` | Delete rule |
| `GET` | `/admin/api/v1/sessionhub/mappings` | List live mappings |
| `DELETE` | `/admin/api/v1/sessionhub/mappings` | Clear all mappings |
| `DELETE` | `/admin/api/v1/sessionhub/mappings/:provider` | Clear a provider's mappings |
| `PUT` | `/admin/api/v1/sessionhub/storage` | Toggle `memory`/`disk` |
| `POST` | `/admin/api/v1/sessionhub/apply` | Dry-run transform for validation |

### Example: create a rule

```bash
curl -X POST http://localhost:8080/admin/api/v1/sessionhub/providers \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "opencode-zen",
    "rule": {
      "enabled": true,
      "headers": [
        {"name": "x-opencode-session", "mode": "map", "prefix": "ses_", "length": 28},
        {"name": "x-opencode-client",  "mode": "static", "value": "cli"}
      ]
    }
  }'
```

## Dashboard

Open **Settings → Session Hub**. It shows:

- An overview of every pool/provider registered on the server, with whether each already has a rule bound (check-mark = bound).
- **Add Rule** — pick target type (pool/provider/fallback/all) then choose the exact target from the live list; the rule Name fills in automatically.
- **Header Rules** — add/remove rules with per-mode options (prefix, length, static value, random list).
- **Live Mappings** — real-time inbound→outbound mappings per provider, plus the Memory/Disk persistence toggle and a **Clear All** action.

Rules added here are applied automatically without a process restart and are saved to disk.

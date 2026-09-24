# Presets

A **preset** (extension with sidecar fields, usually `"type": "sidecar"`) is a
portable recipe for routing traffic through the TLS sidecar and optionally
installing Session Hub header rules and provider overrides.

## What a preset can configure

| Area | Fields |
|------|--------|
| Upstream | `base_url`, `settings.base_url`, `settings.path_template`, `settings.models_path` |
| Client identity | `user_agent`, `default_auth`, `settings.user_agent` |
| Tools | `inject_tools`, `inject_tool_types`, `tool_schemas`, `files` (materialized tool JSON) |
| Retries | `max_attempts`, `retry_delay_ms`, `settings.retry_statuses` |
| Headers (upstream) | `settings.extra_headers` (JSON object string) |
| Headers (client→sidecar) | `settings.forward_headers` (JSON array of header names) |
| Streaming | `settings.force_stream` (`"true"` / `"false"`) |
| Session Hub | `headers[]` rules installed on apply / full-apply |
| OAuth | `oauth`, `settings.oauth_*`, `provides.features: ["oauth"]` |
| Providers | `provides.provider_types` activation |

### Settings keys

| Key | Meaning | Default |
|-----|---------|---------|
| `path_template` | Path appended to `base_url` for chat | `/chat/completions` |
| `models_path` | Path for `GET …/models` | `/models` |
| `extra_headers` | Always-sent upstream headers (JSON object) | — |
| `forward_headers` | Extra inbound headers to forward (JSON array) | identity headers only |
| `retry_statuses` | Retry status set (JSON array of ints) | `403, 429` |
| `force_stream` | Force `stream: true` | force only when inject_tools |
| `tools_path` | Absolute/relative tool schema path | bundled default |
| `default_auth` | Authorization scheme | `Bearer public` |
| `user_agent` | Override UA | neutral / extension value |
| `oauth_server`, `oauth_client_id`, `oauth_verification_base` | Device-flow wiring | — |

JSON-valued settings must be **strings containing JSON**, e.g.
`"extra_headers": "{\"X-Tenant\":\"acme\"}"`.

## Minimal example

```json
{
  "schema": 1,
  "id": "example-upstream",
  "name": "Example Upstream",
  "type": "sidecar",
  "base_url": "https://api.example.com/v1",
  "user_agent": "example/1.0",
  "settings": {
    "path_template": "/chat/completions",
    "extra_headers": "{\"X-Tenant\":\"acme\"}",
    "retry_statuses": "[429,503]"
  },
  "headers": [
    { "name": "x-session", "mode": "map_or_generate", "prefix": "s_", "length": 16, "charset": "hex" }
  ]
}
```

## Apply flow

1. **Import** → stored in the extension list (not applied yet).
2. **Apply** → writes sidecar settings, materializes `files`, sets
   `Applied=true`, activates `provides.provider_types`, optionally enables
   OAuth on matching providers (`provides.features` contains `"oauth"`).
3. **Full apply** → same, plus binds `headers[]` to a chosen Session Hub
   target (pool/provider) and installs rules idempotently.
4. **Unapply** → clears `Applied` (drops UI contribution). Persisted sidecar
   JSON is a single active settings object — the next apply overwrites it.

## Multi-IP / proxies

Sidecar `bind_ips` and per-IP CONNECT proxies are separate from the extension
JSON (environment + Settings → Sidecar). Presets describe *where* to send
traffic and *how* it looks upstream*; binding IPs stay an operator concern.

## Bypass-style profiles

Bypass profiles are ordinary presets: they set `base_url`, UA, tool
injection, and Session Hub header rules. Keep credential material out of the
JSON; use operator-configured keys on the host.

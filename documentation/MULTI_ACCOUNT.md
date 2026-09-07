# Multi-account API pool with distinct client identities

Aurora lets you load-balance several upstream API accounts under one model name and, via the Session Hub, present each account with a distinct, stable client session. This is useful when an upstream expects requests to look like separate clients rather than one shared integration.

High level: **client → Aurora (one URL) → pool round-robin → each account gets its own session**

## 1. Register the accounts as providers

Each account is a provider of the same `type`, at the same `base_url`, with its own `api_key`. Optional `pool_only` hides them from the public model list. If you want each to egress from its own IP, set per-provider `bind_ip` and run Aurora with `network_mode: host` (see DEPLOYMENT.md).

`provider-overrides.json` (or `config.yaml`):

```json
[
  {
    "name": "acc-1",
    "type": "vllm",
    "base_url": "https://opencode.ai/zen/v1",
    "api_key": "sk-...",
    "models": "mimo-v2.5-free",
    "pool_only": true,
    "bind_ip": "203.0.113.10"
  },
  {
    "name": "acc-2",
    "type": "vllm",
    "base_url": "https://opencode.ai/zen/v1",
    "api_key": "sk-...",
    "models": "mimo-v2.5-free",
    "pool_only": true,
    "bind_ip": "203.0.113.11"
  }
]
```

## 2. Group them in a pool

`pool-overrides.json`:

```json
{
  "pools": {
    "opencode-zen": {
      "Members": ["acc-1", "acc-2"],
      "Strategy": "round_robin",
      "HealthAware": true
    }
  }
}
```

Now `opencode-zen/mimo-v2.5-free` routes to `acc-1` or `acc-2` round-robin.

## 3. Give each account a distinct, stable client identity (Session Hub)

Add a Session Hub rule bound to the pool. The Session Hub runs on every outbound request to a member and rewrites identity headers.

`session-hub-rules.yaml`:

```yaml
enabled: true
mapping_storage: disk            # keep the same session per account across restarts
providers:
  opencode-zen:                  # pool name — applies to all members
    enabled: true
    headers:
      - name: x-opencode-session
        mode: map                # inbound → unique stable value per provider
        prefix: ses_
        length: 28
      - name: x-opencode-client
        mode: static
        value: cli
      - name: user-agent
        mode: static
        value: "opencode/1.18.26 ai-sdk/openai/2.0.0 runtime/bun/1.0.0"
```

Effect:

| inbound from your CLI | routed to | outbound session sent upstream |
|-----------------------|-----------|-------------------------------|
| `ses_client_X` | `acc-1` | `ses_A1b2C3…` (stable) |
| `ses_client_X` (next) | `acc-2` | `ses_D4e5F6…` (stable, different) |

Each account therefore keeps its **own** outbound session that is reused on later calls and preserved across restarts (in `disk` mode) — matching the multi-account-unique-clients pattern.

> `map` keeps inbound→outbound stable and deduplicated. Want a brand-new session per request instead? Use `mode: generate`.

## 4. Call through the pool

Standard OpenAI SDK against one URL:

```python
from openai import OpenAI

client = OpenAI(base_url="http://localhost:8080/v1", api_key="your-aurora-key")
r = client.chat.completions.create(
    model="opencode-zen/mimo-v2.5-free",
    messages=[{"role": "user", "content": "hello"}],
)
print(r.choices[0].message.content)
```

The gateway picks a member, applies the pool's header rule, and forwards.

## 5. Manage it from the dashboard

**Settings → Session Hub** shows every pool/provider and whether each already has a rule bound (check-mark = bound). **Add Rule** lets you target a specific pool/provider/fallback, and **Live Mappings** shows the actual inbound→outbound pairs with the Memory/Disk toggle.

## Verification

- `GET /admin/api/v1/sessionhub/status` → shows `storage_mode` and per-provider mapping counts.
- `GET /admin/api/v1/sessionhub/mappings` → list the live inbound→outbound records (one per provider).
- Issue the same model call twice — the two members should show **different** outbound sessions, proving each account presents as a distinct client, and the record count stays at one per account (dedup).

See [SESSION_HUB.md](SESSION_HUB.md) for the full engine reference.

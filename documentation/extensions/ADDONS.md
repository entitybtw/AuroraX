# Addons

Addons extend the gateway with capabilities that are **not** always compiled
into a minimal core. In this model, optional behavior is shipped as extension
JSON first; runtime Go hooks are layered on without baking provider secrets or
vendor-specific flows into default builds.

## What counts as an addon today

| Mechanism | How it ships | Effect |
|-----------|--------------|--------|
| `provides.provider_types` | Extension JSON | Registers optional provider factory types when applied |
| `provides.features` | Extension JSON | Surfaces features (e.g. `external_auth`) and activates related wiring |
| `tool_schemas` / `files` | Extension JSON | Injects tool JSON into upstream requests or materializes scripts on disk |
| `auth` block | Extension JSON | Device-flow or authorization-code (+ PKCE) endpoints wired through an auth addon |
| UI contributions | `ui.*` | Pages, settings tabs, banners, widgets |
| Runtime scripts | `files` → `configs/addons/*.go` | Single-file Go addons evaluated by Yaegi (`internal/addon`) |
| Hook kit | `internal/hooks` | Lifecycle, egress, retry and live-data hooks dispatched to `runtime`/`ui`/`preset` addons |
| Exit rotation | `bind_ips` / `egress_strategy` / `egress_disabled` + `EgressCandidates` | Several exits per provider, grouped into tiers and picked per attempt |

### Example: external auth feature without core vendor defaults

```json
{
  "schema": 1,
  "id": "example-auth",
  "name": "Example Auth",
  "type": "sidecar",
  "provides": { "features": ["external_auth"] },
  "auth": {
    "server": "https://auth.example.com",
    "grant": "authorization_code",
    "authorize_url": "https://auth.example.com/authorize",
    "token_url": "https://auth.example.com/token",
    "token_style": "json",
    "client_id": "public-cli",
    "scopes": "openid profile",
    "redirect_uri": "http://127.0.0.1:54545/callback"
  }
}
```

The gateway applies `auth_method=external` only to providers that match the
extension's provider types or `base_url` — endpoints come from the extension,
not from compiled defaults. Device flow remains available when `grant` is
omitted or `"device"`.

## External auth routes (auth addon contract)

Applying an extension with `provides.features: ["external_auth"]` mounts
`/admin/api/v1/external-auth/*`. The gateway ships no grant flows of its own:
each route is a pure proxy that forwards one call to the first auth addon
answering successfully. Until an addon is loaded every route answers 404 with
`external auth not enabled: apply an extension that ships an auth addon`.

Each call is a single Yaegi function `Method(payload string) string`: the
payload is one JSON string (`{"provider": "<name>", ...raw body}`) and the
reply is one JSON string relayed to the client.

| Addon method | Route | Purpose |
|--------------|-------|---------|
| `Providers` | `GET /external-auth/providers` | Token status for every provider the addon owns |
| `Start` | `POST /external-auth/:provider/start` | Begin a device flow |
| `Poll` | `POST /external-auth/:provider/poll` | Exchange a device code once |
| `TokenStatus` | `GET /external-auth/:provider/status` | Current token state |
| `Refresh` | `POST /external-auth/:provider/refresh` | Refresh one provider's token |
| `RefreshAll` | `POST /external-auth/refresh` | Refresh every stored token (merged list call) |
| `ClearToken` | `DELETE /external-auth/:provider/token` | Drop a stored token |
| `FlowInfo` | `GET /external-auth/:provider/flow` | Active grant (`device` or `authorization_code`) |
| `StartAuthorize` | `POST /external-auth/:provider/authorize` | Create a PKCE session, return authorize URL |
| `CompleteAuthorize` | `POST /external-auth/:provider/authorize/complete` | Exchange a pasted/redirected code |
| `Token` | *(no route)* | Bearer token injected into upstream headers |

Replies: a Go-level addon error maps to 502; a JSON object with `error` and no
`status` maps to 400; anything else relays as 200. The first successful addon
reply is authoritative — other auth addons are not consulted.

## Runtime addons (Yaegi)

`internal/addon` loads **single-file** Go scripts from `configs/addons/`
(override with the store directory). Each file is evaluated by
[Yaegi](https://github.com/traefik/yaegi) in-process:

```go
// addon-kind: theme
package main

func Accent() string { return "#7aa2f7" }
```

- Declare kind with `// addon-kind: theme|preset|ui|auth|runtime` (first
  comment lines) or by filename prefix (`theme-*.go`, `auth-*.go`, …).
- Only `*.go` under the addons directory are accepted — `..` and absolute
  escapes are rejected.
- Load errors are recorded and skipped; a broken addon never blocks gateway
  startup.
- Treat `configs/addons/` as trusted operator configuration (same trust level
  as `configs/*.yaml`).

### Hook kit (`internal/hooks`)

Auth grant flows keep their own bridge (the table above). Everything else goes
through `internal/hooks`, which dispatches to `runtime`, `ui` and `preset`
addons with the same contract: **one JSON string in, one JSON string out**.

An addon opts in by exporting the hook name. An addon that does not export it
is skipped, not treated as an error — that is what lets several extensions
coexist without knowing about each other. A reply of `{"skip": true}` means
"not mine" and the search continues with the next addon.

| Hook | Kind | Purpose |
|------|------|---------|
| `OnInit` | `runtime` | The addon was loaded (after apply, or at startup) |
| `OnSettingsSave` | `runtime` | The operator changed this extension's settings |
| `OnApply` / `OnUnapply` | `runtime` | The extension was enabled or disabled |
| `OnTick` | `runtime` | Periodic best-effort tick |
| `EgressCandidates` | `runtime` | Ask for the exits this extension contributes (see below) |
| `RetryPolicy` | `preset` | Per-request retry/routing decision |
| `Data(key)` | `ui` / `runtime` | One live payload for a page or widget |
| `UI()` | `ui` | An `ExtensionUI` document computed at request time |

`Fire` (events) is best-effort: a failing addon is logged and never breaks the
request or the lifecycle transition that triggered it.

#### Payload

Every hook receives one JSON object. The gateway merges the caller's fields
with the owning extension's identity and settings, so an addon reads its own
keys and intervals without touching the extension store:

```json
{
  "hook": "EgressCandidates",
  "extension": "vpn-egress",
  "dir": "configs/extensions/vpn-egress",
  "provider": "zen-main",
  "settings": { "node_count": "3" },
  "config":   { "subscriptions": "https://…" }
}
```

`settings` holds the values shipped in the manifest; `config` holds the
operator's saved values for `ui.fields` and always wins when both define a key.
`dir` is where the addon may keep state (`os.WriteFile` works — the stdlib is
linked, `os/exec` is not).

#### Contributing exits (`EgressCandidates`)

```json
{"candidates":[{"name":"vpn:node-1","proxy":"socks5://127.0.0.1:1080","tier":0}]}
```

Each entry is either a `proxy` URL (`http`, `https`, `socks5`, `socks5h`) or a
`bind_ip` source address, plus an optional `weight` and `tier`. Entries that
omit `tier` default to **1** (the fallback tier).

Providers rotate between exits per attempt, and a failed attempt moves on to
the next one instead of repeating the failure. Exits are grouped in tiers and
the lowest usable tier is picked; the tiers are:

| Tier | Who sets it | Meaning |
|------|-------------|---------|
| `-1` | extension (`tier: -1`) | Used ahead of the provider's own source addresses; those addresses are the last resort. |
| `0` | the provider (`bind_ips`) and extensions that ask for it | One rotation: configured source addresses and extension exits are picked from together. |
| `1` (default) | extension | Fallback: reached once tier 0 is unusable — every address is cooling down, or the tier was demoted by a rate limit (429/403). A demoted tier probes back on its own after a short window. |

So a provider keeps its single API key while its exit changes, and where the
extension's endpoints sit is a policy decision the extension states, not
something the gateway guesses.

The gateway reports the resulting set — every exit, its tier and source,
whether the operator turned it off, its counters and its last error — at
`GET /admin/api/v1/egress` (and `GET /admin/api/v1/egress/:provider`).
Turning one off is `POST /admin/api/v1/egress/:provider/disable` with
`{"exit": "<name>"}` (or `…/enable` to bring it back); the choice is stored
with the provider and survives a restart, and the exit stays listed while it
is switched off so it remains visible in the dashboard.

### Live data (`Data`)

A page block or widget may carry `source` (plus optional `refresh` in seconds):

```json
{ "kind": "data", "source": "status", "refresh": 15 }
```

The dashboard then polls `GET /admin/api/v1/sidecar/extensions/:id/data/:key`
through the addon and renders the reply. Supported reply shapes — an addon may
return any combination:

```json
{"stats": [{"k": "Answering", "v": "38"}]}
{"kv":    [{"k": "Last refresh", "v": "2026-09-30 12:00:00"}]}
{"items": ["ok  vless  1.2.3.4:443  84ms"]}
{"blocks": [{"kind": "kv", "kv": [{"k": "Nodes", "v": "42"}]}]}
```

Only the requesting extension's own addon is consulted, so one extension can
never read another's data. Blocks without `source` stay static.

Prefer pure JSON (`settings`, `ui`, `provides`, `auth`) when the behavior is
data-shaped so extensions stay portable across gateway versions. Use a Yaegi
addon only when you need real code — network access, probing, parsing or
state.

## Authoring guidelines

1. Prefer JSON knobs over scripts when the behavior is data-shaped.
2. Put vendor endpoints in the `auth` block / `settings`, never in core forks.
3. Keep `files` paths relative and non-escaping (`MaterializeFiles` rejects
   `..` and absolute paths).
4. A `files` value may be a plain string, an array of lines (joined with
   `\n`), or **a ref to a companion file** shipped next to the manifest:
   `{"ref": "{id}/auth-example.go"}`. Refs resolve against the extension
   source URL at import/apply; the gateway also accepts uploaded content for
   refs it cannot fetch (`missing_files` → `{"url": …, "files": {…}}`).
   Prefer refs for addon sources — one real file per source instead of a
   33KB escaped string inside the JSON.
5. Document required permissions in `requirements` / `notes`.
6. Bump `version` on breaking settings changes so store updates are visible.
7. Name addon files `<kind>-<purpose>.go` and declare `// addon-kind:` so
   discovery does not depend on filename alone.

## Relationship to presets and themes

An addon is orthogonal: a sidecar preset can *also* provide account linking; a theme can
*also* ship a widget. The three documentation layers describe the main jobs —
this file covers optional activation and runtime hooks.

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

Planned / sample hook kinds:

- **Theme addon** — custom palette logic / chart painters
- **Preset addon** — custom retry/routing policies
- **UI addon** — dashboard widgets beyond structured blocks
- **Auth addon** — account-linking grant types (device flow, authorization code + PKCE) via the external-auth routes above

Prefer pure JSON (`settings`, `ui`, `provides`, `auth`) when the behavior is
data-shaped so extensions stay portable across gateway versions. Use a Yaegi
addon only when you need real code.

## Authoring guidelines

1. Prefer JSON knobs over scripts when the behavior is data-shaped.
2. Put vendor endpoints in the `auth` block / `settings`, never in core forks.
3. Keep `files` paths relative and non-escaping (`MaterializeFiles` rejects
   `..` and absolute paths).
4. A `files` value may be a plain string **or an array of lines** (joined
   with `\n`) — use the array form for embedded scripts so each source line
   stays on its own line in the JSON.
5. Document required permissions in `requirements` / `notes`.
6. Bump `version` on breaking settings changes so store updates are visible.
7. Name addon files `<kind>-<purpose>.go` and declare `// addon-kind:` so
   discovery does not depend on filename alone.

## Relationship to presets and themes

An addon is orthogonal: a sidecar preset can *also* provide account linking; a theme can
*also* ship a widget. The three documentation layers describe the main jobs —
this file covers optional activation and runtime hooks.

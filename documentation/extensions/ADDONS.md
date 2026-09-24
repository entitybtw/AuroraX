# Addons

Addons extend the gateway with capabilities that are **not** always compiled
into a minimal core. In this model, optional behavior is shipped as extension
JSON first; runtime Go hooks are layered on without baking provider secrets or
vendor-specific flows into default builds.

## What counts as an addon today

| Mechanism | How it ships | Effect |
|-----------|--------------|--------|
| `provides.provider_types` | Extension JSON | Registers optional provider factory types when applied |
| `provides.features` | Extension JSON | Surfaces features (e.g. `oauth`) and activates related wiring |
| `tool_schemas` / `files` | Extension JSON | Injects tool JSON into upstream requests or materializes scripts on disk |
| OAuth block | Extension JSON | Device-flow or authorization-code (+ PKCE) endpoints without core hardcoding |
| UI contributions | `ui.*` | Pages, settings tabs, banners, widgets |
| Runtime scripts | `files` → `configs/addons/*.go` | Single-file Go addons evaluated by Yaegi (`internal/addon`) |

### Example: OAuth feature without core vendor defaults

```json
{
  "schema": 1,
  "id": "example-oauth",
  "name": "Example OAuth",
  "type": "sidecar",
  "provides": { "features": ["oauth"] },
  "oauth": {
    "grant": "authorization_code",
    "authorize_url": "https://auth.example.com/authorize",
    "token_url": "https://auth.example.com/token",
    "token_style": "json",
    "client_id": "public-cli",
    "scopes": "openid profile",
    "redirect_uri": "http://127.0.0.1:54545/callback"
  },
  "settings": {
    "oauth_grant": "authorization_code",
    "oauth_authorize_url": "https://auth.example.com/authorize",
    "oauth_token_url": "https://auth.example.com/token",
    "oauth_client_id": "public-cli",
    "oauth_scopes": "openid profile"
  }
}
```

The gateway applies `auth_method=oauth` only to providers that match the
extension's provider types or `base_url` — endpoints come from the extension,
not from compiled defaults. Device flow remains available when `grant` is
omitted or `"device"`.

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
- **Auth addon** — additional OAuth grant types (authorization code + PKCE)

Prefer pure JSON (`settings`, `ui`, `provides`, `oauth`) when the behavior is
data-shaped so extensions stay portable across gateway versions. Use a Yaegi
addon only when you need real code.

## Authoring guidelines

1. Prefer JSON knobs over scripts when the behavior is data-shaped.
2. Put vendor endpoints in `oauth` / `settings`, never in core forks.
3. Keep `files` paths relative and non-escaping (`MaterializeFiles` rejects
   `..` and absolute paths).
4. Document required permissions in `requirements` / `notes`.
5. Bump `version` on breaking settings changes so store updates are visible.
6. Name addon files `<kind>-<purpose>.go` and declare `// addon-kind:` so
   discovery does not depend on filename alone.

## Relationship to presets and themes

An addon is orthogonal: a sidecar preset can *also* provide OAuth; a theme can
*also* ship a widget. The three documentation layers describe the main jobs —
this file covers optional activation and runtime hooks.

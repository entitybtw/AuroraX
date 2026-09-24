# Addons

Addons extend the gateway with capabilities that are **not** always compiled
into a minimal core. In this model, optional behavior is shipped as extension
JSON first; runtime Go/JS hooks are layered on without baking provider
secrets or vendor-specific flows into default builds.

## What counts as an addon today

| Mechanism | How it ships | Effect |
|-----------|--------------|--------|
| `provides.provider_types` | Extension JSON | Registers optional provider factory types when applied |
| `provides.features` | Extension JSON | Surfaces features (e.g. `oauth`) and activates related wiring |
| `tool_schemas` / `files` | Extension JSON | Injects tool JSON into upstream requests or materializes scripts on disk |
| OAuth block | Extension JSON | Device-flow (or future authorization-code) endpoints without core hardcoding |
| UI contributions | `ui.*` | Pages, settings tabs, banners, widgets |
| Runtime scripts | `files` + future addon runner | Hook points (see below) |

### Example: OAuth feature without core vendor defaults

```json
{
  "schema": 1,
  "id": "example-oauth",
  "name": "Example OAuth",
  "type": "sidecar",
  "provides": { "features": ["oauth"] },
  "oauth": {
    "server": "https://auth.example.com",
    "client_id": "public-cli",
    "verification_base": "https://example.com"
  },
  "settings": {
    "oauth_server": "https://auth.example.com",
    "oauth_client_id": "public-cli",
    "oauth_verification_base": "https://example.com"
  }
}
```

The gateway applies `auth_method=oauth` only to providers that match the
extension's provider types or `base_url` — endpoints come from the extension,
not from compiled defaults.

## Addon type (direction)

Future extensions may declare `"type": "addon"` (or provide
`provides.features` such as `runtime`) to enable sandboxed **Yaegi** Go
scripts shipped in `files` (single-file `.go`). Planned surface:

- **Theme addon** — custom palette logic / chart painters  
- **Preset addon** — custom retry/routing policies  
- **UI addon** — dashboard widgets beyond structured blocks  
- **Auth addon** — additional OAuth grant types (e.g. authorization code + PKCE)

Until the runner lands, prefer pure JSON (`settings`, `ui`, `provides`) so
extensions stay portable across gateway versions.

## Authoring guidelines

1. Prefer JSON knobs over scripts when the behavior is data-shaped.  
2. Put vendor endpoints in `oauth` / `settings`, never in core forks.  
3. Keep `files` paths relative and non-escaping (`MaterializeFiles` rejects
   `..` and absolute paths).  
4. Document required permissions in `requirements` / `notes`.  
5. Bump `version` on breaking settings changes so store updates are visible.

## Relationship to presets and themes

An addon is orthogonal: a sidecar preset can *also* provide OAuth; a theme can
*also* ship a widget. The three documentation layers describe the main jobs —
this file covers optional activation and future runtime hooks.

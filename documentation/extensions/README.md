# Extensions

Aurora extensions are portable JSON bundles. The gateway ships with **no**
built-in extensions — install them from a store URL, a raw JSON URL, or by
importing a file under **Settings → Extensions**.

There are three layers. They can be combined: one extension may declare only a
theme, only sidecar routing, or any mixture of the three.

| Layer | `type` (typical) | What it does |
|-------|------------------|--------------|
| **Themes** | `theme` | Dashboard look: colors, CSS variables, light/dark variants |
| **Presets** | `sidecar` | One-click sidecar + Session Hub configuration (base URL, auth, headers, tools, OAuth) |
| **Addons** | (sidecar / custom) | Optional capabilities via `provides` and `files` (scripts, tool schemas, future runtime hooks) |

See also:

- [THEMES.md](THEMES.md) — CSS variables, open theme surface, light/dark
- [PRESETS.md](PRESETS.md) — sidecar knobs, headers, tools, multi-IP
- [ADDONS.md](ADDONS.md) — `provides`, OAuth wiring, file materialization

## Install / apply

1. **Import** the JSON (store browse or direct URL).
2. **Apply** — activates provider types / writes sidecar settings / installs
   Session Hub header rules as declared.
3. **Full apply** — same as apply, plus binds headers to a Session Hub target
   in one click.
4. **Unapply** — drops the UI contribution; sidecar/provider overrides that
   this extension wrote are not automatically rolled back (edit them in the
   respective tabs if needed).

Themes with `type: "theme"` unapply other themes when applied (only one theme
contributes at a time).

## Minimal sidecar preset

```json
{
  "schema": 1,
  "id": "my-provider",
  "name": "My Provider",
  "type": "sidecar",
  "base_url": "https://api.example.com/v1",
  "user_agent": "my-agent/1.0",
  "default_auth": "Bearer",
  "settings": {
    "path_template": "/chat/completions",
    "models_path": "/models",
    "extra_headers": "{\"X-Tenant\":\"acme\"}",
    "forward_headers": "[\"x-request-id\"]",
    "retry_statuses": "[403,429,503]",
    "force_stream": "true"
  },
  "headers": [
    { "name": "x-session", "mode": "generate", "prefix": "s_", "length": 24, "charset": "hex" }
  ],
  "provides": { "provider_types": ["mytype"], "features": ["oauth"] }
}
```

## Safety notes

- Review extension JSON before install: extensions can change base URLs,
  headers, tool injection, and auth defaults.
- Theme CSS values are sanitized on the dashboard (`--` variable names, no
  `url()` / injection characters).
- `files` content is written under the extension materialization directory on
  apply — treat it as executable configuration.

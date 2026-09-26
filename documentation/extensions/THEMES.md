# Themes

Themes are pure UI extensions (`"type": "theme"`). They do not set `base_url`,
headers, or tools. Applying a theme replaces any other applied theme.

## Structure

```json
{
  "schema": 1,
  "id": "my-theme",
  "name": "My Theme",
  "type": "theme",
  "ui": {
    "accent": "#cba6f7",
    "theme": { "--bg": "#1e1e2e", "--text": "#cdd6f4" },
    "theme_light": { "--bg": "#eff1f5", "--text": "#4c4f69" },
    "theme_dark": { "--bg": "#1e1e2e", "--text": "#cdd6f4" },
    "fields": [
      { "key": "accent", "label": "Accent", "type": "color", "default": "#cba6f7" },
      { "key": "--bg", "label": "Background", "type": "color", "default": "#1e1e2e" }
    ]
  }
}
```

### Variant maps

| Key | Applied as |
|-----|------------|
| `ui.theme` | Base palette (`:root`) and default dark |
| `ui.theme_light` | Merged over base under `[data-theme="light"]` and `@media (prefers-color-scheme: light)` when not forced dark |
| `ui.theme_dark` | Merged over base under `[data-theme="dark"]` |

Operator **config** overrides (`ui.fields` + `config` / `--*` keys) overlay all
three maps that are present.

## Open CSS surface

Any safe custom-property name matching `--[a-z0-9][a-z0-9-]*` (case-insensitive)
is allowed. Values must be plain CSS fragments (no `;` `}` `url(` `expression(`
backslash, or backtick).

Common variables used by the dashboard:

### Surfaces & text
- `--bg`, `--bg-surface`, `--bg-surface-hover`, `--bg-elevated`
- `--text`, `--text-muted`, `--text-inverse`
- `--border`, `--border-subtle`

### Accent & semantic
- `--accent`, `--accent-hover`, `--accent-foreground`
- `--success`, `--warning`, `--danger`, `--info`

### Charts & analytics
- `--chart-grid`, `--chart-text`
- `--chart-tooltip-bg`, `--chart-tooltip-border`, `--chart-tooltip-text`
- `--chart-input`, `--chart-output`
- `--chart-cache-input`, `--chart-cache-output`
- `--chart-requests`, `--chart-cache-hits`
- `--chart-palette-5` … `--chart-palette-8`
- `--chart-cat-1` … `--chart-cat-10` (categorical series)
- `--chart-colors` (comma-separated list for legends)

### Contribution calendar
- `--cal-level-0` … `--cal-level-4`

### Misc
- `--prompt-cache-color-0`, `--prompt-cache-color-1`
- `--alias-row-*`
- `--glass-*`, `--ring-highlight`, `--shadow-tint`
- `--radius-sm`, `--radius-md`, `--radius-lg`
- `--font-body`, `--font-mono`
- `--sidebar-width`, `--space-page`

Setting `--chart-*` / `--cal-level-*` recolors every Overview chart (token
usage, throughput, splits) and the contribution calendar without gateway code
changes. Ship them in both the dark map and `theme_light` so colors track the
light/dark switch — the bundled store themes do exactly that.

## Light / dark switching

The dashboard theme attribute is `data-theme` (`light` | `dark`) with a
system preference fallback. A theme should provide:

1. Dark-oriented `theme` (or `theme_dark`) for dark mode  
2. A light-oriented `theme_light` map for light mode  

Example: a dark-flavor theme ships its light counterpart as `theme_light`.

## Store catalog themes

Store themes ship in flavor pairs: the dark flavor is the base theme and the
light flavor rides in `theme_light` (and vice versa for light-base flavors).
Browse the available theme extensions under **Settings → Sidecar → Browse store**.

The core default without extensions is a flat minimal palette (dark base +
system light override).

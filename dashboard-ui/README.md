# Aurora Admin Dashboard — React (Vite + TypeScript)

This workspace is the new admin dashboard for the Aurora gateway, replacing
the legacy Go-template + Alpine.js dashboard at
`internal/admin/dashboard/`. Both implementations stay compiled into the Go
binary that serves
`/admin/dashboard`.

This dashboard is built for Aurora OSS. UI components must not imply that another edition can be enabled from this source tree. Restricted sections may be displayed as unavailable only when the backend reports them that way; do not add client-side switches that unlock runtime behavior.

## Stack

- **Vite 6** + **React 19** + **TypeScript 5.7** (strict + `noUncheckedIndexedAccess`)
- **Tailwind CSS 4** via `@tailwindcss/vite` and `@theme inline` (no JS config)
- **shadcn/ui** primitives (Radix under the hood) re-themed to the existing
  CSS variables — see `src/styles/legacy-vars.css` and `src/styles/globals.css`
- **TanStack Router** + **TanStack Query v5**
- **Recharts** for analytics charts
- **Zod** for API response validation at the boundary
- **Vitest** + **happy-dom** for unit tests; **Oxlint** + **Oxfmt** for lint/format

## Workflow

```bash
# install — node 22+, pnpm 9+
pnpm install

# dev server, proxies /admin/api, /v1, /p, /health, /metrics → http://localhost:8080
pnpm dev

# typecheck + production build, writes to ../internal/admin/dashboard/dist
pnpm build

# unit tests
pnpm test
```

`pnpm build` is what CI and the Dockerfile run before `go build`. The output
directory (`../internal/admin/dashboard/dist`) is `//go:embed`'d into
the binary, so the React UI ships inside the same single Go executable as
today.

## Routing & base path

All asset URLs are emitted under `/admin/static/...` (via `vite.config.ts`
`base`). The Go handler (`internal/admin/dashboard/dashboard.go`)
serves:

| Path                   | Behavior                                     |
|------------------------|----------------------------------------------|
| `/admin/dashboard`     | SPA shell (`dist/index.html`)                |
| `/admin/dashboard/*`   | SPA fallback for client-side deep links      |
| `/admin/static/assets/*` | Hashed, immutable, long-cached               |
| `/admin/static/*`      | Other static (favicon, manifest), short-cached |

When the gateway is mounted under a sub-path (`BASE_PATH=/g`), the Go handler
substitutes the `__Aurora_BASE_PATH__` token in `index.html` per request.
`src/lib/basepath.ts` reads that prefix from the injected meta tag and
prefixes every API call.

## Theme

The dark/light/system theme tokens live in
`src/styles/legacy-vars.css` — a verbatim copy of the legacy
`internal/admin/dashboard/static/css/dashboard.css` variable block. Tailwind 4
maps those tokens onto utility classes via `@theme inline` in
`src/styles/globals.css`. **Do not redeclare colors in
`tailwind.config.ts`.** Components reference `bg-background`, `text-foreground`,
`border-border`, `bg-surface`, `text-accent`, etc.

## Migration status

The React dashboard is being incrementally migrated from the legacy Go-template dashboard. See the per-phase plan in the project docs.

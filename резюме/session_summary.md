# Резюме сессии

## Последний коммит: `438835fc` → origin/main

### Провайдеры
- `ProviderOverride.Enabled *bool` (nil = включён). `RawConfigs()` пропускает отключённые; `DisabledNames()` возвращает их имена.
- `runtimeRawProviders()` удаляет отключённые из runtime. `providerUpdateRequest` — указатели для частичного update.
- `SanitizedProviderConfig.Enabled`. `buildProviderStatusResponse` — статус `disabled`, исключение из health.
- **Fix** `6cbddba4`: невалидный Tailwind `hover:bg-surface-hover/30.5` → `/30` в ProvidersTab.

### Fallback
- On-disk формат: JSON-массив `[{source, targets, enabled}]` (порядок сохраняется). Legacy JSON-объект читается для обратной совместимости.
- `SwappableFallbackResolver` — live-reload через атомарную подмену делегата.
- `UpdateFallbackConfig` вызывает `ReloadFallback()` после записи — мгновенное применение.
- **Fix** `6cbddba4`: убрана `toSorted()` на фронте — порядок из API больше не меняется.

### Switch / UI
- Switch: borderless pill-трек, белый knob с тенью, accent-цвет on, gray off, spring easing, active:scale. Responsive: sm 42×26/38×22, md 48×28/44×24.
- **Все feature-toggle чекбоксы заменены на Switch** в:
  - GeneralTab (12), CachingTab (6), NetworkingTab (1)
  - ProvidersTab, fallback.tsx
  - combos.tsx, models.tsx (2), workflows.tsx (6 features)
  - auth-keys.tsx (allowed_providers, allowed_models, denied_models)
  - guardrails.tsx (config arrays)
  - GeneralTab token saver model_include/exclude, provider_include/exclude
  - CLIModelFieldGrid.tsx (model selection)
- Search icons: `text-foreground/40` + `pointer-events-none` для контраста.
- Select/input heights ≥ `h-9` для мобильных тач-таргетов.
- Audit logs sort select: `h-8 w-40` (нет обрезки "Highest status").
- Audit search field: `pl-10` (нет наложения placeholder на лупу).

## Важные детали
- `dist/`, `node_modules/`, `pnpm-lock.yaml` не коммитятся — собираются в Docker.
- Провайдеры: toggle мгновенно через runtime refresh; оверрайды переживают рестарт.
- Fallback: live-reload; правила применяются сразу при любом изменении.

## Relevant Files
- internal/gateway/swappable_fallback.go
- internal/application/app.go, runtime_refresh.go
- internal/admin/handler_fallback.go, handler.go, handler_providers_crud.go
- configuration/fallback.go
- configs/fallback.example.json
- dashboard-ui/src/components/ui/switch.tsx
- dashboard-ui/src/components/settings/{GeneralTab,CachingTab,NetworkingTab,ProvidersTab}.tsx
- dashboard-ui/src/routes/{fallback,audit-logs,workflows,usage,guardrails,combos,models,auth-keys}.tsx
- dashboard-ui/src/components/logs/LogSidebarFilters.tsx
- dashboard-ui/src/components/cli-tools/CLIModelFieldGrid.tsx
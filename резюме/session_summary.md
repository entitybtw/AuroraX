# Резюме сессии

## Последний коммит: `4c45aa8d` → origin/main

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
- Switch: borderless pill-трек, белый knob с тенью, accent-цвет on, gray off, spring easing, active:scale.
- Все toggle-чекбоксы заменены на Switch в GeneralTab (12), CachingTab (6), NetworkingTab (1), ProvidersTab (1), fallback.tsx (1).
- Списковые чекбоксы (guardrails, workflows, auth-keys) и диалоги моделей — оставлены native.

## Важные детали
- `dist/`, `node_modules/`, `pnpm-lock.yaml` не коммитятся — собираются в Docker.
- Провайдеры: toggle мгновенно через runtime refresh; оверрайды переживают рестарт.
- Fallback: live-reload; правила применяются сразу при любом изменении.

## Relevant Files
- internal/admin/handler_fallback.go, handler.go, handler_providers_crud.go
- internal/application/app.go, runtime_refresh.go
- internal/gateway/swappable_fallback.go
- configuration/fallback.go
- dashboard-ui/src/components/ui/switch.tsx
- dashboard-ui/src/components/settings/{GeneralTab,CachingTab,NetworkingTab,ProvidersTab}.tsx
- dashboard-ui/src/routes/fallback.tsx

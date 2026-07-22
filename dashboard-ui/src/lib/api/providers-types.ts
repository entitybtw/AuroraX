import { z } from "zod";

/**
 * Mirrors providerStatusResponse / providerStatusItemResponse from
 * internal/admin/handler.go and SanitizedProviderConfig /
 * ProviderRuntimeSnapshot from internal/providers/provider_status.go.
 *
 * Resilience details are kept loose (z.unknown()) — the dashboard does not
 * render them, but they pass through if the operator inspects the network
 * tab.
 */

export const SanitizedRetryConfigSchema = z
  .object({
    max_retries: z.number().int().optional(),
    initial_backoff: z.string().optional(),
    max_backoff: z.string().optional(),
    backoff_factor: z.number().optional(),
    jitter_factor: z.number().optional(),
  })
  .partial();

export const SanitizedCircuitBreakerConfigSchema = z
  .object({
    failure_threshold: z.number().int().optional(),
    success_threshold: z.number().int().optional(),
    timeout: z.string().optional(),
  })
  .partial();

export const SanitizedResilienceConfigSchema = z.object({
  retry: SanitizedRetryConfigSchema.default({}),
  circuit_breaker: SanitizedCircuitBreakerConfigSchema.default({}),
});

export const SanitizedProviderConfigSchema = z.object({
  name: z.string(),
  type: z.string(),
  base_url: z.string().optional(),
  api_version: z.string().optional(),
  models: z.array(z.string()).optional(),
  resilience: SanitizedResilienceConfigSchema.optional(),
});
export type SanitizedProviderConfig = z.infer<typeof SanitizedProviderConfigSchema>;

export const ProviderRuntimeSnapshotSchema = z.object({
  name: z.string(),
  type: z.string(),
  registered: z.boolean(),
  registry_initialized: z.boolean(),
  discovered_model_count: z.number().int(),
  using_cached_models: z.boolean(),
  last_model_fetch_at: z.string().nullable().optional(),
  last_model_fetch_success_at: z.string().nullable().optional(),
  last_model_fetch_error: z.string().optional(),
  last_availability_check_at: z.string().nullable().optional(),
  last_availability_ok_at: z.string().nullable().optional(),
  last_availability_error: z.string().optional(),
});
export type ProviderRuntimeSnapshot = z.infer<typeof ProviderRuntimeSnapshotSchema>;

export type ProviderStatusKind = "healthy" | "degraded" | "unhealthy";

export const ProviderStatusItemSchema = z.object({
  name: z.string(),
  type: z.string(),
  status: z.enum(["healthy", "degraded", "unhealthy"]),
  status_label: z.string(),
  status_reason: z.string(),
  last_error: z.string().optional(),
  config: SanitizedProviderConfigSchema,
  runtime: ProviderRuntimeSnapshotSchema,
  config_source: z.string().optional(),
});
export type ProviderStatusItem = z.infer<typeof ProviderStatusItemSchema>;

export const ProviderStatusSummarySchema = z.object({
  total: z.number().int(),
  healthy: z.number().int(),
  degraded: z.number().int(),
  unhealthy: z.number().int(),
  overall_status: z.enum(["healthy", "degraded", "unhealthy"]),
});
export type ProviderStatusSummary = z.infer<typeof ProviderStatusSummarySchema>;

export const ProviderStatusResponseSchema = z.object({
  summary: ProviderStatusSummarySchema,
  providers: z.array(ProviderStatusItemSchema),
});
export type ProviderStatusResponse = z.infer<typeof ProviderStatusResponseSchema>;

export const RuntimeRefreshStepSchema = z.object({
  name: z.string(),
  status: z.string(),
  message: z.string().optional(),
  error: z.string().optional(),
  duration_ms: z.number().int(),
});
export type RuntimeRefreshStep = z.infer<typeof RuntimeRefreshStepSchema>;

export const RuntimeRefreshReportSchema = z.object({
  status: z.string(),
  started_at: z.string(),
  finished_at: z.string(),
  duration_ms: z.number().int(),
  model_count: z.number().int(),
  provider_count: z.number().int(),
  steps: z.array(RuntimeRefreshStepSchema),
});
export type RuntimeRefreshReport = z.infer<typeof RuntimeRefreshReportSchema>;

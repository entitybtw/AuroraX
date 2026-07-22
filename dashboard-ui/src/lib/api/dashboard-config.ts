/**
 * Zod schemas + types for the /admin/api/v1/dashboard/config endpoint.
 *
 * Server type:  internal/admin/handler.go â†’ DashboardConfigResponse
 * Field names:  legacy uppercase JSON keys (LOGGING_ENABLED, etc.) preserved
 *               so the same backend serves both legacy and React variants.
 */

import { z } from "zod";

const enabledFlag = z.string().optional();

const DashboardSettingsSnapshotSchema = z
  .object({
    client: z
      .object({
        port: z.string().optional().default(""),
        base_path: z.string().optional().default(""),
        body_size_limit: z.string().optional().default(""),
        swagger_enabled: z.boolean().optional().default(false),
        pprof_enabled: z.boolean().optional().default(false),
        admin_endpoints_enabled: z.boolean().optional().default(false),
        admin_ui_enabled: z.boolean().optional().default(false),

        enable_anthropic_ingress: z.boolean().optional().default(false),
        enable_passthrough_routes: z.boolean().optional().default(false),
        allow_passthrough_v1_alias: z.boolean().optional().default(false),
        enabled_passthrough_providers: z.array(z.string()).optional().default([]),
        models_enabled_by_default: z.boolean().optional().default(false),
        model_overrides_enabled: z.boolean().optional().default(false),
        keep_only_aliases_at_models_endpoint: z.boolean().optional().default(false),
        configured_provider_models_mode: z.string().optional().default(""),
      })
      .optional()
      .default({}),
    caching: z
      .object({
        model_cache_backend: z.string().optional().default(""),
        model_cache_local_dir: z.string().optional().default(""),
        model_cache_redis_url: z.string().optional().default(""),
        model_cache_redis_key: z.string().optional().default(""),
        model_cache_redis_ttl_seconds: z.number().int().optional().default(0),
        model_refresh_interval_seconds: z.number().int().optional().default(0),
        model_list_url: z.string().optional().default(""),
        model_list_local_path: z.string().optional().default(""),
        model_list_user_overrides_path: z.string().optional().default(""),
        exact_cache_enabled: z.boolean().optional().default(false),
        exact_cache_redis_url: z.string().optional().default(""),
        exact_cache_redis_key: z.string().optional().default(""),
        exact_cache_ttl_seconds: z.number().int().optional().default(0),
        semantic_cache_enabled: z.boolean().optional().default(false),
        semantic_similarity_threshold: z.number().optional().default(0),
        semantic_prompt_similarity_min: z.number().optional().default(0),
        semantic_ttl_seconds: z.number().int().optional().default(0),
        semantic_max_conversation_messages: z.number().int().optional().default(0),
        semantic_exclude_system_prompt: z.boolean().optional().default(false),
        semantic_embedder_provider: z.string().optional().default(""),
        semantic_embedder_model: z.string().optional().default(""),
        semantic_vector_store_type: z.string().optional().default(""),
        semantic_vector_store_hints: z.array(z.string()).optional().default([]),
        semantic_vector_store_url: z.string().optional().default(""),
        semantic_vector_store_collection: z.string().optional().default(""),
        semantic_vector_store_table: z.string().optional().default(""),
        semantic_vector_store_namespace: z.string().optional().default(""),
        semantic_vector_store_class: z.string().optional().default(""),
        semantic_vector_store_dimension: z.number().int().optional().default(0),
        semantic_vector_store_api_key_set: z.boolean().optional().default(false),
        prompt_cache_mode: z.string().optional().default("auto"),
        prompt_cache_system_prompt: z.boolean().optional().default(true),
        prompt_cache_first_message: z.boolean().optional().default(true),
        prompt_cache_tools: z.boolean().optional().default(false),
        prompt_cache_min_tokens: z.number().int().optional().default(1024),
      })
      .optional()
      .default({}),
    logging: z
      .object({
        enabled: z.boolean().optional().default(false),
        log_bodies: z.boolean().optional().default(false),
        log_headers: z.boolean().optional().default(false),
        buffer_size: z.number().int().optional().default(0),
        flush_interval_seconds: z.number().int().optional().default(0),
        retention_days: z.number().int().optional().default(0),
        only_model_interactions: z.boolean().optional().default(false),
      })
      .optional()
      .default({}),
    observability: z
      .object({
        metrics_enabled: z.boolean().optional().default(false),
        metrics_endpoint: z.string().optional().default(""),
        storage_type: z.string().optional().default(""),
      })
      .optional()
      .default({}),
    storage: z
      .object({
        type: z.string().optional().default("sqlite"),
        sqlite_path: z.string().optional().default(""),
        postgresql_url: z.string().optional().default(""),
        postgresql_max_conns: z.number().int().optional().default(0),
        mongodb_url: z.string().optional().default(""),
        mongodb_database: z.string().optional().default(""),
      })
      .optional()
      .default({}),
    performance: z
      .object({
        http_timeout_seconds: z.number().int().optional().default(0),
        http_response_header_timeout_seconds: z.number().int().optional().default(0),
        workflow_refresh_interval_seconds: z.number().int().optional().default(0),
        retry_max_retries: z.number().int().optional().default(0),
        retry_initial_backoff_milliseconds: z.number().int().optional().default(0),
        retry_max_backoff_milliseconds: z.number().int().optional().default(0),
        retry_backoff_factor: z.number().optional().default(0),
        retry_jitter_factor: z.number().optional().default(0),
        circuit_breaker_failure_threshold: z.number().int().optional().default(0),
        circuit_breaker_success_threshold: z.number().int().optional().default(0),
        circuit_breaker_timeout_milliseconds: z.number().int().optional().default(0),
      })
      .optional()
      .default({}),
    security: z
      .object({
        master_key_configured: z.boolean().optional().default(false),
        guardrails_enabled: z.boolean().optional().default(false),
        batch_guardrails: z.boolean().optional().default(false),
      })
      .optional()
      .default({}),
    pricing: z
      .object({
        usage_enabled: z.boolean().optional().default(false),
        enforce_returning_usage_data: z.boolean().optional().default(false),
        pricing_recalculation_enabled: z.boolean().optional().default(false),
        usage_buffer_size: z.number().int().optional().default(0),
        usage_flush_interval_seconds: z.number().int().optional().default(0),
        usage_retention_days: z.number().int().optional().default(0),
        budgets_enabled: z.boolean().optional().default(false),
        configured_budget_user_path_count: z.number().int().optional().default(0),
      })
      .optional()
      .default({}),
  })
  .passthrough();

const FallbackRuleSnapshotSchema = z.object({
  source: z.string().optional(),
  targets: z.array(z.string()).optional(),
});

const FallbackOverrideSnapshotSchema = z.object({
  model: z.string().optional(),
  mode: z.string().optional(),
});

export const FallbackConfigSnapshotSchema = z
  .object({
    mode: z.string().optional(),
    rule_count: z.number().int().nonnegative().optional(),
    manual_rule_count: z.number().int().nonnegative().optional(),
    manual_rules_configured: z.boolean().optional(),
    error_path_count: z.number().int().nonnegative().optional(),
    has_default: z.boolean().optional(),
    manual_rules: z.array(FallbackRuleSnapshotSchema).optional(),
    overrides: z.array(FallbackOverrideSnapshotSchema).optional(),
  })
  .passthrough();

export const OIDCProviderSnapshotSchema = z.object({
  name: z.string(),
  display_name: z.string(),
  enabled: z.boolean(),
});

export const RuntimeFeatureSnapshotSchema = z.object({
  key: z.string(),
  label: z.string(),
  status: z.string(),
  configured: z.boolean(),
  description: z.string(),
  usage: z.string().optional().default(""),
});

export const DashboardConfigResponseSchema = z
  .object({
    EDITION: z.string().optional().default("oss"),
    CAPABILITIES: z.array(z.string()).optional().default([]),
    capabilities: z.record(z.boolean()).optional().default({}),
    FEATURE_FALLBACK_MODE: z.string().optional(),
    LOGGING_ENABLED: enabledFlag,
    USAGE_ENABLED: enabledFlag,
    BUDGETS_ENABLED: enabledFlag,
    GUARDRAILS_ENABLED: enabledFlag,
    CACHE_ENABLED: enabledFlag,
    REDIS_URL: z.string().optional(),
    SEMANTIC_CACHE_ENABLED: enabledFlag,
    USAGE_PRICING_RECALCULATION_ENABLED: enabledFlag,
    IDENTITY_ENABLED: enabledFlag,
    IDENTITY_OIDC_ENABLED: enabledFlag,
    IDENTITY_OIDC_PROVIDERS: z
      .array(OIDCProviderSnapshotSchema)
      .optional()
      .default([]),
    fallback: FallbackConfigSnapshotSchema.optional().default({}),
    settings: DashboardSettingsSnapshotSchema.optional().default({}),
    runtime_features: z
      .array(RuntimeFeatureSnapshotSchema)
      .optional()
      .default([]),
  })
  .passthrough();

export type DashboardConfigResponse = z.infer<
  typeof DashboardConfigResponseSchema
>;

export function hasCapability(
  config: DashboardConfigResponse | undefined,
  capability: string | undefined,
): boolean {
  if (!capability) return true;
  if (!config) return false;
  return Boolean(config.capabilities?.[capability]) || config.CAPABILITIES.includes(capability);
}

export function isAdvancedEdition(config: DashboardConfigResponse | undefined): boolean {
  return config?.EDITION?.toLowerCase() === "Advanced";
}

export function editionLabel(config: DashboardConfigResponse | undefined): string {
  return isAdvancedEdition(config) ? "Advanced" : "OSS";
}

export function canuseAdvancedFeaturesCapability(
  config: DashboardConfigResponse | undefined,
  capability: string | undefined,
): boolean {
  return isAdvancedEdition(config) && hasCapability(config, capability);
}

/** Treats the optional uppercase-string flags as booleans. Matches the legacy
 *  workflowRuntimeBooleanFlag helper: "on", "true", "1" are all truthy. */
export function flagOn(value: string | undefined | null): boolean {
  if (!value) return false;
  const v = value.toLowerCase();
  return v === "on" || v === "true" || v === "1";
}

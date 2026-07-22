import { z } from "zod";

export const AuthKeySchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().optional().default(""),
  user_path: z.string().optional().default(""),
  tenant_id: z.string().optional().default("default"),
  allowed_providers: z.array(z.string()).optional().default([]),
  allowed_models: z.array(z.string()).optional().default([]),
  denied_models: z.array(z.string()).optional().default([]),
  provider_pool_id: z.string().nullable().optional(),
  redacted_value: z.string().optional().default(""),
  active: z.boolean().default(true),
  expires_at: z.string().nullable().optional(),
  rate_limits: z.object({
    requests_per_minute: z.number().nullable().optional(),
    requests_per_day: z.number().nullable().optional(),
    tokens_per_minute: z.number().nullable().optional(),
    tokens_per_day: z.number().nullable().optional(),
  }).optional(),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
  requests_per_minute: z.number().nullable().optional(),
  requests_per_day: z.number().nullable().optional(),
  tokens_per_minute: z.number().nullable().optional(),
  tokens_per_day: z.number().nullable().optional(),
});
export type AuthKey = z.infer<typeof AuthKeySchema>;
export const AuthKeyListSchema = z.array(AuthKeySchema);

export const AuthKeyIssuedSchema = z.object({
  id: z.string(),
  name: z.string(),
  value: z.string(),
  redacted_value: z.string().optional(),
  expires_at: z.string().nullable().optional(),
  created_at: z.string().optional(),
});
export type AuthKeyIssued = z.infer<typeof AuthKeyIssuedSchema>;

export interface CreateAuthKeyInput {
  name: string;
  description?: string;
  user_path?: string;
  tenant_id?: string;
  allowed_providers?: string[] | undefined;
  allowed_models?: string[] | undefined;
  denied_models?: string[] | undefined;
  provider_pool_id?: string | undefined;
  expires_at?: string;
  requests_per_minute?: number | null | undefined;
  requests_per_day?: number | null | undefined;
  tokens_per_minute?: number | null | undefined;
  tokens_per_day?: number | null | undefined;
}

// --- Stats payload (GET /admin/api/v1/auth-keys/:id/stats) ---

const RateLimitWindowSchema = z.object({
  scope: z.string(),
  limit: z.number(),
  used: z.number(),
  remaining: z.number(),
  reset_at: z.string(),
});
export type AuthKeyRateLimitWindow = z.infer<typeof RateLimitWindowSchema>;

const RateLimitSnapshotSchema = z.object({
  requests_per_minute: RateLimitWindowSchema.optional().nullable(),
  requests_per_day: RateLimitWindowSchema.optional().nullable(),
  tokens_per_minute: RateLimitWindowSchema.optional().nullable(),
  tokens_per_day: RateLimitWindowSchema.optional().nullable(),
});
export type AuthKeyRateLimitSnapshot = z.infer<typeof RateLimitSnapshotSchema>;

const BucketCountSchema = z.object({
  label: z.string(),
  count: z.number(),
});
export type AuthKeyBucketCount = z.infer<typeof BucketCountSchema>;

const DailyStatsSchema = z.object({
  date: z.string(),
  requests: z.number().default(0),
  errors: z.number().default(0),
  cache_hits: z.number().default(0),
});
export type AuthKeyDailyStats = z.infer<typeof DailyStatsSchema>;

export const AuthKeyStatsSchema = z.object({
  key: AuthKeySchema,
  window: z.object({
    start_date: z.string(),
    end_date: z.string(),
    days: z.number(),
    timezone: z.string(),
  }),
  requests: z.object({
    total: z.number().default(0),
    success_count: z.number().default(0),
    redirect_count: z.number().default(0),
    client_error_count: z.number().default(0),
    server_error_count: z.number().default(0),
    error_count: z.number().default(0),
    stream_count: z.number().default(0),
    success_rate: z.number().default(0),
    error_rate: z.number().default(0),
  }),
  latency: z.object({
    min_ns: z.number().default(0),
    avg_ns: z.number().default(0),
    max_ns: z.number().default(0),
  }),
  cache: z.object({
    exact_hits: z.number().default(0),
    semantic_hits: z.number().default(0),
    total_hits: z.number().default(0),
    misses: z.number().default(0),
    hit_rate: z.number().default(0),
  }),
  usage: z.object({
    total_requests: z.number().default(0),
    input_tokens: z.number().default(0),
    output_tokens: z.number().default(0),
    total_tokens: z.number().default(0),
    input_cost: z.number().nullable().optional(),
    output_cost: z.number().nullable().optional(),
    total_cost: z.number().nullable().optional(),
    note_user_path_tie: z.boolean().default(false),
  }),
  top_models: z.array(BucketCountSchema).default([]),
  top_providers: z.array(BucketCountSchema).default([]),
  top_errors: z.array(BucketCountSchema).default([]),
  daily: z.array(DailyStatsSchema).default([]),
  rate_limit_status: RateLimitSnapshotSchema.optional().nullable(),
  last_used_at: z.string().optional().nullable(),
});
export type AuthKeyStats = z.infer<typeof AuthKeyStatsSchema>;

import { z } from "zod";

/**
 * Zod schemas for /admin/api/v1/usage/* responses. Mirrors
 * internal/usage/reader.go (UsageSummary, DailyUsage, ModelUsage,
 * UserPathUsage, CacheOverview*) field-for-field. *Cost fields are
 * nullable in Go (*float64) so we keep them nullable here too.
 */

const Nullable = <T extends z.ZodTypeAny>(schema: T) =>
  schema.nullable().optional();

export const UsageSummarySchema = z.object({
  total_requests: z.number().int(),
  total_input_tokens: z.number(),
  total_output_tokens: z.number(),
  total_tokens: z.number(),
  total_cached_input_tokens: z.number().optional(),
  total_cache_write_input_tokens: z.number().optional(),
  total_input_cost: Nullable(z.number()),
  total_output_cost: Nullable(z.number()),
  total_cost: Nullable(z.number()),
});
export type UsageSummary = z.infer<typeof UsageSummarySchema>;

export const DailyUsageSchema = z.object({
  date: z.string(),
  requests: z.number().int(),
  input_tokens: z.number(),
  output_tokens: z.number(),
  total_tokens: z.number(),
  input_cost: Nullable(z.number()),
  output_cost: Nullable(z.number()),
  total_cost: Nullable(z.number()),
});
export type DailyUsage = z.infer<typeof DailyUsageSchema>;
export const DailyUsageListSchema = z.array(DailyUsageSchema);

export const ModelUsageSchema = z.object({
  model: z.string(),
  provider: z.string(),
  provider_name: z.string().optional(),
  input_tokens: z.number(),
  output_tokens: z.number(),
  input_cost: Nullable(z.number()),
  output_cost: Nullable(z.number()),
  total_cost: Nullable(z.number()),
});
export type ModelUsage = z.infer<typeof ModelUsageSchema>;
export const ModelUsageListSchema = z.array(ModelUsageSchema);

export const UserPathUsageSchema = z.object({
  user_path: z.string(),
  input_tokens: z.number(),
  output_tokens: z.number(),
  total_tokens: z.number(),
  input_cost: Nullable(z.number()),
  output_cost: Nullable(z.number()),
  total_cost: Nullable(z.number()),
});
export type UserPathUsage = z.infer<typeof UserPathUsageSchema>;
export const UserPathUsageListSchema = z.array(UserPathUsageSchema);

export const CacheOverviewSummarySchema = z.object({
  total_hits: z.number().int(),
  exact_hits: z.number().int(),
  semantic_hits: z.number().int(),
  total_input_tokens: z.number(),
  total_output_tokens: z.number(),
  total_tokens: z.number(),
  total_saved_cost: Nullable(z.number()),
});
export type CacheOverviewSummary = z.infer<typeof CacheOverviewSummarySchema>;

export const CacheOverviewDailySchema = z.object({
  date: z.string(),
  hits: z.number().int(),
  exact_hits: z.number().int(),
  semantic_hits: z.number().int(),
  input_tokens: z.number(),
  output_tokens: z.number(),
  total_tokens: z.number(),
  saved_cost: Nullable(z.number()),
});
export type CacheOverviewDaily = z.infer<typeof CacheOverviewDailySchema>;

export const CacheOverviewSchema = z.object({
  summary: CacheOverviewSummarySchema,
  daily: z.array(CacheOverviewDailySchema),
});
export type CacheOverview = z.infer<typeof CacheOverviewSchema>;

export type CacheMode = "uncached" | "cached" | "all";

export interface UsageQueryFilters {
  days?: number;
  startDate?: string;
  endDate?: string;
  interval?: "daily" | "weekly" | "monthly" | "yearly";
  userPath?: string;
  tenant?: string;
  cacheMode?: CacheMode;
}

export function usageQueryParams(filters: UsageQueryFilters): URLSearchParams {
  const params = new URLSearchParams();
  if (filters.days !== undefined) {
    params.set("days", String(filters.days));
  }
  if (filters.startDate) {
    params.set("start_date", filters.startDate);
  }
  if (filters.endDate) {
    params.set("end_date", filters.endDate);
  }
  if (filters.interval) {
    params.set("interval", filters.interval);
  }
  if (filters.userPath) {
    params.set("user_path", filters.userPath);
  }
  if (filters.tenant) {
    params.set("tenant", filters.tenant);
  }
  if (filters.cacheMode) {
    params.set("cache_mode", filters.cacheMode);
  }
  return params;
}

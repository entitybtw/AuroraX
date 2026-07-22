import { apiFetch } from "./client";
import {
  CacheOverviewSchema,
  DailyUsageListSchema,
  ModelUsageListSchema,
  UsageSummarySchema,
  UserPathUsageListSchema,
  type CacheOverview,
  type DailyUsage,
  type ModelUsage,
  type UsageQueryFilters,
  type UsageSummary,
  type UserPathUsage,
} from "./usage-types";

/**
 * Typed clients for /admin/api/v1/usage/* and /admin/api/v1/cache/overview.
 * Mirrors internal/admin/handler_usage.go endpoints. Each function uses the
 * shared apiFetch wrapper (auth header, base path, timezone header, schema
 * validation) — callers get parsed, validated values back.
 */

function toQuery(filters: UsageQueryFilters): Record<string, string | undefined> {
  const out: Record<string, string | undefined> = {};
  if (filters.days !== undefined) {
    out.days = String(filters.days);
  }
  if (filters.startDate) {
    out.start_date = filters.startDate;
  }
  if (filters.endDate) {
    out.end_date = filters.endDate;
  }
  if (filters.interval) {
    out.interval = filters.interval;
  }
  if (filters.userPath) {
    out.user_path = filters.userPath;
  }
  if (filters.tenant) {
    out.tenant = filters.tenant;
  }
  if (filters.cacheMode) {
    out.cache_mode = filters.cacheMode;
  }
  return out;
}

export async function fetchUsageSummary(filters: UsageQueryFilters = {}): Promise<UsageSummary> {
  return apiFetch<UsageSummary>("/admin/api/v1/usage/summary", {
    query: toQuery(filters),
    schema: UsageSummarySchema,
  });
}

export async function fetchDailyUsage(filters: UsageQueryFilters = {}): Promise<DailyUsage[]> {
  return apiFetch<DailyUsage[]>("/admin/api/v1/usage/daily", {
    query: toQuery(filters),
    schema: DailyUsageListSchema,
  });
}

export async function fetchUsageByModel(filters: UsageQueryFilters = {}): Promise<ModelUsage[]> {
  return apiFetch<ModelUsage[]>("/admin/api/v1/usage/models", {
    query: toQuery(filters),
    schema: ModelUsageListSchema,
  });
}

export async function fetchUsageByUserPath(
  filters: UsageQueryFilters = {},
): Promise<UserPathUsage[]> {
  return apiFetch<UserPathUsage[]>("/admin/api/v1/usage/user-paths", {
    query: toQuery(filters),
    schema: UserPathUsageListSchema,
  });
}

export async function fetchCacheOverview(
  filters: UsageQueryFilters = {},
): Promise<CacheOverview> {
  return apiFetch<CacheOverview>("/admin/api/v1/cache/overview", {
    query: toQuery(filters),
    schema: CacheOverviewSchema,
  });
}

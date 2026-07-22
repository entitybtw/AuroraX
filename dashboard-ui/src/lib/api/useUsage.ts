import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import {
  fetchCacheOverview,
  fetchDailyUsage,
  fetchUsageByModel,
  fetchUsageByUserPath,
  fetchUsageSummary,
} from "./usage";
import type {
  CacheOverview,
  DailyUsage,
  ModelUsage,
  UsageQueryFilters,
  UsageSummary,
  UserPathUsage,
} from "./usage-types";

/**
 * TanStack Query hooks for the usage endpoints. Query keys follow the
 * convention defined in the migration plan:
 *   ['usage', 'summary', filters], ['usage', 'daily', filters], ...
 * so cross-page invalidation (e.g. after a runtime refresh) can target a
 * stable namespace.
 */

const STALE = 30_000;

export function useUsageSummary(filters: UsageQueryFilters = {}): UseQueryResult<UsageSummary> {
  return useQuery({
    queryKey: ["usage", "summary", filters],
    queryFn: () => fetchUsageSummary(filters),
    staleTime: STALE,
  });
}

export function useDailyUsage(filters: UsageQueryFilters = {}): UseQueryResult<DailyUsage[]> {
  return useQuery({
    queryKey: ["usage", "daily", filters],
    queryFn: () => fetchDailyUsage(filters),
    staleTime: STALE,
  });
}

export function useUsageByModel(filters: UsageQueryFilters = {}): UseQueryResult<ModelUsage[]> {
  return useQuery({
    queryKey: ["usage", "models", filters],
    queryFn: () => fetchUsageByModel(filters),
    staleTime: STALE,
  });
}

export function useUsageByUserPath(
  filters: UsageQueryFilters = {},
): UseQueryResult<UserPathUsage[]> {
  return useQuery({
    queryKey: ["usage", "user-paths", filters],
    queryFn: () => fetchUsageByUserPath(filters),
    staleTime: STALE,
  });
}

export function useCacheOverview(
  filters: UsageQueryFilters = {},
): UseQueryResult<CacheOverview, Error> {
  return useQuery<CacheOverview, Error>({
    queryKey: ["cache", "overview", filters],
    queryFn: () => fetchCacheOverview(filters),
    staleTime: STALE,
    retry: (failureCount, error) => {
      // 503 means cache analytics are disabled — no point retrying.
      if (
        error &&
        typeof error === "object" &&
        "status" in error &&
        (error as { status: number }).status === 503
      ) {
        return false;
      }
      return failureCount < 1;
    },
  });
}

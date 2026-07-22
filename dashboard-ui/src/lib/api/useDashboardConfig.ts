import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { apiFetch } from "./client";
import {
  DashboardConfigResponseSchema,
  type DashboardConfigResponse,
} from "./dashboard-config";

/**
 * Fetches /admin/api/v1/dashboard/config — the runtime feature-flag bundle
 * the legacy dashboard polls on boot. Used by the sidebar to gate Budgets
 * and Guardrails entries so their visibility matches today.
 */
export function useDashboardConfig(): UseQueryResult<DashboardConfigResponse> {
  return useQuery({
    queryKey: ["dashboard", "config"],
    queryFn: () =>
      apiFetch<DashboardConfigResponse>("/admin/api/v1/dashboard/config", {
        schema: DashboardConfigResponseSchema,
      }),
    // Config rarely changes at runtime; align with legacy behavior of
    // re-fetching only when explicitly refreshed.
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}

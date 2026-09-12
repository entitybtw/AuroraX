import { apiFetch } from "./client";
import {
  ProviderStatusResponseSchema,
  RuntimeRefreshReportSchema,
  type ProviderStatusResponse,
  type RuntimeRefreshReport,
} from "./providers-types";

export type { ProviderStatusResponse, RuntimeRefreshReport };

export function fetchProviderStatus(): Promise<ProviderStatusResponse> {
  return apiFetch("/admin/api/v1/providers/status", {
    schema: ProviderStatusResponseSchema,
  }) as Promise<ProviderStatusResponse>;
}

export function refreshRuntime(): Promise<RuntimeRefreshReport> {
  return apiFetch("/admin/api/v1/runtime/refresh", {
    method: "POST",
    schema: RuntimeRefreshReportSchema,
  }) as Promise<RuntimeRefreshReport>;
}

export interface ProviderFormData {
  name: string;
  type: string;
  base_url: string;
  api_version: string;
  api_key: string;
  models: string;
  enabled?: boolean;
  bind_ip?: string;
  pool_only?: boolean;
  user_agent?: string;
  auto_fetch_models?: boolean;
  /** Narrows discovered models to those matching the declared conditions. */
  autofetch_filter?: AutoFetchFilter | null;
  /** Form-only convenience field: comma-separated substrings, converted to
   *  autofetch_filter before the request is sent. Never serialized. */
  autofetch_filter_text?: string;
  /** When present on an update, renames the provider to this value. */
  new_name?: string;
}

/** A single condition inside an AutoFetchFilter. All set fields must hold. */
export interface AutoFetchFilterCondition {
  /** Keep only models whose ID contains this substring (case-insensitive). */
  contains?: string | undefined;
  /** Drop models whose ID contains this substring (case-insensitive). */
  not_contains?: string | undefined;
  /** Keep only models whose ID matches this Go regexp. */
  regex?: string | undefined;
  /** Keep only models priced at or below this per-million-token value (0 = free). */
  max_price?: number | undefined;
  /** Input-only price ceiling. */
  max_prompt_price?: number | undefined;
  /** Output-only price ceiling. */
  max_completion_price?: number | undefined;
}

/** Conditions combined with `mode` (`all` = AND, default; `any` = OR). */
export interface AutoFetchFilter {
  mode?: "all" | "any" | undefined;
  conditions?: AutoFetchFilterCondition[] | undefined;
}

export function createProvider(data: ProviderFormData): Promise<{ message: string; provider: string }> {
  return apiFetch("/admin/api/v1/providers", {
    method: "POST",
    json: data,
  }) as Promise<{ message: string; provider: string }>;
}

export function updateProvider(name: string, data: Partial<ProviderFormData>): Promise<{ message: string; provider: string }> {
  return apiFetch(`/admin/api/v1/providers/${encodeURIComponent(name)}`, {
    method: "PUT",
    json: data,
  }) as Promise<{ message: string; provider: string }>;
}

export function setProviderEnabled(name: string, enabled: boolean): Promise<{ message: string; provider: string }> {
  return updateProvider(name, { enabled });
}

export function deleteProvider(name: string): Promise<{ message: string; provider: string }> {
  return apiFetch(`/admin/api/v1/providers/${encodeURIComponent(name)}`, {
    method: "DELETE",
  }) as Promise<{ message: string; provider: string }>;
}

export interface PoolUpdateData {
  strategy: string;
  weights: Record<string, number>;
}

export function updatePool(name: string, data: PoolUpdateData): Promise<{ message: string; pool_name: string; strategy: string }> {
  return apiFetch(`/admin/api/v1/pools/${encodeURIComponent(name)}`, {
    method: "PUT",
    json: data,
  }) as Promise<{ message: string; pool_name: string; strategy: string }>;
}

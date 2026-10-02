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
  /** Local source addresses rotated per attempt. Kept in sync with bind_ip. */
  bind_ips?: string[];
  /** round_robin (default), random, weighted or first. */
  egress_strategy?: string;
  /** Exits the operator turned off from the egress status panel. */
  egress_disabled?: string[];
  pool_only?: boolean;
  user_agent?: string;
  disable_api_key?: boolean;
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

export interface BulkCreateResult {
  created: string[];
  failed: { name: string; error: string }[];
}

/**
 * Creates several providers from one form. Names go one at a time so a single
 * failure (a name that already exists, an upstream URL the gateway refuses)
 * does not lose the rest — the caller reports what is left to do.
 */
export async function createProviders(
  data: ProviderFormData,
  names: string[],
): Promise<BulkCreateResult> {
  const created: string[] = [];
  const failed: { name: string; error: string }[] = [];
  for (const name of names) {
    try {
      await createProvider({ ...data, name });
      created.push(name);
    } catch (err) {
      failed.push({
        name,
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }
  return { created, failed };
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

// Provider Presets API

export interface ProviderPreset {
  name: string;
  type: string;
  base_url: string;
  auth_method?: string;
  key_optional?: boolean;
  description: string;
  models?: string;
}

export interface DetectPresetResponse {
  matched: boolean;
  preset?: ProviderPreset;
  message?: string;
}

export function fetchProviderPresets(): Promise<ProviderPreset[]> {
  return apiFetch("/admin/api/v1/providers/presets") as Promise<ProviderPreset[]>;
}

export function detectProviderPreset(
  name: string,
  type: string,
  baseUrl: string
): Promise<DetectPresetResponse> {
  return apiFetch("/admin/api/v1/providers/detect-preset", {
    method: "POST",
    json: { name, type, base_url: baseUrl },
  }) as Promise<DetectPresetResponse>;
}

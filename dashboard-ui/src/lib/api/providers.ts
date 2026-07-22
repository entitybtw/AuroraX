import { apiFetch } from "./client";
import {
  ProviderStatusResponseSchema,
  RuntimeRefreshReportSchema,
  type ProviderStatusResponse,
  type RuntimeRefreshReport,
} from "./providers-types";

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

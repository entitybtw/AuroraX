import { apiFetch } from "./client";
import {
  PoolsResponseSchema,
  PoolOptionsSchema,
  PoolModifyResponseSchema,
  type PoolsResponse,
  type PoolOptions,
  type PoolConfigPayload,
  type PoolModifyResponse,
} from "./pools-types";

export function fetchPools(): Promise<PoolsResponse> {
  return apiFetch("/admin/api/v1/pools", {
    schema: PoolsResponseSchema,
  }) as Promise<PoolsResponse>;
}

export function fetchPoolOptions(): Promise<PoolOptions> {
  return apiFetch("/admin/api/v1/pools/options", {
    schema: PoolOptionsSchema,
  }) as Promise<PoolOptions>;
}

export function createPool(payload: PoolConfigPayload): Promise<PoolModifyResponse> {
  return apiFetch("/admin/api/v1/pools", {
    method: "POST",
    json: payload,
    schema: PoolModifyResponseSchema,
  }) as Promise<PoolModifyResponse>;
}

export function updatePool(
  name: string,
  payload: PoolConfigPayload,
): Promise<PoolModifyResponse> {
  return apiFetch(`/admin/api/v1/pools/${encodeURIComponent(name)}`, {
    method: "PUT",
    json: payload,
    schema: PoolModifyResponseSchema,
  }) as Promise<PoolModifyResponse>;
}

export function deletePool(name: string): Promise<PoolModifyResponse> {
  return apiFetch(`/admin/api/v1/pools/${encodeURIComponent(name)}`, {
    method: "DELETE",
    schema: PoolModifyResponseSchema,
  }) as Promise<PoolModifyResponse>;
}

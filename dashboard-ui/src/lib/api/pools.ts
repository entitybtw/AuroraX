import { apiFetch } from "./client";
import { PoolsResponseSchema, type PoolsResponse } from "./pools-types";

export function fetchPools(): Promise<PoolsResponse> {
  return apiFetch("/admin/api/v1/pools", {
    schema: PoolsResponseSchema,
  }) as Promise<PoolsResponse>;
}

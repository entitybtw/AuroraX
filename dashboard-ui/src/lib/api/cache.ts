import { apiFetch } from "./client";
import { CacheDebugInfoSchema, type CacheDebugInfo, type CacheDebugRequest } from "./cache-types";

export async function debugCacheRequest(input: CacheDebugRequest): Promise<CacheDebugInfo> {
  return apiFetch<CacheDebugInfo>("/admin/api/v1/cache/debug", {
    method: "POST",
    json: input,
    schema: CacheDebugInfoSchema,
  });
}

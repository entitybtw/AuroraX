import { useMutation } from "@tanstack/react-query";
import { debugCacheRequest } from "./cache";
import type { CacheDebugInfo, CacheDebugRequest } from "./cache-types";

export function useCacheDebug() {
  return useMutation<CacheDebugInfo, Error, CacheDebugRequest>({
    mutationFn: (input) => debugCacheRequest(input),
  });
}

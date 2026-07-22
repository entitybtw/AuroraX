import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { fetchPools } from "./pools";
import type { PoolsResponse } from "./pools-types";

export function usePools(): UseQueryResult<PoolsResponse, Error> {
  return useQuery<PoolsResponse, Error>({
    queryKey: ["pools"],
    queryFn: () => fetchPools(),
    staleTime: 10_000,
    refetchInterval: 10_000,
  });
}

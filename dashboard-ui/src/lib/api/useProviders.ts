import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { fetchProviderStatus, refreshRuntime } from "./providers";
import type {
  ProviderStatusResponse,
  RuntimeRefreshReport,
} from "./providers-types";

const PROVIDERS_KEY = ["providers", "status"] as const;

export function useProviderStatus(): UseQueryResult<ProviderStatusResponse, Error> {
  return useQuery<ProviderStatusResponse, Error>({
    queryKey: PROVIDERS_KEY,
    queryFn: fetchProviderStatus,
    staleTime: 30_000,
    refetchInterval: 60_000,
  });
}

export function useRuntimeRefresh(): UseMutationResult<RuntimeRefreshReport, Error, void> {
  const qc = useQueryClient();
  return useMutation<RuntimeRefreshReport, Error, void>({
    mutationFn: refreshRuntime,
    onSuccess: () => {
      // After a runtime refresh, every cache surface is potentially stale.
      qc.invalidateQueries({ queryKey: ["providers"] });
      qc.invalidateQueries({ queryKey: ["usage"] });
      qc.invalidateQueries({ queryKey: ["cache"] });
      qc.invalidateQueries({ queryKey: ["pools"] });
    },
  });
}

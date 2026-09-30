import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { fetchEgressStatus, setEgressExitDisabled } from "./egress";
import type { EgressStatusResponse, EgressToggleResponse } from "./egress-types";

const EGRESS_KEY = ["egress", "status"] as const;

export interface EgressToggleVars {
  provider: string;
  exit: string;
  disabled: boolean;
}

/** Polls the egress status so cooldowns and the last error stay current. */
export function useEgressStatus(): UseQueryResult<EgressStatusResponse, Error> {
  return useQuery<EgressStatusResponse, Error>({
    queryKey: EGRESS_KEY,
    queryFn: fetchEgressStatus,
    staleTime: 2_000,
    refetchInterval: 5_000,
  });
}

export function useToggleEgressExit(): UseMutationResult<
  EgressToggleResponse,
  Error,
  EgressToggleVars
> {
  const qc = useQueryClient();
  return useMutation<EgressToggleResponse, Error, EgressToggleVars>({
    mutationFn: ({ provider, exit, disabled }) => setEgressExitDisabled(provider, exit, disabled),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: EGRESS_KEY });
      // The disabled set is part of the provider's configuration.
      qc.invalidateQueries({ queryKey: ["providers"] });
    },
  });
}

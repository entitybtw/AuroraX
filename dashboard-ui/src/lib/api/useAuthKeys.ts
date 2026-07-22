import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createAuthKey,
  deactivateAuthKey,
  fetchAuthKeyStats,
  fetchAuthKeys,
} from "./auth-keys";
import type { AuthKey, AuthKeyStats, CreateAuthKeyInput } from "./auth-keys-types";

export function useAuthKeys() {
  const queryClient = useQueryClient();

  const query = useQuery<AuthKey[], Error>({
    queryKey: ["auth-keys"],
    queryFn: fetchAuthKeys,
    // Refetch the list every 30s so the active/inactive badge and rate-limit
    // configuration stay in sync if another admin changes them.
    refetchInterval: 30_000,
  });

  const createMutation = useMutation({
    mutationFn: (input: CreateAuthKeyInput) => createAuthKey(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["auth-keys"] });
    },
  });

  const deactivateMutation = useMutation({
    mutationFn: (id: string) => deactivateAuthKey(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["auth-keys"] });
    },
  });

  return {
    ...query,
    createMutation,
    deactivateMutation,
  };
}

/**
 * useAuthKeyStats fetches per-key analytics including live rate-limit
 * consumption, cache hit rate, latency, tokens/cost, top models/providers,
 * and a daily request series. The endpoint is enabled only when an id is
 * provided so callers can pass `null` to disable.
 */
export function useAuthKeyStats(id: string | null, days: number = 30) {
  return useQuery<AuthKeyStats, Error>({
    queryKey: ["auth-key-stats", id, days],
    queryFn: () => fetchAuthKeyStats(id ?? "", days),
    enabled: !!id,
    // Live consumption + recent traffic — refresh every 10s while the drawer
    // is open. React Query will pause polling for hidden tabs automatically.
    refetchInterval: 10_000,
    staleTime: 5_000,
  });
}

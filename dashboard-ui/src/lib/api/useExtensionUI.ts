import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import {
  fetchExtensionUI,
  type ExtensionUIContribution,
} from "@/lib/api/extensions";

/**
 * Merged UI contributions from applied extensions. Cached so Sidebar,
 * AppShell, Overview, Settings, and the extension page route share one fetch.
 */
export function useExtensionUI(): UseQueryResult<ExtensionUIContribution[]> {
  return useQuery({
    queryKey: ["extensions", "ui"],
    queryFn: fetchExtensionUI,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}

export type { ExtensionUIContribution };

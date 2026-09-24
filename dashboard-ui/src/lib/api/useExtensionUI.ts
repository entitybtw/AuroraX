import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import {
  fetchExtensionUI,
  type ExtensionUIContribution,
} from "@/lib/api/extensions";

/**
 * Merged UI contributions from applied extensions. Cached so Sidebar,
 * AppShell, Overview, Settings, and the extension page route share one fetch.
 * On query error the result is treated as an empty contribution list so a
 * failed refresh never keeps stale theme/accent overrides around.
 */
export function useExtensionUI(): UseQueryResult<ExtensionUIContribution[]> {
  const query = useQuery({
    queryKey: ["extensions", "ui"],
    queryFn: fetchExtensionUI,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
  if (query.isError) {
    return { ...query, data: [] as ExtensionUIContribution[] } as UseQueryResult<
      ExtensionUIContribution[]
    >;
  }
  return query;
}

export type { ExtensionUIContribution };

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { fetchExtensionData } from "@/lib/api/extensions";

/**
 * Live payload for a block/widget `source` key on an extension's addon.
 *
 * The addon behind the extension answers with structured content (blocks,
 * key/value pairs, stats or a list); this hook just keeps it fresh. Polling is
 * paused when the tab is in the background.
 */
export function useExtensionData(
  extensionId: string | undefined,
  source: string | undefined,
  refresh?: number | undefined,
): UseQueryResult<unknown> {
  const interval = Math.max(5, refresh ?? 15);
  return useQuery({
    queryKey: ["extension-data", extensionId, source],
    queryFn: () => fetchExtensionData(extensionId ?? "", source ?? ""),
    enabled: Boolean(extensionId && source),
    refetchInterval: interval * 1000,
    refetchIntervalInBackground: false,
    staleTime: (interval * 1000) / 2,
    refetchOnWindowFocus: false,
    retry: false,
  });
}

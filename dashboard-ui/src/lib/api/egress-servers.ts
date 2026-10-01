import { z } from "zod";
import { useQueries, useQuery } from "@tanstack/react-query";
import { fetchExtensionData, fetchExtensions } from "./extensions";

/**
 * The machine-readable endpoint list an egress extension exposes under the
 * `servers` data key (GET .../extensions/<id>/data/servers).
 *
 * The egress registry only knows about endpoints a provider is actually
 * routing through, which is nothing while an extension runs report-only
 * (no tunnel core). This payload is what an extension has bound: the pool or
 * provider view shows these servers whether or not traffic flows through
 * them yet.
 */

export const EgressServerSchema = z.object({
  host: z.string(),
  port: z.number(),
  name: z.string().optional(),
  protocol: z.string().optional(),
  alive: z.boolean().optional(),
  latency_ms: z.number().optional(),
  /** True when the endpoint survived probing and trimming to node_count. */
  selected: z.boolean().optional(),
});
export type EgressServer = z.infer<typeof EgressServerSchema>;

export const EgressServersPayloadSchema = z.object({
  apply_to: z.string().optional(),
  egress_mode: z.string().optional(),
  core_kind: z.string().optional(),
  servers: z.array(EgressServerSchema).default([]),
});
export type EgressServersPayload = z.infer<typeof EgressServersPayloadSchema>;

export interface EgressServerList {
  id: string;
  name: string;
  accent?: string | undefined;
  payload: EgressServersPayload;
}

/**
 * Mirrors the extension-side apply_to match: a comma/newline list where an
 * entry is `*`, an exact name, or a `zen-*` style prefix. Used to decide
 * whether a list binds the pool or provider currently on screen.
 */
export function bindsTo(applyTo: string | undefined, name: string): boolean {
  const patterns = bindEntries(applyTo);
  if (patterns.length === 0) return true;
  const target = name.trim();
  if (target === "") return true;
  return patterns.some((pattern) => {
    if (pattern === "*") return true;
    if (pattern.toLowerCase() === target.toLowerCase()) return true;
    if (!pattern.endsWith("*")) return false;
    return target.toLowerCase().startsWith(pattern.slice(0, -1).toLowerCase());
  });
}

/** Splits an apply_to list into its entries, order preserved. */
export function bindEntries(applyTo: string | undefined): string[] {
  return (applyTo ?? "")
    .split(/[,;\n]/)
    .map((entry) => entry.trim())
    .filter((entry) => entry !== "");
}

/**
 * Binds or unbinds one target (a pool or provider) in an apply_to list.
 *
 * An empty list means "everything", so unbinding the only target has to
 * materialise the remaining targets explicitly — dropping the entry would
 * silently re-bind every one of them. When the result ends up covering every
 * known target it collapses back to an empty list, which reads as "all" and
 * keeps providers added later bound too.
 */
export function toggleBindTarget(
  applyTo: string | undefined,
  target: string,
  allTargets: string[],
): string {
  const wanted = target.trim();
  if (wanted === "") return (applyTo ?? "").trim();
  const entries = bindEntries(applyTo);

  if (!bindsTo(applyTo, wanted)) {
    const next = entries.filter((entry) => entry.toLowerCase() !== wanted.toLowerCase());
    return joinTargets([...next, wanted], allTargets);
  }

  // Bound: drop every entry that covers this target. An empty (or wildcard
  // only) list means everything, so what is left has to be written out
  // explicitly — dropping the entry would silently re-bind everything.
  const remaining = entries.filter((entry) => !bindsTo(entry, wanted));
  if (remaining.length === 0) {
    return joinTargets(
      allTargets.filter((name) => name.toLowerCase() !== wanted.toLowerCase()),
      allTargets,
    );
  }
  return joinTargets(remaining, allTargets);
}

function joinTargets(entries: string[], allTargets: string[]): string {
  const joined = entries.map((entry) => entry.trim()).filter((entry) => entry !== "");
  if (
    allTargets.length > 0 &&
    allTargets.every((name) => bindsTo(joined.join(", "), name))
  ) {
    // Every known target is listed: say "all" instead so the list keeps
    // covering pools and providers added later.
    return "";
  }
  return joined.join(", ");
}

/** The `servers` payloads of every applied extension that provides egress. */
export function useEgressServerLists(): EgressServerList[] {
  const { data: extensions } = useQuery({
    queryKey: ["extensions"],
    queryFn: fetchExtensions,
    staleTime: 30_000,
  });

  const egressExtensions = (extensions ?? []).filter(
    (extension) => extension.applied && (extension.provides?.features ?? []).includes("egress"),
  );

  const results = useQueries({
    queries: egressExtensions.map((extension) => ({
      queryKey: ["extension-servers", extension.id],
      queryFn: async () =>
        EgressServersPayloadSchema.parse(await fetchExtensionData(extension.id, "servers")),
      staleTime: 15_000,
      refetchInterval: 30_000,
      retry: false,
    })),
  });

  const lists: EgressServerList[] = [];
  egressExtensions.forEach((extension, index) => {
    const payload = results[index]?.data;
    if (payload) {
      lists.push({
        id: extension.id,
        name: extension.name,
        accent: extension.ui?.accent ?? undefined,
        payload,
      });
    }
  });
  return lists;
}

/** Selected endpoints of the lists bound to `name`, extensions first. */
export function serversBoundTo(lists: EgressServerList[], name: string): {
  servers: EgressServer[];
  lists: EgressServerList[];
} {
  const bound = lists.filter((list) => bindsTo(list.payload.apply_to, name));
  const servers = bound.flatMap((list) =>
    list.payload.servers.filter((server) => server.selected),
  );
  return { servers, lists: bound };
}

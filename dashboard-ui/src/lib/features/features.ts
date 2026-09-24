import { useDashboardConfig } from "@/lib/api/useDashboardConfig";

/** Catalog of operator-hideable dashboard features. */
export interface FeatureMeta {
  id: string;
  label: string;
  description: string;
}

/**
 * Core pages that stay visible even when the operator hides optional surfaces.
 * models/overview/settings are intentionally not hideable.
 */
export const HIDEABLE_FEATURES: readonly FeatureMeta[] = [
  { id: "pools", label: "Pools", description: "Pool definitions stay on disk; only the page leaves the nav." },
  { id: "guide", label: "Guide", description: "Hide the onboarding / CLI configurator page." },
  { id: "playground", label: "Playground", description: "Hide the interactive chat playground." },
  { id: "combos", label: "Combos", description: "Hide combo presets; model overrides are kept." },
  { id: "fallback", label: "Fallback", description: "Hide fallback UI; fallback rules remain configured." },
  { id: "audit_logs", label: "Audit Logs", description: "Stop showing the audit log UI; historical rows are kept." },
  { id: "console", label: "Live Console", description: "Hide the live request console; stored logs are kept." },
  { id: "usage", label: "Usage", description: "Hide usage analytics; usage data is retained." },
  { id: "cache", label: "Cache", description: "Hide the cache page; cache config and data stay." },
  { id: "auth_keys", label: "API Keys", description: "Hide key management UI; existing keys remain valid." },
  { id: "workflows", label: "Workflows", description: "Hide workflows UI; workflow files stay on disk." },
  { id: "guardrails", label: "Guardrails", description: "Hide guardrails UI; policy config is not deleted." },
  { id: "providers", label: "Settings · Providers", description: "Hide the Providers settings tab only." },
  { id: "infrastructure", label: "Settings · Infrastructure", description: "Hide the Infrastructure settings tab only." },
  { id: "caching", label: "Settings · Caching", description: "Hide the Caching settings tab only." },
  { id: "networking", label: "Settings · Networking", description: "Hide the Networking settings tab only." },
  { id: "sessionhub", label: "Settings · Session Hub", description: "Hide the Session Hub settings tab only." },
  { id: "sidecar", label: "Settings · Sidecar", description: "Hide the Sidecar settings tab only." },
  { id: "extensions", label: "Settings · Extensions", description: "Hide the Extensions settings tab only." },
];

const HIDEABLE_IDS = new Set(HIDEABLE_FEATURES.map((f) => f.id));

export function normalizeHiddenFeatures(values: readonly string[] | undefined | null): string[] {
  if (!values?.length) return [];
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of values) {
    const id = String(raw ?? "").trim().toLowerCase();
    if (!id || seen.has(id)) continue;
    seen.add(id);
    out.push(id);
  }
  return out;
}

export function isHideableFeature(id: string): boolean {
  return HIDEABLE_IDS.has(id);
}

export function hiddenFeatureSet(values: readonly string[] | undefined | null): Set<string> {
  return new Set(normalizeHiddenFeatures(values));
}

/** Reads `settings.ui.hidden_features` from the dashboard config snapshot. */
export function extractHiddenFeatures(config: { settings?: { ui?: { hidden_features?: string[] } } } | undefined | null): string[] {
  return normalizeHiddenFeatures(config?.settings?.ui?.hidden_features);
}

export function useHiddenFeatureIds(): string[] {
  const { data: config } = useDashboardConfig();
  return extractHiddenFeatures(config);
}

export function useIsFeatureHidden(): (featureId: string) => boolean {
  const ids = useHiddenFeatureIds();
  const set = hiddenFeatureSet(ids);
  return (featureId: string) => set.has(featureId);
}

/** Path segment → feature id for route gating. */
export function featureIdForPath(pathname: string): string | undefined {
  const path = pathname.replace(/\/+$/, "") || "/";
  const segment = path.split("/").filter(Boolean).pop() ?? "";
  switch (segment) {
    case "pools":
      return "pools";
    case "guide":
    case "cli-tools":
      return "guide";
    case "playground":
      return "playground";
    case "combos":
      return "combos";
    case "fallback":
      return "fallback";
    case "audit-logs":
      return "audit_logs";
    case "console":
      return "console";
    case "usage":
      return "usage";
    case "cache":
      return "cache";
    case "auth-keys":
      return "auth_keys";
    case "workflows":
      return "workflows";
    case "guardrails":
      return "guardrails";
    default:
      return undefined;
  }
}

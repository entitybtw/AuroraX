import { useDashboardConfig } from "@/lib/api/useDashboardConfig";
import { t } from "@/lib/i18n";


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
  { id: "pools", label: t("Pools"), description: t("Pool definitions stay on disk; only the page leaves the nav.") },
  { id: "guide", label: t("Guide"), description: t("Hide the onboarding / CLI configurator page.") },
  { id: "playground", label: t("Playground"), description: t("Hide the interactive chat playground.") },
  { id: "combos", label: t("Combos"), description: t("Hide combo presets; model overrides are kept.") },
  { id: "fallback", label: t("Fallback"), description: t("Hide fallback UI; fallback rules remain configured.") },
  { id: "audit_logs", label: t("Audit Logs"), description: t("Stop showing the audit log UI; historical rows are kept.") },
  { id: "console", label: t("Live Console"), description: t("Hide the live request console; stored logs are kept.") },
  { id: "usage", label: t("Usage"), description: t("Hide usage analytics; usage data is retained.") },
  { id: "cache", label: t("Cache"), description: t("Hide the cache page; cache config and data stay.") },
  { id: "auth_keys", label: t("API Keys"), description: t("Hide key management UI; existing keys remain valid.") },
  { id: "workflows", label: t("Workflows"), description: t("Hide workflows UI; workflow files stay on disk.") },
  { id: "guardrails", label: t("Guardrails"), description: t("Hide guardrails UI; policy config is not deleted.") },
  { id: "providers", label: t("Settings · Providers"), description: t("Hide the Providers settings tab only.") },
  { id: "infrastructure", label: t("Settings · Infrastructure"), description: t("Hide the Infrastructure settings tab only.") },
  { id: "caching", label: t("Settings · Caching"), description: t("Hide the Caching settings tab only.") },
  { id: "networking", label: t("Settings · Networking"), description: t("Hide the Networking settings tab only.") },
  { id: "sessionhub", label: t("Settings · Session Hub"), description: t("Hide the Session Hub settings tab only.") },
  { id: "sidecar", label: t("Settings · Sidecar"), description: t("Hide the Sidecar settings tab only.") },
  { id: "extensions", label: t("Settings · Extensions"), description: t("Hide the Extensions settings tab only.") },
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

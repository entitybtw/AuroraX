import type { HeaderRule } from "@/components/settings/HeaderRulesEditor";

/**
 * A sidecar preset is a coherent bundle: the sidecar settings plus the Session
 * Hub header rules that an upstream client fingerprint expects. Applying a
 * preset configures both together so the user does not have to reason about
 * the two tabs independently.
 */
export interface SidecarPreset {
  id: string;
  name: string;
  tagline: string;
  description: string;
  /** Partial sidecar settings applied when the preset is used. */
  sidecar: {
    enabled: boolean;
    inject_tools: boolean;
    default_auth: string;
    base_url: string;
    user_agent: string;
  };
  /** Canonical Session Hub header rules for this preset. */
  headers: HeaderRule[];
  /** Human-readable rows rendered in the UI. */
  summary: { label: string; value: string }[];
}

const OPENCODE_HEADERS: HeaderRule[] = [
  {
    name: "x-opencode-session",
    mode: "map_or_generate",
    prefix: "ses_",
    length: 26,
    value: "",
    values: [],
    charset: "hex",
  },
  { name: "x-opencode-client", mode: "static", prefix: "", length: 0, value: "cli", values: [], charset: "" },
  {
    name: "x-opencode-request",
    mode: "generate",
    prefix: "msg_",
    length: 26,
    value: "",
    values: [],
    charset: "hex",
  },
  { name: "x-opencode-project", mode: "static", prefix: "", length: 0, value: "global", values: [], charset: "" },
];

export const SIDECAR_PRESETS: SidecarPreset[] = [
  {
    id: "opencode",
    name: "upstream (free tier)",
    tagline: "free tier and opencode-go free models",
    description:
      "Emulates the the CLI fingerprint (Bun TLS + full tool schema) and installs the matching x-opencode-* Session Hub headers. The free tier requires the session to be exactly 26 hex characters.",
    sidecar: {
      enabled: true,
      inject_tools: true,
      default_auth: "Bearer public",
      base_url: "https://opencode.ai/zen/v1",
      user_agent: "opencode/1.18.31 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14",
    },
    headers: OPENCODE_HEADERS,
    summary: [
      { label: "Session header", value: "x-opencode-session = ses_ + 26 hex" },
      { label: "Request header", value: "x-opencode-request = msg_ + 26 hex" },
      { label: "Client header", value: "x-opencode-client = cli" },
      { label: "Project header", value: "x-opencode-project = global" },
      { label: "Tool injection", value: "Full upstream tool schema" },
      { label: "Auth", value: "Bearer public (free tier)" },
    ],
  },
];

/** A minimal header shape used for matching (fields may be absent). */
export interface MatchableHeader {
  name: string;
  mode: string;
  prefix?: string;
  length?: number;
  value?: string;
  charset?: string;
}

export const DEFAULT_SIDECAR_PRESET_ID = "opencode";

export function getSidecarPreset(id: string): SidecarPreset {
  return SIDECAR_PRESETS.find((p) => p.id === id) ?? (SIDECAR_PRESETS[0] as SidecarPreset);
}

/** Normalize a header rule for comparison (charset defaults to alphanumeric). */
function norm(h: MatchableHeader) {
  return {
    name: h.name,
    mode: h.mode,
    prefix: h.prefix || "",
    length: h.length || 0,
    value: h.value || "",
    charset: (h.charset || "alphanumeric").toLowerCase(),
  };
}

/** True when every preset header is present on the rule with the exact values. */
export function ruleMatchesPreset(
  rule: { headers: MatchableHeader[] } | undefined,
  preset: SidecarPreset,
): boolean {
  if (!rule || !rule.headers || rule.headers.length === 0) return false;
  return preset.headers.every((want) => {
    const got = rule.headers.find((h) => h.name === want.name);
    if (!got) return false;
    const a = norm(want);
    const b = norm(got);
    return (
      a.name === b.name &&
      a.mode === b.mode &&
      a.prefix === b.prefix &&
      a.length === b.length &&
      a.value === b.value &&
      a.charset === b.charset
    );
  });
}

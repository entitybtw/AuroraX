import { z } from "zod";
import { apiFetch } from "./client";

const ExtensionHeaderSchema = z.object({
  name: z.string(),
  mode: z.string(),
  prefix: z.string().optional(),
  length: z.number().optional(),
  charset: z.string().optional(),
  value: z.string().optional(),
  values: z.array(z.string()).optional(),
});

const ExtensionFieldSchema = z.object({
  key: z.string(),
  label: z.string(),
  description: z.string().optional(),
  type: z.string().optional(),
  default: z.string().optional(),
  options: z.array(z.string()).optional(),
  secret: z.boolean().optional(),
});

const ExtensionNavLinkSchema = z.object({
  label: z.string(),
  href: z.string(),
});

const ExtensionNavEntrySchema = z.object({
  id: z.string(),
  label: z.string(),
  to: z.string(),
  icon: z.string().optional(),
  order: z.number().optional(),
});

const ExtensionBannerSchema = z.object({
  id: z.string(),
  level: z.enum(["info", "warning", "error", "success"]).optional(),
  message: z.string(),
  link_label: z.string().optional(),
  link_url: z.string().optional(),
  order: z.number().optional(),
});

const ExtensionWidgetSchema = z.object({
  id: z.string(),
  slot: z.string().optional(),
  title: z.string().optional(),
  kind: z.enum(["text", "stats", "links"]).optional(),
  body: z.string().optional(),
  stats: z.array(z.record(z.string())).optional(),
  links: z.array(ExtensionNavLinkSchema).optional(),
  order: z.number().optional(),
});

const ExtensionUIBlockSchema = z.object({
  kind: z.enum(["heading", "text", "code", "list", "links", "divider", "kv"]),
  text: z.string().optional(),
  language: z.string().optional(),
  items: z.array(z.string()).optional(),
  links: z.array(ExtensionNavLinkSchema).optional(),
  kv: z.array(z.record(z.string())).optional(),
});

const ExtensionUIPageSchema = z.object({
  id: z.string(),
  path: z.string(),
  title: z.string(),
  icon: z.string().optional(),
  order: z.number().optional(),
  summary: z.string().optional(),
  blocks: z.array(ExtensionUIBlockSchema).optional(),
});

const ExtensionSettingsTabSchema = z.object({
  id: z.string(),
  label: z.string(),
  icon: z.string().optional(),
  order: z.number().optional(),
  blocks: z.array(ExtensionUIBlockSchema).optional(),
});

const ExtensionUISchema = z.object({
  accent: z.string().optional(),
  docs_url: z.string().optional(),
  help: z.string().optional(),
  fields: z.array(ExtensionFieldSchema).optional(),
  theme: z.record(z.string()).optional(),
  theme_light: z.record(z.string()).optional(),
  theme_dark: z.record(z.string()).optional(),
  nav: z.array(ExtensionNavEntrySchema).optional(),
  hide_nav: z.array(z.string()).optional(),
  banners: z.array(ExtensionBannerSchema).optional(),
  widgets: z.array(ExtensionWidgetSchema).optional(),
  pages: z.array(ExtensionUIPageSchema).optional(),
  settings_tabs: z.array(ExtensionSettingsTabSchema).optional(),
  hide_settings_tabs: z.array(z.string()).optional(),
  logo_text: z.string().optional(),
  logo_url: z.string().optional(),
});

const ExtensionUIContributionSchema = z.object({
  id: z.string(),
  name: z.string(),
  type: z.string().optional(),
  ui: ExtensionUISchema,
});

const ExtensionToolSchema = z.object({
  name: z.string(),
  source: z.string().optional(),
  inline: z.unknown().optional(),
});

const ExtensionProvidesSchema = z.object({
  provider_types: z.array(z.string()).optional(),
  features: z.array(z.string()).optional(),
});

const ExtensionOAuthSchema = z.object({
  server: z.string().optional(),
  client_id: z.string().optional(),
  verification_base: z.string().optional(),
  user_agent: z.string().optional(),
  scope: z.string().optional(),
  grant: z.string().optional(),
  authorize_url: z.string().optional(),
  token_url: z.string().optional(),
  token_style: z.string().optional(),
  scopes: z.string().optional(),
  state_is_verifier: z.boolean().optional(),
  redirect_uri: z.string().optional(),
});

const ExtensionSchema = z.object({
  id: z.string(),
  name: z.string(),
  tagline: z.string().optional(),
  description: z.string().optional(),
  version: z.string().optional(),
  author: z.string().optional(),
  homepage: z.string().optional(),
  license: z.string().optional(),
  type: z.string().optional(),
  schema: z.number().optional(),
  base_url: z.string().optional(),
  user_agent: z.string().optional(),
  default_auth: z.string().optional(),
  inject_tools: z.boolean().optional(),
  inject_tool_types: z.array(z.string()).optional(),
  max_attempts: z.number().optional(),
  retry_delay_ms: z.number().optional(),
  headers: z.array(ExtensionHeaderSchema).optional(),
  tool_schemas: z.array(ExtensionToolSchema).optional(),
  requirements: z.array(z.string()).optional(),
  notes: z.array(z.string()).optional(),
  settings: z.record(z.string()).optional(),
  config: z.record(z.string()).optional(),
  oauth: ExtensionOAuthSchema.optional(),
  files: z.record(z.string()).optional(),
  provides: ExtensionProvidesSchema.optional(),
  ui: ExtensionUISchema.optional(),
  applied: z.boolean().optional(),
  builtin: z.boolean(),
  order: z.number().optional(),
  source: z.string().optional(),
});

export type Extension = z.infer<typeof ExtensionSchema>;
export type ExtensionHeader = z.infer<typeof ExtensionHeaderSchema>;
export type ExtensionField = z.infer<typeof ExtensionFieldSchema>;
export type ExtensionOAuth = z.infer<typeof ExtensionOAuthSchema>;
export type ExtensionUI = z.infer<typeof ExtensionUISchema>;
export type ExtensionNavEntry = z.infer<typeof ExtensionNavEntrySchema>;
export type ExtensionBanner = z.infer<typeof ExtensionBannerSchema>;
export type ExtensionWidget = z.infer<typeof ExtensionWidgetSchema>;
export type ExtensionUIBlock = z.infer<typeof ExtensionUIBlockSchema>;
export type ExtensionUIPage = z.infer<typeof ExtensionUIPageSchema>;
export type ExtensionSettingsTab = z.infer<typeof ExtensionSettingsTabSchema>;
export type ExtensionUIContribution = z.infer<typeof ExtensionUIContributionSchema>;
export type ExtensionNavLink = z.infer<typeof ExtensionNavLinkSchema>;

/** @deprecated Prefer Extension aliases kept for older imports. */
export type SidecarPreset = Extension;
export type SidecarPresetHeader = ExtensionHeader;
export type SidecarPresetField = ExtensionField;

export async function fetchExtensions(): Promise<Extension[]> {
  const res = await apiFetch<{ extensions?: unknown[]; presets?: unknown[] }>(
    "/admin/api/v1/sidecar/extensions",
  );
  const list = res.extensions ?? res.presets ?? [];
  return z.array(ExtensionSchema).parse(list);
}

/** @deprecated Prefer fetchExtensions. */
export const fetchSidecarPresets = fetchExtensions;

export async function importExtension(input: {
  json?: string;
  url?: string;
}): Promise<Extension> {
  const body = input.url ? { url: input.url } : JSON.parse(input.json ?? "{}");
  const res = await apiFetch<{ extension?: unknown; preset?: unknown }>(
    "/admin/api/v1/sidecar/extensions/import",
    { method: "POST", json: body },
  );
  return ExtensionSchema.parse(res.extension ?? res.preset);
}

/** @deprecated Prefer importExtension. */
export const importSidecarPreset = importExtension;

export async function deleteExtension(id: string): Promise<void> {
  await apiFetch(`/admin/api/v1/sidecar/extensions/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

/** @deprecated Prefer deleteExtension. */
export const deleteSidecarPreset = deleteExtension;

export function exportExtensionUrl(id: string): string {
  const base = (import.meta.env.BASE_URL || "/").replace(/\/$/, "");
  return `${base}/admin/api/v1/sidecar/extensions/${encodeURIComponent(id)}/export`;
}

/** @deprecated Prefer exportExtensionUrl. */
export const exportSidecarPresetUrl = exportExtensionUrl;

export async function applyExtension(
  id: string,
): Promise<{
  headers: ExtensionHeader[];
  tools: string[];
  oauth?: ExtensionOAuth | undefined;
  tools_path?: string | undefined;
}> {
  const res = await apiFetch<{
    headers?: ExtensionHeader[];
    tools?: string[];
    oauth?: ExtensionOAuth;
    tools_path?: string;
  }>(`/admin/api/v1/sidecar/extensions/${encodeURIComponent(id)}/apply`, {
    method: "POST",
  });
  return {
    headers: res.headers ?? [],
    tools: res.tools ?? [],
    oauth: res.oauth,
    tools_path: res.tools_path,
  };
}

/** @deprecated Prefer applyExtension. */
export const applySidecarPreset = applyExtension;

/** Mark an extension as not applied (drops its UI contribution). */
export async function unapplyExtension(id: string): Promise<void> {
  await apiFetch(`/admin/api/v1/sidecar/extensions/${encodeURIComponent(id)}/unapply`, {
    method: "POST",
  });
}

/** Persist config overrides for an extension (merged into ui.theme server-side). */
export async function updateExtensionConfig(
  id: string,
  config: Record<string, string>,
): Promise<void> {
  await apiFetch(`/admin/api/v1/sidecar/extensions/${encodeURIComponent(id)}/config`, {
    method: "PUT",
    json: { config },
  });
}

/** Rename an extension (operator-custom display name). */
export async function renameExtension(id: string, name: string): Promise<Extension> {
  const res = await apiFetch<{ extension?: unknown }>(
    `/admin/api/v1/sidecar/extensions/${encodeURIComponent(id)}`,
    { method: "PUT", json: { name } },
  );
  return ExtensionSchema.parse(res.extension);
}

/** Persist list order by posting the full id sequence. */
export async function reorderExtensions(ids: string[]): Promise<Extension[]> {
  const res = await apiFetch<{ extensions?: unknown[] }>(
    "/admin/api/v1/sidecar/extensions/reorder",
    { method: "POST", json: { ids } },
  );
  return z.array(ExtensionSchema).parse(res.extensions ?? []);
}

/** Apply everywhere: activate the extension and optionally bind session hub headers. */
export async function fullApplyExtension(
  id: string,
  sessionHubTarget?: string,
): Promise<{
  headers: ExtensionHeader[];
  tools: string[];
  oauth?: ExtensionOAuth | undefined;
  tools_path?: string | undefined;
}> {
  const body = sessionHubTarget ? { session_hub_target: sessionHubTarget } : {};
  const res = await apiFetch<{
    headers?: ExtensionHeader[];
    tools?: string[];
    oauth?: ExtensionOAuth;
    tools_path?: string;
  }>(`/admin/api/v1/sidecar/extensions/${encodeURIComponent(id)}/full-apply`, {
    method: "POST",
    json: body,
  });
  return {
    headers: res.headers ?? [],
    tools: res.tools ?? [],
    oauth: res.oauth,
    tools_path: res.tools_path,
  };
}

/** Merge header rules into a Session Hub target (idempotent). */
export async function ensureExtensionHeaders(
  target: string,
  headers: ExtensionHeader[],
): Promise<{ added: string[]; already: string[] }> {
  const res = await apiFetch<{ data: { added: string[]; already: string[] } }>(
    `/admin/api/v1/sessionhub/providers/${encodeURIComponent(target)}/ensure-headers`,
    { method: "POST", json: { headers } },
  );
  return res.data;
}

/** @deprecated Prefer ensureExtensionHeaders. */
export const ensurePresetHeaders = ensureExtensionHeaders;

export async function listExtensionStores(): Promise<string[]> {
  const res = await apiFetch<{ stores?: string[] }>(
    "/admin/api/v1/sidecar/extensions/stores",
  );
  return res.stores ?? [];
}

export async function addExtensionStore(url: string): Promise<string[]> {
  const res = await apiFetch<{ stores?: string[] }>(
    "/admin/api/v1/sidecar/extensions/stores",
    { method: "POST", json: { url } },
  );
  return res.stores ?? [];
}

export async function deleteExtensionStore(url: string): Promise<string[]> {
  const res = await apiFetch<{ stores?: string[] }>(
    `/admin/api/v1/sidecar/extensions/stores?url=${encodeURIComponent(url)}`,
    { method: "DELETE" },
  );
  return res.stores ?? [];
}

export async function checkExtensionUpdate(id: string): Promise<{
  id: string;
  source: string;
  current_version: string;
  remote_version: string;
  update_available: boolean;
}> {
  return apiFetch(
    `/admin/api/v1/sidecar/extensions/${encodeURIComponent(id)}/check-update`,
  );
}

export async function updateExtensionFromSource(id: string): Promise<Extension> {
  const res = await apiFetch<{ extension?: unknown }>(
    `/admin/api/v1/sidecar/extensions/${encodeURIComponent(id)}/update`,
    { method: "POST" },
  );
  return ExtensionSchema.parse(res.extension);
}

export async function browseExtensionStore(params: {
  url: string;
  q?: string;
  type?: string;
}): Promise<unknown> {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v) qs.set(k, String(v));
  }
  return apiFetch(`/admin/api/v1/sidecar/extensions/store/browse?${qs}`, {
    method: "POST",
    json: { url: params.url },
  });
}

export async function installExtensionFromStore(input: {
  url?: string;
  store?: string;
  id?: string;
}): Promise<Extension> {
  const res = await apiFetch<{ extension?: unknown; preset?: unknown }>(
    "/admin/api/v1/sidecar/extensions/store/install",
    { method: "POST", json: input },
  );
  return ExtensionSchema.parse(res.extension ?? res.preset);
}

/**
 * Fetch UI contributions from applied extensions (nav, pages, banners,
 * widgets, theme overrides, settings tabs).
 */
export async function fetchExtensionUI(): Promise<ExtensionUIContribution[]> {
  const res = await apiFetch<{ contributions?: unknown[] }>(
    "/admin/api/v1/sidecar/extensions/ui",
  );
  return z.array(ExtensionUIContributionSchema).parse(res.contributions ?? []);
}

export type { ExtensionUIContribution as ExtensionUIContributionType };

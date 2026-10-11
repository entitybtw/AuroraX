// Lightweight i18n for the dashboard.
//
// Design notes:
//   - The English source string IS the translation key (no key ceremony);
//     a missing entry falls back to the key itself, so English always works
//     even with no catalogs loaded.
//   - The active locale is the same persisted operator selection the
//     extension language packs use (GET/PUT /sidecar/extensions/locale),
//     so one switch in Settings → Extensions changes both the extension
//     strings and the whole dashboard chrome.
//   - Translation catalogs are NOT bundled into the SPA: they ship inside
//     installed language packs (type "localization" with a `dashboard` map)
//     and are fetched from the gateway (GET /sidecar/extensions/locale/catalog).
//     The last successful catalog is cached in localStorage so a signed-out
//     boot (e.g. the login page) still renders in the operator's language.
//   - Changing the language saves the locale and reloads the page, so
//     module-scope translated constants re-evaluate with the new locale.
//   - t() is a plain function: no React context, usable in .ts modules.

import { fetchExtensionLocale, fetchExtensionLocaleCatalog } from "@/lib/api/extensions";

/** Right-to-left scripts — the document direction follows these. */
const RTL_LOCALES = new Set(["ar", "he"]);

/** localStorage keys for the last successfully loaded catalog. */
const CACHE_LOCALE_KEY = "aurora-ui-catalog-locale";
const CACHE_CATALOG_KEY = "aurora-ui-catalog";

type Catalog = Record<string, string>;

let locale = "en";
let catalog: Catalog = {};

/** Current UI locale ("en" = source language). */
export function currentLocale(): string {
  return locale;
}

function applyDocumentLang(): void {
  if (typeof document === "undefined") return;
  document.documentElement.lang = locale;
  document.documentElement.dir = RTL_LOCALES.has(locale) ? "rtl" : "ltr";
}

/** Interpolate {name} placeholders into a translated string. */
function interpolate(template: string, params?: Record<string, string | number>): string {
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (whole, name: string) =>
    Object.prototype.hasOwnProperty.call(params, name) ? String(params[name]) : whole,
  );
}

/**
 * Translate key → localized string.
 *
 * Leading/trailing whitespace of the key is preserved around the lookup so
 * JSX text chunks adjacent to expressions ("Total " / " providers") keep
 * their spacing in every language. Unknown keys return the source text.
 */
export function t(key: string, params?: Record<string, string | number>): string {
  const lead = key.match(/^\s*/)?.[0] ?? "";
  const trail = key.match(/\s*$/)?.[0] ?? "";
  const core = key.slice(lead.length, key.length - trail.length);
  const hit = catalog[core] ?? catalog[key];
  const out = hit ?? core;
  return interpolate(lead + out + trail, params);
}

function readCache(): { locale: string; catalog: Catalog } | null {
  if (typeof localStorage === "undefined") return null;
  try {
    const code = localStorage.getItem(CACHE_LOCALE_KEY);
    const raw = localStorage.getItem(CACHE_CATALOG_KEY);
    if (!code || !raw) return null;
    const parsed = JSON.parse(raw) as Catalog;
    return { locale: code, catalog: typeof parsed === "object" && parsed !== null ? parsed : {} };
  } catch {
    return null;
  }
}

function writeCache(code: string, cat: Catalog): void {
  if (typeof localStorage === "undefined") return;
  try {
    localStorage.setItem(CACHE_LOCALE_KEY, code);
    localStorage.setItem(CACHE_CATALOG_KEY, JSON.stringify(cat));
  } catch {
    // Quota/privacy errors — the next successful fetch simply refills it.
  }
}

/**
 * Load the dashboard catalog for a locale from the gateway, falling back to
 * the localStorage cache (last successful load) and finally to the source
 * language. Successful fetches refresh the cache.
 */
async function loadCatalog(code: string): Promise<void> {
  if (code === "en" || code === "") {
    catalog = {};
    return;
  }
  try {
    const res = await fetchExtensionLocaleCatalog(code);
    catalog = res.catalog;
    if (Object.keys(catalog).length > 0) {
      writeCache(code, catalog);
      return;
    }
  } catch {
    // Unauthenticated/offline — fall through to the cache.
  }
  const cached = readCache();
  catalog = cached && cached.locale === code ? cached.catalog : {};
}

/**
 * Normalize a locale code from the gateway/storage. Any syntactically valid
 * BCP-47-ish code passes through — available languages are driven by the
 * installed language packs, not a hard-coded list. Garbage falls back to
 * "en" so a corrupt persisted value can't blank the UI.
 */
function normalize(code: string): string {
  const norm = code.trim().toLowerCase().replace(/_/g, "-");
  return /^[a-z]{2,8}(-[a-z0-9]{1,8})*$/.test(norm) ? norm : "en";
}

/** Set the locale synchronously (catalog must already be loaded). */
function setLocaleSync(code: string): void {
  locale = code;
  applyDocumentLang();
}

/**
 * Initial load: fetch the persisted locale and its catalog before the app
 * modules render, so module-scope translated constants start out correct.
 * A failed locale fetch (e.g. not signed in yet) falls back to the cached
 * locale+catalog from the last signed-in session, then to English.
 */
export async function initI18n(): Promise<void> {
  let code = "en";
  let fetched = false;
  try {
    const info = await fetchExtensionLocale();
    code = normalize(info.locale);
    fetched = true;
  } catch {
    const cached = readCache();
    code = cached ? normalize(cached.locale) : "en";
  }
  if (fetched) {
    await loadCatalog(code);
  } else {
    const cached = readCache();
    catalog = cached && cached.locale === code ? cached.catalog : {};
  }
  setLocaleSync(code);
}

/**
 * Re-check the persisted locale after authentication becomes available
 * (login, SSO, OIDC callback). Called from the login flows: before a key or
 * session exists the boot-time fetch fails and the UI keeps the cached (or
 * source) language; once credentials are stored we pick the real locale up.
 * If it differs from the language the page was rendered in, the page reloads
 * so module-scope translated constants re-evaluate.
 */
export async function ensureLocaleSynced(): Promise<void> {
  let code = "en";
  try {
    const info = await fetchExtensionLocale();
    code = normalize(info.locale);
  } catch {
    return; // still unauthenticated or endpoint missing — keep what we have
  }
  if (code === locale) return;
  await loadCatalog(code);
  setLocaleSync(code);
  if (typeof window !== "undefined") {
    window.location.reload();
  }
}

/**
 * Switch the UI language: load the catalog, then reload the page so every
 * module-scope constant re-evaluates. Returns the normalized code.
 */
export async function changeLocale(code: string): Promise<string> {
  const next = normalize(code);
  await loadCatalog(next);
  setLocaleSync(next);
  if (typeof window !== "undefined") {
    window.location.reload();
  }
  return next;
}

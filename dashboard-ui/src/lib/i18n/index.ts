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
//   - Catalogs are code-split JSON loaded dynamically per locale.
//   - Changing the language saves the locale and reloads the page, so
//     module-scope translated constants re-evaluate with the new locale.
//   - t() is a plain function: no React context, usable in .ts modules.

import { fetchExtensionLocale } from "@/lib/api/extensions";

/** Dashboard UI locales shipped as catalogs (en = the source language). */
export const SUPPORTED_LOCALES = [
  "ru",
  "uk",
  "de",
  "fr",
  "es",
  "pt-br",
  "it",
  "pl",
  "nl",
  "cs",
  "tr",
  "ja",
  "zh-cn",
  "zh-tw",
  "ko",
  "ar",
  "he",
  "hi",
  "id",
  "vi",
] as const;

/** Right-to-left scripts — the document direction follows these. */
const RTL_LOCALES = new Set(["ar", "he"]);

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

async function loadCatalog(code: string): Promise<void> {
  if (code === "en" || code === "") {
    catalog = {};
    return;
  }
  try {
    // Vite turns this into a glob over ./locales/*.json — one chunk per language.
    const mod = (await import(`./locales/${code}.json`)) as { default?: Catalog } | Catalog;
    catalog = ((mod as { default?: Catalog }).default ?? mod) as Catalog;
  } catch {
    // Unknown/missing catalog → source language rather than a broken UI.
    catalog = {};
  }
}

function normalize(code: string): string {
  const norm = code.trim().toLowerCase().replace(/_/g, "-");
  return SUPPORTED_LOCALES.includes(norm as (typeof SUPPORTED_LOCALES)[number]) ? norm : "en";
}

/** Set the locale synchronously (catalog must already be loaded). */
function setLocaleSync(code: string): void {
  locale = code;
  applyDocumentLang();
}

/**
 * Initial load: fetch the persisted locale and its catalog before the app
 * modules render, so module-scope translated constants start out correct.
 */
export async function initI18n(): Promise<void> {
  let code = "en";
  try {
    const info = await fetchExtensionLocale();
    code = normalize(info.locale);
  } catch {
    code = "en";
  }
  await loadCatalog(code);
  setLocaleSync(code);
}

/**
 * Re-check the persisted locale after authentication becomes available
 * (login, SSO, OIDC callback). Called from the login flows: before a key or
 * session exists the boot-time fetch fails and the UI stays on the source
 * language; once credentials are stored we pick the real locale up. If it
 * differs from the language the page was rendered in, the page reloads so
 * module-scope translated constants re-evaluate.
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

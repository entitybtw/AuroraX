import * as React from "react";
import { useExtensionUI } from "@/lib/api/useExtensionUI";
import { useTheme } from "@/lib/theme/theme";
import type {
  ExtensionBanner,
  ExtensionNavLink,
  ExtensionNavEntry,
  ExtensionSettingsTab,
  ExtensionUIBlock,
  ExtensionUIContribution,
  ExtensionUIPage,
  ExtensionWidget,
} from "@/lib/api/extensions";

interface ExtensionUIContextValue {
  contributions: ExtensionUIContribution[];
  /** CSS custom properties from applied extensions (sanitized). */
  themeVars: Record<string, string>;
  themeVarsLight: Record<string, string>;
  themeVarsDark: Record<string, string>;
  /** Union of hide_nav tokens from all contributions. */
  hideNav: Set<string>;
  /** Union of hide_settings_tabs from all contributions. */
  hideSettingsTabs: Set<string>;
  /** Accent override (last non-empty wins). */
  accent?: string | undefined;
  /** Brand text override (last non-empty wins). */
  logoText?: string | undefined;
  /** Brand logo URL override (last non-empty wins). */
  logoURL?: string | undefined;
  isLoading: boolean;
}

const ExtensionUIContext = React.createContext<ExtensionUIContextValue>({
  contributions: [],
  themeVars: {},
  themeVarsLight: {},
  themeVarsDark: {},
  hideNav: new Set(),
  hideSettingsTabs: new Set(),
  accent: undefined,
  logoText: undefined,
  logoURL: undefined,
  isLoading: false,
});

/**
 * Theme variables may use any safe CSS custom-property name (max customization).
 * Values are validated separately: no CSS injection characters.
 */
function isAllowedThemeVar(key: string): boolean {
  return /^--[a-z0-9][a-z0-9-]{0,62}$/i.test(key);
}

function isSafeThemeValue(value: unknown): value is string {
  if (typeof value !== "string") return false;
  if (value.length === 0 || value.length > 512) return false;
  if (/[;{}<>]/.test(value)) return false;
  if (/url\s*\(/i.test(value) || /expression\s*\(/i.test(value)) return false;
  if (value.includes("\\") || value.includes("`")) return false;
  return true;
}

function isValidAccent(value: unknown): value is string {
  return typeof value === "string" && /^#[0-9a-fA-F]{3,8}$/.test(value);
}

function isThemeContribution(c: ExtensionUIContribution): boolean {
  if (c.type === "theme") return true;
  const theme = c.ui.theme;
  return theme != null && Object.keys(theme).length > 0;
}

function sanitizeMap(map: Record<string, string> | undefined): Record<string, string> {
  const out: Record<string, string> = {};
  if (!map) return out;
  for (const [k, v] of Object.entries(map)) {
    if (!isAllowedThemeVar(k)) continue;
    if (!isSafeThemeValue(v)) continue;
    out[k] = v;
  }
  return out;
}

interface SanitizedThemes {
  base: Record<string, string>;
  light: Record<string, string>;
  dark: Record<string, string>;
  accent?: string | undefined;
}

function sanitizeThemeVars(contributions: ExtensionUIContribution[]): SanitizedThemes {
  const base: Record<string, string> = {};
  const light: Record<string, string> = {};
  const dark: Record<string, string> = {};
  let accent: string | undefined;
  for (const c of contributions) {
    if (isThemeContribution(c) && isValidAccent(c.ui.accent)) {
      accent = c.ui.accent;
    }
    Object.assign(base, sanitizeMap(c.ui.theme));
    Object.assign(light, sanitizeMap(c.ui.theme_light));
    Object.assign(dark, sanitizeMap(c.ui.theme_dark));
  }
  // Accent fix: accent only counts for theme contributions and is applied
  // after the theme map so ui.theme["--accent"] still wins when present.
  if (accent && base["--accent"] === undefined) {
    base["--accent"] = accent;
  }
  if (accent) {
    if (light["--accent"] === undefined) light["--accent"] = accent;
    if (dark["--accent"] === undefined) dark["--accent"] = accent;
  }
  return { base, light, dark, accent };
}

const STYLE_ID = "aurora-ext-theme";

function buildThemeCss(
  base: Record<string, string>,
  light: Record<string, string>,
  dark: Record<string, string>,
): string {
  const rules: string[] = [];
  const decls = (m: Record<string, string>): string =>
    Object.entries(m)
      .map(([k, v]) => `${k}: ${v};`)
      .join(" ");
  if (Object.keys(base).length > 0) {
    rules.push(`:root{${decls(base)}}`);
  }
  if (Object.keys(light).length > 0) {
    const merged = { ...base, ...light };
    rules.push(`[data-theme="light"]{${decls(merged)}}`);
    rules.push(
      `@media (prefers-color-scheme:light){:root:not([data-theme="dark"]){${decls(merged)}}}`,
    );
  }
  if (Object.keys(dark).length > 0) {
    const merged = { ...base, ...dark };
    rules.push(`[data-theme="dark"]{${decls(merged)}}`);
  }
  return rules.join("\n");
}

export function ExtensionUIProvider({ children }: { children: React.ReactNode }): JSX.Element {
  const query = useExtensionUI();
  const preference = useTheme();
  const contributions = React.useMemo(() => query.data ?? [], [query.data]);

  const { base: themeVars, light: themeVarsLight, dark: themeVarsDark, accent } =
    React.useMemo(() => sanitizeThemeVars(contributions), [contributions]);

  const hideNav = React.useMemo(() => {
    const s = new Set<string>();
    for (const c of contributions) {
      for (const h of c.ui.hide_nav ?? []) s.add(h.toLowerCase());
    }
    return s;
  }, [contributions]);

  const hideSettingsTabs = React.useMemo(() => {
    const s = new Set<string>();
    for (const c of contributions) {
      for (const h of c.ui.hide_settings_tabs ?? []) s.add(h.toLowerCase());
    }
    return s;
  }, [contributions]);

  const logoText = React.useMemo(() => {
    let t: string | undefined;
    for (const c of contributions) {
      if (c.ui.logo_text) t = c.ui.logo_text;
    }
    return t;
  }, [contributions]);

  const logoURL = React.useMemo(() => {
    let u: string | undefined;
    for (const c of contributions) {
      if (c.ui.logo_url) u = c.ui.logo_url;
    }
    if (!u) return undefined;
    if (/^https?:\/\//i.test(u) || u.startsWith("/") || u.startsWith("./")) return u;
    return undefined;
  }, [contributions]);

  // Inject theme CSS via a <style> tag so [data-theme] / prefers-color-scheme
  // selectors keep working (inline element styles would beat those selectors).
  React.useEffect(() => {
    const hasVars =
      Object.keys(themeVars).length > 0 ||
      Object.keys(themeVarsLight).length > 0 ||
      Object.keys(themeVarsDark).length > 0;
    let el = document.getElementById(STYLE_ID) as HTMLStyleElement | null;
    if (!hasVars) {
      el?.remove();
      return;
    }
    const css = buildThemeCss(themeVars, themeVarsLight, themeVarsDark);
    if (!el) {
      el = document.createElement("style");
      el.id = STYLE_ID;
      document.head.appendChild(el);
    }
    el.textContent = css;
    return () => {
      // Keep style until contributions change; cleaned on unmount below.
    };
  }, [themeVars, themeVarsLight, themeVarsDark, preference]);

  React.useEffect(() => {
    return () => {
      document.getElementById(STYLE_ID)?.remove();
    };
  }, []);

  // Update theme-color meta with the dominant background when possible.
  React.useEffect(() => {
    const meta = document.querySelector('meta[name="theme-color"]');
    if (!meta) return;
    const computed = getComputedStyle(document.documentElement);
    const bg = themeVars["--bg"] || computed.getPropertyValue("--bg").trim();
    if (bg) meta.setAttribute("content", bg);
  }, [themeVars, themeVarsLight, themeVarsDark, preference]);

  const value = React.useMemo<ExtensionUIContextValue>(
    () => ({
      contributions,
      themeVars,
      themeVarsLight,
      themeVarsDark,
      hideNav,
      hideSettingsTabs,
      accent,
      logoText,
      logoURL,
      isLoading: query.isLoading,
    }),
    [
      contributions,
      themeVars,
      themeVarsLight,
      themeVarsDark,
      hideNav,
      hideSettingsTabs,
      accent,
      logoText,
      logoURL,
      query.isLoading,
    ],
  );

  return (
    <ExtensionUIContext.Provider value={value}>{children}</ExtensionUIContext.Provider>
  );
}

export function useExtensionUIContext(): ExtensionUIContextValue {
  return React.useContext(ExtensionUIContext);
}

/** Flat list of extension sidebar entries (sorted by order then label). */
export function useExtensionNav(): Array<ExtensionNavEntry & { extensionId: string }> {
  const { contributions } = useExtensionUIContext();
  return React.useMemo(() => {
    const entries = contributions.flatMap((c) =>
      (c.ui.nav ?? []).map((n) => ({ ...n, extensionId: c.id })),
    );
    return entries.sort(
      (a, b) => (a.order ?? 0) - (b.order ?? 0) || a.label.localeCompare(b.label),
    );
  }, [contributions]);
}

/** Flat list of extension pages (sorted by order). */
export function useExtensionPages(): Array<
  ExtensionUIPage & { extensionId: string; extensionName: string }
> {
  const { contributions } = useExtensionUIContext();
  return React.useMemo(() => {
    const pages = contributions.flatMap((c) =>
      (c.ui.pages ?? []).map((p) => ({ ...p, extensionId: c.id, extensionName: c.name })),
    );
    return pages.sort(
      (a, b) => (a.order ?? 0) - (b.order ?? 0) || a.title.localeCompare(b.title),
    );
  }, [contributions]);
}

/** Banners sorted by order across all applied extensions. */
export function useExtensionBanners(): Array<ExtensionBanner & { extensionId: string }> {
  const { contributions } = useExtensionUIContext();
  return React.useMemo(() => {
    const banners = contributions.flatMap((c) =>
      (c.ui.banners ?? []).map((b) => ({ ...b, extensionId: c.id })),
    );
    return banners.sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
  }, [contributions]);
}

/** Widgets for a named slot (overview | settings). */
export function useExtensionWidgets(
  slot: string,
): Array<ExtensionWidget & { extensionId: string; extensionName: string }> {
  const { contributions } = useExtensionUIContext();
  return React.useMemo(() => {
    const widgets = contributions.flatMap((c) =>
      (c.ui.widgets ?? [])
        .filter((w) => (w.slot || "overview") === slot)
        .map((w) => ({ ...w, extensionId: c.id, extensionName: c.name })),
    );
    return widgets.sort(
      (a, b) => (a.order ?? 0) - (b.order ?? 0) || (a.title ?? "").localeCompare(b.title ?? ""),
    );
  }, [contributions, slot]);
}

/** Settings tabs contributed by extensions. */
export function useExtensionSettingsTabs(): Array<
  ExtensionSettingsTab & { extensionId: string }
> {
  const { contributions, hideSettingsTabs } = useExtensionUIContext();
  return React.useMemo(() => {
    const tabs = contributions.flatMap((c) =>
      (c.ui.settings_tabs ?? []).map((t) => ({ ...t, extensionId: c.id })),
    );
    return tabs
      .filter((t) => !hideSettingsTabs.has(t.id.toLowerCase()))
      .sort((a, b) => (a.order ?? 0) - (b.order ?? 0) || a.label.localeCompare(b.label));
  }, [contributions, hideSettingsTabs]);
}

/** Help / docs strings from applied extensions (for settings side panel). */
export function useExtensionHelp(): {
  help?: string | undefined;
  docsURL?: string | undefined;
} {
  const { contributions } = useExtensionUIContext();
  return React.useMemo(() => {
    let help: string | undefined;
    let docsURL: string | undefined;
    for (const c of contributions) {
      if (c.ui.help) help = c.ui.help;
      if (c.ui.docs_url) docsURL = c.ui.docs_url;
    }
    return { help, docsURL };
  }, [contributions]);
}

export type { ExtensionUIBlock, ExtensionNavLink };

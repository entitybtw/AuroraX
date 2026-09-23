import * as React from "react";
import { useExtensionUI } from "@/lib/api/useExtensionUI";
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
  /** CSS custom properties from applied extensions (allowlisted keys). */
  themeVars: Record<string, string>;
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
  hideNav: new Set(),
  hideSettingsTabs: new Set(),
  accent: undefined,
  logoText: undefined,
  logoURL: undefined,
  isLoading: false,
});

const THEME_VAR_ALLOWLIST = new Set([
  "--accent",
  "--accent-hover",
  "--accent-foreground",
  "--bg",
  "--bg-surface",
  "--bg-surface-hover",
  "--bg-elevated",
  "--text",
  "--text-muted",
  "--text-inverse",
  "--border",
  "--border-subtle",
  "--success",
  "--success-bg",
  "--warning",
  "--warning-bg",
  "--danger",
  "--danger-bg",
  "--info",
  "--info-bg",
  "--radius-sm",
  "--radius-md",
  "--radius-lg",
  "--radius-control",
  "--radius-card",
  "--radius-sheet",
  "--font-display",
  "--font-sans",
  "--font-mono",
  "--sidebar-width",
  "--space-page",
  "--chart-input",
  "--chart-output",
  "--chart-cat-1",
  "--chart-cat-2",
  "--chart-cat-3",
  "--chart-cat-4",
  "--chart-cat-5",
  "--chart-cat-6",
  "--chart-cat-7",
  "--chart-cat-8",
  "--chart-cat-9",
  "--chart-cat-10",
  "--glass-bg",
  "--glass-border",
  "--shadow-tint",
  "--ring-highlight",
]);

function isAllowedThemeVar(key: string): boolean {
  if (THEME_VAR_ALLOWLIST.has(key)) return true;
  return /^--chart-cat-\d+$/.test(key);
}

function sanitizeThemeVars(
  contributions: ExtensionUIContribution[],
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const c of contributions) {
    const theme = c.ui.theme;
    if (theme) {
      for (const [k, v] of Object.entries(theme)) {
        if (!isAllowedThemeVar(k)) continue;
        if (typeof v !== "string") continue;
        if (/[;{}<>]/.test(v)) continue;
        out[k] = v;
      }
    }
    if (c.ui.accent && /^#[0-9a-fA-F]{3,8}$/.test(c.ui.accent)) {
      out["--accent"] = c.ui.accent;
    }
  }
  return out;
}

export function ExtensionUIProvider({ children }: { children: React.ReactNode }): JSX.Element {
  const query = useExtensionUI();
  const contributions = React.useMemo(() => query.data ?? [], [query.data]);

  const themeVars = React.useMemo(() => sanitizeThemeVars(contributions), [contributions]);

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

  const accent = React.useMemo(() => {
    let a: string | undefined;
    for (const c of contributions) {
      if (c.ui.accent) a = c.ui.accent;
    }
    return a;
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

  React.useEffect(() => {
    const root = document.documentElement;
    const applied: string[] = [];
    for (const [k, v] of Object.entries(themeVars)) {
      root.style.setProperty(k, v);
      applied.push(k);
    }
    return () => {
      for (const k of applied) root.style.removeProperty(k);
    };
  }, [themeVars]);

  const value = React.useMemo<ExtensionUIContextValue>(
    () => ({
      contributions,
      themeVars,
      hideNav,
      hideSettingsTabs,
      accent,
      logoText,
      logoURL,
      isLoading: query.isLoading,
    }),
    [contributions, themeVars, hideNav, hideSettingsTabs, accent, logoText, logoURL, query.isLoading],
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

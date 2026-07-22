/**
 * Three-state theme controller mirroring legacy dashboard semantics:
 *   - "light"  → forces light by setting data-theme="light"
 *   - "dark"   → forces dark  by setting data-theme="dark"
 *   - "system" → removes data-theme so prefers-color-scheme rules apply
 *
 * Storage key matches legacy ("aurora_theme") so an operator who flips
 * keeps their preference.
 */

import { useEffect, useSyncExternalStore } from "react";

export type Theme = "light" | "dark" | "system";

const STORAGE_KEY = "aurora_theme";
type Listener = () => void;
const listeners = new Set<Listener>();

export function isTheme(value: unknown): value is Theme {
  return value === "light" || value === "dark" || value === "system";
}

export function getStoredTheme(): Theme {
  if (typeof window === "undefined") return "system";
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    return isTheme(raw) ? raw : "system";
  } catch {
    return "system";
  }
}

function apply(theme: Theme): void {
  if (typeof document === "undefined") return;
  const root = document.documentElement;
  if (theme === "system") {
    root.removeAttribute("data-theme");
  } else {
    root.setAttribute("data-theme", theme);
  }
}

export function setTheme(theme: Theme): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    // best-effort
  }
  apply(theme);
  for (const listener of listeners) listener();
}

export function toggleTheme(): void {
  const order: readonly Theme[] = ["light", "system", "dark"];
  const current = getStoredTheme();
  const idx = order.indexOf(current);
  const next = order[(idx + 1) % order.length];
  if (next) setTheme(next);
}

function subscribe(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/**
 * useTheme returns the stored theme preference and re-renders consumers
 * when it changes (in this tab or another).
 */
export function useTheme(): Theme {
  const subscribeFn = (listener: Listener): (() => void) => {
    const unsub = subscribe(listener);
    if (typeof window === "undefined") return unsub;
    const onStorage = (event: StorageEvent): void => {
      if (event.key === STORAGE_KEY || event.key === null) listener();
    };
    window.addEventListener("storage", onStorage);
    return () => {
      unsub();
      window.removeEventListener("storage", onStorage);
    };
  };
  return useSyncExternalStore(
    subscribeFn,
    getStoredTheme,
    () => "system" as Theme,
  );
}

/**
 * Hook that applies the stored theme to document.documentElement on mount
 * and whenever the preference changes. Drop one instance into the root.
 */
export function useApplyThemeAttr(): void {
  const theme = useTheme();
  useEffect(() => {
    apply(theme);
  }, [theme]);
}

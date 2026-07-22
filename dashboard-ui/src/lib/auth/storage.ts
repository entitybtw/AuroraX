/**
 * Local-storage-backed API key store + auth-stale event bus.
 *
 * Mirrors the legacy dashboard's contract so an operator who switches
 * keeps their saved key:
 *   - same storage key:  "aurora_api_key"
 *   - same Bearer prefix tolerance ("Bearer …" pasted verbatim is unwrapped)
 *
 * The fetch wrapper reads getApiKey() and dispatches an "auth-stale"
 * event when a 401 is observed; the React shell listens for that event
 * and opens the auth dialog. This avoids prop-drilling auth state.
 */

const STORAGE_KEY = "aurora_api_key";
const VERIFIED_STORAGE_KEY = "aurora_api_key_verified";
type Listener = () => void;

const listeners = new Set<Listener>();

/** Strip a leading "Bearer " prefix and surrounding whitespace. */
export function normalizeApiKey(raw: string | null | undefined): string {
  if (!raw) return "";
  const trimmed = raw.trim();
  if (!trimmed) return "";
  if (/^Bearer\s*$/i.test(trimmed)) return "";
  const matched = /^Bearer\s+(.+)$/i.exec(trimmed);
  return matched && matched[1] ? matched[1].trim() : trimmed;
}

export function getApiKey(): string {
  if (typeof window === "undefined") return "";
  try {
    return normalizeApiKey(window.localStorage.getItem(STORAGE_KEY));
  } catch {
    return "";
  }
}

export function setApiKey(value: string): void {
  if (typeof window === "undefined") return;
  const normalized = normalizeApiKey(value);
  try {
    if (normalized) {
      window.localStorage.setItem(STORAGE_KEY, normalized);
      window.localStorage.removeItem(VERIFIED_STORAGE_KEY);
    } else {
      window.localStorage.removeItem(STORAGE_KEY);
      window.localStorage.removeItem(VERIFIED_STORAGE_KEY);
    }
  } catch {
    // Private mode or storage disabled — best effort.
  }
  notify();
}

export function markApiKeyVerified(): void {
  if (typeof window === "undefined") return;
  try {
    if (getApiKey()) {
      window.localStorage.setItem(VERIFIED_STORAGE_KEY, "true");
    }
  } catch {
    // Private mode or storage disabled — best effort.
  }
  notify();
}

export function isApiKeyVerified(): boolean {
  if (typeof window === "undefined") return false;
  try {
    return !!getApiKey() && window.localStorage.getItem(VERIFIED_STORAGE_KEY) === "true";
  } catch {
    return false;
  }
}

export function clearApiKey(): void {
  setApiKey("");
}

/** Subscribe to changes (storage events + same-tab updates). */
export function subscribe(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function notify(): void {
  for (const listener of listeners) listener();
}

if (typeof window !== "undefined") {
  window.addEventListener("storage", (storageEvt) => {
    if (storageEvt.key === STORAGE_KEY || storageEvt.key === null) notify();
  });
}

/* ----------------------------- auth-stale bus ---------------------------- */

const STALE_EVENT = "aurora:auth-stale";

/** Fired by the fetch wrapper when a 401 indicates the saved key is stale. */
export function emitAuthStale(): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.removeItem(VERIFIED_STORAGE_KEY);
  } catch {
    // Private mode or storage disabled — best effort.
  }
  notify();
  window.dispatchEvent(new CustomEvent(STALE_EVENT));
}

export function onAuthStale(handler: () => void): () => void {
  if (typeof window === "undefined") return () => undefined;
  const listener = (): void => handler();
  window.addEventListener(STALE_EVENT, listener);
  return () => window.removeEventListener(STALE_EVENT, listener);
}

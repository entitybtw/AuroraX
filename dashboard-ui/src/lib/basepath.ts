/**
 * Resolves the gateway BASE_PATH the SPA was served under so all fetches
 * and history navigations stay under the same prefix the operator mounted
 * the gateway at (e.g. BASE_PATH=/g for a sub-path reverse proxy).
 *
 * The Go dashboard handler injects the prefix into the
 * <meta name="aurora-base-path"> tag at request time. We snapshot it on
 * boot so subsequent reads are O(1) and survive React Strict Mode double
 * mounts.
 *
 * Mirrors the legacy template logic at:
 *   internal/admin/dashboard/templates/layout.html lines 14–74
 */

let cached: string | null = null;

const META_NAME = "aurora-base-path";

function readFromMeta(): string {
  if (typeof document === "undefined") {
    return "/";
  }
  const tag = document.querySelector<HTMLMetaElement>(
    `meta[name="${META_NAME}"]`,
  );
  const raw = tag?.content?.trim() ?? "/";
  // The Go handler replaces __AURORA_BASE_PATH__ before serving; if the
  // placeholder reaches the browser unchanged (dev server, broken build),
  // fall back to "/" so the SPA still works locally.
  if (!raw || raw === "__AURORA_BASE_PATH__") {
    return "/";
  }
  return raw;
}

export function getBasePath(): string {
  if (cached !== null) {
    return cached;
  }
  cached = readFromMeta();
  if (typeof window !== "undefined") {
    window.AURORA_BASE_PATH = cached;
  }
  return cached;
}

/**
 * Prefixes a leading-slash app path with the configured base path. Returns
 * the input unchanged when the base path is "/" or when the input is
 * already prefixed, protocol-relative, or absolute.
 */
export function withBasePath(urlPath: string): string {
  if (!urlPath || urlPath.charAt(0) !== "/" || urlPath.startsWith("//")) {
    return urlPath;
  }
  const base = getBasePath();
  if (base === "/") {
    return urlPath;
  }
  if (urlPath === base || urlPath.startsWith(base + "/")) {
    return urlPath;
  }
  return base + urlPath;
}

// Test-only hook to reset memoization between cases.
export function __resetBasePathCacheForTests(): void {
  cached = null;
}

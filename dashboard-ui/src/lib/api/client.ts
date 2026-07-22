/**
 * Typed gateway API client.
 *
 * Mirrors legacy dashboard semantics from
 * internal/admin/dashboard/static/js/dashboard.js (`headers()`,
 * `handleFetchResponse()`):
 *
 *   - Resolves the configured BASE_PATH so deep-mounted deployments work.
 *   - Attaches `Authorization: Bearer ${apiKey}` when a key is stored.
 *   - Sends `X-Aurora-Timezone: ${IANA-tz}` so server-side date filters
 *     match the user's wall clock (matches legacy header name exactly).
 *   - Treats 401 as a stale-auth signal: dispatches the auth-stale event
 *     and throws ApiError so callers can short-circuit.
 *   - Parses the response JSON and validates against an optional Zod
 *     schema (caller-supplied) so types are inferred from the schema.
 */

import type { ZodType } from "zod";
import { withBasePath } from "../basepath";
import { emitAuthStale, getApiKey } from "../auth/storage";
import { getAccessToken, getRefreshToken, clearSession, setSessionTokens } from "../auth/session";

export class ApiError extends Error {
  readonly status: number;
  readonly body: unknown;
  readonly url: string;
  constructor(message: string, status: number, body: unknown, url: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.body = body;
    this.url = url;
  }
}

export class AuthRequiredError extends ApiError {
  constructor(url: string, body: unknown) {
    super("Authentication required", 401, body, url);
    this.name = "AuthRequiredError";
  }
}

function extractErrorMessage(parsed: unknown, fallback: string): string {
  if (typeof parsed === "string" && parsed.trim() !== "") {
    return parsed;
  }
  if (typeof parsed !== "object" || parsed === null) {
    return fallback;
  }
  if ("error" in parsed) {
    const errorValue = (parsed as { error: unknown }).error;
    if (typeof errorValue === "string" && errorValue.trim() !== "") {
      return errorValue;
    }
    if (typeof errorValue === "object" && errorValue !== null) {
      if ("message" in errorValue && typeof (errorValue as { message?: unknown }).message === "string") {
        const message = (errorValue as { message: string }).message.trim();
        if (message !== "") {
          return message;
        }
      }
    }
  }
  if ("message" in parsed && typeof (parsed as { message?: unknown }).message === "string") {
    const message = (parsed as { message: string }).message.trim();
    if (message !== "") {
      return message;
    }
  }
  return fallback;
}

export interface ApiFetchOptions extends Omit<RequestInit, "body"> {
  /** Optional JSON-serializable body (sent as application/json). */
  json?: unknown;
  /** Raw body, takes precedence over `json` when set. */
  body?: BodyInit | null;
  /** Optional query parameters. Skipped if value is undefined/null/"". */
  query?: Record<string, string | number | boolean | undefined | null>;
  /** Optional Zod schema to validate the parsed JSON response. */
  schema?: ZodType<unknown>;
}

function buildURL(
  path: string,
  query: ApiFetchOptions["query"] | undefined,
): string {
  const prefixed = withBasePath(path);
  if (!query) return prefixed;
  const usp = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === "") continue;
    usp.set(key, String(value));
  }
  const qs = usp.toString();
  if (!qs) return prefixed;
  return prefixed.includes("?") ? `${prefixed}&${qs}` : `${prefixed}?${qs}`;
}

function ianaTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

async function tryRefreshToken(): Promise<boolean> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) return false;

  try {
    const baseUrl = withBasePath("/admin/api/v1/auth/refresh");
    const res = await fetch(baseUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
      credentials: "same-origin",
    });

    if (!res.ok) {
      clearSession();
      return false;
    }

    const data = await res.json();
    if (data.access_token && data.refresh_token) {
      setSessionTokens(data);
      return true;
    }

    clearSession();
    return false;
  } catch {
    clearSession();
    return false;
  }
}

export async function apiFetch<T = unknown>(
  path: string,
  options: ApiFetchOptions = {},
): Promise<T> {
  const url = buildURL(path, options.query);
  const headers = new Headers(options.headers);
  headers.set("Accept", "application/json");
  if (!headers.has("X-Aurora-Timezone")) {
    headers.set("X-Aurora-Timezone", ianaTimezone());
  }

  // Prefer JWT access token over stored API key.
  const accessToken = getAccessToken();
  const apiKey = getApiKey();
  if (accessToken && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${accessToken}`);
  } else if (apiKey && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${apiKey}`);
  }

  let body: BodyInit | null | undefined = options.body;
  if (options.json !== undefined) {
    headers.set("Content-Type", "application/json");
    body = JSON.stringify(options.json);
  }

  const init: RequestInit = {
    ...options,
    headers,
    body: body ?? null,
    credentials: options.credentials ?? "same-origin",
  };
  // Strip our extension fields before passing to fetch.
  delete (init as Partial<ApiFetchOptions>).json;
  delete (init as Partial<ApiFetchOptions>).query;
  delete (init as Partial<ApiFetchOptions>).schema;

  let res = await fetch(url, init);

  // On 401 with a JWT, attempt a silent token refresh and retry once.
  if (res.status === 401 && getAccessToken()) {
    const refreshed = await tryRefreshToken();
    if (refreshed) {
      const newToken = getAccessToken();
      if (newToken) {
        headers.set("Authorization", `Bearer ${newToken}`);
      }
      init.headers = headers;
      res = await fetch(url, init);
    }
  }

  if (res.status === 204 || res.status === 205) {
    return undefined as T;
  }

  let parsed: unknown = null;
  const contentType = res.headers.get("Content-Type") ?? "";
  if (contentType.includes("application/json")) {
    try {
      parsed = await res.json();
    } catch {
      parsed = null;
    }
  } else {
    parsed = await res.text();
  }

  if (res.status === 401) {
    emitAuthStale();
    throw new AuthRequiredError(url, parsed);
  }

  if (!res.ok) {
    const message = extractErrorMessage(parsed, `${res.status} ${res.statusText}`);
    throw new ApiError(message, res.status, parsed, url);
  }

  if (options.schema) {
    const result = options.schema.safeParse(parsed);
    if (!result.success) {
      throw new ApiError(
        `Schema validation failed for ${url}: ${result.error.message}`,
        res.status,
        parsed,
        url,
      );
    }
    return result.data as T;
  }
  return parsed as T;
}

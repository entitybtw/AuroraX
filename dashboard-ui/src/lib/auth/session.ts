const ACCESS_KEY = "aurora_access_token";
const REFRESH_KEY = "aurora_refresh_token";
const USER_KEY = "aurora_user_info";

type Listener = () => void;
const listeners = new Set<Listener>();

export interface SessionUser {
  id: string;
  email: string;
  display_name?: string;
  status: string;
  mfa_bypass: boolean;
  last_login_at?: string;
  created_at: string;
  updated_at: string;
}

export interface SessionRole {
  id: string;
  name: string;
  description?: string;
  is_system: boolean;
}

export interface UserInfo {
  user: SessionUser;
  roles: SessionRole[];
  permissions: Array<{
    action: string;
    resource: string;
    effect: string;
  }>;
}

// Raw response from /auth/login and /auth/oidc/callback
export interface LoginResponse {
  user: SessionUser;
  roles: SessionRole[];
  role_ids: string[];
  tokens: SessionTokens;
  permissions?: Array<{ action: string; resource: string; effect: string }>;
}

export interface SessionTokens {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

export function getAccessToken(): string {
  try {
    return sessionStorage.getItem(ACCESS_KEY) ?? "";
  } catch {
    return "";
  }
}

export function getRefreshToken(): string {
  try {
    return localStorage.getItem(REFRESH_KEY) ?? "";
  } catch {
    return "";
  }
}

export function getUserInfo(): UserInfo | null {
  try {
    const raw = sessionStorage.getItem(USER_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

export function setSession(tokens: SessionTokens, user: UserInfo): void {
  try {
    sessionStorage.setItem(ACCESS_KEY, tokens.access_token);
    localStorage.setItem(REFRESH_KEY, tokens.refresh_token);
    sessionStorage.setItem(USER_KEY, JSON.stringify(user));
  } catch {
    // Private mode — best effort.
  }
  notify();
}

export function setSessionTokens(tokens: SessionTokens): void {
  try {
    sessionStorage.setItem(ACCESS_KEY, tokens.access_token);
    localStorage.setItem(REFRESH_KEY, tokens.refresh_token);
  } catch {
    // Private mode — best effort.
  }
  notify();
}

export function setSessionFromLogin(resp: LoginResponse): void {
  setSession(resp.tokens, {
    user: resp.user,
    roles: resp.roles,
    permissions: resp.permissions ?? [],
  });
  if (!resp.permissions) {
    fetchMe();
  }
}

export async function fetchMe(): Promise<void> {
  const token = getAccessToken();
  if (!token) return;
  try {
    const { withBasePath } = await import("../basepath");
    const res = await fetch(withBasePath("/admin/api/v1/auth/me"), {
      headers: { Authorization: `Bearer ${token}` },
      credentials: "same-origin",
    });
    if (!res.ok) return;
    const data = await res.json();
    if (data.permissions) {
      const existing = getUserInfo();
      if (existing) {
        setSessionFromUserInfo({ ...existing, permissions: data.permissions });
      } else if (data.user) {
        setSessionFromUserInfo({
          user: data.user,
          roles: data.roles ?? [],
          permissions: data.permissions,
        });
      }
    }
  } catch {
    // Best effort.
  }
}

export function setSessionFromUserInfo(user: UserInfo): void {
  try {
    sessionStorage.setItem(USER_KEY, JSON.stringify(user));
  } catch {
    // Best effort.
  }
  notify();
}

export function clearSession(): void {
  try {
    sessionStorage.removeItem(ACCESS_KEY);
    localStorage.removeItem(REFRESH_KEY);
    sessionStorage.removeItem(USER_KEY);
  } catch {
    // Best effort.
  }
  notify();
}

export function isLoggedIn(): boolean {
  return !!getAccessToken();
}

export function subscribe(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function notify(): void {
  for (const listener of listeners) listener();
}

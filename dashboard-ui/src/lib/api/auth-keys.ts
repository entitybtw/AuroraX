import { apiFetch } from "./client";
import {
  AuthKeyIssuedSchema,
  AuthKeyListSchema,
  AuthKeyStatsSchema,
  type AuthKey,
  type AuthKeyIssued,
  type AuthKeyStats,
  type CreateAuthKeyInput,
} from "./auth-keys-types";

export function fetchAuthKeys(): Promise<AuthKey[]> {
  return apiFetch("/admin/api/v1/auth-keys", {
    schema: AuthKeyListSchema,
  }) as Promise<AuthKey[]>;
}

export function fetchAuthKeyStats(id: string, days: number = 30): Promise<AuthKeyStats> {
  const search = new URLSearchParams({ days: String(days) });
  return apiFetch(
    `/admin/api/v1/auth-keys/${encodeURIComponent(id)}/stats?${search.toString()}`,
    { schema: AuthKeyStatsSchema },
  ) as Promise<AuthKeyStats>;
}

export function createAuthKey(input: CreateAuthKeyInput): Promise<AuthKeyIssued> {
  const body: Record<string, unknown> = { name: input.name };
  if (input.description) body.description = input.description;
  if (input.user_path) body.user_path = input.user_path;
  if (input.tenant_id) body.tenant_id = input.tenant_id;
  if (input.allowed_providers?.length) body.allowed_providers = input.allowed_providers;
  if (input.allowed_models?.length) body.allowed_models = input.allowed_models;
  if (input.denied_models?.length) body.denied_models = input.denied_models;
  if (input.provider_pool_id) body.provider_pool_id = input.provider_pool_id;
  
  if (input.expires_at) {
    // Convert HTML date input (YYYY-MM-DD) to RFC3339 required by backend
    if (input.expires_at.length === 10) {
      body.expires_at = `${input.expires_at}T23:59:59Z`;
    } else {
      body.expires_at = input.expires_at;
    }
  }
  
  const limits: Record<string, number> = {};
  if (input.requests_per_minute) limits.requests_per_minute = input.requests_per_minute;
  if (input.requests_per_day) limits.requests_per_day = input.requests_per_day;
  if (input.tokens_per_minute) limits.tokens_per_minute = input.tokens_per_minute;
  if (input.tokens_per_day) limits.tokens_per_day = input.tokens_per_day;
  
  if (Object.keys(limits).length > 0) {
    body.rate_limits = limits;
  }
  
  return apiFetch("/admin/api/v1/auth-keys", {
    method: "POST",
    json: body,
    schema: AuthKeyIssuedSchema,
  }) as Promise<AuthKeyIssued>;
}

export function deactivateAuthKey(id: string): Promise<void> {
  return apiFetch(`/admin/api/v1/auth-keys/${encodeURIComponent(id)}/deactivate`, {
    method: "POST",
  }) as Promise<void>;
}

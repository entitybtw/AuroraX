import { apiFetch } from "./client";
import { z } from "zod";

export const OAuthProviderStatusSchema = z.object({
  name: z.string(),
  has_token: z.boolean(),
  expired: z.boolean(),
  email: z.string().optional(),
  account_id: z.string().optional(),
});

export const OAuthStartDeviceFlowResponseSchema = z.object({
  user_code: z.string(),
  verification_uri_complete: z.string(),
  verification_base: z.string().optional(),
  expires_in: z.number(),
  interval: z.number(),
});

export const OAuthPollTokenResponseSchema = z.object({
  status: z.string(),
  access_token: z.string().optional(),
  error: z.string().optional(),
});

export const OAuthTokenStatusSchema = z.object({
  has_token: z.boolean(),
  expired: z.boolean(),
  expires_at: z.string().optional(),
  email: z.string().optional(),
  account_id: z.string().optional(),
  server: z.string().optional(),
  provider_name: z.string().optional(),
});

export const OAuthFlowInfoSchema = z.object({
  provider: z.string(),
  grant: z.string(),
  has_token: z.boolean(),
  authorization_code: z.boolean(),
});

export const OAuthStartAuthorizeResponseSchema = z.object({
  authorize_url: z.string(),
  state: z.string(),
  redirect_uri: z.string(),
  expires_in: z.number(),
  grant: z.string(),
});

export const OAuthCompleteAuthorizeResponseSchema = z.object({
  status: z.string(),
  access_token: z.string().optional(),
});

export type OAuthProviderStatus = z.infer<typeof OAuthProviderStatusSchema>;
export type OAuthStartDeviceFlowResponse = z.infer<typeof OAuthStartDeviceFlowResponseSchema>;
export type OAuthPollTokenResponse = z.infer<typeof OAuthPollTokenResponseSchema>;
export type OAuthTokenStatus = z.infer<typeof OAuthTokenStatusSchema>;
export type OAuthFlowInfo = z.infer<typeof OAuthFlowInfoSchema>;
export type OAuthStartAuthorizeResponse = z.infer<typeof OAuthStartAuthorizeResponseSchema>;
export type OAuthCompleteAuthorizeResponse = z.infer<typeof OAuthCompleteAuthorizeResponseSchema>;

export async function fetchOAuthProviders(): Promise<OAuthProviderStatus[]> {
  return apiFetch("/admin/api/v1/oauth/providers", {
    schema: z.array(OAuthProviderStatusSchema),
  });
}

export async function startOAuthDeviceFlow(
  providerName: string
): Promise<OAuthStartDeviceFlowResponse> {
  return apiFetch(`/admin/api/v1/oauth/${providerName}/start`, {
    method: "POST",
    schema: OAuthStartDeviceFlowResponseSchema,
  });
}

export async function pollOAuthToken(
  providerName: string,
  deviceCode: string,
  interval?: number,
  expiresIn?: number
): Promise<OAuthPollTokenResponse> {
  return apiFetch(`/admin/api/v1/oauth/${providerName}/poll`, {
    method: "POST",
    json: { device_code: deviceCode, interval, expires_in: expiresIn },
    schema: OAuthPollTokenResponseSchema,
  });
}

export async function fetchOAuthStatus(
  providerName: string
): Promise<OAuthTokenStatus> {
  return apiFetch(`/admin/api/v1/oauth/${providerName}/status`, {
    schema: OAuthTokenStatusSchema,
  });
}

export async function clearOAuthToken(providerName: string): Promise<{ status: string }> {
  return apiFetch(`/admin/api/v1/oauth/${providerName}/token`, {
    method: "DELETE",
    schema: z.object({ status: z.string() }),
  });
}

export async function fetchOAuthFlow(providerName: string): Promise<OAuthFlowInfo> {
  return apiFetch(`/admin/api/v1/oauth/${providerName}/flow`, {
    schema: OAuthFlowInfoSchema,
  });
}

export async function startOAuthAuthorize(
  providerName: string,
  body?: { redirect_uri?: string; scope?: string }
): Promise<OAuthStartAuthorizeResponse> {
  return apiFetch(`/admin/api/v1/oauth/${providerName}/authorize`, {
    method: "POST",
    json: body ?? {},
    schema: OAuthStartAuthorizeResponseSchema,
  });
}

export async function completeOAuthAuthorize(
  providerName: string,
  body: { code?: string; state?: string; code_and_state?: string; url?: string }
): Promise<OAuthCompleteAuthorizeResponse> {
  return apiFetch(`/admin/api/v1/oauth/${providerName}/authorize/complete`, {
    method: "POST",
    json: body,
    schema: OAuthCompleteAuthorizeResponseSchema,
  });
}
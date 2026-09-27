import { apiFetch } from "./client";
import { z } from "zod";

export const ExternalAuthProviderStatusSchema = z.object({
  name: z.string(),
  has_token: z.boolean(),
  expired: z.boolean(),
  email: z.string().optional(),
  account_id: z.string().optional(),
});

export const ExternalAuthStartDeviceFlowResponseSchema = z.object({
  user_code: z.string(),
  verification_uri_complete: z.string(),
  verification_base: z.string().optional(),
  expires_in: z.number(),
  interval: z.number(),
});

export const ExternalAuthPollTokenResponseSchema = z.object({
  status: z.string(),
  access_token: z.string().optional(),
  error: z.string().optional(),
});

export const ExternalAuthTokenStatusSchema = z.object({
  has_token: z.boolean(),
  expired: z.boolean(),
  expires_at: z.string().optional(),
  email: z.string().optional(),
  account_id: z.string().optional(),
  server: z.string().optional(),
  provider_name: z.string().optional(),
});

export const ExternalAuthFlowInfoSchema = z.object({
  provider: z.string(),
  grant: z.string(),
  has_token: z.boolean(),
  authorization_code: z.boolean(),
});

export const ExternalAuthStartAuthorizeResponseSchema = z.object({
  authorize_url: z.string(),
  state: z.string(),
  redirect_uri: z.string(),
  expires_in: z.number(),
  grant: z.string(),
});

export const ExternalAuthCompleteAuthorizeResponseSchema = z.object({
  status: z.string(),
  access_token: z.string().optional(),
});

export type ExternalAuthProviderStatus = z.infer<typeof ExternalAuthProviderStatusSchema>;
export type ExternalAuthStartDeviceFlowResponse = z.infer<typeof ExternalAuthStartDeviceFlowResponseSchema>;
export type ExternalAuthPollTokenResponse = z.infer<typeof ExternalAuthPollTokenResponseSchema>;
export type ExternalAuthTokenStatus = z.infer<typeof ExternalAuthTokenStatusSchema>;
export type ExternalAuthFlowInfo = z.infer<typeof ExternalAuthFlowInfoSchema>;
export type ExternalAuthStartAuthorizeResponse = z.infer<typeof ExternalAuthStartAuthorizeResponseSchema>;
export type ExternalAuthCompleteAuthorizeResponse = z.infer<typeof ExternalAuthCompleteAuthorizeResponseSchema>;

export async function fetchExternalAuthProviders(): Promise<ExternalAuthProviderStatus[]> {
  return apiFetch("/admin/api/v1/external-auth/providers", {
    schema: z.array(ExternalAuthProviderStatusSchema),
  });
}

export async function startExternalAuthDeviceFlow(
  providerName: string
): Promise<ExternalAuthStartDeviceFlowResponse> {
  return apiFetch(`/admin/api/v1/external-auth/${providerName}/start`, {
    method: "POST",
    schema: ExternalAuthStartDeviceFlowResponseSchema,
  });
}

export async function pollExternalAuthToken(
  providerName: string,
  deviceCode: string,
  interval?: number,
  expiresIn?: number
): Promise<ExternalAuthPollTokenResponse> {
  return apiFetch(`/admin/api/v1/external-auth/${providerName}/poll`, {
    method: "POST",
    json: { device_code: deviceCode, interval, expires_in: expiresIn },
    schema: ExternalAuthPollTokenResponseSchema,
  });
}

export async function fetchExternalAuthTokenStatus(
  providerName: string
): Promise<ExternalAuthTokenStatus> {
  return apiFetch(`/admin/api/v1/external-auth/${providerName}/status`, {
    schema: ExternalAuthTokenStatusSchema,
  });
}

export async function clearExternalAuthToken(providerName: string): Promise<{ status: string }> {
  return apiFetch(`/admin/api/v1/external-auth/${providerName}/token`, {
    method: "DELETE",
    schema: z.object({ status: z.string() }),
  });
}

export async function fetchExternalAuthFlow(providerName: string): Promise<ExternalAuthFlowInfo> {
  return apiFetch(`/admin/api/v1/external-auth/${providerName}/flow`, {
    schema: ExternalAuthFlowInfoSchema,
  });
}

export async function startExternalAuthAuthorize(
  providerName: string,
  body?: { redirect_uri?: string; scope?: string }
): Promise<ExternalAuthStartAuthorizeResponse> {
  return apiFetch(`/admin/api/v1/external-auth/${providerName}/authorize`, {
    method: "POST",
    json: body ?? {},
    schema: ExternalAuthStartAuthorizeResponseSchema,
  });
}

export async function completeExternalAuthAuthorize(
  providerName: string,
  body: { code?: string; state?: string; code_and_state?: string; url?: string }
): Promise<ExternalAuthCompleteAuthorizeResponse> {
  return apiFetch(`/admin/api/v1/external-auth/${providerName}/authorize/complete`, {
    method: "POST",
    json: body,
    schema: ExternalAuthCompleteAuthorizeResponseSchema,
  });
}
import { apiFetch } from "./client";
import {
  EgressStatusResponseSchema,
  EgressToggleResponseSchema,
  type EgressStatusResponse,
  type EgressToggleResponse,
} from "./egress-types";

export type { EgressStatusResponse, EgressToggleResponse };

/** Every provider that has exits, with their per-exit status. */
export function fetchEgressStatus(): Promise<EgressStatusResponse> {
  return apiFetch("/admin/api/v1/egress", {
    schema: EgressStatusResponseSchema,
  }) as Promise<EgressStatusResponse>;
}

/** Turns one exit off (or back on) for a provider. */
export function setEgressExitDisabled(
  provider: string,
  exit: string,
  disabled: boolean,
): Promise<EgressToggleResponse> {
  const action = disabled ? "disable" : "enable";
  return apiFetch(`/admin/api/v1/egress/${encodeURIComponent(provider)}/${action}`, {
    method: "POST",
    json: { exit },
    schema: EgressToggleResponseSchema,
  }) as Promise<EgressToggleResponse>;
}

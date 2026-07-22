import { apiFetch } from "./client";
import {
  ModelOverrideListSchema,
  ModelOverrideSchema,
  type ModelOverrideView,
  type UpsertModelOverrideInput,
} from "./model-overrides-types";

export function fetchModelOverrides(): Promise<ModelOverrideView[]> {
  return apiFetch("/admin/api/v1/model-overrides", {
    schema: ModelOverrideListSchema,
  }) as Promise<ModelOverrideView[]>;
}

export function upsertModelOverride(input: UpsertModelOverrideInput): Promise<ModelOverrideView> {
  return apiFetch(`/admin/api/v1/model-overrides/${encodeURIComponent(input.selector)}`, {
    method: "PUT",
    json: { enabled: input.enabled, user_paths: input.user_paths },
    schema: ModelOverrideSchema,
  }) as Promise<ModelOverrideView>;
}

export function deleteModelOverride(selector: string): Promise<void> {
  return apiFetch(`/admin/api/v1/model-overrides/${encodeURIComponent(selector)}`, {
    method: "DELETE",
  }) as Promise<void>;
}

import { apiFetch } from "./client";
import { AliasListSchema, AliasSchema, type AliasView, type UpsertAliasInput } from "./aliases-types";

export function fetchAliases(): Promise<AliasView[]> {
  return apiFetch("/admin/api/v1/aliases", { schema: AliasListSchema }) as Promise<AliasView[]>;
}

export function upsertAlias(input: UpsertAliasInput): Promise<AliasView> {
  return apiFetch(`/admin/api/v1/aliases/${encodeURIComponent(input.name)}`, {
    method: "PUT",
    json: {
      target_model: input.target_model,
      target_provider: input.target_provider ?? "",
      description: input.description ?? "",
      enabled: input.enabled,
    },
    schema: AliasSchema,
  }) as Promise<AliasView>;
}

export function deleteAlias(name: string): Promise<void> {
  return apiFetch(`/admin/api/v1/aliases/${encodeURIComponent(name)}`, {
    method: "DELETE",
  }) as Promise<void>;
}

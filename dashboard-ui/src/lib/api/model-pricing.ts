import { apiFetch } from "./client";
import {
  ModelPricingBackupListSchema,
  ModelPricingImportResponseSchema,
  ModelPricingListSchema,
  ModelPricingViewSchema,
  type ImportModelPricingInput,
  type ModelPricingBackup,
  type ModelPricingImportResponse,
  type ModelPricingView,
  type UpsertModelPricingInput,
} from "./model-pricing-types";

export function fetchModelPricing(): Promise<ModelPricingView[]> {
  return apiFetch("/admin/api/v1/model-pricing", {
    schema: ModelPricingListSchema,
  }) as Promise<ModelPricingView[]>;
}

export function fetchModelPricingItem(selector: string): Promise<ModelPricingView> {
  return apiFetch(`/admin/api/v1/model-pricing/${encodeURIComponent(selector)}`, {
    schema: ModelPricingViewSchema,
  }) as Promise<ModelPricingView>;
}

export function upsertModelPricingOverride(input: UpsertModelPricingInput): Promise<ModelPricingView> {
  return apiFetch(`/admin/api/v1/model-pricing/${encodeURIComponent(input.selector)}`, {
    method: "PUT",
    json: { pricing: input.pricing },
    schema: ModelPricingViewSchema,
  }) as Promise<ModelPricingView>;
}

export function deleteModelPricingOverride(selector: string): Promise<void> {
  return apiFetch(`/admin/api/v1/model-pricing/${encodeURIComponent(selector)}`, {
    method: "DELETE",
  }) as Promise<void>;
}

export function exportModelPricingOverrides(): Promise<string> {
  return apiFetch("/admin/api/v1/model-pricing/export") as Promise<string>;
}

export function importModelPricingOverrides(input: ImportModelPricingInput): Promise<ModelPricingImportResponse> {
  return apiFetch("/admin/api/v1/model-pricing/import", {
    method: "POST",
    json: { format: "yaml", mode: input.mode, content: input.content },
    schema: ModelPricingImportResponseSchema,
  }) as Promise<ModelPricingImportResponse>;
}

export function fetchModelPricingBackups(): Promise<ModelPricingBackup[]> {
  return apiFetch("/admin/api/v1/model-pricing/backups", {
    schema: ModelPricingBackupListSchema,
  }) as Promise<ModelPricingBackup[]>;
}

export function restoreModelPricingBackup(name: string): Promise<{ message: string; backup_name: string }> {
  return apiFetch(`/admin/api/v1/model-pricing/backups/${encodeURIComponent(name)}/restore`, {
    method: "POST",
  }) as Promise<{ message: string; backup_name: string }>;
}

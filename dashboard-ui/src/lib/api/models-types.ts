import { z } from "zod";

export const ModelMetadataSchema = z.object({}).passthrough();

export const ModelInfoSchema = z
  .object({
    id: z.string(),
    object: z.string().optional(),
    created: z.number().optional(),
    owned_by: z.string().optional(),
    metadata: ModelMetadataSchema.optional(),
  })
  .passthrough();

export const ModelAccessSchema = z
  .object({
    selector: z.string(),
    default_enabled: z.boolean(),
    effective_enabled: z.boolean(),
    user_paths: z.array(z.string()).optional().default([]),
  })
  .passthrough();

export const ModelInventoryItemSchema = z
  .object({
    model: ModelInfoSchema.optional(),
    provider_name: z.string().optional(),
    provider_type: z.string().optional(),
    access: ModelAccessSchema.optional(),
  })
  .passthrough();

export type ModelInventoryItem = z.infer<typeof ModelInventoryItemSchema>;
export const ModelInventoryListSchema = z.array(ModelInventoryItemSchema);

export const CategoryCountSchema = z
  .object({
    category: z.string().optional(),
    name: z.string().optional(),
    count: z.number().int().optional(),
  })
  .passthrough();
export type CategoryCount = z.infer<typeof CategoryCountSchema>;
export const CategoryCountListSchema = z.array(CategoryCountSchema);

export function modelDisplayName(item: ModelInventoryItem): string {
  const provider = String(item.provider_name ?? "").trim();
  const modelId = String(item.model?.id ?? (item as { id?: unknown }).id ?? "").trim();
  if (provider && modelId) return `${provider}/${modelId}`;
  return modelId || provider || "unknown-model";
}

export function modelSecondaryName(item: ModelInventoryItem): string {
  return [item.provider_type, item.provider_name].filter(Boolean).join(" · ");
}

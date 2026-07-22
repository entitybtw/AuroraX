import { z } from "zod";

const nullableNumber = z.number().nullable().optional();

export const ModelPricingSchema = z
  .object({
    currency: z.string().optional().default("USD"),
    input_per_mtok: nullableNumber,
    output_per_mtok: nullableNumber,
    cached_input_per_mtok: nullableNumber,
    cache_write_per_mtok: nullableNumber,
    reasoning_output_per_mtok: nullableNumber,
    batch_input_per_mtok: nullableNumber,
    batch_output_per_mtok: nullableNumber,
    audio_input_per_mtok: nullableNumber,
    audio_output_per_mtok: nullableNumber,
    per_image: nullableNumber,
    input_per_image: nullableNumber,
    per_second_input: nullableNumber,
    per_second_output: nullableNumber,
    per_character_input: nullableNumber,
    per_request: nullableNumber,
    per_page: nullableNumber,
  })
  .passthrough();

export const ModelPricingImportResponseSchema = z.object({
  mode: z.string(),
  applied: z.boolean().optional().default(false),
  backup_name: z.string().optional().default(""),
  current_override_keys: z.array(z.string()).optional().default([]),
  incoming_keys: z.array(z.string()).optional().default([]),
  added_keys: z.array(z.string()).optional().default([]),
  changed_keys: z.array(z.string()).optional().default([]),
  removed_keys: z.array(z.string()).optional().default([]),
});

export const ModelPricingBackupSchema = z.object({
  name: z.string(),
  path: z.string().optional().default(""),
  size_bytes: z.number().int().optional().default(0),
  modified_at: z.string(),
});

export const ModelPricingBackupListSchema = z.array(ModelPricingBackupSchema);

export const ModelPricingViewSchema = z.object({
  selector: z.string(),
  override_selector: z.string().optional().default(""),
  provider_name: z.string().optional().default(""),
  provider_type: z.string().optional().default(""),
  model_id: z.string().optional().default(""),
  display_name: z.string().optional().default(""),
  source: z.string().optional().default("missing"),
  has_override: z.boolean().optional().default(false),
  overridden_fields: z.array(z.string()).optional().default([]),
  base_pricing: ModelPricingSchema.optional(),
  override_pricing: ModelPricingSchema.optional(),
  effective_pricing: ModelPricingSchema.optional(),
});

export type ModelPricing = z.infer<typeof ModelPricingSchema>;
export type ModelPricingImportResponse = z.infer<typeof ModelPricingImportResponseSchema>;
export type ModelPricingBackup = z.infer<typeof ModelPricingBackupSchema>;
export type ModelPricingView = z.infer<typeof ModelPricingViewSchema>;
export const ModelPricingListSchema = z.array(ModelPricingViewSchema);

export interface UpsertModelPricingInput {
  selector: string;
  pricing: ModelPricing;
}

export interface ImportModelPricingInput {
  content: string;
  mode: "dry_run" | "merge" | "replace";
}

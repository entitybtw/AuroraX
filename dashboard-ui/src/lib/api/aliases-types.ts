import { z } from "zod";

export const AliasSchema = z.object({
  name: z.string(),
  target_model: z.string(),
  target_provider: z.string().optional().default(""),
  description: z.string().optional().default(""),
  enabled: z.boolean().default(true),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
  resolved_model: z.string().optional().default(""),
  provider_type: z.string().optional().default(""),
  valid: z.boolean().default(false),
});

export type AliasView = z.infer<typeof AliasSchema>;
export const AliasListSchema = z.array(AliasSchema);

export interface UpsertAliasInput {
  name: string;
  target_model: string;
  target_provider?: string;
  description?: string;
  enabled: boolean;
}

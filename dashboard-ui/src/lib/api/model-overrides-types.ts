import { z } from "zod";

export const ModelOverrideSchema = z.object({
  selector: z.string(),
  provider_name: z.string().optional().default(""),
  model: z.string().optional().default(""),
  enabled: z.boolean().nullable().optional(),
  user_paths: z.array(z.string()).optional().default([]),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
  scope_kind: z.string().optional().default(""),
});

export type ModelOverrideView = z.infer<typeof ModelOverrideSchema>;
export const ModelOverrideListSchema = z.array(ModelOverrideSchema);

export interface UpsertModelOverrideInput {
  selector: string;
  enabled: boolean;
  user_paths: string[];
}

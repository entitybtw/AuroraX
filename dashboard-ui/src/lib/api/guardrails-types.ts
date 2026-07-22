import { z } from "zod";

export const GuardrailTypeFieldOptionSchema = z.object({
  value: z.string(),
  label: z.string(),
});

export const GuardrailTypeFieldSchema = z.object({
  key: z.string(),
  label: z.string(),
  input: z.string().default("text"),
  placeholder: z.string().optional(),
  help: z.string().optional(),
  options: z.array(GuardrailTypeFieldOptionSchema).optional(),
});
export type GuardrailTypeField = z.infer<typeof GuardrailTypeFieldSchema>;

export const GuardrailTypeDefSchema = z.object({
  type: z.string(),
  label: z.string(),
  fields: z.array(GuardrailTypeFieldSchema).optional().default([]),
});
export type GuardrailTypeDef = z.infer<typeof GuardrailTypeDefSchema>;
export const GuardrailTypeDefListSchema = z.array(GuardrailTypeDefSchema);

export const GuardrailSchema = z.object({
  name: z.string(),
  type: z.string(),
  direction: z.enum(["input", "output", "both"]).optional().default("input"),
  description: z.string().optional().default(""),
  user_path: z.string().optional().default(""),
  summary: z.string().optional(),
  config: z.record(z.unknown()).optional().default({}),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
});
export type Guardrail = z.infer<typeof GuardrailSchema>;
export const GuardrailListSchema = z.array(GuardrailSchema);

export interface UpsertGuardrailInput {
  name: string;
  type: string;
  direction?: "input" | "output" | "both";
  description?: string;
  user_path?: string;
  config?: Record<string, unknown>;
}

import { z } from "zod";
import { apiFetch } from "./client";

const ComboSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().optional(),
  models: z.array(z.string()),
  enabled: z.boolean(),
  source: z.string(),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
});
const ComboViewSchema = z.object({
  combo: ComboSchema,
  valid: z.boolean(),
  errors: z.array(z.string()).optional(),
  warnings: z.array(z.string()).optional(),
  primary: z.string().optional(),
  fallbacks: z.array(z.string()).optional(),
  readonly: z.boolean(),
});
const CombosResponseSchema = z.object({ combos: z.array(ComboViewSchema) });

export type Combo = z.infer<typeof ComboSchema>;
export type ComboView = z.infer<typeof ComboViewSchema>;
export interface ComboPayload { name: string; description?: string; models: string[]; enabled: boolean; }

export async function fetchCombos(): Promise<ComboView[]> {
  const data = await apiFetch<z.infer<typeof CombosResponseSchema>>("/admin/api/v1/combos", { schema: CombosResponseSchema });
  return data.combos;
}

export function createCombo(payload: ComboPayload): Promise<ComboView> {
  return apiFetch("/admin/api/v1/combos", { method: "POST", json: payload, schema: ComboViewSchema }) as Promise<ComboView>;
}

export function updateCombo(id: string, payload: ComboPayload): Promise<ComboView> {
  return apiFetch(`/admin/api/v1/combos/${encodeURIComponent(id)}`, { method: "PUT", json: payload, schema: ComboViewSchema }) as Promise<ComboView>;
}

export function deleteCombo(id: string): Promise<void> {
  return apiFetch(`/admin/api/v1/combos/${encodeURIComponent(id)}`, { method: "DELETE" }) as Promise<void>;
}

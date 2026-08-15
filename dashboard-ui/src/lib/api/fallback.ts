import { z } from "zod";
import { apiFetch } from "./client";

const FallbackRuleSchema = z.object({
  source: z.string(),
  targets: z.array(z.string()),
  enabled: z.boolean().optional(),
});

const FallbackConfigSchema = z.object({
  rules: z.array(FallbackRuleSchema),
});

export type FallbackRule = z.infer<typeof FallbackRuleSchema>;
export type FallbackConfig = z.infer<typeof FallbackConfigSchema>;

export async function fetchFallback(): Promise<FallbackRule[]> {
  const data = await apiFetch<FallbackConfig>("/admin/api/v1/fallback", {
    schema: FallbackConfigSchema,
  });
  return data.rules;
}

export async function updateFallback(rules: FallbackRule[]): Promise<FallbackRule[]> {
  const data = await apiFetch<FallbackConfig>("/admin/api/v1/fallback", {
    method: "PUT",
    json: { rules },
    schema: FallbackConfigSchema,
  });
  return data.rules;
}

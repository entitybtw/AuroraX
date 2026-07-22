import { z } from "zod";

export const CacheDebugInfoSchema = z.object({
  path: z.string(),
  cache_type: z.string().optional(),
  exact_cache_key: z.string().optional(),
  semantic_params_hash: z.string().optional(),
  semantic_cache_key: z.string().optional(),
  semantic_threshold: z.number().optional(),
  prompt_similarity_threshold: z.number().optional(),
  exact_ttl_seconds: z.number().int().optional(),
  semantic_ttl_seconds: z.number().int().optional(),
  streaming: z.boolean().optional(),
  cacheable: z.boolean().optional(),
  miss_reason: z.string().optional(),
  guardrails_hash: z.string().optional(),
  embedder_identity: z.string().optional(),
  effective_content_type: z.string().optional(),
});

export type CacheDebugInfo = z.infer<typeof CacheDebugInfoSchema>;

export interface CacheDebugRequest {
  method?: string;
  path?: string;
  headers?: Record<string, string>;
  body: unknown;
}

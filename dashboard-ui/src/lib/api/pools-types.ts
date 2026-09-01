import { z } from "zod";

export const PoolMemberSnapshotSchema = z.object({
  provider_name: z.string(),
  healthy: z.boolean(),
  active_requests: z.number().int().optional().default(0),
  total_requests: z.number().int(),
  total_errors: z.number().int(),
  latency_ewma_us: z.number().int().optional().default(0),
  weight: z.number().int().optional().default(0),
  capabilities: z.array(z.string()).optional().default([]),
});
export type PoolMemberSnapshot = z.infer<typeof PoolMemberSnapshotSchema>;

export const PoolSnapshotSchema = z.object({
  name: z.string(),
  strategy: z.string(),
  members: z.array(PoolMemberSnapshotSchema),
  health_aware: z.boolean().optional().default(true),
  source: z.string().optional().default("config"),
  user_agent: z.string().optional(),
  auto_fetch_models: z.boolean().nullable().optional(),
});
export type PoolSnapshot = z.infer<typeof PoolSnapshotSchema>;

export const PoolsSummarySchema = z.object({
  total: z.number().int(),
  healthy_members: z.number().int(),
  total_members: z.number().int(),
});
export type PoolsSummary = z.infer<typeof PoolsSummarySchema>;

export const PoolsResponseSchema = z.object({
  summary: PoolsSummarySchema,
  pools: z.array(PoolSnapshotSchema),
});
export type PoolsResponse = z.infer<typeof PoolsResponseSchema>;

export const PoolOptionsSchema = z.object({
  providers: z.array(
    z.object({
      name: z.string(),
      type: z.string(),
    }),
  ),
});
export type PoolOptions = z.infer<typeof PoolOptionsSchema>;

export const PoolModifyResponseSchema = z.object({
  message: z.string(),
  pool: z.string(),
  runtime_applied: z.boolean(),
  requires_runtime_refresh: z.boolean(),
  runtime_refresh_error: z.string().optional(),
});
export type PoolModifyResponse = z.infer<typeof PoolModifyResponseSchema>;

// Supported load-balancing strategies shown in the UI.
export const POOL_STRATEGIES = ["round_robin", "weighted"] as const;
export type PoolStrategy = (typeof POOL_STRATEGIES)[number];

// Payload sent to create/update a pool.
export interface PoolConfigPayload {
  name: string;
  members: string[];
  strategy: string;
  weights?: Record<string, number>;
  health_aware?: boolean;
  user_agent?: string;
  auto_fetch_models?: boolean;
}

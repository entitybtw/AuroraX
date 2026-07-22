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

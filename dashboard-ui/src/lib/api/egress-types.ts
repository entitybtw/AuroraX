import { z } from "zod";

/**
 * Mirrors ExitStatus / ProviderEgressStatus from internal/egress/egress.go
 * and egressStatusResponse / egressToggleResponse from
 * internal/admin/handler_egress.go.
 *
 * The exits list is the drill-down behind a provider: every source address
 * and every extension-supplied exit, whether the operator turned it off,
 * whether it is cooling down, and why it last failed.
 */

export const ExitStatusSchema = z.object({
  name: z.string(),
  /** "config" for the provider's own source addresses, "extension" for exits
   *  contributed by an extension (VPN endpoints). */
  source: z.string(),
  /** "ip" for a local source address, "proxy" for a tunnelled exit. */
  kind: z.string(),
  /** 0 = preferred, 1 = fallback, -1 = used ahead of the preferred tier. */
  tier: z.number().int(),
  address: z.string().optional(),
  proxy: z.string().optional(),
  weight: z.number().int().optional(),
  /** False while the exit is cooling down after repeated failures. */
  available: z.boolean(),
  /** True when the operator turned this exit off. */
  disabled: z.boolean(),
  /** True while this exit's whole tier is rate-limited. */
  tier_limited: z.boolean(),
  failures: z.number().int(),
  requests: z.number(),
  ok: z.number(),
  cooldown_until: z.string().optional(),
  last_outcome: z.string().optional(),
  last_error: z.string().optional(),
  last_at: z.string().optional(),
});
export type ExitStatus = z.infer<typeof ExitStatusSchema>;

export const ProviderEgressStatusSchema = z.object({
  provider: z.string(),
  strategy: z.string(),
  exits: z.array(ExitStatusSchema),
});
export type ProviderEgressStatus = z.infer<typeof ProviderEgressStatusSchema>;

export const EgressStatusResponseSchema = z.object({
  providers: z.array(ProviderEgressStatusSchema),
});
export type EgressStatusResponse = z.infer<typeof EgressStatusResponseSchema>;

export const EgressToggleResponseSchema = z.object({
  provider: z.string(),
  exit: z.string(),
  disabled: z.boolean(),
  status: ProviderEgressStatusSchema.optional(),
});
export type EgressToggleResponse = z.infer<typeof EgressToggleResponseSchema>;

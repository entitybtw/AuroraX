import { z } from "zod";

export const WorkflowGuardrailStepSchema = z.object({
  ref: z.string(),
  step: z.number().default(0),
});
export type WorkflowGuardrailStep = z.infer<typeof WorkflowGuardrailStepSchema>;

export const WorkflowFeaturesSchema = z.object({
  caching: z.boolean().default(false),
  budget: z.boolean().default(false),
  guardrails: z.boolean().default(false),
  failover: z.boolean().default(false),
  rate_limit: z.boolean().default(false),
  audit: z.boolean().default(false),
  usage_tracking: z.boolean().default(false),
}).passthrough();
export type WorkflowFeatures = z.infer<typeof WorkflowFeaturesSchema>;

export const WorkflowSchema = z.object({
  id: z.string(),
  name: z.string().optional().default(""),
  description: z.string().optional().default(""),
  version: z.number().default(1),
  workflow_hash: z.string().optional().default(""),
  created_at: z.string().optional(),
  scope: z.object({
    scope_provider: z.string().optional().default(""),
    scope_model: z.string().optional().default(""),
    scope_user_path: z.string().optional().default(""),
  }).optional(),
  workflow_payload: z.object({
    features: z.object({
      cache: z.boolean().default(false),
      audit: z.boolean().default(false),
      usage: z.boolean().default(false),
      budget: z.boolean().default(false),
      guardrails: z.boolean().default(false),
      fallback: z.boolean().default(false),
    }).optional(),
    guardrails: z.array(WorkflowGuardrailStepSchema).optional().default([]),
  }).optional(),
}).transform((val) => {
  return {
    id: val.id,
    name: val.name,
    description: val.description,
    version: val.version,
    workflow_hash: val.workflow_hash,
    created_at: val.created_at,
    
    // Flatten scope
    scope_provider: val.scope?.scope_provider || "",
    scope_model: val.scope?.scope_model || "",
    scope_user_path: val.scope?.scope_user_path || "",
    
    // Flatten features and adapt naming
    features: {
      caching: !!val.workflow_payload?.features?.cache,
      budget: !!val.workflow_payload?.features?.budget,
      guardrails: !!val.workflow_payload?.features?.guardrails,
      failover: !!val.workflow_payload?.features?.fallback,
      rate_limit: true, // Legacy compatibility mapping
      audit: !!val.workflow_payload?.features?.audit,
      usage_tracking: !!val.workflow_payload?.features?.usage,
    },
    
    // Map guardrails
    guardrails: val.workflow_payload?.guardrails || [],
  };
});
export type Workflow = z.infer<typeof WorkflowSchema>;
export const WorkflowListSchema = z.array(WorkflowSchema);

export interface UpsertWorkflowInput {
  scope_provider?: string;
  scope_model?: string;
  scope_user_path?: string;
  name?: string;
  description?: string;
  features?: Partial<WorkflowFeatures>;
  guardrails?: WorkflowGuardrailStep[];
}

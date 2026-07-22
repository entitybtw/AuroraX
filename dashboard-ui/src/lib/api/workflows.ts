import { apiFetch } from "./client";
import {
  WorkflowListSchema,
  WorkflowSchema,
  type UpsertWorkflowInput,
  type Workflow,
} from "./workflows-types";

export function fetchWorkflows(): Promise<Workflow[]> {
  return apiFetch("/admin/api/v1/workflows", {
    schema: WorkflowListSchema,
  }) as Promise<Workflow[]>;
}

export function upsertWorkflow(input: UpsertWorkflowInput): Promise<Workflow> {
  const backendPayload = {
    scope_provider_name: input.scope_provider || "",
    scope_model: input.scope_model || "",
    scope_user_path: input.scope_user_path || "",
    name: input.name || "",
    description: input.description || "",
    workflow_payload: {
      schema_version: 1,
      features: {
        cache: !!input.features?.caching,
        audit: !!input.features?.audit,
        usage: !!input.features?.usage_tracking,
        budget: !!input.features?.budget,
        guardrails: !!input.features?.guardrails,
        fallback: !!input.features?.failover,
      },
      guardrails: input.guardrails || [],
    }
  };
  
  return apiFetch("/admin/api/v1/workflows", {
    method: "POST",
    json: backendPayload,
    schema: WorkflowSchema,
  }) as Promise<Workflow>;
}

export function deactivateWorkflow(id: string): Promise<void> {
  return apiFetch(`/admin/api/v1/workflows/${encodeURIComponent(id)}/deactivate`, {
    method: "POST",
  }) as Promise<void>;
}

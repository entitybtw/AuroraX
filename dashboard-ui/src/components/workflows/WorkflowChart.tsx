import type { Workflow } from "@/lib/api/workflows-types";
import { WorkflowChart as AuditWorkflowChart } from "@/components/charts/WorkflowChart";
import type { AuditEntry } from "@/lib/api/audit-types";

interface ChartProps {
  workflow: Workflow;
}

export function WorkflowChart({ workflow }: ChartProps) {
  // Construct a dummy audit entry to reuse the complex visual Audit chart component
  const dummyEntry = {
    id: "preview",
    timestamp: new Date().toISOString(),
    provider: workflow.scope_provider || "Upstream",
    model: workflow.scope_model || "Endpoint",
    status_code: 200,
    auth_method: "Token",
    data: {
      workflow_features: {
        cache: !!workflow.features?.caching,
        budget: !!workflow.features?.budget,
        guardrails: !!workflow.features?.guardrails,
        fallback: !!workflow.features?.failover,
        audit: !!workflow.features?.audit,
        usage: !!workflow.features?.usage_tracking,
      },
    },
    cache_hit: false,
    // We add an empty usage object if audit/usage tracking is on so the audit logic knows the async pipelines ran
    usage: workflow.features?.usage_tracking || workflow.features?.audit ? { total_tokens: 0 } : undefined,
  } as unknown as AuditEntry;

  return (
    <div className="w-full overflow-hidden">
      <AuditWorkflowChart entry={dummyEntry} className="border-0 bg-transparent p-0 shadow-none" />
    </div>
  );
}

import * as React from "react";
import { ArrowDown, Info } from "lucide-react";
import { PageHeader } from "@/components/ui/page-header";
import { EmptyState, Pill, Surface } from "@/components/ui/surface";
import { useDashboardConfig } from "@/lib/api/useDashboardConfig";
import { cn } from "@/lib/utils";

export function FallbackPage(): JSX.Element {
  const config = useDashboardConfig();
  const fallback = config.data?.fallback;
  const rules = fallback?.manual_rules ?? [];

  if (config.isLoading) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader
          title="Fallback Chains"
          subtitle="View ordered model fallback chains that route requests through a primary model with automatic failover."
        />
        <Surface className="flex items-center gap-2 p-6 text-sm text-muted-foreground">
          Loading fallback configuration…
        </Surface>
      </div>
    );
  }

  if (config.isError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader
          title="Fallback Chains"
          subtitle="View ordered model fallback chains that route requests through a primary model with automatic failover."
        />
        <Banner tone="warning">
          {config.error instanceof Error ? config.error.message : "Unable to load dashboard config."}
        </Banner>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Fallback Chains"
        subtitle="View ordered model fallback chains that route requests through a primary model with automatic failover."
      />

      <Surface className="p-4 text-sm text-muted-foreground">
        When a request targets a fallback chain name, the first model acts as
        primary. If it fails, Aurora retries the next model in the list, and so
        on, until a model succeeds or the chain is exhausted.
      </Surface>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="border border-border bg-surface p-5 flex flex-col gap-3">
          <div className="text-[10px] font-bold tracking-widest uppercase text-muted-foreground">
            Fallback Mode
          </div>
          <div className="font-mono text-sm text-foreground">
            {fallback?.mode ?? "not configured"}
          </div>
        </div>

        <div className="border border-border bg-surface p-5 flex flex-col gap-3">
          <div className="text-[10px] font-bold tracking-widest uppercase text-muted-foreground">
            Manual Rules
          </div>
          <div className="flex items-center gap-2">
            <Pill tone={fallback?.manual_rules_configured ? "success" : "muted"}>
              {fallback?.manual_rules_configured ? "Configured" : "Not configured"}
            </Pill>
            <span className="text-xs text-muted-foreground">
              {rules.length} rule{rules.length !== 1 ? "s" : ""}
            </span>
          </div>
        </div>
      </div>

      {rules.length === 0 ? (
        <EmptyState
          title="No fallback rules configured"
          description="Add manual fallback rules to the fallback configuration to define ordered model chains with automatic failover."
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {rules.map((rule, idx) => (
            <RuleCard key={idx} source={rule.source ?? "unknown"} targets={rule.targets ?? []} />
          ))}
        </div>
      )}
    </div>
  );
}

/* ── Rule Card ──────────────────────────────────────────────────────── */

function RuleCard({
  source,
  targets,
}: {
  source: string;
  targets: string[];
}): JSX.Element {
  return (
    <div className="border border-border bg-surface p-5 flex flex-col gap-4">
      <div className="flex items-center gap-2 flex-wrap">
        <Pill tone="accent">source</Pill>
        <h3 className="font-mono text-sm font-semibold text-foreground truncate">
          {source}
        </h3>
      </div>

      {targets.length > 0 ? (
        <div className="border border-border/60 bg-background/40 p-3">
          <div className="text-[10px] font-bold tracking-widest uppercase text-muted-foreground mb-2">
            Fallback Chain
          </div>
          <div className="flex flex-col">
            {targets.map((target, idx) => (
              <React.Fragment key={`${target}-${idx}`}>
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground w-5 text-right shrink-0">
                    {idx + 1}.
                  </span>
                  <span className="font-mono text-sm text-foreground truncate">
                    {target}
                  </span>
                  <Pill tone={idx === 0 ? "accent" : "muted"} className="ml-auto shrink-0">
                    {idx === 0 ? "primary" : `fallback ${idx}`}
                  </Pill>
                </div>
                {idx < targets.length - 1 ? (
                  <div className="flex items-center justify-center py-0.5">
                    <ArrowDown className="h-3 w-3 text-muted-foreground" />
                  </div>
                ) : null}
              </React.Fragment>
            ))}
          </div>
        </div>
      ) : (
        <div className="text-xs text-muted-foreground flex items-center gap-1.5">
          <Info className="h-3 w-3" />
          No targets configured
        </div>
      )}
    </div>
  );
}

/* ── Helpers ────────────────────────────────────────────────────────── */

function Banner({
  children,
  tone,
}: {
  children: string;
  tone: "warning" | "success";
}): JSX.Element {
  return (
    <div
      className={cn(
        "border px-4 py-3 text-sm",
        tone === "warning"
          ? "border-warning/30 bg-warning/15 text-warning"
          : "border-success/30 bg-success/15 text-success",
      )}
    >
      {children}
    </div>
  );
}

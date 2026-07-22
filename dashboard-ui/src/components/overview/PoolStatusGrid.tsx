import { Loader2, ExternalLink } from "lucide-react";
import { cn } from "@/lib/utils";
import type { PoolsResponse, PoolSnapshot } from "@/lib/api/pools-types";
import { formatRequests } from "@/lib/format/numbers";
import { Link } from "@tanstack/react-router";

export interface PoolStatusGridProps {
  data: PoolsResponse | undefined;
  isLoading: boolean;
  error: Error | null;
}

function poolStatusClass(pool: PoolSnapshot) {
  if (!pool.members || pool.members.length === 0) return "bg-muted text-muted-foreground border-border";
  const allHealthy = pool.members.every(m => m.healthy);
  const anyHealthy = pool.members.some(m => m.healthy);
  if (allHealthy) return "bg-success/10 text-success border-success/20";
  if (anyHealthy) return "bg-warning/10 text-warning border-warning/20";
  return "bg-destructive/10 text-destructive border-destructive/20";
}

function poolStatusLabel(pool: PoolSnapshot) {
  if (!pool.members || pool.members.length === 0) return "Empty";
  const allHealthy = pool.members.every(m => m.healthy);
  const anyHealthy = pool.members.some(m => m.healthy);
  if (allHealthy) return "Healthy";
  if (anyHealthy) return "Degraded";
  return "Unhealthy";
}

function formatLatency(micros: number | undefined) {
  const value = Number(micros || 0);
  if (value <= 0) return "-";
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}s`;
  if (value >= 1_000) return `${Math.round(value / 1_000)}ms`;
  return `${Math.round(value)}us`;
}

export function PoolStatusGrid({
  data,
  isLoading,
  error,
}: PoolStatusGridProps): JSX.Element {

  const empty = !data || data.pools.length === 0;

  return (
    <section
      aria-label="Provider pools"
      className="border border-border/40 bg-surface"
    >
      <header className="p-6 flex flex-wrap items-start justify-between gap-3 border-b border-border/40">
        <div>
          <div className="text-[10px] font-bold tracking-widest uppercase text-accent mb-2">[ Pools ]</div>
          <h2 className="font-serif text-xl font-normal text-foreground">Provider Pools</h2>
          <p className="mt-1 text-xs text-muted-foreground">Load balancing, latency scoring, and active requests.</p>
        </div>
        {data ? (
          <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
            <span className="inline-flex items-center gap-1.5 font-semibold"><span className="h-1.5 w-1.5 bg-success" />{data.summary.healthy_members} / {data.summary.total_members} healthy</span>
          </div>
        ) : null}
      </header>

      {isLoading && !data ? (
        <div className="flex items-center gap-2 p-6 text-xs text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading pools...
        </div>
      ) : error ? (
        <div className="border border-destructive/25 bg-destructive/10 m-6 px-3 py-6 text-xs text-destructive">
          Failed to load pools: {error.message}
        </div>
      ) : empty ? (
        <div className="p-6 text-xs text-muted-foreground">
          No pools configured.
        </div>
      ) : (
        <div className="divide-y divide-border/40">
          {data.pools.map((pool) => {
            const activeRequests = pool.members.reduce((sum, m) => sum + m.active_requests, 0);

            return (
              <div key={pool.name} className="transition-colors duration-200 hover:bg-surface-hover/30">
                <div className="flex items-start justify-between gap-3 p-6">
                  <div className="min-w-0">
                    <div className="flex items-center gap-3">
                      <h4 className="font-mono text-sm font-semibold text-foreground">
                        {pool.name}
                      </h4>
                      <Link
                        to="/admin/dashboard/pools"
                        className="inline-flex items-center gap-0.5 border border-border/40 px-1.5 py-0.5 text-[9px] font-medium uppercase tracking-wider text-muted-foreground hover:text-accent hover:border-accent/30 transition-colors"
                        title="View pool details"
                      >
                        <ExternalLink className="h-2.5 w-2.5" />
                      </Link>
                    </div>
                    <span className="mt-2 inline-block border border-border/40 px-2 py-0.5 text-[10px] text-muted-foreground font-semibold uppercase tracking-wider">
                      {pool.strategy}
                    </span>
                  </div>
                  <span className={cn("inline-flex items-center px-2 py-0.5 text-[10px] font-bold uppercase tracking-widest border", poolStatusClass(pool))}>
                    {poolStatusLabel(pool)}
                  </span>
                </div>

                <div className="flex gap-8 px-6 py-4 border-t border-border/40 bg-surface-hover/20">
                  <div className="flex flex-col gap-1">
                    <span className="text-[10px] text-muted-foreground uppercase tracking-widest font-bold">Members</span>
                    <span className="font-mono text-lg font-bold">{pool.members.length}</span>
                  </div>
                  <div className="flex flex-col gap-1">
                    <span className="text-[10px] text-muted-foreground uppercase tracking-widest font-bold">Active Requests</span>
                    <span className="font-mono text-lg font-bold text-accent">{activeRequests}</span>
                  </div>
                </div>

                <div className="border-t border-border/40 divide-y divide-border/40 text-xs">
                  {pool.members.map(member => (
                    <div key={member.provider_name} className="flex justify-between items-center px-6 py-3 hover:bg-surface-hover/20 transition-colors">
                      <div className="flex items-center gap-2">
                        <span className={cn("w-2 h-2", member.healthy ? "bg-success" : "bg-destructive")}></span>
                        <span className="font-mono font-semibold">{member.provider_name}</span>
                      </div>
                      <div className="flex gap-4 font-mono text-muted-foreground text-[10px] sm:text-xs">
                        <span title="Active requests"><span className="text-foreground">act:</span> {member.active_requests}</span>
                        <span title="Total requests"><span className="text-foreground">tot:</span> {formatRequests(member.total_requests)}</span>
                        <span title="Total errors" className={member.total_errors > 0 ? "text-destructive" : ""}><span className={member.total_errors > 0 ? "text-destructive" : "text-foreground"}>err:</span> {formatRequests(member.total_errors)}</span>
                        <span title="Latency EWMA"><span className="text-foreground">lat:</span> {formatLatency(member.latency_ewma_us)}</span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </section>
  );
}

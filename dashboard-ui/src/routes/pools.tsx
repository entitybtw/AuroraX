import * as React from "react";
import { usePools } from "@/lib/api/usePools";
import type { PoolSnapshot } from "@/lib/api/pools-types";
import { PageHeader } from "@/components/ui/page-header";
import { cn } from "@/lib/utils";
import { PoolSceneSVG } from "@/components/pools/PoolSceneSVG";
import { formatRequests } from "@/lib/format/numbers";
import {
  Shuffle,
  Users,
  Activity,
  HeartPulse,
  Orbit,
  Loader2,
} from "lucide-react";

/* ------------------------------------------------------------------ */
/*  Pools page                                                         */
/* ------------------------------------------------------------------ */

export function PoolsPage(): JSX.Element {
  const pools = usePools();
  const [selectedIdx, setSelectedIdx] = React.useState(0);
  const poolList = pools.data?.pools ?? [];
  const selected = poolList[selectedIdx] ?? null;

  React.useEffect(() => {
    if (selectedIdx >= poolList.length) {
      setSelectedIdx(0);
    }
  }, [poolList.length, selectedIdx]);

  if (pools.isLoading && !pools.data) {
    return <LoadingState />;
  }

  if (pools.error) {
    return <ErrorState message={pools.error.message} />;
  }

  if (poolList.length === 0) {
    return <EmptyState />;
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Provider Pools"
        subtitle="Real-time Pool load-balancing "
      />

      <PoolSelector
        pools={poolList}
        selectedIdx={selectedIdx}
        onSelect={setSelectedIdx}
      />

      {selected && <PoolView pool={selected} key={selected.name} />}

      {pools.data && (
        <div className="flex flex-wrap items-center gap-3">
          <SummaryBadge
            icon={Orbit}
            label="Total pools"
            value={String(pools.data.summary.total)}
          />
          <SummaryBadge
            icon={HeartPulse}
            label="Members online"
            value={`${pools.data.summary.healthy_members} / ${pools.data.summary.total_members}`}
            highlight={
              pools.data.summary.healthy_members ===
              pools.data.summary.total_members
            }
          />
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Pool selector tabs                                                 */
/* ------------------------------------------------------------------ */

function PoolSelector({
  pools,
  selectedIdx,
  onSelect,
}: {
  pools: PoolSnapshot[];
  selectedIdx: number;
  onSelect: (i: number) => void;
}) {
  return (
    <div className="flex flex-wrap gap-1 border border-border/40 bg-surface p-1">
      {pools.map((pool, i) => {
        const isActive = selectedIdx === i;
        const allHealthy = pool.members.every((m) => m.healthy);
        return (
          <button
            key={pool.name}
            type="button"
            onClick={() => onSelect(i)}
            className={cn(
              "group relative flex items-center gap-2 px-3 py-2 text-xs font-medium transition-all duration-200",
              isActive
                ? "bg-accent/12 text-accent"
                : "text-muted-foreground hover:bg-surface-hover/60 hover:text-foreground",
            )}
          >
            <span
              className={cn(
                "h-1.5 w-1.5  transition-colors",
                allHealthy ? "bg-success" : "bg-warning",
              )}
            />
            <span className="font-semibold">{pool.name}</span>
            <span className="ml-1 rounded bg-background/60 px-1.5 py-0.5 text-[10px] tabular-nums text-muted-foreground">
              {pool.members.length}
            </span>
          </button>
        );
      })}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Pool view                                                          */
/* ------------------------------------------------------------------ */

function PoolView({ pool }: { pool: PoolSnapshot }) {
  const [activeHover, setHovered] = React.useState<string | null>(null);
  const totalActive = pool.members.reduce((s, m) => s + m.active_requests, 0);

  const health =
    pool.members.every((m) => m.healthy)
      ? { label: "All healthy", className: "text-success", dot: "bg-success" }
      : pool.members.some((m) => m.healthy)
        ? { label: "Degraded", className: "text-warning", dot: "bg-warning" }
        : { label: "Unhealthy", className: "text-destructive", dot: "bg-destructive" };

  return (
    <div className="flex flex-col gap-4">
      <PoolSceneSVG pool={pool} onMemberHover={setHovered} />

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <StatCard
          icon={Shuffle}
          label="Strategy"
          value={pool.strategy.replace(/_/g, " ")}
          sub="Load-balancing algorithm"
        />
        <StatCard
          icon={Users}
          label="Members"
          value={String(pool.members.length)}
          sub="Configured providers"
        />
        <StatCard
          icon={Activity}
          label="Active requests"
          value={String(totalActive)}
          sub="Currently in-flight"
          accent
        />
        <StatCard
          icon={HeartPulse}
          label="Health"
          value={health.label}
          valueClass={health.className}
          dot={health.dot}
          sub={`${pool.members.filter(m => m.healthy).length}/${pool.members.length} online`}
        />
      </div>

      <MemberTable
        members={pool.members}
        activeHover={activeHover}
      />
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Member table                                                       */
/* ------------------------------------------------------------------ */

function MemberTable({
  members,
  activeHover,
}: {
  members: PoolSnapshot["members"];
  activeHover: string | null;
}) {
  return (
    <div className="overflow-hidden border border-border/40 bg-surface">
      <div className="flex items-center justify-between border-b border-border/40 px-4 py-2.5">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          Member breakdown
        </h3>
        <span className="text-[10px] text-muted-foreground">
          {members.length} member{members.length !== 1 ? "s" : ""}
        </span>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-border/30 bg-background/30 text-left text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
              <th className="px-4 py-2.5">Provider</th>
              <th className="px-4 py-2.5">Status</th>
              <th className="px-4 py-2.5 text-right">Active</th>
              <th className="px-4 py-2.5 text-right">Total</th>
              <th className="px-4 py-2.5 text-right">Errors</th>
              <th className="px-4 py-2.5 text-right">Latency</th>
              <th className="px-4 py-2.5 text-right">Weight</th>
            </tr>
          </thead>
          <tbody>
            {members.map((m) => {
              const isHovered = activeHover === m.provider_name;
              return (
                <tr
                  key={m.provider_name}
                  className={cn(
                    "border-b border-border/20 transition-colors duration-150",
                    isHovered
                      ? "bg-accent/8"
                      : "hover:bg-background/40",
                  )}
                >
                  <td className="px-4 py-2.5">
                    <div className="flex items-center gap-2">
                      <span className={cn("h-1.5 w-1.5  shrink-0", m.healthy ? "bg-success" : "bg-destructive")} />
                      <span className="font-mono font-medium text-foreground">
                        {m.provider_name}
                      </span>
                    </div>
                  </td>
                  <td className="px-4 py-2.5">
                    <span
                      className={cn(
                        "inline-flex items-center gap-1 px-2 py-0.5 text-[10px] font-semibold",
                        m.healthy
                          ? "bg-success/10 text-success"
                          : "bg-destructive/10 text-destructive",
                      )}
                    >
                      {m.healthy ? "Healthy" : "Unhealthy"}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-right font-mono tabular-nums text-foreground">
                    {m.active_requests}
                  </td>
                  <td className="px-4 py-2.5 text-right font-mono tabular-nums text-foreground">
                    {formatRequests(m.total_requests)}
                  </td>
                  <td
                    className={cn(
                      "px-4 py-2.5 text-right font-mono tabular-nums",
                      m.total_errors > 0 ? "text-destructive" : "text-foreground",
                    )}
                  >
                    {formatRequests(m.total_errors)}
                  </td>
                  <td className="px-4 py-2.5 text-right font-mono tabular-nums text-foreground">
                    {formatLatency(m.latency_ewma_us)}
                  </td>
                  <td className="px-4 py-2.5 text-right font-mono tabular-nums text-foreground">
                    {m.weight ?? "-"}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Stat card                                                          */
/* ------------------------------------------------------------------ */

function StatCard({
  icon: Icon,
  label,
  value,
  sub,
  accent,
  valueClass,
  dot,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string;
  sub: string;
  accent?: boolean;
  valueClass?: string;
  dot?: string;
}) {
  return (
    <div className="group relative border border-border/40 bg-surface p-3 transition-all duration-200 hover:border-accent/20">
      <div className="flex items-center gap-2 text-muted-foreground">
        <Icon className="h-3.5 w-3.5" />
        <span className="text-[10px] font-semibold uppercase tracking-wider">
          {label}
        </span>
      </div>
      <div className="mt-1.5 flex items-center gap-1.5">
        {dot && <span className={cn("h-2 w-2 ", dot)} />}
        <span
          className={cn(
            "font-mono text-sm font-bold",
            valueClass ?? (accent ? "text-accent" : "text-foreground"),
          )}
        >
          {value}
        </span>
      </div>
      <div className="mt-0.5 text-[10px] text-muted-foreground">
        {sub}
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Summary badge                                                      */
/* ------------------------------------------------------------------ */

function SummaryBadge({
  icon: Icon,
  label,
  value,
  highlight,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string;
  highlight?: boolean;
}) {
  return (
    <div className="inline-flex items-center gap-2  border border-border/40 bg-surface px-3 py-1.5 text-xs">
      <Icon className="h-3 w-3 text-muted-foreground" />
      <span className="text-muted-foreground">{label}</span>
      <span
        className={cn(
          "font-semibold",
          highlight ? "text-success" : "text-foreground",
        )}
      >
        {value}
      </span>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  State components                                                   */
/* ------------------------------------------------------------------ */

function LoadingState() {
  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Provider Pools"
        subtitle="Real-time Pool load-balancing "
      />
      <div className="flex flex-col items-center justify-center gap-4 py-20 text-sm text-muted-foreground">
        <Loader2 className="h-6 w-6 animate-spin text-accent" />
        <span>Loading pools…</span>
      </div>
    </div>
  );
}

function ErrorState({ message }: { message: string }) {
  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Provider Pools"
        subtitle="Real-time Pool load-balancing "
      />
      <div className="rounded-lg border border-destructive/25 bg-destructive/10 px-4 py-6 text-sm text-destructive">
        Failed to load pools: {message}
      </div>
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Provider Pools"
        subtitle="Real-time Pool load-balancing "
      />
      <div className="flex flex-col items-center justify-center gap-3 py-20 text-sm text-muted-foreground">
        <Orbit className="h-8 w-8 opacity-30" />
        <span>No provider pools configured.</span>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function formatLatency(micros: number | undefined): string {
  const v = Number(micros || 0);
  if (v <= 0) return "-";
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(2)}s`;
  if (v >= 1_000) return `${Math.round(v / 1_000)}ms`;
  return `${Math.round(v)}us`;
}

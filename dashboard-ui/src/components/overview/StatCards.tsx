import { Loader2 } from "lucide-react";
import { formatCost, formatPercent, formatRequests, formatTokens } from "@/lib/format/numbers";
import type { CacheOverview, UsageSummary } from "@/lib/api/usage-types";

export interface StatCardsProps {
  summary: UsageSummary | undefined;
  cacheOverview: CacheOverview | undefined;
  isLoading: boolean;
  isCacheUnavailable: boolean;
}

const stats = [
  { key: "requests", label: "Requests", kicker: "Total volume" },
  { key: "tokens", label: "Tokens Processed", kicker: "All models" },
  { key: "cost", label: "Estimated Cost", kicker: "This period" },
  { key: "cache", label: "Cache Rate", kicker: "Hit ratio" },
] as const;

export function StatCards({
  summary,
  cacheOverview,
  isLoading,
  isCacheUnavailable,
}: StatCardsProps): JSX.Element {
  const totalRequests = summary?.total_requests ?? 0;
  const totalTokens = summary?.total_tokens ?? 0;
  const totalCost = summary?.total_cost ?? null;

  const cacheHits = cacheOverview?.summary.total_hits ?? 0;
  const cacheRate =
    totalRequests + cacheHits > 0
      ? (cacheHits / (totalRequests + cacheHits)) * 100
      : null;

  function getValue(key: string) {
    switch (key) {
      case "requests": return formatRequests(totalRequests);
      case "tokens": return formatTokens(totalTokens);
      case "cost": return formatCost(totalCost);
      case "cache": return isCacheUnavailable ? "—" : cacheRate === null ? "—" : formatPercent(cacheRate);
      default: return "—";
    }
  }

  function getColor(key: string) {
    if (key === "cost") return "text-success";
    return "text-foreground";
  }

  return (
    <section aria-label="Usage summary" className="border border-border/40 divide-y divide-border/40">
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 divide-y sm:divide-y-0 sm:divide-x divide-border/40">
        {stats.map((stat, idx) => (
          <div
            key={stat.key}
            className="p-6 flex flex-col justify-between group hover:bg-surface-hover/30 transition-colors duration-200 relative"
          >
            <span className="absolute top-4 right-6 text-[10px] font-mono text-border group-hover:text-accent transition-colors">
              {String(idx + 1).padStart(2, "0")}
            </span>
            <div>
              <p className="text-[10px] font-bold tracking-widest uppercase text-accent mb-1">
                {stat.label}
              </p>
              <p className="text-[11px] text-muted-foreground">{stat.kicker}</p>
            </div>
            <p className="mt-4 flex items-baseline gap-x-2">
              {isLoading ? (
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground/50" />
              ) : (
                <span className={`text-[40px] leading-none font-bold tracking-tight ${getColor(stat.key)}`} style={{ fontFeatureSettings: '"tnum"' }}>
                  {getValue(stat.key)}
                </span>
              )}
            </p>
          </div>
        ))}
      </div>
    </section>
  );
}

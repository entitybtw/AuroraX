import * as React from "react";
import { StatCards } from "@/components/overview/StatCards";
import { DateRangeSelect } from "@/components/overview/DateRangeSelect";
import { TokenUsageChart } from "@/components/charts/TokenUsageChart";
import { ThroughputChart } from "@/components/charts/ThroughputChart";
import { CostByModelChart } from "@/components/charts/CostByModelChart";
import { ContributionCalendar } from "@/components/overview/ContributionCalendar";
import { ProviderStatusGrid } from "@/components/overview/ProviderStatusGrid";
import { PoolStatusGrid } from "@/components/overview/PoolStatusGrid";
import { RuntimeRefreshButton } from "@/components/overview/RuntimeRefreshButton";
import { useDateRange } from "@/lib/date-picker/useDateRange";
import { formatCost, formatRequests, formatTokens } from "@/lib/format/numbers";
import {
  useCacheOverview,
  useDailyUsage,
  useUsageByModel,
  useUsageSummary,
} from "@/lib/api/useUsage";
import { useProviderStatus } from "@/lib/api/useProviders";
import { usePools } from "@/lib/api/usePools";
import { useDashboardConfig } from "@/lib/api/useDashboardConfig";
import type { DashboardConfigResponse } from "@/lib/api/dashboard-config";
import type { UsageQueryFilters } from "@/lib/api/usage-types";
import { PageHeader } from "@/components/ui/page-header";
import { TenantScopeSelect } from "@/components/tenant/TenantScopeSelect";
import { useTenantScope } from "@/lib/tenant/tenant-scope";

function EditionHero({ config }: { config: DashboardConfigResponse | undefined }): JSX.Element {
  const edition = "OSS";
  const capabilityCount = config?.CAPABILITIES.length ?? 0;
  return (
    <section className="border border-border/40 bg-surface/35 px-4 py-3 text-foreground">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <div className="text-[10px] font-black uppercase tracking-[0.24em] text-accent">
            Open-source edition
          </div>
          <h2 className="mt-1 font-serif text-xl font-normal tracking-tight">
            Community gateway
          </h2>
          <p className="mt-1 max-w-2xl text-xs text-muted-foreground">
            Core routing, analytics, and provider operations are available license-free.
          </p>
        </div>
        <div className="flex flex-wrap gap-2 text-[10px] uppercase tracking-[0.18em]">
          <span className="border border-accent/30 bg-accent/10 px-3 py-1 text-accent">{edition}</span>
          <span className="border border-border/40 bg-background/40 px-3 py-1 text-muted-foreground">{capabilityCount} caps</span>
          <span className="border border-border/40 bg-background/40 px-3 py-1 text-muted-foreground">free</span>
        </div>
      </div>
    </section>
  );
}

export function OverviewPage(): JSX.Element {
  const range = useDateRange("30d");
  const { tenant } = useTenantScope();

  const filters = React.useMemo<UsageQueryFilters>(
    () => ({
      startDate: range.startDate,
      endDate: range.endDate,
      interval: "daily",
      tenant,
    }),
    [range.startDate, range.endDate, tenant],
  );

  const calendarFilters = React.useMemo<UsageQueryFilters>(
    () => ({ days: 365, interval: "daily", tenant }),
    [tenant],
  );

  const summary = useUsageSummary(filters);
  const daily = useDailyUsage(filters);
  const calendarDaily = useDailyUsage(calendarFilters);
  const cache = useCacheOverview(filters);
  const models = useUsageByModel(filters);
  const providers = useProviderStatus();
  const pools = usePools();
  const dashboardConfig = useDashboardConfig();

  const cacheUnavailable =
    cache.isError &&
    typeof cache.error === "object" &&
    cache.error !== null &&
    "status" in cache.error &&
    (cache.error as { status: number }).status === 503;

  const insights = React.useMemo(() => {
    const days = daily.data ?? [];
    if (days.length === 0) {
      return { peakDay: "—", peakRequests: 0, activeDays: 0, avgRequests: 0 };
    }
    let peakDay = days[0]!;
    let activeDays = 0;
    let totalRequests = 0;
    for (const d of days) {
      const r = d.requests ?? 0;
      totalRequests += r;
      if (r > 0) activeDays += 1;
      if (r > (peakDay.requests ?? 0)) peakDay = d;
    }
    return {
      peakDay: peakDay.date ?? "—",
      peakRequests: peakDay.requests ?? 0,
      activeDays,
      avgRequests: days.length > 0 ? Math.round(totalRequests / days.length) : 0,
    };
  }, [daily.data]);

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Overview"
        kicker="Dashboard"
        subtitle={`${range.startDate} → ${range.endDate}`}
        actions={
          <>
            <TenantScopeSelect />
            <DateRangeSelect
              range={range}
            />
            <RuntimeRefreshButton />
          </>
        }
      />

      <EditionHero config={dashboardConfig.data} />

      <StatCards
        summary={summary.data}
        cacheOverview={cache.data}
        isLoading={summary.isLoading || cache.isLoading}
        isCacheUnavailable={cacheUnavailable}
      />

      <div className="border border-border/40 divide-y divide-border/40">
        <div className="grid grid-cols-1 sm:grid-cols-3 divide-y sm:divide-y-0 sm:divide-x divide-border/40">
          <div className="p-6 hover:bg-surface-hover/30 transition-colors">
            <div className="text-[10px] font-bold uppercase tracking-widest text-accent">Peak day</div>
            <div className="mt-2 text-[15px] font-medium text-foreground">
              {insights.peakDay}{insights.peakRequests > 0 ? ` @ ${insights.peakRequests.toLocaleString()} req` : ""}
            </div>
          </div>
          <div className="p-6 hover:bg-surface-hover/30 transition-colors">
            <div className="text-[10px] font-bold uppercase tracking-widest text-accent">Active days</div>
            <div className="mt-2 text-[15px] font-medium text-foreground">{insights.activeDays}</div>
          </div>
          <div className="p-6 hover:bg-surface-hover/30 transition-colors">
            <div className="text-[10px] font-bold uppercase tracking-widest text-accent">Avg requests / day</div>
            <div className="mt-2 text-[15px] font-medium text-foreground">{insights.avgRequests.toLocaleString()}</div>
          </div>
        </div>
      </div>

      <div className="border border-border/40 divide-y sm:divide-y-0 sm:divide-x divide-border/40 grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4">
        <div className="p-6 hover:bg-surface-hover/30 transition-colors group">
          <div className="text-[10px] font-bold uppercase tracking-widest text-accent">Input tokens</div>
          <div className="mt-2 text-[20px] font-semibold tracking-tight text-foreground">
            {formatTokens(summary.data?.total_input_tokens)}
          </div>
          <div className="mt-1 text-[12px] text-muted-foreground">Prompt-side volume</div>
        </div>
        <div className="p-6 hover:bg-surface-hover/30 transition-colors group">
          <div className="text-[10px] font-bold uppercase tracking-widest text-accent">Output tokens</div>
          <div className="mt-2 text-[20px] font-semibold tracking-tight text-foreground">
            {formatTokens(summary.data?.total_output_tokens)}
          </div>
          <div className="mt-1 text-[12px] text-muted-foreground">Completion-side volume</div>
        </div>
        <div className="p-6 hover:bg-surface-hover/30 transition-colors group">
          <div className="text-[10px] font-bold uppercase tracking-widest text-accent">Cache hits</div>
          <div className="mt-2 text-[20px] font-semibold tracking-tight text-foreground">
            {cacheUnavailable ? "—" : formatRequests(cache.data?.summary.total_hits)}
          </div>
          <div className="mt-1 text-[12px] text-muted-foreground">Exact + semantic hits</div>
        </div>
        <div className="p-6 hover:bg-surface-hover/30 transition-colors group">
          <div className="text-[10px] font-bold uppercase tracking-widest text-accent">Cache saved</div>
          <div className="mt-2 text-[20px] font-semibold tracking-tight text-foreground">
            {cacheUnavailable ? "—" : formatCost(cache.data?.summary.total_saved_cost)}
          </div>
          <div className="mt-1 text-[12px] text-muted-foreground">Estimated local savings</div>
        </div>
      </div>

      <div className="border border-border/40">
        <div className="border-b border-border/40 px-5 py-3">
          <h3 className="text-[11px] font-bold uppercase tracking-widest text-accent">Provider Prompt Caching</h3>
          <p className="text-[12px] text-muted-foreground mt-0.5">
            Tokens read from or written to upstream provider caches via cache_control directives
          </p>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 divide-y sm:divide-y-0 sm:divide-x divide-border/40">
          <div className="p-6 hover:bg-surface-hover/30 transition-colors group">
            <div className="text-[10px] font-bold uppercase tracking-widest text-accent">Prompt cache read</div>
            <div className="mt-2 text-[20px] font-semibold tracking-tight text-foreground">
              {formatTokens(summary.data?.total_cached_input_tokens)}
            </div>
            <div className="mt-1 text-[12px] text-muted-foreground">Cached input tokens charged at reduced rate</div>
          </div>
          <div className="p-6 hover:bg-surface-hover/30 transition-colors group">
            <div className="text-[10px] font-bold uppercase tracking-widest text-accent">Prompt cache write</div>
            <div className="mt-2 text-[20px] font-semibold tracking-tight text-foreground">
              {formatTokens(summary.data?.total_cache_write_input_tokens)}
            </div>
            <div className="mt-1 text-[12px] text-muted-foreground">Cache creation tokens (first-time writes)</div>
          </div>
        </div>
      </div>

      <div className="border border-border/40 grid grid-cols-1 xl:grid-cols-3">
        <div className="xl:col-span-2 border-b xl:border-b-0 xl:border-r border-border/40">
          <TokenUsageChart
            daily={daily.data}
            cacheDaily={cache.data?.daily}
            isLoading={daily.isLoading}
          />
        </div>
        <ThroughputChart
          daily={daily.data}
          cacheDaily={cache.data?.daily}
          isLoading={daily.isLoading}
        />
      </div>

      <ContributionCalendar
        daily={calendarDaily.data}
        weeks={53}
        isLoading={calendarDaily.isLoading}
      />

      <CostByModelChart models={models.data} isLoading={models.isLoading} />

      <PoolStatusGrid 
        data={pools.data} 
        isLoading={pools.isLoading} 
        error={pools.error ?? null} 
      />

      <ProviderStatusGrid
        data={providers.data}
        isLoading={providers.isLoading}
        error={providers.error ?? null}
      />
    </div>
  );
}

import * as React from "react";
import { useQuery } from "@tanstack/react-query";
import { format } from "date-fns";
import {
  DownloadIcon,
  ChevronRightIcon,
  AlertTriangleIcon,
  RefreshCwIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState, Pill } from "@/components/ui/surface";
import { DateRangeSelect } from "@/components/overview/DateRangeSelect";
import { useDateRange } from "@/lib/date-picker/useDateRange";
import { useDashboardConfig } from "@/lib/api/useDashboardConfig";
import { flagOn } from "@/lib/api/dashboard-config";
import { fetchAuditLog, exportAuditLogCsv } from "@/lib/api/audit";
import type {
  AuditEntry,
  AuditLogStatsBucket,
  AuditQueryParams,
} from "@/lib/api/audit-types";
import { LogVolumeChart } from "@/components/charts/LogVolumeChart";
import { LogDetailSheet } from "@/components/logs/LogDetailSheet";
import { LogSidebarFilters, type LogFilterValues } from "@/components/logs/LogSidebarFilters";
import { useLiveLogs } from "@/lib/hooks/useLiveLogs";
import { TenantScopeSelect } from "@/components/tenant/TenantScopeSelect";
import { useTenantScope } from "@/lib/tenant/tenant-scope";

const PAGE_LIMIT = 50;
const LIVE_TAIL_INTERVAL_MS = 3000;

function statusToneClass(code?: number): string {
  if (!code) return "border border-border bg-background/45 text-muted-foreground";
  if (code >= 200 && code < 300) return "border border-success/25 bg-success/10 text-success";
  if (code >= 400 && code < 500) return "border border-warning/25 bg-warning/10 text-warning";
  if (code >= 500) return "border border-destructive/25 bg-destructive/10 text-destructive";
  return "border border-border bg-background/45 text-muted-foreground";
}

function formatCost(cost: number | null | undefined): string {
  if (cost === null || cost === undefined) return "\u2014";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 4,
    maximumFractionDigits: 6,
  }).format(cost);
}

function formatDuration(ns?: number): string {
  if (!ns) return "\u2014";
  const ms = ns / 1_000_000;
  if (ms < 1000) return `${ms.toFixed(0)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

function topBucketLabel(buckets: AuditLogStatsBucket[] | undefined): string {
  if (!buckets || buckets.length === 0) return "\u2014";
  return `${buckets[0]!.label}: ${buckets[0]!.count}`;
}

export function AuditLogsPage(): JSX.Element {
  const { data: config } = useDashboardConfig();
  const range = useDateRange("30d");
  const { tenant } = useTenantScope();

  const [search, setSearch] = React.useState("");
  const [method, setMethod] = React.useState("");
  const [statusCode, setStatusCode] = React.useState("");
  const [stream, setStream] = React.useState("");
  const [requestedModel, setRequestedModel] = React.useState("");
  const [provider, setProvider] = React.useState("");
  const [path, setPath] = React.useState("");
  const [userPath, setUserPath] = React.useState("");
  const [errorType, setErrorType] = React.useState("");
  const [sort, setSort] = React.useState("-timestamp");
  const [offset, setOffset] = React.useState(0);

  const [liveTail, setLiveTail] = React.useState(false);
  const [exporting, setExporting] = React.useState(false);

  const [drawerOpen, setDrawerOpen] = React.useState(false);
  const [selectedEntry, setSelectedEntry] = React.useState<AuditEntry | null>(null);

  const sidebarFilters: LogFilterValues = { search, method, statusCode, stream, requestedModel, provider, path, userPath, errorType };

  function setSidebarFilters(f: LogFilterValues) {
    setSearch(f.search);
    setMethod(f.method);
    setStatusCode(f.statusCode);
    setStream(f.stream);
    setRequestedModel(f.requestedModel);
    setProvider(f.provider);
    setPath(f.path);
    setUserPath(f.userPath);
    setErrorType(f.errorType);
    setOffset(0);
  }

  const clearFilters = () => setSidebarFilters({ search: "", method: "", statusCode: "", stream: "", requestedModel: "", provider: "", path: "", userPath: "", errorType: "" });

  const queryParams = React.useMemo<AuditQueryParams>(() => {
    const p: AuditQueryParams = { offset, limit: PAGE_LIMIT, sort, include_stats: "true" };
    if (range.startDate) p.start_date = range.startDate;
    if (range.endDate) p.end_date = range.endDate;
    if (search) p.search = search;
    if (method) p.method = method;
    if (statusCode) p.status_code = statusCode;
    if (stream) p.stream = stream;
    if (requestedModel) p.requested_model = requestedModel;
    if (provider) p.provider = provider;
    if (path) p.path = path;
    if (userPath) p.user_path = userPath;
    if (tenant) p.tenant = tenant;
    if (errorType) p.error_type = errorType;
    return p;
  }, [range.startDate, range.endDate, search, method, statusCode, stream, requestedModel, provider, path, userPath, tenant, errorType, sort, offset]);

  const { data: pageData, isLoading, refetch } = useQuery({
    queryKey: ["audit-logs", queryParams],
    queryFn: () => fetchAuditLog(queryParams),
    refetchInterval: liveTail ? LIVE_TAIL_INTERVAL_MS : false,
  });

  const entries = pageData?.entries || [];
  const total = pageData?.total || 0;
  const stats = pageData?.stats;

  const { liveEntries, isConnected, clear: clearLiveEntries } = useLiveLogs({
    enabled: liveTail,
    tenant,
  });

  const displayEntries = React.useMemo(() => {
    if (!liveTail || liveEntries.length === 0) return entries;
    const liveIds = new Set(liveEntries.map((e) => e.id));
    return [...liveEntries, ...entries.filter((e) => !liveIds.has(e.id))];
  }, [entries, liveEntries, liveTail]);

  const exportCSV = async () => {
    try {
      setExporting(true);
      const csv = await exportAuditLogCsv(queryParams);
      const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.setAttribute("href", url);
      a.setAttribute("download", `audit-export-${format(new Date(), "yyyy-MM-dd-HHmm")}.csv`);
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (err) {
      console.error("Export failed", err);
    } finally {
      setExporting(false);
    }
  };

  const handleRefresh = React.useCallback(() => {
    clearLiveEntries();
    refetch();
  }, [clearLiveEntries, refetch]);

  const openDetail = (entry: AuditEntry) => {
    setSelectedEntry(entry);
    setDrawerOpen(true);
  };

  const detailIndex = selectedEntry ? displayEntries.findIndex((e) => e.id === selectedEntry.id) : -1;

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col sm:flex-row sm:items-end justify-between gap-4 pb-6 pt-4 border-b border-border/60">
        <div className="min-w-0 flex-1">
          <h1 className="font-serif text-[34px] font-normal leading-tight tracking-tight text-foreground">Audit Logs</h1>
          <p className="mt-1.5 text-[15px] text-muted-foreground">View detailed request logs and conversations.</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <TenantScopeSelect />
          <DateRangeSelect range={range} onChange={() => setOffset(0)} />
        </div>
      </header>

      {!flagOn(config?.LOGGING_ENABLED) && (
        <div className="border border-info/30 bg-info/10 p-4 text-info">
          <div className="flex">
            <AlertTriangleIcon className="h-5 w-5 shrink-0 mt-0.5" />
            <div className="ml-3">
              <h3 className="text-sm font-medium">Audit logging is disabled</h3>
              <p className="mt-2 text-sm opacity-90">Enable LOGGING_ENABLED in your configuration to capture new request logs. Existing logs can still be viewed.</p>
            </div>
          </div>
        </div>
      )}

      {stats && stats.visible > 0 && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
          <AnalyticsCard title="Status" headline={topBucketLabel(stats.status)} buckets={stats.status} />
          <AnalyticsCard title="Methods" headline={topBucketLabel(stats.methods)} buckets={stats.methods} />
          <AnalyticsCard title="Providers" headline={topBucketLabel(stats.providers)} buckets={stats.providers} />
          <AnalyticsCard title="Models" headline={topBucketLabel(stats.models)} buckets={stats.models} />
          <AnalyticsCard title="Errors" headline={`${stats.error_count ?? 0} errors \u00b7 ${stats.stream_count ?? 0} stream`} buckets={stats.errors} />
          <AnalyticsCard title="Runtime" headline={`${stats.failover_count ?? 0} failover \u00b7 ${(stats.total_duration_ms ?? 0).toFixed(0)}ms`} buckets={stats.auth_methods} />
        </div>
      )}

      <LogVolumeChart entries={displayEntries} isConnected={liveTail ? isConnected : undefined} />

      <div className="flex gap-0 border border-border/60 overflow-hidden bg-surface">
        <LogSidebarFilters filters={sidebarFilters} onChange={setSidebarFilters} onClear={clearFilters} entryCount={total} />

        <div className="flex-1 flex flex-col min-w-0">
          <div className="flex items-center justify-between gap-3 border-b border-border/40 bg-muted/20 px-4 py-2.5">
            <div className="flex items-center gap-2">
              <select className="field-input h-7 w-32 bg-background text-xs" value={sort} onChange={(e) => { setSort(e.target.value); setOffset(0); }}>
                <option value="-timestamp">Newest</option>
                <option value="timestamp">Oldest</option>
                <option value="-duration_ns">Slowest</option>
                <option value="duration_ns">Fastest</option>
                <option value="-status_code">Highest status</option>
                <option value="status_code">Lowest status</option>
                <option value="provider">Provider A-Z</option>
                <option value="requested_model">Model A-Z</option>
              </select>
              <div className="h-5 w-px bg-border/40" />
              <span className="text-xs text-muted-foreground tabular-nums">
                {offset + 1}-{Math.min(offset + PAGE_LIMIT, total)} of {total}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="outline" size="sm" className="h-7 text-[10px]" onClick={handleRefresh}>
                <RefreshCwIcon className="mr-1 h-3 w-3" />
                Refresh
              </Button>
              <Button variant={liveTail ? "default" : "outline"} size="sm" className="h-7 text-[10px] font-bold uppercase tracking-wider" onClick={() => setLiveTail(!liveTail)}>
                {liveTail ? "Live" : "Live tail"}
              </Button>
              <Button variant="outline" size="sm" className="h-7 text-[10px]" disabled={exporting} onClick={exportCSV}>
                <DownloadIcon className="mr-1 h-3 w-3" />
                CSV
              </Button>
            </div>
          </div>

          <div className="flex-1 overflow-auto min-h-[400px] max-h-[600px]">
            {isLoading ? (
              <div className="flex items-center justify-center p-12 text-sm text-muted-foreground">Loading audit logs </div>
            ) : displayEntries.length === 0 ? (
              <EmptyState title="No audit log entries found.">
                Try widening the date range or clearing filters.
              </EmptyState>
            ) : (
              <div className="divide-y divide-border/40">
                {displayEntries.map((entry) => {
                  const usage = entry.usage;
                  const totalTokens = usage?.total_tokens ?? entry.total_tokens ?? 0;
                  const totalCost = usage?.total_cost ?? entry.total_cost;
                  return (
                    <div
                      key={entry.id}
                      className="flex cursor-pointer items-center gap-4 px-4 py-3 hover:bg-surface-hover/40 transition-colors group"
                      onClick={() => openDetail(entry)}
                    >
                      <span className={`inline-flex items-center px-1.5 py-0.5 text-[10px] font-bold tracking-wide uppercase shrink-0 ${statusToneClass(entry.status_code)}`}>
                        {entry.status_code || "-"}
                      </span>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 truncate">
                          {entry.method && <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground shrink-0">{entry.method}</span>}
                          {(entry.requested_model || entry.model) && (
                            <span className="font-mono text-[12px] font-semibold text-foreground truncate">{entry.requested_model || entry.model}</span>
                          )}
                          {entry.path && <span className="font-mono text-[11px] text-muted-foreground truncate hidden sm:inline">{entry.path}</span>}
                        </div>
                        <div className="flex items-center gap-2 mt-0.5">
                          {entry.provider && <span className="text-[10px] uppercase tracking-wider text-muted-foreground font-medium">{entry.provider_name || entry.provider}</span>}
                          {entry.error_type && <Pill tone="danger">{entry.error_type}</Pill>}
                          {entry.stream && <Pill tone="accent">stream</Pill>}
                          {entry.alias_used && <Pill tone="muted">alias</Pill>}
                          {entry.cache_hit && <Pill tone="success">cache</Pill>}
                        </div>
                      </div>
                      <div className="flex items-center gap-3 shrink-0 text-xs">
                        {totalTokens > 0 && (
                          <span className="font-mono text-muted-foreground hidden sm:inline">{totalTokens.toLocaleString()} tok</span>
                        )}
                        {totalCost !== undefined && totalCost !== null && (
                          <span className="font-mono text-success font-semibold hidden sm:inline">{formatCost(totalCost)}</span>
                        )}
                        <span className="font-mono text-muted-foreground w-16 text-right">{formatDuration(entry.duration_ns)}</span>
                        <span className="font-mono text-[11px] text-muted-foreground w-24 text-right hidden md:block">
                          {entry.timestamp ? format(new Date(entry.timestamp), "HH:mm:ss") : ""}
                        </span>
                        <ChevronRightIcon className="h-3.5 w-3.5 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {total > PAGE_LIMIT && (
            <div className="flex items-center justify-between border-t bg-muted/10 px-4 py-2.5">
              <div className="text-xs text-muted-foreground">{total} total</div>
              <div className="flex items-center gap-2">
                <Button variant="outline" size="sm" className="h-7 text-xs" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_LIMIT))}>
                  Previous
                </Button>
                <Button variant="outline" size="sm" className="h-7 text-xs" disabled={offset + PAGE_LIMIT >= total} onClick={() => setOffset(offset + PAGE_LIMIT)}>
                  Next
                </Button>
              </div>
            </div>
          )}
        </div>
      </div>

      <LogDetailSheet
        entry={selectedEntry}
        open={drawerOpen}
        onOpenChange={(open) => { if (!open) { setDrawerOpen(false); setSelectedEntry(null); } }}
        onNavigate={(dir) => {
          const idx = dir === "prev" ? detailIndex - 1 : detailIndex + 1;
          if (idx >= 0 && idx < displayEntries.length) {
            setSelectedEntry(displayEntries[idx]!);
          } else if (dir === "next" && idx >= displayEntries.length && offset + PAGE_LIMIT < total) {
            setOffset(offset + PAGE_LIMIT);
          } else if (dir === "prev" && idx < 0 && offset > 0) {
            setOffset(Math.max(0, offset - PAGE_LIMIT));
          }
        }}
        hasPrev={detailIndex > 0 || offset > 0}
        hasNext={detailIndex < displayEntries.length - 1 || offset + PAGE_LIMIT < total}
      />
    </div>
  );
}

function AnalyticsCard({ title, headline, buckets }: { title: string; headline: string; buckets: AuditLogStatsBucket[] | undefined }) {
  return (
    <div className="flex flex-col gap-1.5 rounded-lg border border-border bg-surface p-3">
      <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{title}</div>
      <div className="text-sm font-semibold text-foreground tabular-nums">{headline}</div>
      <div className="flex flex-wrap gap-1">
        {(buckets ?? []).slice(0, 3).map((b) => (
          <Pill key={`${title}-${b.label}`} tone="muted">{b.label}: {b.count}</Pill>
        ))}
      </div>
    </div>
  );
}

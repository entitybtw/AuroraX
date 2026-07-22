import * as React from "react";
import { AlertCircle, CheckCircle2, ChevronDown, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import type {
  ProviderStatusItem,
  ProviderStatusKind,
  ProviderStatusResponse,
} from "@/lib/api/providers-types";

const STORAGE_KEY = "aurora_provider_status_expanded";

const STATUS_TONE: Record<ProviderStatusKind, { badge: string; card: string; dot: string }> = {
  healthy: {
    badge: "border-success/30 bg-success/10 text-success",
    card: "border-success/30 bg-success/5",
    dot: "bg-success",
  },
  degraded: {
    badge: "border-warning/30 bg-warning/10 text-warning",
    card: "border-warning/30 bg-warning/5",
    dot: "bg-warning",
  },
  unhealthy: {
    badge: "border-destructive/30 bg-destructive/10 text-destructive",
    card: "border-destructive/30 bg-destructive/5",
    dot: "bg-destructive",
  },
};

const STATUS_ORDER: Record<ProviderStatusKind, number> = {
  unhealthy: 0,
  degraded: 1,
  healthy: 2,
};

function StatusBadge({ status, label }: { status: ProviderStatusKind; label: string }): JSX.Element {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 px-2 py-0.5 text-[10px] font-bold uppercase tracking-widest border",
        STATUS_TONE[status].badge,
      )}
    >
      {status === "healthy" ? (
        <CheckCircle2 className="h-3 w-3" />
      ) : (
        <AlertCircle className="h-3 w-3" />
      )}
      {label}
    </span>
  );
}

interface ProviderRowProps {
  item: ProviderStatusItem;
  expanded: boolean;
  onToggle(): void;
  index: number;
}

function formatDate(value: string | null | undefined): string {
  if (!value) return "never";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function ProviderRow({ item, expanded, onToggle, index }: ProviderRowProps): JSX.Element {
  const tone = STATUS_TONE[item.status];
  const configuredModels = item.config.models?.length ?? 0;

  return (
    <li className="transition-colors duration-200 hover:bg-surface-hover/30">
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={expanded}
        className="flex flex-col items-stretch justify-between gap-3 p-6 text-left w-full h-full"
      >
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-3">
              <span className="text-[10px] font-mono text-border">{String(index + 1).padStart(2, "0")}</span>
              <span className={cn("h-2 w-2", tone.dot)} />
              <span className="truncate font-mono text-sm font-semibold text-foreground">{item.name}</span>
            </div>
            <div className="mt-3 ml-8 flex flex-wrap items-center gap-2 text-[10px] text-muted-foreground font-semibold tracking-wider uppercase">
              <span className="border border-border/40 px-2 py-0.5">{item.type || "—"}</span>
              <span className="border border-border/40 px-2 py-0.5">{item.runtime.discovered_model_count} discovered</span>
              {configuredModels > 0 ? <span className="border border-border/40 px-2 py-0.5">{configuredModels} configured</span> : null}
            </div>
          </div>
          <StatusBadge status={item.status} label={item.status_label} />
        </div>
        <div className="ml-8 space-y-2">
          <p className="line-clamp-2 text-[13px] leading-relaxed text-muted-foreground">{item.status_reason}</p>
          <div className="flex items-center gap-2 text-[11px] text-muted-foreground border border-border/30 px-3 py-1.5">
            <span>Last fetch {formatDate(item.runtime.last_model_fetch_success_at)}</span>
            <ChevronDown className={cn("h-4 w-4 ml-auto transition-transform", expanded && "rotate-180")} />
          </div>
        </div>
      </button>
      {expanded ? (
        <div className="border-t border-border/40 bg-surface-hover/20 px-6 py-4 text-xs ml-8 mr-6 mb-6">
          {item.last_error ? (
            <p className="border border-destructive/25 bg-destructive/10 px-3 py-2 font-mono text-[11px] text-destructive mb-3">
              {item.last_error}
            </p>
          ) : null}
          <dl className="grid grid-cols-2 gap-2 text-[11px] text-muted-foreground">
            <div className="border border-border/30 bg-background/30 px-3 py-2">
              <dt className="font-medium text-foreground">Registered</dt>
              <dd>{item.runtime.registered ? "yes" : "no"}</dd>
            </div>
            <div className="border border-border/30 bg-background/30 px-3 py-2">
              <dt className="font-medium text-foreground">Cached models</dt>
              <dd>{item.runtime.using_cached_models ? "yes" : "no"}</dd>
            </div>
            <div className="border border-border/30 bg-background/30 px-3 py-2">
              <dt className="font-medium text-foreground">Last fetch</dt>
              <dd>{formatDate(item.runtime.last_model_fetch_success_at)}</dd>
            </div>
            <div className="border border-border/30 bg-background/30 px-3 py-2">
              <dt className="font-medium text-foreground">Registry</dt>
              <dd>{item.runtime.registry_initialized ? "initialized" : "pending"}</dd>
            </div>
            {item.config.base_url ? (
              <div className="col-span-2 border border-border/30 bg-background/30 px-3 py-2">
                <dt className="font-medium text-foreground">Base URL</dt>
                <dd className="break-all font-mono">{item.config.base_url}</dd>
              </div>
            ) : null}
          </dl>
        </div>
      ) : null}
    </li>
  );
}

function loadExpanded(): Set<string> {
  if (typeof window === "undefined") return new Set();
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return new Set();
    const arr: unknown = JSON.parse(raw);
    if (Array.isArray(arr)) {
      return new Set(arr.filter((v): v is string => typeof v === "string"));
    }
  } catch {
    // ignore
  }
  return new Set();
}

function saveExpanded(set: Set<string>): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(Array.from(set)));
  } catch {
    // ignore
  }
}

export interface ProviderStatusGridProps {
  data: ProviderStatusResponse | undefined;
  isLoading: boolean;
  error: Error | null;
}

export function ProviderStatusGrid({
  data,
  isLoading,
  error,
}: ProviderStatusGridProps): JSX.Element {
  const [expanded, setExpanded] = React.useState<Set<string>>(() => loadExpanded());

  React.useEffect(() => {
    saveExpanded(expanded);
  }, [expanded]);

  const providers = React.useMemo(
    () => [...(data?.providers ?? [])].toSorted((a, b) => STATUS_ORDER[a.status] - STATUS_ORDER[b.status] || a.name.localeCompare(b.name)),
    [data?.providers],
  );

  function toggle(name: string): void {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  }

  return (
    <section
      aria-label="Provider status"
      className="border border-border/40 bg-surface"
    >
      <header className="p-6 flex flex-wrap items-start justify-between gap-3 border-b border-border/40">
        <div>
          <div className="text-[10px] font-bold tracking-widest uppercase text-accent mb-2">[ Providers ]</div>
          <h2 className="font-serif text-xl font-normal text-foreground">Runtime Status</h2>
          <p className="mt-1 text-xs text-muted-foreground">Health and model discovery status.</p>
        </div>
        {data?.summary ? (
          <div className="flex items-center gap-3 text-[11px] text-muted-foreground">
            <span className="inline-flex items-center gap-1.5"><span className="h-1.5 w-1.5 bg-success" />{data.summary.healthy} healthy</span>
            <span className="inline-flex items-center gap-1.5"><span className="h-1.5 w-1.5 bg-warning" />{data.summary.degraded} degraded</span>
            <span className="inline-flex items-center gap-1.5"><span className="h-1.5 w-1.5 bg-destructive" />{data.summary.unhealthy} unhealthy</span>
          </div>
        ) : null}
      </header>
      {isLoading && !data ? (
        <div className="flex items-center gap-2 p-6 text-xs text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading provider status…
        </div>
      ) : error ? (
        <div className="border border-destructive/25 bg-destructive/10 m-6 px-3 py-6 text-xs text-destructive">
          Failed to load provider status: {error.message}
        </div>
      ) : !data || providers.length === 0 ? (
        <div className="p-6 text-xs text-muted-foreground">
          No providers configured.
        </div>
      ) : (
        <ul className="divide-y divide-border/40">
          {providers.map((item, idx) => (
            <ProviderRow
              key={item.name}
              item={item}
              expanded={expanded.has(item.name)}
              onToggle={() => toggle(item.name)}
              index={idx}
            />
          ))}
        </ul>
      )}
    </section>
  );
}

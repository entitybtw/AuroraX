import * as React from "react";
import { XIcon, ChevronLeftIcon, ChevronRightIcon, MessageSquareIcon, InfoIcon, GlobeIcon, KeyRoundIcon, FingerprintIcon, ContainerIcon, NetworkIcon, ClockIcon, DollarSignIcon, DatabaseIcon, AlertCircleIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { WorkflowChart } from "@/components/charts/WorkflowChart";
import { ConversationTimeline } from "@/components/logs/ConversationTimeline";
import { fetchConversation } from "@/lib/api/audit";
import { buildConversationMessages, buildConversationSummary } from "@/lib/api/conversation-builder";
import type { AuditEntry } from "@/lib/api/audit-types";
import type { ConvMessage, ConvSummary } from "@/lib/api/conversation-builder";
import { cn } from "@/lib/utils";

type TabId = "details" | "interactions";

function statusToneClass(code?: number): string {
  if (!code) return "text-muted-foreground border-border/40 bg-background/40";
  if (code >= 200 && code < 300) return "text-success border-success/30 bg-success/10";
  if (code >= 400 && code < 500) return "text-warning border-warning/30 bg-warning/10";
  if (code >= 500) return "text-destructive border-destructive/30 bg-destructive/10";
  return "text-muted-foreground border-border/40 bg-background/40";
}

function formatCost(cost: number | null | undefined): string {
  if (cost === null || cost === undefined) return "—";
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", minimumFractionDigits: 4, maximumFractionDigits: 6 }).format(cost);
}

function formatDuration(ns?: number): string {
  if (!ns) return "—";
  const ms = ns / 1_000_000;
  if (ms < 1000) return `${ms.toFixed(0)}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(2)}s`;
  return `${(ms / 60_000).toFixed(1)}m`;
}

function tryPretty(value: unknown): string {
  if (value === null || value === undefined || value === "") return "";
  if (typeof value !== "string") return JSON.stringify(value, null, 2);
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

function hasPayload(value: unknown): boolean {
  if (value === null || value === undefined) return false;
  if (typeof value === "string") return value.trim().length > 0;
  if (Array.isArray(value)) return value.length > 0;
  if (typeof value === "object") return Object.keys(value).length > 0;
  return true;
}

function displayValue(value: string | undefined | null): string {
  if (!value) return "—";
  return value;
}

interface LogDetailSheetProps {
  entry: AuditEntry | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onNavigate?: (direction: "prev" | "next") => void;
  hasPrev?: boolean;
  hasNext?: boolean;
}

export function LogDetailSheet({ entry, open, onOpenChange, onNavigate, hasPrev, hasNext }: LogDetailSheetProps) {
  const [tab, setTab] = React.useState<TabId>("details");
  const [convState, setConvState] = React.useState<{ messages: ConvMessage[]; summary: ConvSummary } | null>(null);
  const [convLoading, setConvLoading] = React.useState(false);

  React.useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onOpenChange(false);
      if (e.key === "ArrowLeft" && onNavigate && hasPrev) onNavigate("prev");
      if (e.key === "ArrowRight" && onNavigate && hasNext) onNavigate("next");
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onNavigate, hasPrev, hasNext, onOpenChange]);

  React.useEffect(() => {
    setTab("details");
    setConvState(null);
  }, [entry?.id, open]);

  React.useEffect(() => {
    if (tab !== "interactions" || !entry || convState) return;
    setConvLoading(true);
    fetchConversation(entry.id)
      .then((resp) => {
        const messages = buildConversationMessages(resp.entries ?? [entry], resp.anchor_id || entry.id);
        const summary = buildConversationSummary(resp.entries ?? [entry]);
        setConvState({ messages, summary });
      })
      .catch(() => setConvState(null))
      .finally(() => setConvLoading(false));
  }, [tab, entry, convState]);

  if (!open) return null;

  const body = entry?.data?.request_body ?? entry?.request_body;
  const responseBody = entry?.data?.response_body ?? entry?.response_body;
  const reqHeaders = entry?.data?.request_headers;
  const resHeaders = entry?.data?.response_headers;
  const errorMessage = entry?.data?.error_message || entry?.error_type || "";
  const usage = entry?.usage;

  return (
    <>
      <div className="fixed inset-0 z-40 bg-background/80 backdrop-blur-sm transition-opacity" onClick={() => onOpenChange(false)} />

      <div className={cn(
        "fixed inset-y-0 right-0 z-50 flex flex-col",
        "w-full sm:w-[120vw] lg:w-[1400px]",
        "border-l border-border/40 bg-surface/90 shadow-2xl backdrop-blur-xl",
      )}>
        <div className="flex items-center justify-between border-b border-border/30 px-5 py-3.5 bg-background/40">
          <div className="flex items-center gap-3">
            <div className="flex gap-1">
              <Button variant="ghost" size="icon" className="h-7 w-7 " disabled={!hasPrev || !onNavigate} onClick={() => onNavigate?.("prev")}>
                <ChevronLeftIcon className="h-3.5 w-3.5" />
              </Button>
              <Button variant="ghost" size="icon" className="h-7 w-7 " disabled={!hasNext || !onNavigate} onClick={() => onNavigate?.("next")}>
                <ChevronRightIcon className="h-3.5 w-3.5" />
              </Button>
            </div>
            <div className="h-5 w-px bg-border/30" />
            {entry && (
              <>
                <span className={cn("inline-flex items-center  px-2.5 py-0.5 text-[11px] font-bold tracking-wide uppercase border", statusToneClass(entry.status_code))}>
                  {entry.status_code || "—"}
                </span>
                {entry.method && <span className="text-[11px] font-bold uppercase tracking-[0.1em] text-muted-foreground">{entry.method}</span>}
                {entry.requested_model && (
                  <span className="font-mono text-[13px] font-semibold text-foreground truncate max-w-[200px]" title={entry.requested_model}>
                    {entry.requested_model}
                  </span>
                )}
              </>
            )}
          </div>
          <Button variant="ghost" size="icon" onClick={() => onOpenChange(false)} className="h-8 w-8  hover:bg-muted/60">
            <XIcon className="h-4 w-4" />
          </Button>
        </div>

        <div className="flex items-center gap-1 border-b border-border/30 bg-background/20 px-4 py-0">
          <TabButton active={tab === "details"} onClick={() => setTab("details")} Icon={InfoIcon} label="Details" />
          <TabButton active={tab === "interactions"} onClick={() => setTab("interactions")} Icon={MessageSquareIcon} label="Interactions" />
        </div>

        <div className="flex-1 overflow-y-auto">
          {tab === "details" && entry && (
            <div className="flex flex-col">
              <div className="border-b border-border/20 bg-background/15 p-5">
                <WorkflowChart entry={entry} />
              </div>

              <div className="grid grid-cols-1 gap-3 p-5 lg:grid-cols-2">
                <MetaCard title="Request" className="lg:col-span-2">
                  <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-3 lg:grid-cols-4">
                    <MetaField icon={GlobeIcon} label="Path" value={displayValue(entry.path)} />
                    <MetaField icon={GlobeIcon} label="Method" value={displayValue(entry.method)} />
                    <MetaField icon={DollarSignIcon} label="Model" value={displayValue(entry.requested_model || entry.model)} />
                    <MetaField icon={NetworkIcon} label="Provider" value={displayValue(entry.provider_name || entry.provider)} />
                    <MetaField icon={KeyRoundIcon} label="API Key" value={displayValue(entry.auth_key_id)} />
                    <MetaField icon={FingerprintIcon} label="Auth Method" value={displayValue(entry.auth_method)} />
                    <MetaField icon={ContainerIcon} label="User Path" value={displayValue(entry.user_path)} />
                    <MetaField icon={NetworkIcon} label="Client IP" value={displayValue(entry.client_ip)} />
                  </div>
                </MetaCard>

                <MetaCard title="Response">
                  <div className="grid grid-cols-2 gap-2.5">
                    <MetaField icon={GlobeIcon} label="Status" value={displayValue(entry.status_code?.toString())} />
                    <MetaField icon={ClockIcon} label="Duration" value={formatDuration(entry.duration_ns)} />
                    <MetaField icon={DatabaseIcon} label="Total Tokens" value={usage?.total_tokens?.toLocaleString() ?? entry.total_tokens?.toLocaleString() ?? "—"} />
                    <MetaField icon={DollarSignIcon} label="Cost" value={formatCost(usage?.total_cost ?? entry.total_cost)} highlight />
                  </div>
                </MetaCard>

                <MetaCard title="Metadata">
                  <div className="grid grid-cols-2 gap-2.5">
                    <MetaField icon={DatabaseIcon} label="Request ID" value={displayValue(entry.request_id)} />
                    <MetaField icon={NetworkIcon} label="Resolved Model" value={displayValue(entry.resolved_model)} />
                    <MetaField icon={ContainerIcon} label="Workflow" value={displayValue(entry.workflow_version_id)} />
                    <MetaField icon={ClockIcon} label="Mode" value={entry.stream ? "Streaming" : "Non-streaming"} />
                  </div>
                </MetaCard>

                {(entry.cache_hit || entry.cache_type || entry.cache_mode || entry.failover_target) && (
                  <MetaCard title="Cache & Failover">
                    <div className="grid grid-cols-2 gap-2.5">
                      {entry.cache_hit !== undefined && <MetaField icon={DatabaseIcon} label="Cache Hit" value={entry.cache_hit ? "Yes" : "No"} />}
                      {entry.cache_type && <MetaField icon={DatabaseIcon} label="Cache Type" value={entry.cache_type} />}
                      {entry.cache_mode && <MetaField icon={DatabaseIcon} label="Cache Mode" value={entry.cache_mode} />}
                      {entry.failover_target && <MetaField icon={NetworkIcon} label="Failover Target" value={entry.failover_target} />}
                    </div>
                  </MetaCard>
                )}

                {usage && (
                  <MetaCard title="Usage Breakdown" className="lg:col-span-2">
                    <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-4">
                      <MetaField icon={DatabaseIcon} label="Input Tokens" value={usage.input_tokens?.toLocaleString() ?? "—"} />
                      <MetaField icon={DatabaseIcon} label="Output Tokens" value={usage.output_tokens?.toLocaleString() ?? "—"} />
                      <MetaField icon={DollarSignIcon} label="Input Cost" value={formatCost(usage.input_cost as number | null | undefined)} />
                      <MetaField icon={DollarSignIcon} label="Output Cost" value={formatCost(usage.output_cost as number | null | undefined)} />
                      {usage.prompt_cache_read_tokens !== undefined && (
                        <MetaField icon={DatabaseIcon} label="Cache Read Tokens" value={usage.prompt_cache_read_tokens?.toLocaleString() ?? "—"} />
                      )}
                      {usage.prompt_cache_write_tokens !== undefined && (
                        <MetaField icon={DatabaseIcon} label="Cache Write Tokens" value={usage.prompt_cache_write_tokens?.toLocaleString() ?? "—"} />
                      )}
                      {entry.input_tokens !== undefined && (
                        <MetaField icon={DatabaseIcon} label="Legacy Input Tokens" value={entry.input_tokens?.toLocaleString() ?? "—"} />
                      )}
                      {entry.output_tokens !== undefined && (
                        <MetaField icon={DatabaseIcon} label="Legacy Output Tokens" value={entry.output_tokens?.toLocaleString() ?? "—"} />
                      )}
                    </div>
                  </MetaCard>
                )}
              </div>

              <div className="space-y-3 px-5 pb-5">
                {hasPayload(errorMessage) && (
                  <div className="overflow-hidden border border-destructive/20 bg-destructive/[0.03]">
                    <div className="flex items-center gap-2 border-b border-destructive/10 px-4 py-2.5">
                      <AlertCircleIcon className="h-4 w-4 text-destructive" />
                      <span className="text-[11px] font-bold uppercase tracking-wider text-destructive">Error</span>
                    </div>
                    <pre className="max-h-48 overflow-auto p-4 font-mono text-[12px] leading-relaxed whitespace-pre-wrap text-destructive/90">{errorMessage}</pre>
                  </div>
                )}

                {hasPayload(reqHeaders) || hasPayload(body) ? (
                  <div className="overflow-hidden border border-border/40">
                    <div className="flex items-center gap-2 border-b border-border/20 bg-background/20 px-4 py-2.5">
                      <ContainerIcon className="h-4 w-4 text-muted-foreground" />
                      <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Request Body</span>
                    </div>
                    {hasPayload(reqHeaders) && (
                      <details>
                        <summary className="cursor-pointer px-4 py-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground outline-none hover:bg-background/20">Headers</summary>
                        <pre className="max-h-48 overflow-auto border-t border-border/20 bg-background/30 p-4 font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-foreground">{tryPretty(reqHeaders)}</pre>
                      </details>
                    )}
                    {hasPayload(body) && (
                      <pre className="max-h-96 overflow-auto p-4 font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-foreground bg-background/20">{tryPretty(body)}</pre>
                    )}
                  </div>
                ) : null}

                {hasPayload(resHeaders) || hasPayload(responseBody) ? (
                  <div className="overflow-hidden border border-border/40">
                    <div className="flex items-center gap-2 border-b border-border/20 bg-background/20 px-4 py-2.5">
                      <ContainerIcon className="h-4 w-4 text-muted-foreground" />
                      <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Response Body</span>
                    </div>
                    {hasPayload(resHeaders) && (
                      <details>
                        <summary className="cursor-pointer px-4 py-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground outline-none hover:bg-background/20">Headers</summary>
                        <pre className="max-h-48 overflow-auto border-t border-border/20 bg-background/30 p-4 font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-foreground">{tryPretty(resHeaders)}</pre>
                      </details>
                    )}
                    {hasPayload(responseBody) && (
                      <pre className="max-h-96 overflow-auto p-4 font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-foreground bg-background/20">{tryPretty(responseBody)}</pre>
                    )}
                  </div>
                ) : null}
              </div>
            </div>
          )}

          {tab === "interactions" && (
            <div className="p-5">
              {convLoading ? (
                <div className="flex flex-col items-center justify-center gap-2 py-16">
                  <div className="h-5 w-5 animate-spin  border-2 border-muted-foreground/30 border-t-accent" />
                  <p className="text-sm text-muted-foreground/60">Loading conversation\u2026</p>
                </div>
              ) : convState ? (
                <ConversationTimeline messages={convState.messages} summary={convState.summary} />
              ) : (
                <div className="flex flex-col items-center justify-center gap-2 py-16">
                  <MessageSquareIcon className="h-8 w-8 text-muted-foreground/30" />
                  <p className="text-sm text-muted-foreground/60">Unable to load conversation data.</p>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </>
  );
}

function TabButton({ active, onClick, Icon, label }: { active: boolean; onClick: () => void; Icon: React.ComponentType<{ className?: string }>; label: string }) {
  return (
    <button
      onClick={onClick}
      className={cn(
        "relative flex items-center gap-2 px-4 py-2.5 text-[11px] font-semibold uppercase tracking-[0.08em] transition-colors outline-none",
        active
          ? "text-foreground after:absolute after:bottom-0 after:left-2 after:right-2 after:h-0.5 after: after:bg-accent"
          : "text-muted-foreground/60 hover:text-muted-foreground",
      )}
    >
      <Icon className="h-3.5 w-3.5" />
      {label}
    </button>
  );
}

function MetaCard({ title, children, className }: { title: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={cn("overflow-hidden border border-border/40 bg-surface/60", className)}>
      <div className="border-b border-border/20 bg-background/15 px-4 py-2.5">
        <span className="text-[10px] font-bold uppercase tracking-[0.12em] text-muted-foreground">{title}</span>
      </div>
      <div className="p-4">{children}</div>
    </div>
  );
}

function MetaField({ icon: Icon, label, value, highlight }: { icon: React.ComponentType<{ className?: string }>; label: string; value: string; highlight?: boolean }) {
  return (
    <div className="flex items-center gap-2.5 border border-border/30 bg-background/40 px-3 py-2">
      <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground/60" />
      <div className="min-w-0 flex-1">
        <div className="text-[9px] font-semibold uppercase tracking-wider text-muted-foreground/60">{label}</div>
        <div className={cn(
          "truncate font-mono text-[12px] font-medium",
          highlight ? "text-success" : "text-foreground",
        )} title={value}>{value}</div>
      </div>
    </div>
  );
}

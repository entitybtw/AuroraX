import * as React from "react";
import {
  ArrowRightCircle,
  BarChart3,
  BookOpen,
  Cpu,
  CreditCard,
  Database,
  Globe,
  Gavel,
  KeyRound,
  Network,
  RefreshCcw,
} from "lucide-react";
import { cn } from "@/lib/utils";
import type { AuditEntry, AuditWorkflowFeatures, AuditFailoverSnapshot } from "@/lib/api/audit-types";

/**
 * Visual representation of a single audit entry's request pipeline.
 * Flow: Origin → Access → (Cache) → (Limit) → (Policy) → Engine → (Backup) → Reply,
 * plus a deferred row for Metrics and Journal when configured.
 */

export interface WorkflowChartProps {
  entry: AuditEntry;
  className?: string;
}

interface NodeState {
  tone: "default" | "success" | "warning" | "danger" | "skipped" | "neutral";
  badge?: string | undefined;
  sublabel?: string | undefined;
}

type StopStage = "none" | "auth" | "cache" | "budget" | "guardrails" | "ai";

function statusTone(code?: number): NodeState {
  if (!code) return { tone: "default" };
  if (code >= 500) return { tone: "danger", sublabel: String(code) };
  if (code >= 400) return { tone: "warning", sublabel: String(code) };
  if (code >= 200 && code < 300) return { tone: "success", sublabel: String(code) };
  return { tone: "neutral", sublabel: String(code) };
}

function nestedErrorCode(value: unknown, depth = 0): string {
  if (depth > 4 || value === null || value === undefined) return "";
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (!trimmed || (trimmed[0] !== "{" && trimmed[0] !== "[")) return trimmed;
    try {
      return nestedErrorCode(JSON.parse(trimmed), depth + 1);
    } catch {
      return trimmed;
    }
  }
  if (Array.isArray(value)) {
    for (const item of value) {
      const code = nestedErrorCode(item, depth + 1);
      if (code) return code;
    }
    return "";
  }
  if (typeof value !== "object") return "";
  const record = value as Record<string, unknown>;
  const code = String(record.code || record.error_code || record.errorCode || "").trim();
  if (code) return code;
  if (record.error !== undefined) return nestedErrorCode(record.error, depth + 1);
  if (record.message !== undefined) return String(record.message).trim();
  return "";
}

function entryErrorText(entry: AuditEntry): string {
  return [
    entry.error_type,
    entry.data?.error_code,
    entry.data?.error_message,
    nestedErrorCode(entry.data?.response_body),
    nestedErrorCode(entry.response_body),
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

function toneClass(tone: NodeState["tone"]): string {
  switch (tone) {
    case "success":
      return "border-success/40 bg-success/10 text-success [--node-glow:color-mix(in_srgb,var(--success)_35%,transparent)]";
    case "warning":
      return "border-warning/40 bg-warning/10 text-warning [--node-glow:color-mix(in_srgb,var(--warning)_35%,transparent)]";
    case "danger":
      return "border-destructive/40 bg-destructive/10 text-destructive [--node-glow:color-mix(in_srgb,var(--danger)_35%,transparent)]";
    case "skipped":
      return "border-border bg-surface/50 text-muted-foreground opacity-60";
    case "neutral":
      return "border-accent/35 bg-accent/10 text-accent [--node-glow:color-mix(in_srgb,var(--accent)_30%,transparent)]";
    default:
      return "border-border bg-surface text-foreground";
  }
}

function connTone(tone: NodeState["tone"]): string {
  switch (tone) {
    case "success":
      return "from-border via-success/40 to-border";
    case "warning":
      return "from-border via-warning/40 to-border";
    case "danger":
      return "from-border via-destructive/40 to-border";
    case "skipped":
      return "from-border/40 via-border/20 to-border/40 opacity-50";
    default:
      return "from-border via-border to-border";
  }
}

function linkTone(from: NodeState, to: NodeState): NodeState["tone"] {
  if (from.tone === "skipped" || to.tone === "skipped") return "skipped";
  if (to.tone === "danger" || to.tone === "warning" || to.tone === "neutral") return to.tone;
  if (from.tone === "success" && to.tone === "success") return "success";
  return "default";
}

interface NodeProps {
  Icon: React.ComponentType<{ className?: string }>;
  label: string;
  state: NodeState;
}

function Node({ Icon, label, state }: NodeProps): JSX.Element {
  return (
    <div
      className={cn(
        "relative flex min-h-[76px] w-24 shrink-0 flex-col items-center justify-center gap-1 border px-2 py-2 text-center shadow-[inset_0_1px_0_rgb(255_255_255/0.08)] transition-colors",
        "before:absolute before:inset-0 before:-z-10 before:before:opacity-60 before:blur-xl before:content-[''] before:[background:radial-gradient(60%_60%_at_50%_40%,var(--node-glow,transparent),transparent)]",
        toneClass(state.tone),
      )}
    >
      <div className="flex h-6 w-6 items-center justify-center  bg-background/60">
        <Icon className="h-3 w-3" />
      </div>
      <span className="text-[10px] font-semibold uppercase tracking-wider">{label}</span>
      {state.badge && (
        <span className=" border border-current/40 px-1.5 text-[8px] font-semibold tracking-wide">
          {state.badge}
        </span>
      )}
      {state.sublabel && (
        <span className="font-mono text-[10px] text-muted-foreground break-all px-1 leading-tight">{state.sublabel}</span>
      )}
    </div>
  );
}

function Conn({ tone }: { tone: NodeState["tone"] }): JSX.Element {
  return (
    <div
      aria-hidden
      className={cn("h-px w-8 shrink-0 bg-gradient-to-r", connTone(tone))}
    />
  );
}

function LabeledConn({ tone, label }: { tone: NodeState["tone"]; label: string }): JSX.Element {
  return (
    <div className="flex w-16 shrink-0 flex-col items-center gap-1" aria-hidden>
      <span className="text-[10px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">{label}</span>
      <div className={cn("h-px w-full bg-gradient-to-r", connTone(tone))} />
    </div>
  );
}

function readFeatures(entry: AuditEntry): AuditWorkflowFeatures {
  if (entry.data?.workflow_features) {
    return entry.data.workflow_features;
  }
  return {
    cache: false,
    audit: !!entry.usage || !!entry.timestamp, // every audited entry has audit on by definition
    usage: !!entry.usage,
    budget: false,
    guardrails: false,
    fallback: false,
  };
}

function readFailover(entry: AuditEntry): AuditFailoverSnapshot | undefined {
  if (entry.data?.failover) return entry.data.failover;
  if (entry.failover_target) {
    return { used: true, target_model: entry.failover_target };
  }
  return undefined;
}

function reached(stage: "cache" | "budget" | "guardrails" | "ai" | "failover", stopStage: StopStage): boolean {
  if (stopStage === "auth") return false;
  if (stage === "cache") return true;
  if (stopStage === "cache") return false;
  if (stage === "budget") return true;
  if (stopStage === "budget") return false;
  if (stage === "guardrails") return true;
  if (stopStage === "guardrails") return false;
  if (stage === "ai") return true;
  if (stopStage === "ai") return false;
  return true;
}

export function WorkflowChart({ entry, className }: WorkflowChartProps): JSX.Element {
  const features = readFeatures(entry);
  const failover = readFailover(entry);
  const errorText = entryErrorText(entry);
  const cacheHit = entry.cache_hit || !!entry.cache_type;
  const cacheLabel = cacheHit
    ? entry.cache_type === "semantic"
      ? "Hit · Semantic"
      : "Hit · Exact"
    : undefined;
  const responseTone = statusTone(entry.status_code);
  const budgetExceeded = errorText.includes("budget_exceeded") || errorText.includes("budget exceeded");
  const authFailed = errorText.includes("authentication") || errorText.includes("unauthorized") || entry.status_code === 401;
  const guardrailsFailed = errorText.includes("guardrail") || errorText.includes("policy");
  const aiFailed = !cacheHit && responseTone.tone !== "success" && !authFailed && !budgetExceeded && !guardrailsFailed;
  const stopStage: StopStage = authFailed
    ? "auth"
    : cacheHit
      ? "cache"
      : budgetExceeded
        ? "budget"
        : guardrailsFailed
          ? "guardrails"
          : aiFailed
            ? "ai"
            : "none";

  const cacheReached = reached("cache", stopStage);
  const budgetReached = reached("budget", stopStage);
  const guardrailsReached = reached("guardrails", stopStage);
  const aiReached = reached("ai", stopStage);
  const failoverReached = reached("failover", stopStage);

  const auth: NodeState = {
    tone: authFailed ? "danger" : entry.auth_method ? "success" : "default",
    sublabel: entry.auth_method || undefined,
  };
  const cacheState: NodeState = {
    tone: cacheReached ? (cacheHit ? "success" : "default") : "skipped",
    badge: cacheReached ? cacheLabel : "Not reached",
  };
  const budgetState: NodeState = {
    tone: budgetReached ? (budgetExceeded ? "danger" : "success") : "skipped",
    badge: budgetReached ? (budgetExceeded ? "Exceeded" : undefined) : "Not reached",
  };
  const guardrailState: NodeState = {
    tone: guardrailsReached ? (guardrailsFailed ? "danger" : "success") : "skipped",
    badge: guardrailsReached ? (guardrailsFailed ? "Failed" : undefined) : "Not reached",
  };
  const aiState: NodeState = cacheHit
    ? { tone: "skipped" }
    : !aiReached
      ? { tone: "skipped", badge: "Not reached", sublabel: entry.provider_name || entry.provider || undefined }
      : { tone: aiFailed ? responseTone.tone : responseTone.tone === "success" ? "success" : "default", sublabel: entry.provider_name || entry.provider || undefined };
  const failoverState: NodeState = failover?.used
    ? { tone: "success", badge: "Redirected", sublabel: failover.target_model || entry.failover_target || undefined }
    : { tone: failoverReached ? "default" : "skipped", badge: failoverReached ? undefined : "Not reached" };
  const responseState: NodeState = responseTone;

  const showCache = !!features.cache || cacheHit;
  const showBudget = !!features.budget;
  const showGuardrails = !!features.guardrails;
  const showFailover = !!features.fallback || !!failover?.used;
  const showUsage = !!features.usage || !!entry.usage;
  const showAudit = !!features.audit || !!entry.timestamp || !!entry.id;
  const showAsync = showUsage || showAudit;
  const usageState: NodeState = { tone: showUsage ? "success" : "default" };
  const auditState: NodeState = { tone: showAudit ? "success" : "default" };
  const firstAsyncState = showUsage ? usageState : auditState;
  const asyncLinkTone = linkTone(responseState, firstAsyncState);
  const lastBeforeAIState = showGuardrails
    ? guardrailState
    : showBudget
      ? budgetState
      : showCache
        ? cacheState
        : auth;

  return (
    <div
      className={cn(
        "rounded-2xl border border-border/70 bg-surface/40 p-4 backdrop-blur-md w-full",
        "shadow-[inset_0_1px_0_rgb(255_255_255/0.04)]",
        className,
      )}
    >
      {entry.workflow_version_id && (
        <div className="mb-3 inline-flex max-w-full flex-wrap items-center gap-2  border border-border/70 bg-background/40 px-2 py-0.5 font-mono text-[10px] text-muted-foreground">
          <span>workflow id</span>
          <span className="break-all text-foreground">{entry.workflow_version_id}</span>
        </div>
      )}

      <div className="flex w-full items-center justify-center overflow-x-auto pb-1">
        <div className="flex items-center min-w-max">
          <Node Icon={Globe} label="Origin" state={{ tone: "default" }} />
          <Conn tone={auth.tone === "danger" ? "danger" : "default"} />
          <Node Icon={KeyRound} label="Access" state={auth} />
          {showCache && (
            <>
              <Conn tone={linkTone(auth, cacheState)} />
              <Node Icon={Database} label="Cache" state={cacheState} />
            </>
          )}
          {showBudget && (
            <>
              <Conn tone={linkTone(showCache ? cacheState : auth, budgetState)} />
              <Node Icon={CreditCard} label="Limit" state={budgetState} />
            </>
          )}
          {showGuardrails && (
            <>
              <Conn tone={linkTone(showBudget ? budgetState : showCache ? cacheState : auth, guardrailState)} />
              <Node Icon={Gavel} label="Policy" state={guardrailState} />
            </>
          )}
          <Conn tone={linkTone(lastBeforeAIState, aiState)} />
          <Node Icon={Cpu} label="Engine" state={aiState} />
          {showFailover && (
            <>
              <Conn tone={linkTone(aiState, failoverState)} />
              <Node Icon={RefreshCcw} label="Backup" state={failoverState} />
            </>
          )}
          <Conn tone={linkTone(showFailover ? failoverState : aiState, responseState)} />
          <Node Icon={ArrowRightCircle} label="Reply" state={responseState} />
          {showAsync && (
            <>
              <LabeledConn tone={asyncLinkTone} label="Deferred" />
              {showUsage && (
                <Node Icon={BarChart3} label="Metrics" state={usageState} />
              )}
              {showAudit && (
                <>
                  {showUsage && <Conn tone={linkTone(usageState, auditState)} />}
                  <Node Icon={BookOpen} label="Journal" state={auditState} />
                </>
              )}
              {!showUsage && !showAudit && (
                <span className="shrink-0 text-xs text-muted-foreground ml-2">
                  <Network className="mr-1 inline h-3 w-3" /> No async pipeline
                </span>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

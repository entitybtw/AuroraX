import * as React from "react";
import { TerminalIcon, UserIcon, BotIcon, AlertTriangleIcon, HammerIcon, BracesIcon, BrainCircuitIcon, ChevronDownIcon, SparklesIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import type { ConvMessage, ConvSummary, ConvToolCall } from "@/lib/api/conversation-builder";

const ROLE_CONFIG: Record<string, { Icon: React.ComponentType<{ className?: string }>; color: string; label: string }> = {
  system: { Icon: TerminalIcon, color: "border-purple-500/40 bg-purple-500/10 text-purple-500", label: "System" },
  user: { Icon: UserIcon, color: "border-blue-500/40 bg-blue-500/10 text-blue-500", label: "User" },
  assistant: { Icon: BotIcon, color: "border-emerald-500/40 bg-emerald-500/10 text-emerald-500", label: "Assistant" },
  tool: { Icon: HammerIcon, color: "border-amber-500/40 bg-amber-500/10 text-amber-500", label: "Tool" },
  function_call: { Icon: BracesIcon, color: "border-orange-500/40 bg-orange-500/10 text-orange-500", label: "Function" },
  function_result: { Icon: BracesIcon, color: "border-cyan-500/40 bg-cyan-500/10 text-cyan-500", label: "Result" },
  error: { Icon: AlertTriangleIcon, color: "border-destructive/40 bg-destructive/10 text-destructive", label: "Error" },
};

interface ConversationTimelineProps {
  messages: ConvMessage[];
  summary: ConvSummary;
}

function ReasoningBlock({ text }: { text: string }) {
  const [open, setOpen] = React.useState(false);
  const lines = text.split("\n").length;
  return (
    <div className="mb-3 overflow-hidden border border-amber-500/25 bg-amber-500/5">
      <button
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-2.5 px-3.5 py-2.5 text-left outline-none transition-colors hover:bg-amber-500/5"
      >
        <BrainCircuitIcon className="h-3.5 w-3.5 shrink-0 text-amber-500" />
        <span className="text-[11px] font-semibold uppercase tracking-wider text-amber-600 dark:text-amber-400">
          Reasoning
        </span>
        <span className="text-[10px] font-mono text-muted-foreground">{lines} lines</span>
        <ChevronDownIcon className={cn(
          "ml-auto h-3.5 w-3.5 text-muted-foreground transition-transform duration-200",
          open && "rotate-180",
        )} />
      </button>
      {open && (
        <div className="border-t border-amber-500/15 px-3.5 py-3">
          <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words font-mono text-[11px] leading-relaxed text-muted-foreground">
            {text}
          </pre>
        </div>
      )}
    </div>
  );
}

function ToolCallBlock({ call }: { call: ConvToolCall }) {
  const [open, setOpen] = React.useState(false);
  return (
    <details
      className="mt-2 overflow-hidden border border-border/50 bg-background/40"
      open={open}
      onToggle={(e) => setOpen(e.currentTarget.open)}
    >
      <summary className="flex cursor-pointer items-center gap-2.5 px-3.5 py-2.5 font-mono text-[11px] text-muted-foreground outline-none transition-colors hover:bg-background/60">
        <HammerIcon className="h-3.5 w-3.5 shrink-0 text-amber-500" />
        <span className="font-semibold text-foreground">{call.name}</span>
        {call.id && <span className="text-[10px] text-muted-foreground/70">({call.id})</span>}
        <ChevronDownIcon className={cn(
          "ml-auto h-3.5 w-3.5 text-muted-foreground transition-transform duration-200",
          open && "rotate-180",
        )} />
      </summary>
      <div className="border-t border-border/30 space-y-3 px-3.5 py-3">
        {call.argsPretty && (
          <div>
            <div className="mb-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Arguments</div>
            <pre className="max-h-48 overflow-auto border border-border/30 bg-background/60 p-3 font-mono text-[10px] leading-relaxed whitespace-pre-wrap text-foreground">{call.argsPretty}</pre>
          </div>
        )}
        {call.result && (
          <div>
            <div className="mb-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Result</div>
            <pre className="max-h-48 overflow-auto border border-border/30 bg-background/60 p-3 font-mono text-[10px] leading-relaxed whitespace-pre-wrap text-muted-foreground">{call.result}</pre>
          </div>
        )}
      </div>
    </details>
  );
}

function MessageRow({ message }: { message: ConvMessage }) {
  const config = (ROLE_CONFIG[message.role] ?? ROLE_CONFIG.user)!;
  const { Icon, color } = config;
  const isAssistant = message.role === "assistant";

  return (
    <div className={cn(
      "group relative flex gap-4 px-1 py-3 transition-colors",
      message.isAnchor && "bg-accent/[0.03] ring-1 ring-accent/20 -mx-2 px-3",
    )}>
      <div className="flex shrink-0 flex-col items-center">
        <div className={cn(
          "flex h-8 w-8 items-center justify-center  border transition-transform duration-150 group-hover:scale-105",
          color,
        )}>
          <Icon className="h-4 w-4" />
        </div>
        {message.toolCalls.length > 0 && (
          <div className="mt-1.5 h-full w-px bg-gradient-to-b from-border/60 to-transparent" />
        )}
      </div>

      <div className="min-w-0 flex-1 space-y-2">
        <div className="flex items-center gap-2.5">
          <span className="text-[12px] font-bold tracking-tight text-foreground">{message.roleLabel}</span>
          {message.timestamp && (
            <span className="text-[10px] font-mono text-muted-foreground/60">{new Date(message.timestamp).toLocaleTimeString()}</span>
          )}
          {message.isAnchor && (
            <span className="inline-flex items-center gap-1  border border-accent/25 bg-accent/10 px-2 py-0.5 text-[9px] font-bold uppercase tracking-widest text-accent">
              <SparklesIcon className="h-2.5 w-2.5" />
              Selected
            </span>
          )}
          {message.toolName && (
            <span className=" border border-amber-500/20 bg-amber-500/10 px-2 py-0.5 text-[9px] font-mono font-semibold text-amber-600 dark:text-amber-400">
              {message.toolName}
            </span>
          )}
        </div>

        {message.reasoning && <ReasoningBlock text={message.reasoning} />}

        {message.text && (
          <div className="max-w-prose text-[13px] leading-relaxed text-foreground/90 whitespace-pre-wrap break-words">
            {message.text}
          </div>
        )}

        {message.toolCalls.map((tc, i) => (
          <ToolCallBlock key={tc.id ?? `${message.uid}-tc-${i}`} call={tc} />
        ))}

        {isAssistant && message.agentMeta && (
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border border-border/30 bg-background/40 px-3 py-2 text-[10px] font-mono">
            {message.agentMeta.provider && (
              <span className="font-semibold text-foreground">{message.agentMeta.provider}</span>
            )}
            {message.agentMeta.model && (
              <span className="text-muted-foreground">{message.agentMeta.model}</span>
            )}
            <span className="text-muted-foreground/70">{message.agentMeta.inputTokens} in / {message.agentMeta.outputTokens} out</span>
            <span className="text-muted-foreground/70">{message.agentMeta.durationLabel}</span>
            {message.agentMeta.cost !== null && (
              <span className="font-semibold text-success">${message.agentMeta.cost.toFixed(6)}</span>
            )}
            {message.agentMeta.cacheHit && (
              <span className="rounded border border-info/30 bg-info/10 px-1.5 py-0.5 text-[9px] font-semibold text-info">cached</span>
            )}
            {message.agentMeta.streaming && (
              <span className="rounded border border-accent/30 bg-accent/10 px-1.5 py-0.5 text-[9px] font-semibold text-accent">stream</span>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function SummaryCard({ label, value, highlight, danger }: { label: string; value: string; highlight?: boolean; danger?: boolean }) {
  return (
    <div className={cn(
      "border px-3.5 py-2.5 transition-colors",
      danger ? "border-destructive/20 bg-destructive/[0.04]" : "border-border/50 bg-background/30",
    )}>
      <div className="text-[10px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">{label}</div>
      <div className={cn(
        "mt-0.5 font-mono text-[13px] font-semibold tabular-nums",
        danger ? "text-destructive" : highlight ? "text-success" : "text-foreground",
      )}>{value}</div>
    </div>
  );
}

export function ConversationTimeline({ messages, summary }: ConversationTimelineProps) {
  if (messages.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center gap-2 py-16">
        <BotIcon className="h-8 w-8 text-muted-foreground/30" />
        <p className="text-sm text-muted-foreground/60">No conversation messages available for this entry.</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <SummaryCard label="Calls" value={String(summary.callCount)} />
        <SummaryCard label="Total Tokens" value={summary.totalTokens.toLocaleString()} />
        <SummaryCard label="Duration" value={summary.totalDurationLabel} />
        <SummaryCard label="Cost" value={summary.cost !== null ? `$${summary.cost.toFixed(6)}` : "—"} highlight={summary.cost !== null} />
        <SummaryCard label="Providers" value={summary.providers || "—"} />
        <SummaryCard label="Models" value={summary.models || "—"} />
        <SummaryCard label="Cache Hits" value={String(summary.cacheHits)} />
        <SummaryCard label="Errors" value={String(summary.errors)} danger={summary.errors > 0} />
      </div>

      <div className="divide-y divide-border/20">
        {messages.map((msg) => (
          <MessageRow key={msg.uid} message={msg} />
        ))}
      </div>
    </div>
  );
}

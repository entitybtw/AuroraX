import type { AuditEntry } from "@/lib/api/audit-types";

export type ConvRole =
  | "system"
  | "user"
  | "assistant"
  | "tool"
  | "function_call"
  | "function_result"
  | "error";

export interface ConvToolCall {
  id?: string;
  name: string;
  argsPretty: string;
  result?: string;
}

export interface ConvMessage {
  uid: string;
  entryId: string;
  role: ConvRole;
  roleLabel: string;
  text: string;
  reasoning?: string | undefined;
  timestamp?: string | undefined;
  isAnchor: boolean;
  toolCalls: ConvToolCall[];
  toolName?: string | undefined;
  agentMeta?: ConvAgentMeta | undefined;
}

export interface ConvAgentMeta {
  provider?: string | undefined;
  model?: string | undefined;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  durationMs: number;
  durationLabel: string;
  cost: number | null;
  cacheHit: boolean;
  cacheLabel?: string | undefined;
  streaming: boolean;
}

export interface ConvSummary {
  entryCount: number;
  callCount: number;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  cost: number | null;
  cacheHits: number;
  errors: number;
  okCount: number;
  streamingCount: number;
  totalDurationMs: number;
  totalDurationLabel: string;
  providers: string;
  models: string;
}

export interface ConvTimelineStep {
  uid: string;
  kind: "system" | "error" | "user" | "assistant" | "tool";
  label: string;
  sub?: string | undefined;
}

const ROLE_LABELS: Record<ConvRole, string> = {
  system: "System",
  user: "User",
  assistant: "Assistant",
  tool: "Tool",
  function_call: "Function call",
  function_result: "Function result",
  error: "Error",
};

function pretty(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") {
    try {
      return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
      return value;
    }
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function extractText(content: unknown): string {
  if (content === null || content === undefined) return "";
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content
      .map((part) => {
        if (typeof part === "string") return part;
        if (part && typeof part === "object") {
          const p = part as { type?: string; text?: string; content?: unknown };
          if (p.type === "text" && typeof p.text === "string") return p.text;
          if (p.type === "input_text" && typeof p.text === "string") return p.text;
          if (p.type === "output_text" && typeof p.text === "string") return p.text;
          if (typeof p.text === "string") return p.text;
        }
        return "";
      })
      .filter(Boolean)
      .join("\n");
  }
  if (typeof content === "object") {
    const c = content as { text?: string };
    if (typeof c.text === "string") return c.text;
  }
  return "";
}

function extractToolCalls(raw: unknown): ConvToolCall[] {
  if (!Array.isArray(raw)) return [];
  return raw
    .map((tc) => {
      if (!tc || typeof tc !== "object") return null;
      const obj = tc as Record<string, unknown>;
      const fn = (obj.function as Record<string, unknown> | undefined) ?? {};
      const name = String(obj.name ?? fn.name ?? "");
      if (!name) return null;
      const args = obj.arguments ?? fn.arguments ?? "";
      return {
        id: typeof obj.id === "string" ? obj.id : undefined,
        name,
        argsPretty: pretty(args),
      } as ConvToolCall;
    })
    .filter((x): x is ConvToolCall => x !== null);
}

function durationLabel(ms: number): string {
  if (!ms) return "—";
  if (ms < 1000) return `${ms.toFixed(0)}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(2)}s`;
  return `${(ms / 60_000).toFixed(1)}m`;
}

function buildEntryAgentMeta(entry: AuditEntry): ConvAgentMeta {
  const ms = entry.duration_ns ? entry.duration_ns / 1_000_000 : 0;
  const usage = entry.usage ?? {};
  // Tolerate both legacy flat payload attributes and nested usage attributes properly mapping input_tokens
  const inputTokens = Number(usage.input_tokens ?? usage.total_input_tokens ?? entry.input_tokens ?? 0);
  const outputTokens = Number(usage.output_tokens ?? usage.total_output_tokens ?? entry.output_tokens ?? 0);
  const totalTokens = Number(usage.total_tokens ?? entry.total_tokens ?? inputTokens + outputTokens);
  const cost =
    usage.total_cost !== undefined && usage.total_cost !== null
      ? usage.total_cost
      : entry.total_cost ?? null;
  const cacheHit = !!entry.cache_hit || !!entry.cache_type;
  return {
    provider: entry.provider_name || entry.provider || undefined,
    model: entry.requested_model || entry.model || undefined,
    inputTokens,
    outputTokens,
    totalTokens,
    durationMs: ms,
    durationLabel: durationLabel(ms),
    cost,
    cacheHit,
    cacheLabel: cacheHit ? (entry.cache_type === "semantic" ? "cache: semantic" : "cache: exact") : undefined,
    streaming: !!entry.stream,
  };
}

function pushMessage(
  out: ConvMessage[],
  partial: Omit<ConvMessage, "uid" | "roleLabel">,
): void {
  out.push({
    ...partial,
    uid: `${partial.entryId}-${out.length}`,
    roleLabel: ROLE_LABELS[partial.role] ?? partial.role,
  });
}

function getRequestBody(entry: AuditEntry): Record<string, unknown> | null {
  const fromData = entry.data?.request_body;
  if (fromData && typeof fromData === "object" && !Array.isArray(fromData)) {
    return fromData as Record<string, unknown>;
  }
  if (typeof entry.request_body === "string") {
    try {
      const parsed = JSON.parse(entry.request_body);
      if (parsed && typeof parsed === "object") return parsed as Record<string, unknown>;
    } catch {
      // fall through
    }
  }
  return null;
}

function getResponseBody(entry: AuditEntry): Record<string, unknown> | null {
  const fromData = entry.data?.response_body;
  if (fromData && typeof fromData === "object" && !Array.isArray(fromData)) {
    return fromData as Record<string, unknown>;
  }
  if (typeof entry.response_body === "string") {
    try {
      const parsed = JSON.parse(entry.response_body);
      if (parsed && typeof parsed === "object") return parsed as Record<string, unknown>;
    } catch {
      // fall through
    }
  }
  return null;
}

export function buildConversationMessages(entries: AuditEntry[], anchorId: string): ConvMessage[] {
  if (!entries.length) return [];
  const sorted = [...entries].toSorted((a, b) => {
    const at = a.timestamp ? new Date(a.timestamp).getTime() : 0;
    const bt = b.timestamp ? new Date(b.timestamp).getTime() : 0;
    return at - bt;
  });

  const messages: ConvMessage[] = [];
  for (const entry of sorted) {
    const isAnchor = entry.id === anchorId;
    const ts = entry.timestamp;
    const req = getRequestBody(entry);
    const res = getResponseBody(entry);
    const meta = buildEntryAgentMeta(entry);

    if (req) {
      const instructions = req.instructions;
      if (typeof instructions === "string" && instructions.trim()) {
        pushMessage(messages, {
          entryId: entry.id,
          role: "system",
          text: instructions.trim(),
          timestamp: ts,
          isAnchor,
          toolCalls: [],
        });
      }

      const reqMessages = Array.isArray(req.messages) ? (req.messages as unknown[]) : [];
      for (const raw of reqMessages) {
        if (!raw || typeof raw !== "object") continue;
        const m = raw as Record<string, unknown>;
        const role = String(m.role ?? "user").toLowerCase();
        const text = extractText(m.content);
        if (role === "tool") {
          if (text) {
            pushMessage(messages, {
              entryId: entry.id,
              role: "function_result",
              text,
              timestamp: ts,
              isAnchor,
              toolCalls: [],
              toolName: typeof m.name === "string" ? m.name : undefined,
            });
          }
          continue;
        }
        if (role === "assistant") {
          const toolCalls = extractToolCalls(m.tool_calls);
          if (text || toolCalls.length) {
            pushMessage(messages, {
              entryId: entry.id,
              role: "assistant",
              text,
              timestamp: ts,
              isAnchor,
              toolCalls,
            });
          }
          continue;
        }
        if (text) {
          pushMessage(messages, {
            entryId: entry.id,
            role: (role === "system" ? "system" : role === "user" ? "user" : "user") as ConvRole,
            text,
            timestamp: ts,
            isAnchor,
            toolCalls: [],
          });
        }
      }
    }

    if (res) {
      const choices = Array.isArray(res.choices) ? (res.choices as unknown[]) : [];
      const first = choices[0] as Record<string, unknown> | undefined;
      if (first && typeof first === "object") {
        const msg = first.message as Record<string, unknown> | undefined;
        if (msg) {
          const role = String(msg.role ?? "assistant").toLowerCase();
          const text = extractText(msg.content);
          const rawReasoning = msg.reasoning_content ?? msg.reasoning;
          const reasoning = typeof rawReasoning === "string" ? rawReasoning : undefined;
          const toolCalls = extractToolCalls(msg.tool_calls);
          if (text || toolCalls.length || reasoning) {
            pushMessage(messages, {
              entryId: entry.id,
              role: (role === "assistant" ? "assistant" : "assistant") as ConvRole,
              text,
              reasoning,
              timestamp: ts,
              isAnchor,
              toolCalls,
              agentMeta: meta,
            });
          }
        }
      }
    }

    const errorMessage = entry.data?.error_message;
    if (errorMessage && typeof errorMessage === "string" && errorMessage.trim()) {
      pushMessage(messages, {
        entryId: entry.id,
        role: "error",
        text: errorMessage,
        timestamp: ts,
        isAnchor,
        toolCalls: [],
      });
    }
  }

  return messages;
}

export function buildConversationSummary(entries: AuditEntry[]): ConvSummary {
  let inputTokens = 0;
  let outputTokens = 0;
  let totalTokens = 0;
  let cost: number | null = null;
  let cacheHits = 0;
  let errors = 0;
  let okCount = 0;
  let streamingCount = 0;
  let totalDurationMs = 0;

  const providerSet = new Set<string>();
  const modelSet = new Set<string>();

  for (const entry of entries) {
    const meta = buildEntryAgentMeta(entry);
    inputTokens += meta.inputTokens;
    outputTokens += meta.outputTokens;
    totalTokens += meta.totalTokens;
    if (meta.cost !== null) cost = (cost ?? 0) + meta.cost;
    if (meta.cacheHit) cacheHits += 1;
    if (meta.streaming) streamingCount += 1;
    totalDurationMs += meta.durationMs;
    const code = entry.status_code ?? 0;
    if (code >= 400) errors += 1;
    else if (code >= 200) okCount += 1;
    if (meta.provider) providerSet.add(meta.provider);
    if (meta.model) modelSet.add(meta.model);
  }

  return {
    entryCount: entries.length,
    callCount: entries.length,
    inputTokens,
    outputTokens,
    totalTokens,
    cost,
    cacheHits,
    errors,
    okCount,
    streamingCount,
    totalDurationMs,
    totalDurationLabel: durationLabel(totalDurationMs),
    providers: Array.from(providerSet).join(", "),
    models: Array.from(modelSet).join(", "),
  };
}

export function buildConversationTimeline(messages: ConvMessage[]): ConvTimelineStep[] {
  return messages.map((m, idx) => ({
    uid: m.uid || `step-${idx}`,
    kind:
      m.role === "user" || m.role === "assistant" || m.role === "tool" || m.role === "error" || m.role === "system"
        ? (m.role as ConvTimelineStep["kind"])
        : "tool",
    label: m.roleLabel,
    sub: m.toolCalls.length > 0 ? `${m.toolCalls.length} tool calls` : undefined,
  }));
}

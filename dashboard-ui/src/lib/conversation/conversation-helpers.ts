/**
 * Port of internal/admin/dashboard/static/js/modules/conversation-helpers.js.
 *
 * All functions are pure with the exception of
 * renderBodyWithConversationHighlights, which composes HTML for the audit
 * conversation drawer. Behavior is preserved bit-for-bit so the existing
 * Vitest fixtures (legacy *.test.cjs converted in test/) continue to
 * pass against this implementation.
 *
 * The HTML output is built from JSON.stringify-derived text that is then
 * HTML-escaped via escapeHTML before being injected into a fixed set of
 * <span> wrappers. The audit drawer is the only consumer.
 */

const SECTION_KEYS = new Set([
  "instructions",
  "messages",
  "input",
  "previous_response_id",
  "choices",
  "output",
]);

export function extractText(content: unknown): string {
  if (content === null || content === undefined) {
    return "";
  }
  if (typeof content === "string") {
    return content.trim();
  }
  if (Array.isArray(content)) {
    const parts = content
      .map((part) => {
        if (typeof part === "string") {
          return part;
        }
        if (!part || typeof part !== "object") {
          return "";
        }
        const obj = part as Record<string, unknown>;
        if (typeof obj.text === "string") {
          return obj.text;
        }
        if (typeof obj.output_text === "string") {
          return obj.output_text;
        }
        return "";
      })
      .filter(Boolean);
    return parts.join("\n").trim();
  }
  if (typeof content === "object") {
    const obj = content as Record<string, unknown>;
    if (typeof obj.text === "string") {
      return obj.text.trim();
    }
    try {
      return JSON.stringify(content, null, 2);
    } catch {
      return "";
    }
  }
  return String(content).trim();
}

export function extractTextSegments(content: unknown): string[] {
  if (content === null || content === undefined) {
    return [];
  }
  if (typeof content === "string") {
    return content ? [content] : [];
  }
  if (Array.isArray(content)) {
    return content.flatMap((part) => {
      if (typeof part === "string") {
        return part ? [part] : [];
      }
      if (!part || typeof part !== "object") {
        return [];
      }
      const obj = part as Record<string, unknown>;
      if (typeof obj.text === "string") {
        return obj.text ? [obj.text] : [];
      }
      if (typeof obj.output_text === "string") {
        return obj.output_text ? [obj.output_text] : [];
      }
      return [];
    });
  }
  if (typeof content === "object") {
    const obj = content as Record<string, unknown>;
    if (typeof obj.text === "string") {
      return obj.text ? [obj.text] : [];
    }
    return [];
  }
  const text = String(content);
  return text ? [text] : [];
}

export interface ResponsesInputMessage {
  role: string;
  text: string;
}

export function extractResponsesInputMessages(input: unknown): ResponsesInputMessage[] {
  if (input === null || input === undefined) {
    return [];
  }
  if (typeof input === "string") {
    const text = input.trim();
    return text ? [{ role: "user", text }] : [];
  }
  if (!Array.isArray(input)) {
    const text = extractText(input);
    return text ? [{ role: "user", text }] : [];
  }
  const out: ResponsesInputMessage[] = [];
  for (const item of input) {
    if (!item || typeof item !== "object") {
      continue;
    }
    const obj = item as Record<string, unknown>;
    const role = String(obj.role ?? "user").toLowerCase();
    const text = extractText(obj.content);
    if (!text) {
      continue;
    }
    out.push({ role, text });
  }
  return out;
}

export function extractResponsesOutputText(item: unknown): string {
  if (!item || typeof item !== "object") {
    return "";
  }
  const obj = item as Record<string, unknown>;
  if (!Array.isArray(obj.content)) {
    return extractText(obj.content);
  }
  const parts = obj.content
    .map((part) => {
      if (!part || typeof part !== "object") {
        return "";
      }
      const partObj = part as Record<string, unknown>;
      return typeof partObj.text === "string" ? partObj.text : "";
    })
    .filter(Boolean);
  return parts.join("\n").trim();
}

export function extractRequestPromptTextSegments(body: unknown): string[] {
  if (!body || typeof body !== "object") {
    return [];
  }
  const obj = body as Record<string, unknown>;
  const segments: string[] = [];
  segments.push(...extractTextSegments(obj.instructions));

  if (Array.isArray(obj.messages)) {
    for (const message of obj.messages) {
      if (!message || typeof message !== "object") {
        continue;
      }
      const msgObj = message as Record<string, unknown>;
      segments.push(...extractTextSegments(msgObj.content));
    }
  }

  const input = obj.input;
  if (typeof input === "string") {
    segments.push(input);
  } else if (Array.isArray(input)) {
    for (const item of input) {
      if (!item || typeof item !== "object") {
        continue;
      }
      const itemObj = item as Record<string, unknown>;
      segments.push(...extractTextSegments(itemObj.content));
      if (typeof itemObj.text === "string") {
        segments.push(itemObj.text);
      }
    }
  } else if (input && typeof input === "object") {
    const inputObj = input as Record<string, unknown>;
    segments.push(...extractTextSegments(inputObj.content));
    if (typeof inputObj.text === "string") {
      segments.push(inputObj.text);
    }
  }

  return segments.map((segment) => String(segment ?? "")).filter((s) => s.length > 0);
}

function tryParseJSON(value: unknown): unknown {
  if (typeof value !== "string") {
    return null;
  }
  try {
    return JSON.parse(value);
  } catch {
    return null;
  }
}

function findNestedErrorMessage(value: unknown, depth = 0): string {
  const visited = new Set<object>();
  const stack: unknown[] = [value];

  while (stack.length > 0) {
    const current = stack.shift();
    if (!current || typeof current !== "object") {
      continue;
    }
    if (visited.has(current as object)) {
      continue;
    }
    visited.add(current as object);

    if (Array.isArray(current)) {
      for (const item of current) {
        stack.push(item);
      }
      continue;
    }

    const obj = current as Record<string, unknown>;
    const error = obj.error;
    if (typeof error === "string" && error.trim()) {
      return normalizeErrorMessageText(error, depth);
    }
    if (error && typeof error === "object") {
      const errObj = error as Record<string, unknown>;
      if (typeof errObj.message === "string" && errObj.message.trim()) {
        return normalizeErrorMessageText(errObj.message, depth);
      }
      stack.push(error);
    }

    if (typeof obj.message === "string" && obj.message.trim()) {
      if (
        obj.error !== undefined ||
        obj.code !== undefined ||
        obj.status !== undefined ||
        obj.type !== undefined
      ) {
        return normalizeErrorMessageText(obj.message, depth);
      }
    }

    for (const key of Object.keys(obj)) {
      if (key === "error") {
        continue;
      }
      stack.push(obj[key]);
    }
  }

  return "";
}

function normalizeErrorMessageText(text: unknown, depth: number): string {
  const trimmed = String(text ?? "").trim();
  if (!trimmed) {
    return "";
  }
  if (depth >= 4) {
    return trimmed;
  }
  const parsed = tryParseJSON(trimmed);
  if (!parsed || typeof parsed !== "object") {
    return trimmed;
  }
  const nested = findNestedErrorMessage(parsed, depth + 1);
  if (nested) {
    return nested;
  }
  const fallback = extractText(parsed);
  return fallback || trimmed;
}

export interface AuditEntry {
  path?: string | null;
  data?: {
    request_body?: unknown;
    response_body?: unknown;
    error_message?: unknown;
  } | null;
}

export function extractConversationErrorMessage(entry: AuditEntry | null | undefined): string {
  if (!entry || !entry.data) {
    return "";
  }
  const responseBodyMessage = findNestedErrorMessage(entry.data.response_body);
  if (responseBodyMessage) {
    return responseBodyMessage;
  }
  const rawError = entry.data.error_message;
  if (rawError === null || rawError === undefined) {
    return "";
  }
  if (typeof rawError === "string") {
    const trimmed = rawError.trim();
    if (!trimmed) {
      return "";
    }
    const parsed = tryParseJSON(trimmed);
    const parsedMessage = findNestedErrorMessage(parsed);
    if (parsedMessage) {
      return parsedMessage;
    }
    return trimmed;
  }
  const structuredMessage = findNestedErrorMessage(rawError);
  if (structuredMessage) {
    return structuredMessage;
  }
  return extractText(rawError);
}

export function looksLikeResponsesOutput(output: unknown): boolean {
  if (!Array.isArray(output)) {
    return false;
  }
  return output.some((item) => {
    if (!item || typeof item !== "object") {
      return false;
    }
    const obj = item as Record<string, unknown>;
    if (
      obj.type === "message" ||
      obj.role === "assistant" ||
      obj.role === "user" ||
      obj.role === "system"
    ) {
      return true;
    }
    if (!Array.isArray(obj.content)) {
      return false;
    }
    return obj.content.some((part) => {
      if (!part || typeof part !== "object") {
        return false;
      }
      const partObj = part as Record<string, unknown>;
      return (
        typeof partObj.text === "string" ||
        partObj.type === "output_text" ||
        partObj.type === "input_text"
      );
    });
  });
}

export function isConversationExcludedPath(path: string | null | undefined): boolean {
  if (!path) {
    return false;
  }
  const p = String(path).toLowerCase();
  return (
    p === "/v1/embeddings" ||
    p === "/v1/embeddings/" ||
    p.startsWith("/v1/embeddings?") ||
    p.startsWith("/v1/embeddings/")
  );
}

export function isConversationalPath(path: string | null | undefined): boolean {
  if (!path) {
    return false;
  }
  const p = String(path).toLowerCase();
  return (
    p === "/v1/chat/completions" ||
    p === "/v1/chat/completions/" ||
    p.startsWith("/v1/chat/completions?") ||
    p.startsWith("/v1/chat/completions/") ||
    p === "/v1/responses" ||
    p === "/v1/responses/" ||
    p.startsWith("/v1/responses?") ||
    p.startsWith("/v1/responses/")
  );
}

export function hasConversationPayload(entry: AuditEntry | null | undefined): boolean {
  const requestBody = entry?.data?.request_body as Record<string, unknown> | null | undefined;
  const responseBody = entry?.data?.response_body as Record<string, unknown> | null | undefined;

  const reqHas =
    requestBody &&
    (Array.isArray(requestBody.messages) ||
      requestBody.input !== undefined ||
      typeof requestBody.instructions === "string" ||
      typeof requestBody.previous_response_id === "string");

  const respHas =
    responseBody &&
    (Array.isArray(responseBody.choices) || looksLikeResponsesOutput(responseBody.output));

  return Boolean(reqHas || respHas);
}

export function canShowConversation(entry: AuditEntry | null | undefined): boolean {
  if (!entry) {
    return false;
  }
  if (isConversationExcludedPath(entry.path)) {
    return false;
  }
  return isConversationalPath(entry.path) || hasConversationPayload(entry);
}

function jsonBracketDelta(text: string): number {
  let depth = 0;
  let inString = false;
  let escaped = false;
  const src = String(text ?? "");
  for (let i = 0; i < src.length; i++) {
    const ch = src[i];
    if (inString) {
      if (escaped) {
        escaped = false;
        continue;
      }
      if (ch === "\\") {
        escaped = true;
        continue;
      }
      if (ch === '"') {
        inString = false;
      }
      continue;
    }
    if (ch === '"') {
      inString = true;
      continue;
    }
    if (ch === "{" || ch === "[") {
      depth++;
      continue;
    }
    if (ch === "}" || ch === "]") {
      depth--;
    }
  }
  return depth;
}

function findConversationSectionEnd(
  lines: string[],
  startIdx: number,
  valuePart: string,
): number {
  const value = String(valuePart ?? "").trim();
  if (!(value.startsWith("{") || value.startsWith("["))) {
    return startIdx;
  }
  let depth = jsonBracketDelta(valuePart);
  let idx = startIdx;
  while (depth > 0 && idx + 1 < lines.length) {
    idx++;
    depth += jsonBracketDelta(lines[idx] ?? "");
  }
  return idx;
}

function conversationHighlightRoleClass(key: string): string {
  if (key === "instructions") {
    return "conversation-system";
  }
  if (key === "messages" || key === "input" || key === "previous_response_id") {
    return "conversation-user";
  }
  return "conversation-assistant";
}

export function escapeHTML(value: unknown): string {
  return String(value === null || value === undefined ? "" : value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function jsonStringContent(value: string): string {
  try {
    return JSON.stringify(String(value)).slice(1, -1);
  } catch {
    return "";
  }
}

export interface PromptCacheHighlight {
  characters: number;
  segments: string[];
}

interface PromptCacheHighlightState {
  remaining: number;
  segments: string[];
  segmentIndex: number;
}

function createPromptCacheHighlightState(
  highlight: PromptCacheHighlight | null | undefined,
): PromptCacheHighlightState | null {
  if (!highlight || typeof highlight !== "object") {
    return null;
  }
  const characters = Number(highlight.characters ?? 0);
  if (!Number.isFinite(characters) || characters <= 0) {
    return null;
  }
  const segments = Array.isArray(highlight.segments)
    ? highlight.segments.map((s) => String(s ?? "")).filter(Boolean)
    : [];
  if (segments.length === 0) {
    return null;
  }
  return {
    remaining: Math.floor(characters),
    segments,
    segmentIndex: 0,
  };
}

function renderLineWithPromptCacheHighlight(
  line: string,
  state: PromptCacheHighlightState | null,
): string {
  if (!state || state.remaining <= 0 || state.segmentIndex >= state.segments.length) {
    return escapeHTML(line);
  }

  let rendered = "";
  let cursor = 0;
  let searchFrom = 0;

  while (state.remaining > 0 && state.segmentIndex < state.segments.length) {
    const segment = state.segments[state.segmentIndex] ?? "";
    const encodedSegment = jsonStringContent(segment);
    if (!encodedSegment) {
      state.segmentIndex++;
      continue;
    }
    const idx = line.indexOf(encodedSegment, searchFrom);
    if (idx < 0) {
      break;
    }
    const highlightedChars = Math.min(state.remaining, segment.length);
    const encodedHighlight = jsonStringContent(segment.slice(0, highlightedChars));
    if (!encodedHighlight) {
      state.segmentIndex++;
      continue;
    }
    rendered += escapeHTML(line.slice(cursor, idx));
    rendered +=
      '<span class="audit-prompt-cache-highlight">' +
      escapeHTML(encodedHighlight) +
      "</span>";
    cursor = idx + encodedHighlight.length;
    searchFrom = idx + encodedSegment.length;
    state.remaining -= highlightedChars;
    if (highlightedChars >= segment.length) {
      state.segmentIndex++;
      continue;
    }
    break;
  }

  if (!rendered) {
    return escapeHTML(line);
  }
  return rendered + escapeHTML(line.slice(cursor));
}

export interface RenderBodyDeps {
  formatJSON?: (value: unknown) => string;
  canShowConversation?: (entry: AuditEntry | null | undefined) => boolean;
  promptCacheHighlight?: PromptCacheHighlight | null;
}

const SECTION_HEADER_PATTERN = /^(\s*)"([^"]+)"\s*:\s*(.*)$/;

export function renderBodyWithConversationHighlights(
  entry: AuditEntry | null | undefined,
  value: unknown,
  deps: RenderBodyDeps = {},
): string {
  const formatJSON =
    typeof deps.formatJSON === "function" ? deps.formatJSON : (v: unknown) => String(v);
  const canShow =
    typeof deps.canShowConversation === "function"
      ? deps.canShowConversation
      : () => false;
  const promptCacheState = createPromptCacheHighlightState(deps.promptCacheHighlight);

  const raw = formatJSON(value);
  if (!raw || raw === "Not captured") {
    return escapeHTML(raw);
  }

  const showConversation = canShow(entry);
  if (!showConversation) {
    return raw
      .split("\n")
      .map((line) => renderLineWithPromptCacheHighlight(line, promptCacheState))
      .join("\n");
  }

  const lines = raw.split("\n");
  const rendered: string[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i] ?? "";
    const match = line.match(SECTION_HEADER_PATTERN);
    if (match && SECTION_KEYS.has(match[2] ?? "")) {
      const key = match[2] ?? "";
      const valuePart = match[3] ?? "";
      const end = findConversationSectionEnd(lines, i, valuePart);
      const roleClass = conversationHighlightRoleClass(key);
      const block = lines
        .slice(i, end + 1)
        .map((l) => renderLineWithPromptCacheHighlight(l, promptCacheState))
        .join("\n");
      rendered.push(
        '<span class="conversation-body-highlight ' +
          roleClass +
          '" data-conversation-trigger="1">' +
          block +
          "</span>",
      );
      i = end + 1;
      continue;
    }
    rendered.push(renderLineWithPromptCacheHighlight(line, promptCacheState));
    i++;
  }

  return rendered.join("\n");
}

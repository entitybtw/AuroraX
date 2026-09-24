// Executes exactly one upstream request in a fresh Bun process.
//
// A single process must only issue one request: reusing the connection makes
// subsequent free-tier requests fail, so the adapter spawns this script per
// request and this script exits right after the response is written.
//
// Tool injection is scoped by AURORA_SIDECAR_INJECT_TOOLS (set by adapter
// from overrides + provider type). When enabled and the body has no tools,
// the schema is loaded from AURORA_SIDECAR_TOOLS_PATH (extension-supplied)
// and falls back to the bundled default-tools.json.
// Streaming aggregation, identity header validation and 403/429 retry are
// kept so free-tier upstreams stay compatible.
import bundledTools from "./default-tools.json" with { type: "json" };

const TOOLS_PATH = process.env.AURORA_SIDECAR_TOOLS_PATH ?? "";

let tools = bundledTools;
if (TOOLS_PATH) {
  try {
    const text = await Bun.file(TOOLS_PATH).text();
    const parsed = JSON.parse(text);
    if (Array.isArray(parsed) && parsed.length > 0) {
      tools = parsed;
    }
  } catch {
    // keep bundled schema when the extension path is missing/invalid
  }
}

// AURORA_SIDECAR_UPSTREAM_URL is set by adapter.js on spawn. Never fall back
// to AURORA_SIDECAR_BASE_URL — that is the sidecar's own bind address.
const UPSTREAM = (process.env.AURORA_SIDECAR_UPSTREAM_URL ?? "").trim();
const USER_AGENT = process.env.AURORA_SIDECAR_USER_AGENT ?? "";
const DEFAULT_AUTH =
  process.env.AURORA_SIDECAR_DEFAULT_AUTH ?? "Bearer public";
const INJECT_TOOLS =
  (process.env.AURORA_SIDECAR_INJECT_TOOLS ?? "true") !== "false";
const MAX_ATTEMPTS = Number(process.env.AURORA_SIDECAR_MAX_ATTEMPTS ?? "4");
const RETRY_DELAY_MS = Number(
  process.env.AURORA_SIDECAR_RETRY_DELAY_MS ?? "750",
);
const PATH_TEMPLATE = (
  process.env.AURORA_SIDECAR_PATH_TEMPLATE ?? "/chat/completions"
).trim();
const RETRY_STATUSES = (
  process.env.AURORA_SIDECAR_RETRY_STATUSES ?? "403,429"
)
  .split(",")
  .map((s) => Number(s.trim()))
  .filter((n) => Number.isFinite(n) && n > 0);
let EXTRA_HEADERS = {};
try {
  EXTRA_HEADERS = JSON.parse(process.env.AURORA_SIDECAR_EXTRA_HEADERS ?? "{}");
} catch {
  EXTRA_HEADERS = {};
}
const FORCE_STREAM_RAW = process.env.AURORA_SIDECAR_FORCE_STREAM ?? "";
// Optional CONNECT proxy used to egress from a specific IP (multi-IP support).
const PROXY = (process.env.AURORA_SIDECAR_PROXY ?? "").trim();
const input = await Bun.stdin.text();

let envelope;
try {
  envelope = JSON.parse(input);
} catch {
  emitError(400, "invalid json body");
}

function emitError(status, message) {
  process.stdout.write(
    JSON.stringify({ __status: status, error: { message } }),
  );
  process.exit(0);
}

function randomId(prefix) {
  return prefix + crypto.randomUUID().replace(/-/g, "").slice(0, 26);
}

// Some free tiers validate identity headers strictly, e.g.:
//   x-opencode-session  must be "ses_" + exactly 26 hex chars
//   x-opencode-request  must be "msg_" + 26 hex chars
// Session Hub (or the caller) may supply these; use them when they match,
// otherwise generate a valid value so the request never breaks.
const SESSION_RE = /^ses_[0-9a-f]{26}$/;
const REQUEST_RE = /^msg_[0-9a-f]{26}$/;

function identityHeaders(inbound) {
  const out = {};
  // Forward any unknown inbound identity-ish headers as-is.
  for (const [k, v] of Object.entries(inbound || {})) {
    if (typeof v === "string" && v) out[k] = v;
  }
  // Normalize well-known free-tier shapes when present or when defaults apply.
  if (inbound?.["x-opencode-client"] || "x-opencode-client" in (inbound || {})) {
    // keep inbound
  }
  const session = inbound?.["x-opencode-session"];
  const request = inbound?.["x-opencode-request"];
  if (session !== undefined || request !== undefined || inbound?.["x-opencode-client"] || inbound?.["x-opencode-project"]) {
    out["x-opencode-client"] = inbound?.["x-opencode-client"] || "cli";
    out["x-opencode-project"] = inbound?.["x-opencode-project"] || "global";
    out["x-opencode-request"] =
      request && REQUEST_RE.test(request) ? request : randomId("msg_");
    out["x-opencode-session"] =
      session && SESSION_RE.test(session) ? session : randomId("ses_");
  }
  return out;
}

// Preserve the caller's intent so a non-streaming request can be re-emitted as
// a single JSON object.
const wantStream = envelope.payload?.stream === true;
const payload = envelope.payload ?? envelope;
const authorization =
  typeof envelope.authorization === "string" && envelope.authorization
    ? envelope.authorization
    : DEFAULT_AUTH;

// Free tiers often require the full tool schema and streaming, else they
// respond with FreeTierError regardless of the TLS fingerprint.
// force_stream: overrides.force_stream (via env) wins; empty = force only
// when tools are injected (legacy free-tier behaviour).
if (INJECT_TOOLS && (!Array.isArray(payload.tools) || payload.tools.length === 0)) {
  payload.tools = tools;
}
if (INJECT_TOOLS && payload.tool_choice === undefined) {
  payload.tool_choice = "auto";
}
const forceStream =
  FORCE_STREAM_RAW === "true"
    ? true
    : FORCE_STREAM_RAW === "false"
      ? false
      : INJECT_TOOLS;
if (forceStream) {
  payload.stream = true;
}

function buildHeaders() {
  const h = {
    Authorization: authorization,
    "Content-Type": "application/json",
    ...identityHeaders(envelope.headers ?? {}),
    // Preset extra headers (anthropic-beta, cookies, org ids, …).
    // Identity / Authorization above win if keys collide after string keys.
  };
  if (USER_AGENT) h["User-Agent"] = USER_AGENT;
  for (const [k, v] of Object.entries(EXTRA_HEADERS)) {
    if (typeof v === "string" && v && !(k in h)) h[k] = v;
  }
  return h;
}

if (!UPSTREAM) {
  emitError(503, "sidecar upstream not configured");
}

const pathTemplate = PATH_TEMPLATE.startsWith("/")
  ? PATH_TEMPLATE
  : "/" + PATH_TEMPLATE;
const url = UPSTREAM.replace(/\/+$/, "") + pathTemplate;
let resp;
let raw = "";
for (let attempt = 1; attempt <= MAX_ATTEMPTS; attempt++) {
  const opts = {
    method: "POST",
    headers: buildHeaders(),
    body: JSON.stringify(payload),
  };
  if (PROXY) opts.proxy = PROXY;
  resp = await fetch(url, opts);
  raw = await resp.text();
  // Transient statuses come from overrides.retry_statuses (default 403/429).
  if (!RETRY_STATUSES.includes(resp.status)) break;
  if (attempt < MAX_ATTEMPTS) {
    await new Promise((r) => setTimeout(r, RETRY_DELAY_MS * attempt));
  }
}

const upstreamStatus = resp.status;

if (!resp.ok) {
  process.stdout.write(JSON.stringify({ __status: upstreamStatus, error: { message: raw || `upstream ${upstreamStatus}` } }));
  process.exit(0);
}

// Caller wanted streaming: pass the SSE through unchanged.
if (wantStream) {
  process.stdout.write(JSON.stringify({ __status: 200, __sse: raw }));
  process.exit(0);
}

// Caller wanted a single response: aggregate SSE chunks into one JSON object.
const contentParts = [];
const reasoningParts = [];
const toolCalls = new Map(); // index -> { id, name, args }
let finish = "stop";
let mainId = "",
  created = 0,
  model = "",
  role = "assistant";
let usage = {};

for (const line of raw.split("\n")) {
  if (!line.startsWith("data:")) continue;
  const data = line.slice(5).trim();
  if (data === "[DONE]") continue;
  let chunk;
  try {
    chunk = JSON.parse(data);
  } catch {
    continue;
  }

  if (!mainId && chunk.id) mainId = chunk.id;
  if (!created && chunk.created) created = chunk.created;
  if (!model && chunk.model) model = chunk.model;

  const delta = chunk.choices && chunk.choices[0] && chunk.choices[0].delta;
  if (!delta) continue;
  if (delta.role) role = delta.role;
  if (delta.content) contentParts.push(delta.content);
  if (delta.reasoning) reasoningParts.push(delta.reasoning);

  if (delta.tool_calls) {
    for (const tc of delta.tool_calls) {
      const idx = tc.index ?? 0;
      let cur = toolCalls.get(idx) || {
        id: "",
        type: tc.type || "function",
        function: { name: "", arguments: "" },
      };
      if (tc.id) cur.id = tc.id;
      if (tc.function?.name) cur.function.name += tc.function.name;
      if (tc.function?.arguments) cur.function.arguments += tc.function.arguments;
      toolCalls.set(idx, cur);
    }
  }
  const fr = chunk.choices && chunk.choices[0] && chunk.choices[0].finish_reason;
  if (fr) finish = fr;
  if (chunk.usage) usage = chunk.usage;
}

let content = contentParts.join("");
if (!content && reasoningParts.length > 0) {
  content = reasoningParts.join("");
}

const message = { role: role || "assistant", content: content || null };
if (reasoningParts.length > 0) {
  message.reasoning_content = reasoningParts.join("");
}
if (toolCalls.size > 0) {
  message.tool_calls = Array.from(toolCalls.values());
}

const result = {
  id: mainId || randomId("chatcmpl-"),
  object: "chat.completion",
  created: created || Math.floor(Date.now() / 1000),
  model: model || "",
  choices: [{ index: 0, message, finish_reason: finish || "stop" }],
  usage,
};

process.stdout.write(JSON.stringify({ __status: 200, body: result }));

// Executes exactly one OpenCode zen request in a fresh Bun process.
//
// A single process must only issue one request: reusing the connection makes
// subsequent zen requests fail, so the adapter spawns this script per request
// and this script exits right after the response is written.
//
// The free tier only responds to streaming requests carrying the full OpenCode
// tool schema, so we always stream upstream. When the caller asked for a
// non-streaming response, the SSE chunks are aggregated back into a single
// JSON chat.completion object.
import tools from "./default-tools.json" with { type: "json" };

const UPSTREAM = process.env.OPENCODE_ZEN_BASE_URL ?? "https://opencode.ai/zen/v1";
const USER_AGENT =
  process.env.OPENCODE_ZEN_USER_AGENT ??
  "opencode/1.18.31 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14";
const INJECT_TOOLS = (process.env.OPENCODE_ZEN_INJECT_TOOLS ?? "true") !== "false";
const input = await Bun.stdin.text();

let envelope;
try {
  envelope = JSON.parse(input);
} catch {
  process.stdout.write(JSON.stringify({ error: { message: "invalid json body" } }));
  process.exit(1);
}

const payload = envelope.payload ?? envelope;
const authorization =
  typeof envelope.authorization === "string" && envelope.authorization
    ? envelope.authorization
    : "Bearer public";

// Preserve the caller's intent so a non-streaming request can be re-emitted as
// a single JSON object.
const wantStream = payload.stream === true;

// The zen free tier requires the full OpenCode tool schema and streams, else it
// responds with FreeTierError regardless of the TLS fingerprint.
if (INJECT_TOOLS && (!Array.isArray(payload.tools) || payload.tools.length === 0)) {
  payload.tools = tools;
}
if (payload.tool_choice === undefined) {
  payload.tool_choice = "auto";
}
payload.stream = true;

const randomId = (prefix) =>
  prefix + crypto.randomUUID().replace(/-/g, "").slice(0, 26);

const headers = {
  Authorization: authorization,
  "Content-Type": "application/json",
  "User-Agent": USER_AGENT,
  "x-opencode-client": "cli",
  "x-opencode-project": "9a15059a80937175227c853c8d7c79984cdbc2b6",
  "x-opencode-request": randomId("msg_"),
  "x-opencode-session": randomId("ses_"),
};

const resp = await fetch(UPSTREAM.replace(/\/+$/, "") + "/chat/completions", {
  method: "POST",
  headers,
  body: JSON.stringify(payload),
});

const contentType = (resp.headers.get("content-type") || "").toLowerCase();
const raw = await resp.text();

// Non-2xx: pass through the error body as-is.
if (!resp.ok) {
  process.stdout.write(raw);
  process.exit(0);
}

// Caller wanted streaming: pass the SSE through unchanged.
if (wantStream) {
  process.stdout.write(raw);
  process.exit(0);
}

// Caller wanted a single response: aggregate SSE chunks into one JSON object.
const contentParts = [];
const toolCalls = new Map(); // index -> { id, name, args }
let finish = "stop";
let mainId = "", created = 0, model = "", role = "assistant";
let usage = {};

for (const line of raw.split("\n")) {
  if (!line.startsWith("data:")) continue;
  const data = line.slice(5).trim();
  if (data === "[DONE]") continue;
  let chunk;
  try { chunk = JSON.parse(data); } catch { continue; }

  if (!mainId && chunk.id) mainId = chunk.id;
  if (!created && chunk.created) created = chunk.created;
  if (!model && chunk.model) model = chunk.model;

  const delta = chunk.choices && chunk.choices[0] && chunk.choices[0].delta;
  if (!delta) continue;
  if (delta.role) role = delta.role;
  if (delta.content) contentParts.push(delta.content);

  // Streamed tool calls arrive in fragments keyed by an index.
  if (delta.tool_calls) {
    for (const tc of delta.tool_calls) {
      const idx = tc.index ?? 0;
      let cur = toolCalls.get(idx) || { id: "", type: tc.type || "function", function: { name: "", arguments: "" } };
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

const message = { role: "assistant", content: contentParts.join("") || null };
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

process.stdout.write(JSON.stringify(result));

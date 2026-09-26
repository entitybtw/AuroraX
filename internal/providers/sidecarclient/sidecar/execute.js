// Single upstream request executor (in-process, extension-driven).
//
// Extracted from one-shot.js so the adapter can run a request inline instead
// of spawning a fresh Bun process per request. A fresh connection is still
// enforced (Connection: close) because reusing the upstream socket makes some
// free tiers reject subsequent requests.
//
// Tool injection is scoped by AURORA_SIDECAR_INJECT_TOOLS (set by adapter
// from overrides + provider type). When enabled and the body has no tools,
// the schema is loaded from AURORA_SIDECAR_TOOLS_PATH (extension-supplied)
// and falls back to the bundled default-tools.json. The schema is cached in
// memory and only re-read when the file mtime changes.
import bundledTools from "./default-tools.json" with { type: "json" };

const emit = (obj) => JSON.stringify(obj);

function randomId(prefix) {
  return prefix + crypto.randomUUID().replace(/-/g, "").slice(0, 26);
}

// Forward the identity headers the adapter selected (names come from the
// applied extension's forward_headers / env). No client-specific names or
// value shapes are hardcoded in the core.
function identityHeaders(inbound) {
  const out = {};
  for (const [k, v] of Object.entries(inbound || {})) {
    if (typeof v === "string" && v) out[k] = v;
  }
  return out;
}

// Cached tools schema: { mtimeMs, tools }. Re-read only when the file changes.
let toolsCache = { mtimeMs: -1, path: null, tools: bundledTools };

function loadTools(toolsPath) {
  if (!toolsPath) return bundledTools;
  if (toolsCache.path === toolsPath) {
    try {
      const st = Bun.statSync(toolsPath);
      if (st && st.mtimeMs === toolsCache.mtimeMs) return toolsCache.tools;
    } catch {
      return toolsCache.tools;
    }
  }
  try {
    const st = Bun.statSync(toolsPath);
    const parsed = JSON.parse(require("node:fs").readFileSync(toolsPath, "utf8"));
    if (Array.isArray(parsed) && parsed.length > 0) {
      toolsCache = {
        mtimeMs: st ? st.mtimeMs : -1,
        path: toolsPath,
        tools: parsed,
      };
      return parsed;
    }
  } catch {
    // keep previous/bundled schema
  }
  return toolsCache.tools;
}

// executeRequest runs one upstream chat completions call.
//
// cfg (all values come from the adapter / environment):
//   upstream, userAgent, defaultAuth, injectTools, maxAttempts, retryDelayMs,
//   pathTemplate, retryStatuses, extraHeaders, forceStream, proxy, toolsPath
//
// envelope: { authorization?, headers?, payload }
//
// Returns a plain object envelope: { __status, __sse? , body?, error? }.
export async function executeRequest(envelope, cfg) {
  const payload = envelope.payload ?? envelope;
  const clientWantsStream = payload.stream === true;
  const authorization =
    typeof envelope.authorization === "string" && envelope.authorization
      ? envelope.authorization
      : cfg.defaultAuth;

  const tools = cfg.injectTools ? loadTools(cfg.toolsPath) : null;
  if (cfg.injectTools && tools) {
    // Merge instead of replace: clients (chat UIs, agent runners) may ship
    // their own tools, and the upstream gate still needs the extension's
    // schema (e.g. bash/read) present alongside them. Previously a non-empty
    // client tools array suppressed injection entirely and the upstream
    // rejected the request as a non-official client.
    const existing = Array.isArray(payload.tools) ? payload.tools : [];
    const seen = new Set();
    for (const t of existing) {
      const name = t && (t.function?.name || t.name);
      if (name) seen.add(name);
    }
    const missing = tools.filter((t) => {
      const name = t && (t.function?.name || t.name);
      return name && !seen.has(name);
    });
    payload.tools = missing.length > 0 ? [...existing, ...missing] : existing;
  }
  if (cfg.injectTools && payload.tool_choice === undefined) {
    payload.tool_choice = "auto";
  }
  const forceStream =
    cfg.forceStream === true || cfg.forceStream === false
      ? cfg.forceStream
      : cfg.injectTools;
  if (forceStream) {
    payload.stream = true;
  }

  function buildHeaders() {
    const h = {
      Authorization: authorization,
      "Content-Type": "application/json",
      Connection: "close",
      ...identityHeaders(envelope.headers ?? {}),
    };
    if (cfg.userAgent) h["User-Agent"] = cfg.userAgent;
    for (const [k, v] of Object.entries(cfg.extraHeaders ?? {})) {
      if (typeof v === "string" && v && !(k in h)) h[k] = v;
    }
    return h;
  }

  if (!cfg.upstream) {
    return { __status: 503, error: { message: "sidecar upstream not configured" } };
  }

  const pathTemplate = cfg.pathTemplate.startsWith("/")
    ? cfg.pathTemplate
    : "/" + cfg.pathTemplate;
  const url = cfg.upstream.replace(/\/+$/, "") + pathTemplate;
  const wantStream = clientWantsStream;

  let resp;
  let raw = "";
  for (let attempt = 1; attempt <= cfg.maxAttempts; attempt++) {
    const opts = {
      method: "POST",
      headers: buildHeaders(),
      body: JSON.stringify(payload),
    };
    if (cfg.proxy) opts.proxy = cfg.proxy;
    resp = await fetch(url, opts);
    raw = await resp.text();
    if (!cfg.retryStatuses.includes(resp.status)) break;
    if (attempt < cfg.maxAttempts) {
      await new Promise((r) => setTimeout(r, cfg.retryDelayMs * attempt));
    }
  }

  const upstreamStatus = resp.status;
  if (!resp.ok) {
    return {
      __status: upstreamStatus,
      error: { message: raw || `upstream ${upstreamStatus}` },
    };
  }

  if (wantStream) {
    return { __status: 200, __sse: raw };
  }

  return { __status: 200, body: aggregateSse(raw) };
}

// aggregateSse folds an SSE stream into one chat.completion JSON object.
function aggregateSse(raw) {
  const contentParts = [];
  const reasoningParts = [];
  const toolCalls = new Map();
  let finish = "stop";
  let mainId = "";
  let created = 0;
  let model = "";
  let role = "assistant";
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

  return {
    id: mainId || randomId("chatcmpl-"),
    object: "chat.completion",
    created: created || Math.floor(Date.now() / 1000),
    model: model || "",
    choices: [{ index: 0, message, finish_reason: finish || "stop" }],
    usage,
  };
}

export { randomId, identityHeaders };

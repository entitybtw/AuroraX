// Generic TLS-fingerprint proxy sidecar (extension-driven).
//
// Upstream traffic can be routed through this sidecar when an extension
// supplies the base URL, User-Agent, auth and tool scope. It exists because
// some upstreams fingerprint the TLS handshake and only accept Bun's
// BoringSSL ClientHello (plus the full upstream tool schema), which a Go
// binary cannot reproduce. Credentials are forwarded untouched.
//
// Requests run inline in this process via execute.js (no per-request Bun
// spawn). Streaming responses are passed through live: the upstream SSE body
// is piped to the client as it arrives, so many concurrent clients each see
// immediate time-to-first-byte instead of waiting for a fully buffered
// generation (which made parallel sessions hit their own timeouts). Every
// request runs independently — there is no shared queue or per-session
// serialization. Set AURORA_SIDECAR_ISOLATED=true to spawn a fresh Bun
// process per request instead (legacy; buffered, only needed if an upstream
// rejects a reused process even with Connection: close).
//
// Multi-IP: requests rotate over every known CONNECT proxy (one per source
// address), and the caller may send `x-aurora-bind-ip` as a preference for
// where to start. The list is the entrypoint's AURORA_SIDECAR_BIND_PROXIES
// plus the gateway's live file (AURORA_SIDECAR_BIND_PROXIES_FILE), which
// follows the providers' bind_ips as they are edited, so addresses join and
// leave the rotation without a restart.
//
// Provider scoping: the caller may send `x-aurora-provider-type`. Tool
// injection and identity headers are only applied when that type is listed
// in AURORA_SIDECAR_INJECT_TYPES (comma-separated; empty = all types).
//
//
//	AURORA_SIDECAR_UPSTREAM_URL   upstream base URL     (overrides sidecar-overrides.base_url)
//	AURORA_SIDECAR_HOST           bind host             (default 127.0.0.1)
//	AURORA_SIDECAR_PORT           bind port             (default 8090)
//	AURORA_SIDECAR_ISOLATED       spawn per-request     (default false)
//	AURORA_SIDECAR_INJECT_TOOLS   inject tool schema    (default true)
//	AURORA_SIDECAR_INJECT_TYPES   scope tool injection  (empty = all types)
//	AURORA_SIDECAR_DEFAULT_AUTH   fallback Authorization (default "Bearer public")
//	AURORA_SIDECAR_USER_AGENT     upstream User-Agent   (default empty)
//	AURORA_SIDECAR_BIND_PROXIES   ip:port[,ip:port...]  (default empty)
//	AURORA_SIDECAR_BIND_PROXIES_FILE  live proxy list JSON (gateway-published)
//	AURORA_SIDECAR_OVERRIDES_PATH sidecar-overrides.json (mtime-cached)
//	AURORA_SIDECAR_TOOLS_PATH     extension tool schema JSON (optional)
//	AURORA_SIDECAR_UPSTREAM_HEADERS_TIMEOUT_MS  headers deadline (default 120000)
//
// Note: AURORA_SIDECAR_BASE_URL is the sidecar's own listen URL for the
// gateway — it is never used as UPSTREAM (self-proxy hang).
//
// Endpoints:
//
//	POST /v1/chat/completions   proxied (inline by default)
//	GET  /v1/models             proxied directly
//	GET  /health                liveness probe
//	GET  /proxies               configured bind proxies
//
// Overrides (sidecar-overrides.json, mtime-cached, hot-reloaded):
//
//	base_url, path_template, models_path, default_auth, user_agent,
//	inject_tools, inject_tool_types, tools_path, max_attempts,
//	retry_delay_ms, extra_headers, forward_headers, retry_statuses,
//	force_stream

import { executeRequest } from "./execute.js";
// Bun 1.3 has no Bun.statSync (it is undefined, not a function); the node:fs
// shim is what the rest of the sidecar reads files with.
import { statSync } from "node:fs";

const env = (name, fallback) => process.env[name] ?? fallback;

const ONESHOT = new URL("./one-shot.js", import.meta.url).pathname;
const ISOLATED = (process.env.AURORA_SIDECAR_ISOLATED ?? "false") === "true";
const PORT = Number(process.env.AURORA_SIDECAR_PORT ?? "8090");
const HOST = process.env.AURORA_SIDECAR_HOST ?? "127.0.0.1";
const OVERRIDES_PATH =
  process.env.AURORA_SIDECAR_OVERRIDES_PATH ?? "configs/sidecar-overrides.json";

// Static inject-type scope from env (comma-separated). Empty = all types.
const INJECT_TYPES = new Set(
  (process.env.AURORA_SIDECAR_INJECT_TYPES ?? "")
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean),
);

// Identity headers forwarded upstream. The set is configuration-driven:
// extensions list them under settings.forward_headers (persisted into
// sidecar-overrides.json) and operators can add more via
// AURORA_SIDECAR_IDENTITY_HEADERS (comma-separated). The core never
// hardcodes a specific client's header names.
const envIdentityHeaders = (process.env.AURORA_SIDECAR_IDENTITY_HEADERS ?? "")
  .split(",")
  .map((s) => s.trim().toLowerCase())
  .filter(Boolean);

let overridesCache = {
  mtimeMs: -1,
  baseUrl: "",
  pathTemplate: "/chat/completions",
  modelsPath: "/models",
  injectToolTypes: [],
  injectTools: null,
  toolsPath: "",
  userAgent: "",
  defaultAuth: "",
  maxAttempts: null,
  retryDelayMs: null,
  extraHeaders: {},
  forwardHeaders: [],
  retryStatuses: [403, 429],
  forceStream: null,
};

function parseOverrides(parsed) {
  return {
    mtimeMs: -1,
    baseUrl:
      typeof parsed.base_url === "string" && parsed.base_url.trim()
        ? parsed.base_url.trim()
        : "",
    pathTemplate:
      typeof parsed.path_template === "string" && parsed.path_template.trim()
        ? parsed.path_template.trim()
        : "/chat/completions",
    modelsPath:
      typeof parsed.models_path === "string" && parsed.models_path.trim()
        ? parsed.models_path.trim()
        : "/models",
    injectToolTypes: Array.isArray(parsed.inject_tool_types)
      ? parsed.inject_tool_types
      : [],
    injectTools:
      typeof parsed.inject_tools === "boolean" ? parsed.inject_tools : null,
    toolsPath:
      typeof parsed.tools_path === "string" ? parsed.tools_path : "",
    userAgent:
      typeof parsed.user_agent === "string" ? parsed.user_agent.trim() : "",
    defaultAuth:
      typeof parsed.default_auth === "string" ? parsed.default_auth : "",
    maxAttempts:
      typeof parsed.max_attempts === "number" && parsed.max_attempts >= 1
        ? parsed.max_attempts
        : null,
    retryDelayMs:
      typeof parsed.retry_delay_ms === "number" && parsed.retry_delay_ms >= 0
        ? parsed.retry_delay_ms
        : null,
    extraHeaders:
      parsed.extra_headers && typeof parsed.extra_headers === "object"
        ? parsed.extra_headers
        : {},
    forwardHeaders: Array.isArray(parsed.forward_headers)
      ? parsed.forward_headers.map((s) => String(s).toLowerCase()).filter(Boolean)
      : [],
    retryStatuses: Array.isArray(parsed.retry_statuses)
      ? parsed.retry_statuses.filter((n) => typeof n === "number")
      : [403, 429],
    forceStream:
      typeof parsed.force_stream === "boolean" ? parsed.force_stream : null,
  };
}

// Load sidecar-overrides.json (mtime-cached). Falls back to env when absent.
async function loadOverridesAsync() {
  try {
    const st = statSync(OVERRIDES_PATH);
    if (!st) return overridesCache;
    if (overridesCache.mtimeMs === st.mtimeMs) {
      return overridesCache;
    }
    const text = await Bun.file(OVERRIDES_PATH).text();
    let parsed = {};
    try {
      parsed = JSON.parse(text);
    } catch {
      return overridesCache;
    }
    overridesCache = parseOverrides(parsed);
    overridesCache.mtimeMs = st.mtimeMs;
  } catch {
    // keep last good cache
  }
  return overridesCache;
}

// Sync bootstrap so UPSTREAM exists before first request; later requests
// re-read mtime so base_url changes apply without restarting the process.
function loadOverridesSync() {
  try {
    const text = require("node:fs").readFileSync(OVERRIDES_PATH, "utf8");
    const parsed = JSON.parse(text);
    overridesCache = parseOverrides(parsed);
  } catch {
    // keep defaults
  }
  return overridesCache;
}

loadOverridesSync();

// Map of local IP -> CONNECT proxy URL (http://127.0.0.1:port), seeded from
// the entrypoint's static list. The gateway publishes the live set — it
// follows every provider's bind_ips — to a JSON file, re-read whenever that
// file changes, so addresses added or removed at runtime join or leave the
// rotation without restarting anything.
const BIND_PROXIES = new Map();
const proxiesRaw = process.env.AURORA_SIDECAR_BIND_PROXIES ?? "";
for (const entry of proxiesRaw.split(",")) {
  const trimmed = entry.trim();
  if (!trimmed) continue;
  const idx = trimmed.lastIndexOf(":");
  if (idx === -1) continue;
  const ip = trimmed.slice(0, idx).trim();
  const port = trimmed.slice(idx + 1).trim();
  if (ip && port) BIND_PROXIES.set(ip, `http://127.0.0.1:${port}`);
}

const BIND_PROXIES_FILE = (
  process.env.AURORA_SIDECAR_BIND_PROXIES_FILE ??
  "/app/configs/sidecar-bind-proxies.json"
).trim();
let proxiesFileCache = { mtimeMs: -1, map: null };

function bindProxies() {
  if (!BIND_PROXIES_FILE) return BIND_PROXIES;
  try {
    const st = statSync(BIND_PROXIES_FILE);
    if (st && proxiesFileCache.mtimeMs !== st.mtimeMs) {
      const parsed = JSON.parse(
        require("node:fs").readFileSync(BIND_PROXIES_FILE, "utf8"),
      );
      const next = new Map();
      for (const entry of parsed?.proxies ?? []) {
        if (!entry || !entry.ip || !entry.port) continue;
        next.set(String(entry.ip), `http://127.0.0.1:${Number(entry.port)}`);
      }
      proxiesFileCache = { mtimeMs: st.mtimeMs, map: next };
    }
  } catch {
    // No file yet (first boot, before the gateway published a set): keep the
    // last known list and fall back to the entrypoint's environment.
  }
  return proxiesFileCache.map ?? BIND_PROXIES;
}

function resolveUpstream() {
  const explicit = process.env.AURORA_SIDECAR_UPSTREAM_URL;
  if (explicit && explicit.trim()) return explicit.trim();
  return overridesCache.baseUrl || "";
}

function scopeAllows(providerType) {
  if (INJECT_TYPES.size > 0) {
    return !providerType || INJECT_TYPES.has(providerType);
  }
  return true;
}

function overridesAllow(providerType, overrides) {
  const types = overrides.injectToolTypes;
  if (Array.isArray(types) && types.length > 0) {
    return !providerType || types.includes(providerType);
  }
  return scopeAllows(providerType);
}

function upstream(base, path) {
  if (!base) {
    throw new Error(
      "sidecar upstream not configured (AURORA_SIDECAR_UPSTREAM_URL)",
    );
  }
  return base.replace(/\/+$/, "") + path;
}

function json(data, status = 200) {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "content-type": "application/json" },
  });
}

// Build the executeRequest config from live overrides + env.
function buildCfg(overrides, opts) {
  return {
    upstream: opts.upstream,
    userAgent: opts.userAgent,
    defaultAuth: opts.defaultAuth,
    injectTools: opts.injectTools,
    maxAttempts:
      overrides.maxAttempts ??
      Number(process.env.AURORA_SIDECAR_MAX_ATTEMPTS ?? "4"),
    retryDelayMs:
      overrides.retryDelayMs ??
      Number(process.env.AURORA_SIDECAR_RETRY_DELAY_MS ?? "750"),
    pathTemplate: overrides.pathTemplate,
    retryStatuses: overrides.retryStatuses,
    extraHeaders: overrides.extraHeaders,
    forceStream: overrides.forceStream,
    proxy: opts.proxy,
    proxies: opts.proxies,
    toolsPath: opts.toolsPath,
    upstreamTimeoutMs: Number(
      process.env.AURORA_SIDECAR_UPSTREAM_HEADERS_TIMEOUT_MS ?? "120000",
    ),
  };
}

Bun.serve({
  hostname: HOST,
  port: PORT,
  // 0 disables Bun's idle connection reaper: a long aggregated (non-stream)
  // generation legitimately produces no bytes for minutes, and the gateway
  // on localhost is this server's only client — killing idle sockets there
  // only decapitated in-flight requests.
  idleTimeout: 0,
  async fetch(req) {
    const url = new URL(req.url);

    if (url.pathname === "/health") {
      return new Response("ok");
    }

    if (url.pathname === "/proxies") {
      // ?check=1 reports whether each CONNECT proxy actually carries traffic:
      // a configured address whose local proxy is gone (the entrypoint's
      // listener died, or the bridge was never started) looks identical to a
      // working one in the plain list, which is exactly the "switch IP does
      // not work" case. The probe is a CONNECT to the upstream host and
      // reports 200 without reading the tunnel.
      const check = url.searchParams.get("check") === "1";
      const entries = Array.from(bindProxies().entries());
      const upstreamURL = resolveUpstream();
      const target = (() => {
        try {
          return upstreamURL ? new URL(upstreamURL).host : "";
        } catch {
          return "";
        }
      })();
      const out = await Promise.all(
        entries.map(async ([ip, proxy]) => {
          const entry = { ip, proxy };
          if (!check || !target) return entry;
          const started = Date.now();
          try {
            const ctrl = new AbortController();
            const timer = setTimeout(() => ctrl.abort(), 5000);
            // Dial through the proxy and keep the tunnel open only long
            // enough to know the CONNECT was accepted.
            const resp = await fetch(`https://${target}/health`, {
              method: "GET",
              proxy,
              signal: ctrl.signal,
            }).catch((err) => ({ ok: false, status: 0, error: String(err) }));
            clearTimeout(timer);
            entry.probe_ms = Date.now() - started;
            entry.reachable = Boolean(resp && resp.status !== 0);
            entry.probe_status = resp?.status ?? 0;
            if (!entry.reachable) entry.error = String(resp?.error ?? "");
          } catch (err) {
            entry.reachable = false;
            entry.error = String(err);
          }
          return entry;
        }),
      );
      return json(out);
    }

    const overrides = await loadOverridesAsync();
    const UPSTREAM = resolveUpstream();

    if (!UPSTREAM) {
      return json({ error: { message: "sidecar upstream not configured" } }, 503);
    }

    const defaultAuth =
      overrides.defaultAuth ||
      process.env.AURORA_SIDECAR_DEFAULT_AUTH ||
      "Bearer public";

    if (url.pathname === "/v1/models") {
      const auth = req.headers.get("authorization") || defaultAuth;
      const ua = overrides.userAgent || process.env.AURORA_SIDECAR_USER_AGENT || "";
      // Forward the same identity headers the chat path sends. The model
      // list is served by the same edge as chat, and sending a bare request
      // there made the gateway look like a different client than the one the
      // chat calls emulate: the fetch came back empty or partial while chat
      // worked, which is the "models fetch badly" symptom. Names come from
      // the applied extension (forward_headers) plus the operator env list.
      const identity = {};
      for (const name of new Set([
        ...envIdentityHeaders,
        ...overrides.forwardHeaders,
      ])) {
        const value = req.headers.get(name);
        if (value) identity[name] = value;
      }
      const modelHeaders = {
        Authorization: auth,
        ...(ua ? { "User-Agent": ua } : {}),
        ...identity,
      };
      for (const [k, v] of Object.entries(overrides.extraHeaders ?? {})) {
        if (typeof v === "string" && v && !(k in modelHeaders)) modelHeaders[k] = v;
      }
      // Model listing is read-only and cheap to repeat: retry a transient
      // upstream hiccup instead of handing the gateway an empty list, which
      // it treats as "this provider has no models" for the whole refresh
      // interval. Rate limits are not retried (the quota belongs to the
      // address, and the gateway rotates exits itself).
      let upstreamResp;
      const attempts = Math.max(1, Math.min(3, Number(overrides.maxAttempts ?? 3)));
      for (let attempt = 1; attempt <= attempts; attempt++) {
        try {
          upstreamResp = await fetch(upstream(UPSTREAM, overrides.modelsPath), {
            headers: modelHeaders,
          });
        } catch (err) {
          if (attempt === attempts) {
            return json(
              { error: { message: `model list fetch failed: ${err}` } },
              502,
            );
          }
          await new Promise((r) => setTimeout(r, 500 * attempt));
          continue;
        }
        const transient =
          upstreamResp.status >= 500 ||
          upstreamResp.status === 408 ||
          upstreamResp.status === 0;
        if (!transient || attempt === attempts) break;
        try {
          await upstreamResp.body?.cancel();
        } catch {
          // already closed
        }
        await new Promise((r) => setTimeout(r, 500 * attempt));
      }
      return new Response(await upstreamResp.text(), {
        status: upstreamResp.status,
        headers: { "content-type": "application/json" },
      });
    }

    // Gateway ingress always posts to /v1/chat/completions; upstream path
    // may differ via overrides.path_template.
    if (!url.pathname.endsWith("/chat/completions")) {
      return json(
        { error: { message: `unsupported path: ${url.pathname}` } },
        404,
      );
    }

    const authorization = req.headers.get("authorization") || defaultAuth;
    const bindIP = (req.headers.get("x-aurora-bind-ip") || "").trim();
    const providerType = (
      req.headers.get("x-aurora-provider-type") || ""
    ).trim();
    // The gateway sends x-aurora-bind-ip with the provider's primary source
    // address. Treat it as a preference, not a pin: pinning every request to
    // one address would defeat the point of configuring several, and the
    // dial happens here in the sidecar, not in the gateway's egress registry.
    // With a single address configured the list holds one entry, so the
    // preference is still honoured.
    const proxies = bindProxies();
    // The gateway picks the exit for this attempt and hands it over in
    // x-aurora-exit-proxy. A proxy exit is a local HTTP CONNECT bridge in
    // front of the tunnel (Bun speaks HTTP proxies, not SOCKS5), and it
    // replaces the source-address rotation for this request: the upstream
    // sees the tunnel's address, not one of the local ones.
    const exitProxy = (req.headers.get("x-aurora-exit-proxy") || "").trim();
    // x-aurora-pin-exit marks the address the gateway picked for *this*
    // attempt: the sidecar then keeps its retries on that one address
    // instead of rotating behind the gateway's back. Without it the header
    // stays a preference and the sidecar may walk the list, which is what a
    // caller talking to the sidecar directly wants.
    const pinExit = (req.headers.get("x-aurora-pin-exit") || "").trim() === "1";
    const requested = bindIP ? proxies.get(bindIP) || "" : "";
    const proxy = exitProxy || requested;
    const proxyList = exitProxy
      ? [exitProxy]
      : pinExit && requested
        ? [requested]
        : Array.from(proxies.values());
    // An address the gateway asked for that has no local proxy would send
    // the request out of the default route — from some *other* address —
    // while the gateway records the attempt against `bindIP`. Refuse instead
    // of silently leaving from the wrong place: the gateway then excludes
    // this exit from the rotation and reports it, which is what makes a
    // missing proxy visible.
    if (bindIP && !requested && !exitProxy) {
      return json(
        {
          error: {
            message: `no bind proxy for exit ${bindIP}; the address is configured but not served`,
            exit: bindIP,
          },
        },
        502,
      );
    }

    const injectTools =
      overrides.injectTools !== null
        ? overrides.injectTools
        : env("AURORA_SIDECAR_INJECT_TOOLS", "true") !== "false";
    const inject = injectTools && overridesAllow(providerType, overrides);

    // Forward identity headers so operator-managed (extension) rules drive
    // the upstream identity. Names come from env + overrides.forward_headers.
    const forwardNames = new Set([
      ...envIdentityHeaders,
      ...overrides.forwardHeaders,
    ]);
    const headers = {};
    for (const name of forwardNames) {
      const value = req.headers.get(name);
      if (value) headers[name] = value;
    }
    const body = await req.text();

    const toolsPath =
      overrides.toolsPath || process.env.AURORA_SIDECAR_TOOLS_PATH || "";
    const userAgent =
      overrides.userAgent || process.env.AURORA_SIDECAR_USER_AGENT || "";

    let payload;
    try {
      payload = JSON.parse(body);
    } catch {
      return json({ error: { message: "invalid json body" } }, 400);
    }
    const envelope = { authorization, headers, payload };
    const cfg = buildCfg(overrides, {
      upstream: UPSTREAM,
      userAgent,
      defaultAuth,
      injectTools: inject,
      proxy,
      proxies: proxyList,
      toolsPath,
    });

    let out;
    let exitCode = 0;
    let stderr = "";
    if (ISOLATED) {
      const proc = Bun.spawn([process.execPath, ONESHOT], {
        stdin: "pipe",
        stdout: "pipe",
        stderr: "pipe",
        env: {
          ...process.env,
          AURORA_SIDECAR_UPSTREAM_URL: UPSTREAM,
          AURORA_SIDECAR_USER_AGENT: userAgent,
          AURORA_SIDECAR_DEFAULT_AUTH: defaultAuth,
          AURORA_SIDECAR_PROXY: proxy,
          AURORA_SIDECAR_PROXIES: proxyList.join(","),
          AURORA_SIDECAR_INJECT_TOOLS: inject ? "true" : "false",
          AURORA_SIDECAR_TOOLS_PATH: toolsPath,
          AURORA_SIDECAR_PATH_TEMPLATE: overrides.pathTemplate,
          AURORA_SIDECAR_MAX_ATTEMPTS: String(cfg.maxAttempts),
          AURORA_SIDECAR_RETRY_DELAY_MS: String(cfg.retryDelayMs),
          AURORA_SIDECAR_RETRY_STATUSES: overrides.retryStatuses.join(","),
          AURORA_SIDECAR_EXTRA_HEADERS: JSON.stringify(overrides.extraHeaders),
          AURORA_SIDECAR_FORCE_STREAM:
            overrides.forceStream === null || overrides.forceStream === undefined
              ? ""
              : overrides.forceStream
                ? "true"
                : "false",
        },
      });
      proc.stdin.write(JSON.stringify(envelope));
      proc.stdin.end();
      [out, exitCode] = await Promise.all([
        new Response(proc.stdout).text(),
        proc.exited,
      ]);
      if (exitCode !== 0 && out.trim() === "") {
        stderr = await new Response(proc.stderr).text();
      }
    } else {
      try {
        const result = await executeRequest(
          // __signal: when the client disconnects, stop the upstream request
          // instead of finishing a generation nobody will read.
          { authorization, headers, payload, __signal: req.signal },
          { ...cfg, passthrough: true },
        );
        if (result && result.__stream) {
          // Live SSE passthrough: bytes flow to each concurrent client as
          // the upstream produces them (no full-body buffering).
          return new Response(result.__stream, {
            status: 200,
            headers: {
              "content-type": "text/event-stream",
              "cache-control": "no-cache",
            },
          });
        }
        out = JSON.stringify(result);
      } catch (err) {
        return json(
          { error: { message: `sidecar execution failed: ${err}` } },
          502,
        );
      }
    }

    if (exitCode !== 0 && out.trim() === "") {
      return json(
        { error: { message: `sidecar process failed: ${stderr || exitCode}` } },
        502,
      );
    }

    // Parse the execute envelope: { __status, __sse?, __retry_after?, body?, error? }.
    let status = 200,
      bodyText = out;
    let retryAfter = "";
    try {
      const envelopeOut = JSON.parse(out);
      if (typeof envelopeOut.__status === "number") {
        status = envelopeOut.__status;
        if (typeof envelopeOut.__retry_after === "string") {
          retryAfter = envelopeOut.__retry_after.trim();
        }
        if (envelopeOut.__sse !== undefined) {
          bodyText = envelopeOut.__sse;
        } else if (envelopeOut.body !== undefined) {
          bodyText = JSON.stringify(envelopeOut.body);
        } else if (envelopeOut.error) {
          bodyText = JSON.stringify(envelopeOut.error);
        }
      }
    } catch {
      // Not an envelope (plain SSE or JSON); pass through as-is.
    }

    const isSSE = bodyText.trimStart().startsWith("data:");
    const contentType = isSSE ? "text/event-stream" : "application/json";
    const responseHeaders = { "content-type": contentType };
    if (retryAfter) responseHeaders["retry-after"] = retryAfter;
    return new Response(bodyText, {
      status: status >= 400 ? status : 200,
      headers: responseHeaders,
    });
  },
});

console.log(
  `sidecar listening on http://${HOST}:${PORT} -> ${resolveUpstream() || "(unset)"}` +
    (ISOLATED ? " [isolated]" : " [inline]") +
    (bindProxies().size ? ` (${bindProxies().size} bind proxies)` : ""),
);

// Generic TLS-fingerprint proxy sidecar (extension-driven).
//
// Upstream traffic can be routed through this sidecar when an extension
// supplies the base URL, User-Agent, auth and tool scope. It exists because
// some free tiers fingerprint the TLS handshake and only accept Bun's
// BoringSSL ClientHello (plus the full upstream tool schema), which a Go
// binary cannot reproduce. Credentials are forwarded untouched.
//
// Multi-IP: the caller may send `x-aurora-bind-ip`. When it matches an entry
// in AURORA_SIDECAR_BIND_PROXIES (comma-separated `ip:port` CONNECT proxies),
// the request is relayed through that proxy so it egresses from the chosen IP.
//
// Provider scoping: the caller may send `x-aurora-provider-type`. Tool
// injection and identity headers are only applied when that type is listed
// in AURORA_SIDECAR_INJECT_TYPES (comma-separated; empty = all types).
//
//
//	AURORA_SIDECAR_UPSTREAM_URL   upstream base URL     (overrides sidecar-overrides.base_url)
//	AURORA_SIDECAR_HOST           bind host             (default 127.0.0.1)
//	AURORA_SIDECAR_PORT           bind port             (default 8090)
//	AURORA_SIDECAR_INJECT_TOOLS   inject tool schema    (default true)
//	AURORA_SIDECAR_INJECT_TYPES   scope tool injection  (empty = all types)
//	AURORA_SIDECAR_DEFAULT_AUTH   fallback Authorization (default "Bearer public")
//	AURORA_SIDECAR_USER_AGENT     upstream User-Agent   (default empty)
//	AURORA_SIDECAR_BIND_PROXIES   ip:port[,ip:port...]  (default empty)
//	AURORA_SIDECAR_OVERRIDES_PATH sidecar-overrides.json (mtime-cached)
//	AURORA_SIDECAR_TOOLS_PATH     extension tool schema JSON (optional)
//
// Note: AURORA_SIDECAR_BASE_URL is the sidecar's own listen URL for the
// gateway — it is never used as UPSTREAM (self-proxy hang).
//
// Endpoints:
//
//	POST /v1/chat/completions   proxied via a fresh Bun process
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

const env = (name, fallback) => process.env[name] ?? fallback;

const ONESHOT = new URL("./one-shot.js", import.meta.url).pathname;
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

// Identity headers forwarded to one-shot (extension/session hub driven).
// Always includes the well-known free-tier identity set; extra names can be
// added via overrides.forward_headers.
const IDENTITY_HEADERS = [
  "x-opencode-session",
  "x-opencode-client",
  "x-opencode-request",
  "x-opencode-project",
  "x-session-id",
  "x-client-id",
];

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
    const st = Bun.statSync(OVERRIDES_PATH);
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

// Map of local IP -> CONNECT proxy URL (http://127.0.0.1:port).
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

Bun.serve({
  hostname: HOST,
  port: PORT,
  idleTimeout: 240,
  async fetch(req) {
    const url = new URL(req.url);

    if (url.pathname === "/health") {
      return new Response("ok");
    }

    if (url.pathname === "/proxies") {
      return json(
        Array.from(BIND_PROXIES.entries()).map(([ip, proxy]) => ({
          ip,
          proxy,
        })),
      );
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
      const upstreamResp = await fetch(
        upstream(UPSTREAM, overrides.modelsPath),
        {
          headers: {
            Authorization: auth,
            ...(ua ? { "User-Agent": ua } : {}),
          },
        },
      );
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
    const proxy = BIND_PROXIES.get(bindIP) || "";

    const injectTools =
      overrides.injectTools !== null
        ? overrides.injectTools
        : env("AURORA_SIDECAR_INJECT_TOOLS", "true") !== "false";
    const inject = injectTools && overridesAllow(providerType, overrides);

    // Forward identity headers so operator-managed (extension) rules drive
    // the upstream identity. one-shot validates/falls back as needed.
    const forwardNames = new Set([
      ...IDENTITY_HEADERS,
      ...overrides.forwardHeaders,
    ]);
    const headers = {};
    for (const name of forwardNames) {
      const value = req.headers.get(name);
      if (value) headers[name] = value;
    }
    const body = await req.text();

    const toolsPath =
      overrides.toolsPath ||
      process.env.AURORA_SIDECAR_TOOLS_PATH ||
      "";
    const userAgent =
      overrides.userAgent || process.env.AURORA_SIDECAR_USER_AGENT || "";

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
        AURORA_SIDECAR_INJECT_TOOLS: inject ? "true" : "false",
        AURORA_SIDECAR_TOOLS_PATH: toolsPath,
        AURORA_SIDECAR_PATH_TEMPLATE: overrides.pathTemplate,
        AURORA_SIDECAR_MAX_ATTEMPTS: String(
          overrides.maxAttempts ??
            Number(process.env.AURORA_SIDECAR_MAX_ATTEMPTS ?? "4"),
        ),
        AURORA_SIDECAR_RETRY_DELAY_MS: String(
          overrides.retryDelayMs ??
            Number(process.env.AURORA_SIDECAR_RETRY_DELAY_MS ?? "750"),
        ),
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

    proc.stdin.write(
      JSON.stringify({ authorization, headers, payload: JSON.parse(body) }),
    );
    proc.stdin.end();

    const [out, exitCode] = await Promise.all([
      new Response(proc.stdout).text(),
      proc.exited,
    ]);

    if (exitCode !== 0 && out.trim() === "") {
      const err = await new Response(proc.stderr).text();
      return json(
        {
          error: { message: `sidecar process failed: ${err || exitCode}` },
        },
        502,
      );
    }

    // Parse the one-shot envelope: { __status, __sse?, body? }.
    let status = 200,
      bodyText = out;
    try {
      const envelope = JSON.parse(out);
      if (typeof envelope.__status === "number") {
        status = envelope.__status;
        if (envelope.__sse !== undefined) {
          bodyText = envelope.__sse;
        } else if (envelope.body !== undefined) {
          bodyText = JSON.stringify(envelope.body);
        } else if (envelope.error) {
          bodyText = JSON.stringify(envelope.error);
        }
      }
    } catch {
      // Not an envelope (plain SSE or JSON); pass through as-is.
    }

    const isSSE = bodyText.trimStart().startsWith("data:");
    const contentType = isSSE ? "text/event-stream" : "application/json";
    return new Response(bodyText, {
      status: status >= 400 ? status : 200,
      headers: { "content-type": contentType },
    });
  },
});

console.log(
  `sidecar listening on http://${HOST}:${PORT} -> ${resolveUpstream() || "(unset)"}` +
    (BIND_PROXIES.size ? ` (${BIND_PROXIES.size} bind proxies)` : ""),
);

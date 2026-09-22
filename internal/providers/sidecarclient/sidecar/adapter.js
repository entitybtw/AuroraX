// free tier proxy sidecar.
//
// All zen traffic (free tier, OAuth tokens and API keys) can be routed through
// this sidecar. It exists because the zen free tier fingerprints the TLS
// handshake and only accepts Bun's BoringSSL ClientHello (plus the full
// upstream tool schema), which a Go binary cannot reproduce. OAuth access
// tokens and API keys are forwarded untouched, so paid accounts keep working.
//
// Multi-IP: the caller may send `x-aurora-bind-ip`. When it matches an entry in
// AURORA_SIDECAR_BIND_PROXIES (comma-separated `ip:port` CONNECT proxies), the
// request is relayed through that proxy so it egresses from the chosen IP.
//
// Configuration (all optional):
//
//	AURORA_SIDECAR_UPSTREAM_URL      upstream base URL          (default https://opencode.ai/zen/v1)
//	AURORA_SIDECAR_HOST            bind host                  (default 127.0.0.1)
//	AURORA_SIDECAR_PORT            bind port                  (default 8090)
//	AURORA_SIDECAR_INJECT_TOOLS    inject upstream tools      (default true)
//	AURORA_SIDECAR_DEFAULT_AUTH    fallback Authorization     (default "Bearer public")
//	AURORA_SIDECAR_USER_AGENT      upstream User-Agent        (default opencode client UA)
//	AURORA_SIDECAR_BIND_PROXIES    ip:port[,ip:port...]       (default empty)
//
// Endpoints:
//
//	POST /v1/chat/completions   proxied via a fresh Bun process
//	GET  /v1/models             proxied directly
//	GET  /health                liveness probe
//	GET  /proxies               configured bind proxies

const UPSTREAM = process.env.AURORA_SIDECAR_UPSTREAM_URL ?? "https://opencode.ai/zen/v1";
const ONESHOT = new URL("./one-shot.js", import.meta.url).pathname;
const PORT = Number(process.env.AURORA_SIDECAR_PORT ?? "8090");
const HOST = process.env.AURORA_SIDECAR_HOST ?? "127.0.0.1";
const DEFAULT_AUTH = process.env.AURORA_SIDECAR_DEFAULT_AUTH ?? "Bearer public";
const USER_AGENT =
  process.env.AURORA_SIDECAR_USER_AGENT ??
  "opencode/1.18.31 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14";

// Map of local IP -> CONNECT proxy URL (http://127.0.0.1:port).
const BIND_PROXIES = new Map();
for (const entry of (process.env.AURORA_SIDECAR_BIND_PROXIES ?? "").split(",")) {
  const trimmed = entry.trim();
  if (!trimmed) continue;
  const idx = trimmed.lastIndexOf(":");
  if (idx === -1) continue;
  const ip = trimmed.slice(0, idx).trim();
  const port = trimmed.slice(idx + 1).trim();
  if (ip && port) BIND_PROXIES.set(ip, `http://127.0.0.1:${port}`);
}

function upstream(path) {
  return UPSTREAM.replace(/\/+$/, "") + path;
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
      return json(Array.from(BIND_PROXIES.entries()).map(([ip, proxy]) => ({ ip, proxy })));
    }

    if (url.pathname === "/v1/models") {
      const auth = req.headers.get("authorization") || DEFAULT_AUTH;
      const upstreamResp = await fetch(upstream("/models"), {
        headers: { Authorization: auth, "User-Agent": USER_AGENT },
      });
      return new Response(await upstreamResp.text(), {
        status: upstreamResp.status,
        headers: { "content-type": "application/json" },
      });
    }

    if (!url.pathname.endsWith("/chat/completions")) {
      return json({ error: { message: `unsupported path: ${url.pathname}` } }, 404);
    }

    // Forward the credential from the incoming request so OAuth access tokens
    // and API keys reach the upstream unchanged. Fall back to the free-tier
    // "public" key when the caller did not supply one.
    const authorization = req.headers.get("authorization") || DEFAULT_AUTH;
    const bindIP = (req.headers.get("x-aurora-bind-ip") || "").trim();
    const proxy = BIND_PROXIES.get(bindIP) || "";
    // Forward Session Hub identity headers so operator-managed rules drive the
    // upstream identity. one-shot validates the values and falls back to its
    // own generation when they would be rejected by the free tier.
    const headers = {};
    for (const name of [
      "x-opencode-session",
      "x-opencode-client",
      "x-opencode-request",
      "x-opencode-project",
    ]) {
      const value = req.headers.get(name);
      if (value) headers[name] = value;
    }
    const body = await req.text();

    const proc = Bun.spawn([process.execPath, ONESHOT], {
      stdin: "pipe",
      stdout: "pipe",
      stderr: "pipe",
      env: {
        ...process.env,
        AURORA_SIDECAR_UPSTREAM_URL: UPSTREAM,
        AURORA_SIDECAR_USER_AGENT: USER_AGENT,
        AURORA_SIDECAR_PROXY: proxy,
      },
    });

    proc.stdin.write(JSON.stringify({ authorization, headers, payload: JSON.parse(body) }));
    proc.stdin.end();

    const [out, exitCode] = await Promise.all([
      new Response(proc.stdout).text(),
      proc.exited,
    ]);

    if (exitCode !== 0 && out.trim() === "") {
      const err = await new Response(proc.stderr).text();
      return json(
        { error: { message: `sidecar process failed: ${err || exitCode}` } },
        502,
      );
    }

    // Parse the one-shot envelope: { __status, __sse?, body? }.
    let status = 200, bodyText = out;
    try {
      const env = JSON.parse(out);
      if (typeof env.__status === "number") {
        status = env.__status;
        if (env.__sse !== undefined) {
          bodyText = env.__sse;
        } else if (env.body !== undefined) {
          bodyText = JSON.stringify(env.body);
        } else if (env.error) {
          bodyText = JSON.stringify(env.error);
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
  `opencode sidecar listening on http://${HOST}:${PORT} -> ${UPSTREAM}` +
    (BIND_PROXIES.size ? ` (${BIND_PROXIES.size} bind proxies)` : ""),
);

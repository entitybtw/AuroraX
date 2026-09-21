// OpenCode Zen proxy sidecar.
//
// All zen traffic (free tier, OAuth tokens and API keys) can be routed through
// this sidecar. It exists because the zen free tier fingerprints the TLS
// handshake and only accepts Bun's BoringSSL ClientHello (plus the full
// OpenCode tool schema), which a Go binary cannot reproduce. OAuth access
// tokens and API keys are forwarded untouched, so paid accounts keep working.
//
// Configuration (all optional):
//
//	OPENCODE_ZEN_BASE_URL        upstream base URL          (default https://opencode.ai/zen/v1)
//	AURORA_SIDECAR_HOST    bind host                 (default 127.0.0.1)
//	AURORA_SIDECAR_PORT    bind port                 (default 8090)
//	OPENCODE_ZEN_INJECT_TOOLS    inject OpenCode tools     (default true)
//	OPENCODE_ZEN_DEFAULT_AUTH    fallback Authorization    (default "Bearer public")
//	OPENCODE_ZEN_USER_AGENT      upstream User-Agent       (default opencode client UA)
//
// Endpoints:
//
//	POST /v1/chat/completions   proxied via a fresh Bun process
//	GET  /v1/models             proxied directly
//	GET  /health                liveness probe

const UPSTREAM = process.env.OPENCODE_ZEN_BASE_URL ?? "https://opencode.ai/zen/v1";
const ONESHOT = new URL("./one-shot.js", import.meta.url).pathname;
const PORT = Number(process.env.AURORA_SIDECAR_PORT ?? "8090");
const HOST = process.env.AURORA_SIDECAR_HOST ?? "127.0.0.1";
const DEFAULT_AUTH = process.env.OPENCODE_ZEN_DEFAULT_AUTH ?? "Bearer public";
const USER_AGENT =
  process.env.OPENCODE_ZEN_USER_AGENT ??
  "opencode/1.18.31 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14";

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
    const body = await req.text();

    const proc = Bun.spawn([process.execPath, ONESHOT], {
      stdin: "pipe",
      stdout: "pipe",
      stderr: "pipe",
      env: {
        ...process.env,
        OPENCODE_ZEN_BASE_URL: UPSTREAM,
        OPENCODE_ZEN_USER_AGENT: USER_AGENT,
      },
    });

    proc.stdin.write(JSON.stringify({ authorization, payload: JSON.parse(body) }));
    proc.stdin.end();

    const [out, exitCode] = await Promise.all([
      new Response(proc.stdout).text(),
      proc.exited,
    ]);

    if (exitCode !== 0 && out.trim() === "") {
      const err = await new Response(proc.stderr).text();
      return json(
        { error: { message: `zen sidecar process failed: ${err || exitCode}` } },
        502,
      );
    }

    // Parase the one-shot envelope: { __status, __sse?, body? }.
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

console.log(`opencode zen sidecar listening on http://${HOST}:${PORT} -> ${UPSTREAM}`);

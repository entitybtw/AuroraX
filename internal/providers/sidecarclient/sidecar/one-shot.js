// Executes exactly one upstream request in a fresh Bun process (legacy path).
//
// The adapter now runs requests in-process via execute.js. This wrapper stays
// for operators who opt into strict per-request process isolation with
// AURORA_SIDECAR_ISOLATED=true (reusing a connection can make some upstreams
// reject subsequent requests).
//
// Tool injection is scoped by AURORA_SIDECAR_INJECT_TOOLS (set by adapter
// from overrides + provider type). When enabled and the body has no tools,
// the schema is loaded from AURORA_SIDECAR_TOOLS_PATH (extension-supplied)
// and falls back to the bundled default-tools.json.
import { executeRequest } from "./execute.js";

const cfg = {
  upstream: (process.env.AURORA_SIDECAR_UPSTREAM_URL ?? "").trim(),
  userAgent: process.env.AURORA_SIDECAR_USER_AGENT ?? "",
  defaultAuth: process.env.AURORA_SIDECAR_DEFAULT_AUTH ?? "Bearer public",
  injectTools:
    (process.env.AURORA_SIDECAR_INJECT_TOOLS ?? "true") !== "false",
  maxAttempts: Number(process.env.AURORA_SIDECAR_MAX_ATTEMPTS ?? "4"),
  retryDelayMs: Number(process.env.AURORA_SIDECAR_RETRY_DELAY_MS ?? "750"),
  pathTemplate: (
    process.env.AURORA_SIDECAR_PATH_TEMPLATE ?? "/chat/completions"
  ).trim(),
  retryStatuses: (process.env.AURORA_SIDECAR_RETRY_STATUSES ?? "403,429")
    .split(",")
    .map((s) => Number(s.trim()))
    .filter((n) => Number.isFinite(n) && n > 0),
  extraHeaders: (() => {
    try {
      return JSON.parse(process.env.AURORA_SIDECAR_EXTRA_HEADERS ?? "{}");
    } catch {
      return {};
    }
  })(),
  forceStream: (() => {
    const raw = process.env.AURORA_SIDECAR_FORCE_STREAM ?? "";
    return raw === "true" ? true : raw === "false" ? false : null;
  })(),
  proxy: (process.env.AURORA_SIDECAR_PROXY ?? "").trim(),
  toolsPath: process.env.AURORA_SIDECAR_TOOLS_PATH ?? "",
};

function emit(obj) {
  process.stdout.write(JSON.stringify(obj));
  process.exit(0);
}

const input = await Bun.stdin.text();
let envelope;
try {
  envelope = JSON.parse(input);
} catch {
  emit({ __status: 400, error: { message: "invalid json body" } });
}

if (cfg.forceStream === null) cfg.forceStream = cfg.injectTools;

const result = await executeRequest(envelope, cfg);
emit(result);

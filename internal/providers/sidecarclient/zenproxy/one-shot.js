// Executes exactly one upstream zen request in a fresh Bun process.
//
// A single process must only issue one request: reusing the connection makes
// subsequent zen requests fail, so the adapter spawns this script per request
// and this script exits right after the response is written.
//
// The adapter forwards the inbound Authorization header so access tokens
// (st_*) and API keys keep working; when none is supplied the 
// "public" key is used.
import tools from "./default-tools.json" with { type: "json" };

const UPSTREAM = process.env.?? "https:///zen/v1";
const USER_AGENT =
  process.env.??
  "/1.18.31 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14";
const INJECT_TOOLS = (process.env.?? "true") !== "false";
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

// The zen requires the full upstream tool schema, otherwise it
// responds with regardless of the TLS fingerprint.
if (INJECT_TOOLS && (!Array.isArray(payload.tools) || payload.tools.length === 0)) {
  payload.tools = tools;
}
if (payload.tool_choice === undefined) {
  payload.tool_choice = "auto";
}
if (payload.stream === undefined) {
  payload.stream = true;
}

const randomId = (prefix) =>
  prefix + crypto.randomUUID().replace(/-/g, "").slice(0, 26);

const headers = {
  Authorization: authorization,
  "Content-Type": "application/json",
  "User-Agent": USER_AGENT,
  "x-": "cli",
  "x-": "9a15059a80937175227c853c8d7c79984cdbc2b6",
  "x-": randomId("msg_"),
  "x-": randomId("ses_"),
};

const resp = await fetch(UPSTREAM.replace(/\/+$/, "") + "/chat/completions", {
  method: "POST",
  headers,
  body: JSON.stringify(payload),
});

process.stdout.write(await resp.text());

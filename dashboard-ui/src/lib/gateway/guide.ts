import { withBasePath } from "@/lib/basepath";
import { t } from "@/lib/i18n";


export interface GatewayEndpointRow {
  method: string;
  path: string;
  label: string;
  description: string;
}

export interface GatewayFeatureRow {
  label: string;
  value: string;
}

export const OPENAI_ENDPOINT_ROWS: readonly GatewayEndpointRow[] = [
  { method: "POST", path: "/v1/chat/completions", label: t("Chat completions"), description: t("OpenAI-format chat endpoint.") },
  { method: "POST", path: "/v1/responses", label: t("Create response"), description: t("OpenAI-format responses endpoint.") },
  { method: "GET", path: "/v1/responses/:id", label: t("Get response"), description: t("Retrieve a specific response by ID.") },
  { method: "POST", path: "/v1/responses/:id/cancel", label: t("Cancel response"), description: t("Cancel an in-progress response.") },
  { method: "DELETE", path: "/v1/responses/:id", label: t("Delete response"), description: t("Delete a stored response.") },
  { method: "POST", path: "/v1/responses/input_tokens", label: t("Input tokens"), description: t("Calculate input tokens for a responses request.") },
  { method: "POST", path: "/v1/responses/compact", label: t("Compact response"), description: t("Compact a series of response items.") },
  { method: "GET", path: "/v1/responses/:id/input_items", label: t("Input items"), description: t("Retrieve input items for a specific response.") },
  { method: "POST", path: "/v1/embeddings", label: t("Embeddings"), description: t("Generate embeddings through configured providers.") },
  { method: "POST", path: "/v1/files", label: t("Upload file"), description: t("Upload files for providers that support file workflows.") },
  { method: "GET", path: "/v1/files", label: t("List files"), description: t("List uploaded files.") },
  { method: "GET", path: "/v1/files/:id", label: t("Get file"), description: t("Retrieve a specific file by ID.") },
  { method: "DELETE", path: "/v1/files/:id", label: t("Delete file"), description: t("Delete an uploaded file.") },
  { method: "GET", path: "/v1/files/:id/content", label: t("File content"), description: t("Download file content.") },
  { method: "POST", path: "/v1/batches", label: t("Create batch"), description: t("Create batch jobs for supported workloads.") },
  { method: "GET", path: "/v1/batches", label: t("List batches"), description: t("List all batch jobs.") },
  { method: "GET", path: "/v1/batches/:id", label: t("Get batch"), description: t("Retrieve a specific batch by ID.") },
  { method: "POST", path: "/v1/batches/:id/cancel", label: t("Cancel batch"), description: t("Cancel an in-progress batch.") },
  { method: "GET", path: "/v1/batches/:id/results", label: t("Batch results"), description: t("Retrieve the results of a completed batch.") },
];

export const ANTHROPIC_ENDPOINT_ROWS: readonly GatewayEndpointRow[] = [
  { method: "POST", path: "/v1/messages", label: t("Messages"), description: t("Anthropic-format chat endpoint. Accepts Anthropic request bodies, translates to OpenAI format internally, dispatches to any configured provider (OpenAI, Groq, etc.), and converts the response back to Anthropic format.") },
  { method: "POST", path: "/v1/messages?stream=true", label: t("Messages (streaming)"), description: t("Streaming variant — returns Anthropic SSE events (message_start, content_block_delta, message_delta, message_stop) translated from the upstream provider's streaming response.") },
  { method: "POST", path: "/v1/messages/count_tokens", label: t("Count tokens"), description: t("Anthropic Token Counting API. Estimates input tokens for a message without generating a response. Used by Claude Code and Anthropic SDKs to manage context limits.") },
];

export const INFRA_ENDPOINT_ROWS: readonly GatewayEndpointRow[] = [
  { method: "GET", path: "/health", label: t("Health check"), description: t("Check whether the gateway is up.") },
  { method: "GET", path: "/v1/models", label: t("Models"), description: t("List exposed models and aliases (shared by OpenAI and Anthropic SDKs).") },
  { method: "POST", path: "/v1/rerank", label: t("Rerank"), description: t("Rerank documents against a query through configured reranker providers.") },
  { method: "ANY", path: "/p/{provider}/...", label: t("Provider passthrough"), description: t("Call provider-native endpoints through the gateway when enabled.") },
];

export const GATEWAY_FEATURE_ROWS: readonly GatewayFeatureRow[] = [
  { label: t("Authentication"), value: "Use Authorization: Bearer <master key or managed API key>." },
  { label: t("User path routing"), value: "Send X-Aurora-User-Path to route audit, budget, workflow, and model-access behavior." },
  { label: t("API compatibility"), value: "Point OpenAI- and Anthropic-compatible SDKs at the configured base URL and use /v1 endpoints." },
  { label: t("Aliases"), value: "Use model aliases from the Models page as the model field." },
  { label: t("Passthrough"), value: "Use /p/{provider}/... to reach provider-native routes when passthrough is enabled." },
  { label: t("Observability"), value: "Usage, audit logs, budgets, workflows, and provider health update from gateway traffic." },
];

export function gatewayEndpoint(path: string): string {
  const normalized = path.startsWith("/") ? path : `/${path}`;
  const origin = typeof window === "undefined" ? "" : window.location.origin;
  return origin + withBasePath(normalized);
}

export interface PlaygroundRequestBody {
  model: string;
  messages: Array<{ role: "system" | "user"; content: string }>;
  temperature: number;
  stream: boolean;
}

export interface EmbeddingRequestBody {
  model: string;
  input: string | string[];
  encoding_format?: string;
  dimensions?: number;
  [key: string]: unknown;
}

export interface RerankRequestBody {
  model: string;
  query: string;
  documents: string[];
  top_n?: number | undefined;
  return_documents?: boolean | undefined;
}

export function buildPlaygroundRequestBody(input: {
  model: string;
  systemPrompt: string;
  userPrompt: string;
  stream?: boolean;
}): PlaygroundRequestBody {
  const messages: PlaygroundRequestBody["messages"] = [];
  const systemPrompt = input.systemPrompt.trim();
  const userPrompt = input.userPrompt.trim();
  if (systemPrompt) {
    messages.push({ role: "system", content: systemPrompt });
  }
  messages.push({ role: "user", content: userPrompt || "Hello from AuroraX." });
  return {
    model: input.model,
    messages,
    temperature: 0.7,
    stream: input.stream ?? false,
  };
}

export function gatewayCurlExample(body: Record<string, unknown>): string {
  const cont = " " + String.fromCharCode(92);
  return [
    `curl ${JSON.stringify(gatewayEndpoint("/v1/chat/completions"))}${cont}`,
    `  -H "Authorization: Bearer $AURORA_API_KEY"${cont}`,
    `  -H "Content-Type: application/json"${cont}`,
    `  -H "X-Aurora-User-Path: /team/example"${cont}`,
    `  -d ${JSON.stringify(JSON.stringify(body, null, 2))}`,
  ].join("\n");
}

export type PoolCallStyle = "prefixed" | "provider_hint";

export interface PoolEmbeddingsRequestBody {
  model: string;
  provider?: string;
  input: string[];
}

export interface PoolCallStyleDoc {
  style: PoolCallStyle;
  label: string;
  description: string;
}

export const POOL_CALL_STYLE_DOCS: readonly PoolCallStyleDoc[] = [
  {
    style: "prefixed",
    label: t("Pool name as model prefix"),
    description:
      t("Put the pool name in front of the model id, separated by a slash (e.g. \"jina-pool/jina-embeddings-v3\"). The gateway recognises the prefix as a pool and dispatches to one member via the configured strategy. The gateway also filters pool members by capability — chat requests only go to chat-capable members, embedding requests to embedding-capable members, etc."),
  },
  {
    style: "provider_hint",
    label: t("Pool name in the provider field"),
    description:
      t("Send the bare model id and put the pool name in a separate \"provider\" field. Useful when an SDK preserves the model name verbatim or when you want the same client code to switch between a pool and a single provider just by changing one field. Capability-based filtering applies the same way as the prefixed style."),
  },
];

export const POOL_USAGE_NOTES: readonly string[] = [
  "Both call styles route through the pool's load-balancing strategy and share the same per-member health and error counters.",
  "On transient upstream errors (5xx, 429, network), the gateway automatically retries the request on a different eligible member before returning an error to the client.",
  "Pool members are filtered by capability at dispatch time — a rerank request only routes to members with reranker capability, an embedding request only to embedding-capable members, and so on. The capabilities field on each member shows what it supports.",
  "Pool member counters in /admin/api/v1/pools update with every dispatched request — watch total_requests and total_errors to see traffic spread across members.",
  "Pool member counters are persisted to disk and restored on restart. A runtime config refresh also merges existing counters into the new pool members so accumulated stats survive reload.",
];

export function buildPoolEmbeddingsBody(
  style: PoolCallStyle,
  poolName: string,
  modelId: string,
): PoolEmbeddingsRequestBody {
  const trimmedPool = poolName.trim();
  const trimmedModel = modelId.trim();
  if (style === "prefixed") {
    return {
      model: `${trimmedPool}/${trimmedModel}`,
      input: ["Hello from a pooled embedding request."],
    };
  }
  return {
    model: trimmedModel,
    provider: trimmedPool,
    input: ["Hello from a pooled embedding request."],
  };
}

export function buildEmbeddingRequestBody(input: {
  model: string;
  input: string[];
}): EmbeddingRequestBody {
  return {
    model: input.model,
    input: input.input,
    encoding_format: "float",
  };
}

export function embeddingCurlExample(body: EmbeddingRequestBody): string {
  const cont = " " + String.fromCharCode(92);
  return [
    `curl ${JSON.stringify(gatewayEndpoint("/v1/embeddings"))}${cont}`,
    `  -H "Authorization: Bearer $AURORA_API_KEY"${cont}`,
    `  -H "Content-Type: application/json"${cont}`,
    `  -d ${JSON.stringify(JSON.stringify(body, null, 2))}`,
  ].join("\n");
}

export function buildRerankRequestBody(input: {
  model: string;
  query: string;
  documents: string[];
  top_n?: number;
}): RerankRequestBody {
  return {
    model: input.model,
    query: input.query,
    documents: input.documents,
    top_n: input.top_n,
    return_documents: true,
  };
}

export function rerankCurlExample(body: RerankRequestBody): string {
  const cont = " " + String.fromCharCode(92);
  return [
    `curl ${JSON.stringify(gatewayEndpoint("/v1/rerank"))}${cont}`,
    `  -H "Authorization: Bearer $AURORA_API_KEY"${cont}`,
    `  -H "Content-Type: application/json"${cont}`,
    `  -d ${JSON.stringify(JSON.stringify(body, null, 2))}`,
  ].join("\n");
}

export interface ClaudeCodeSetupRow {
  label: string;
  value: string;
}

export const CLAUDE_CODE_SETUP: readonly ClaudeCodeSetupRow[] = [
  {
    label: t("CLI version"),
    value: "Claude Code v2.1.158+",
  },
  {
    label: t("Auth"),
    value: "Set ANTHROPIC_AUTH_TOKEN to your master key or managed API key. The gateway checks Authorization: Bearer before x-api-key.",
  },
  {
    label: t("Model aliases"),
    value: "Create model aliases in the Models page to map short names to any gateway model. Use the alias name with --model.",
  },
  {
    label: t("Subagent models"),
    value: "Set CLAUDE_CODE_SUBAGENT_MODEL to route Claude Code's subagent calls to a different alias (e.g., a cheaper model).",
  },
  {
    label: t("Base URL"),
    value: "Set ANTHROPIC_BASE_URL to the gateway URL. All /v1/messages calls are translated from Anthropic format to OpenAI format and back.",
  },
];

export const CLAUDE_CODE_EXAMPLE = (baseUrl: string): string => {
  return [
    "# Set environment variables and launch Claude Code:",
    `export ANTHROPIC_BASE_URL="${baseUrl}"`,
    "export ANTHROPIC_AUTH_TOKEN=\"your-master-key\"",
    'export ANTHROPIC_MODEL="my-model-alias"',
    "",
    "# Or pass the model inline:",
    'claude --model "my-model-alias"',
    "",
    "# PowerShell equivalents:",
    '$env:ANTHROPIC_BASE_URL="http://localhost:8080"',
    '$env:ANTHROPIC_AUTH_TOKEN="local-dev-key"',
    '$env:ANTHROPIC_MODEL="my-model-alias"',
    "claude --model my-model-alias",
  ].join("\n");
};

export function poolEmbeddingsCurlExample(body: PoolEmbeddingsRequestBody): string {
  const cont = " " + String.fromCharCode(92);
  return [
    `curl ${JSON.stringify(gatewayEndpoint("/v1/embeddings"))}${cont}`,
    `  -H "Authorization: Bearer $AURORA_API_KEY"${cont}`,
    `  -H "Content-Type: application/json"${cont}`,
    `  -d ${JSON.stringify(JSON.stringify(body, null, 2))}`,
  ].join("\n");
}


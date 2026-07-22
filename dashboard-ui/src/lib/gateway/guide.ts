import { withBasePath } from "@/lib/basepath";

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
  { method: "POST", path: "/v1/chat/completions", label: "Chat completions", description: "OpenAI-format chat endpoint." },
  { method: "POST", path: "/v1/responses", label: "Create response", description: "OpenAI-format responses endpoint." },
  { method: "GET", path: "/v1/responses/:id", label: "Get response", description: "Retrieve a specific response by ID." },
  { method: "POST", path: "/v1/responses/:id/cancel", label: "Cancel response", description: "Cancel an in-progress response." },
  { method: "DELETE", path: "/v1/responses/:id", label: "Delete response", description: "Delete a stored response." },
  { method: "POST", path: "/v1/responses/input_tokens", label: "Input tokens", description: "Calculate input tokens for a responses request." },
  { method: "POST", path: "/v1/responses/compact", label: "Compact response", description: "Compact a series of response items." },
  { method: "GET", path: "/v1/responses/:id/input_items", label: "Input items", description: "Retrieve input items for a specific response." },
  { method: "POST", path: "/v1/embeddings", label: "Embeddings", description: "Generate embeddings through configured providers." },
  { method: "POST", path: "/v1/files", label: "Upload file", description: "Upload files for providers that support file workflows." },
  { method: "GET", path: "/v1/files", label: "List files", description: "List uploaded files." },
  { method: "GET", path: "/v1/files/:id", label: "Get file", description: "Retrieve a specific file by ID." },
  { method: "DELETE", path: "/v1/files/:id", label: "Delete file", description: "Delete an uploaded file." },
  { method: "GET", path: "/v1/files/:id/content", label: "File content", description: "Download file content." },
  { method: "POST", path: "/v1/batches", label: "Create batch", description: "Create batch jobs for supported workloads." },
  { method: "GET", path: "/v1/batches", label: "List batches", description: "List all batch jobs." },
  { method: "GET", path: "/v1/batches/:id", label: "Get batch", description: "Retrieve a specific batch by ID." },
  { method: "POST", path: "/v1/batches/:id/cancel", label: "Cancel batch", description: "Cancel an in-progress batch." },
  { method: "GET", path: "/v1/batches/:id/results", label: "Batch results", description: "Retrieve the results of a completed batch." },
];

export const ANTHROPIC_ENDPOINT_ROWS: readonly GatewayEndpointRow[] = [
  { method: "POST", path: "/v1/messages", label: "Messages", description: "Anthropic-format chat endpoint. Accepts Anthropic request bodies, translates to OpenAI format internally, dispatches to any configured provider (OpenAI, Groq, etc.), and converts the response back to Anthropic format." },
  { method: "POST", path: "/v1/messages?stream=true", label: "Messages (streaming)", description: "Streaming variant — returns Anthropic SSE events (message_start, content_block_delta, message_delta, message_stop) translated from the upstream provider's streaming response." },
  { method: "POST", path: "/v1/messages/count_tokens", label: "Count tokens", description: "Anthropic Token Counting API. Estimates input tokens for a message without generating a response. Used by Claude Code and Anthropic SDKs to manage context limits." },
];

export const INFRA_ENDPOINT_ROWS: readonly GatewayEndpointRow[] = [
  { method: "GET", path: "/health", label: "Health check", description: "Check whether the gateway is up." },
  { method: "GET", path: "/v1/models", label: "Models", description: "List exposed models and aliases (shared by OpenAI and Anthropic SDKs)." },
  { method: "POST", path: "/v1/rerank", label: "Rerank", description: "Rerank documents against a query through configured reranker providers." },
  { method: "ANY", path: "/p/{provider}/...", label: "Provider passthrough", description: "Call provider-native endpoints through the gateway when enabled." },
];

export const GATEWAY_FEATURE_ROWS: readonly GatewayFeatureRow[] = [
  { label: "Authentication", value: "Use Authorization: Bearer <master key or managed API key>." },
  { label: "User path routing", value: "Send X-Aurora-User-Path to route audit, budget, workflow, and model-access behavior." },
  { label: "API compatibility", value: "Point OpenAI- and Anthropic-compatible SDKs at the configured base URL and use /v1 endpoints." },
  { label: "Aliases", value: "Use model aliases from the Models page as the model field." },
  { label: "Passthrough", value: "Use /p/{provider}/... to reach provider-native routes when passthrough is enabled." },
  { label: "Observability", value: "Usage, audit logs, budgets, workflows, and provider health update from gateway traffic." },
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
  messages.push({ role: "user", content: userPrompt || "Hello from Aurora." });
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
    label: "Pool name as model prefix",
    description:
      'Put the pool name in front of the model id, separated by a slash (e.g. "jina-pool/jina-embeddings-v3"). The gateway recognises the prefix as a pool and dispatches to one member via the configured strategy. The gateway also filters pool members by capability — chat requests only go to chat-capable members, embedding requests to embedding-capable members, etc.',
  },
  {
    style: "provider_hint",
    label: "Pool name in the provider field",
    description:
      'Send the bare model id and put the pool name in a separate "provider" field. Useful when an SDK preserves the model name verbatim or when you want the same client code to switch between a pool and a single provider just by changing one field. Capability-based filtering applies the same way as the prefixed style.',
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
    label: "CLI version",
    value: "Claude Code v2.1.158+",
  },
  {
    label: "Auth",
    value: "Set ANTHROPIC_AUTH_TOKEN to your master key or managed API key. The gateway checks Authorization: Bearer before x-api-key.",
  },
  {
    label: "Model aliases",
    value: "Create model aliases in the Models page to map short names to any gateway model. Use the alias name with --model.",
  },
  {
    label: "Subagent models",
    value: "Set CLAUDE_CODE_SUBAGENT_MODEL to route Claude Code's subagent calls to a different alias (e.g., a cheaper model).",
  },
  {
    label: "Base URL",
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


import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useMemo, useState, useCallback, useEffect, useRef } from "react";
import { Loader2, Send, Code, Terminal, Clock, Activity, Settings2, FileText, ListOrdered } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState, Surface } from "@/components/ui/surface";
import { useModels } from "@/lib/api/useModels";
import type { ModelInventoryItem } from "@/lib/api/models-types";
import { modelDisplayName, modelSecondaryName } from "@/lib/api/models-types";
import { apiFetch } from "@/lib/api/client";
import { getApiKey } from "@/lib/auth/storage";
import { getAccessToken } from "@/lib/auth/session";
import { buildPlaygroundRequestBody, buildEmbeddingRequestBody, buildRerankRequestBody, gatewayEndpoint, gatewayCurlExample, embeddingCurlExample, rerankCurlExample } from "@/lib/gateway/guide";
import { formatTokens } from "@/lib/format/numbers";

type PlaygroundMode = "chat" | "embeddings" | "rerank";

interface PlaygroundStats {
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  latencyMs: number;
  tokensPerSecond: number;
}

interface ChatCompletionPayload {
  model?: string;
  choices?: Array<{ message?: { content?: string; reasoning?: string } }>;
  usage?: {
    total_tokens?: number;
    prompt_tokens?: number;
    completion_tokens?: number;
    input_tokens?: number;
    output_tokens?: number;
    completion_tokens_details?: { reasoning_tokens?: number };
  };
}

interface EmbeddingResult {
  object?: string;
  index?: number;
  embedding?: number[];
}

interface EmbeddingResponse {
  data?: EmbeddingResult[];
  model?: string;
  usage?: { prompt_tokens?: number; total_tokens?: number };
}

interface RerankResult {
  index: number;
  relevance_score: number;
  document?: { text?: string };
}

interface RerankResponse {
  results?: RerankResult[];
  model?: string;
  usage?: { prompt_tokens?: number; total_tokens?: number };
}

const MODE_LABELS: Record<PlaygroundMode, string> = {
  chat: "Chat",
  embeddings: "Embeddings",
  rerank: "Rerank",
};

interface RerankScenario {
  query: string;
  documents: string[];
  bestMatch: number;
}

const RERANK_SCENARIOS: RerankScenario[] = [
  {
    query: "How do transformer attention mechanisms differ from recurrent neural networks?",
    bestMatch: 2,
    documents: [
      "Transformer models use self-attention to process all tokens in parallel, capturing long-range dependencies without sequential recurrence. The attention mechanism computes query-key-value weights across every position, enabling direct token-to-token interactions regardless of distance in the sequence.",
      "Recurrent neural networks (RNNs) process sequences step by step, maintaining a hidden state that carries information forward. They are inherently sequential, which makes them slower to train on long sequences and prone to vanishing gradients despite LSTM/GRU improvements.",
      "Self-attention in Transformers calculates a weighted sum of all values based on pairwise similarity between queries and keys. This produces a context-aware representation where each token can attend to every other token in a single forward pass, unlike RNNs where information must propagate through time steps.",
      "Long Short-Term Memory (LSTM) networks address the vanishing gradient problem in standard RNNs by introducing input, forget, and output gates. These gates regulate information flow through the cell state, allowing gradients to propagate more effectively over longer sequences.",
      "Convolutional neural networks (CNNs) use sliding filters to detect local patterns and are well-suited for grid-structured data like images. They lack built-in sequence modeling and rely on stacking layers or dilated convolutions to increase receptive field.",
      "The key-value store analogy helps understand attention: the query represents what you are looking for, keys represent what each token offers, and values represent the actual information contributed. The attention score is the similarity between query and key, determining how much of each value to include.",
      "GPT models use decoder-only transformer architecture with masked self-attention, where each token can only attend to previous tokens. This causal masking enables autoregressive generation while preserving the parallel training benefits of the transformer.",
      "Graph Neural Networks (GNNs) operate on graph-structured data by aggregating information from neighboring nodes through message-passing layers. Unlike transformers which operate on fully connected token sets, GNNs respect the explicit connectivity structure of the input graph.",
    ],
  },
  {
    query: "What are the key advantages of serverless computing over traditional container orchestration?",
    bestMatch: 0,
    documents: [
      "Serverless computing abstracts infrastructure entirely: the cloud provider manages resource allocation, scaling, and availability. Developers deploy functions or applications without provisioning servers, and billing is based on actual execution time and resource consumption rather than reserved capacity.",
      "Kubernetes automates deployment, scaling, and management of containerized applications across clusters of machines. It provides declarative configuration, self-healing, service discovery, and rolling updates, but requires significant operational expertise to manage cluster infrastructure, networking, and security policies.",
      "AWS Lambda pioneered the Functions-as-a-Service model, executing code in response to events with automatic scaling from zero to thousands of concurrent executions. Cold starts introduce latency when a new execution environment must be initialized before handling a request.",
      "Container orchestration platforms like Kubernetes offer fine-grained control over resource limits, pod scheduling policies, affinity rules, and persistent storage volumes. This level of control is essential for stateful workloads, high-throughput data pipelines, and applications with strict latency requirements.",
      "Serverless platforms excel in variable or unpredictable traffic patterns because they scale to zero when idle and instantaneously scale up under load. This eliminates the cost of maintaining idle capacity, making serverless cost-effective for intermittent workloads and event-driven architectures.",
      "Docker containers package applications with their dependencies and runtime environment, ensuring consistency across development, testing, and production. Containers share the host OS kernel and are more lightweight than virtual machines, but still require an underlying container runtime and orchestration layer.",
      "The primary limitation of serverless is execution duration caps, memory constraints, and state management challenges. Long-running processes, WebSocket connections, and workloads requiring GPU acceleration often exceed serverless platform limits, making containers or VMs more suitable.",
      "Terraform enables infrastructure-as-code management across cloud providers, allowing teams to define serverless functions, container clusters, networking, and databases in declarative configuration files. It treats infrastructure provisioning as version-controlled, reproducible code rather than manual setup.",
    ],
  },
  {
    query: "How does quantum computing threaten current cryptographic systems?",
    bestMatch: 0,
    documents: [
      "Shor's algorithm solves integer factorization and discrete logarithm problems in polynomial time on a sufficiently large quantum computer. This directly breaks RSA, DSA, and Diffie-Hellman key exchange, which rely on the computational hardness of these problems for classical computers.",
      "Post-quantum cryptography (PQC) aims to develop cryptographic algorithms resistant to both quantum and classical attacks. The NIST PQC standardization process has selected lattice-based, code-based, and hash-based primitives as candidates for replacing current public-key infrastructure.",
      "Grover's algorithm provides a quadratic speedup for unstructured search problems, effectively halving the security level of symmetric-key cryptographic systems. Countering Grover's algorithm requires doubling key sizes: AES-128 becomes as secure as AES-256 against quantum adversaries using Grover's search.",
      "Quantum key distribution (QKD) uses quantum mechanical properties to establish secure communication channels with information-theoretic security. Any attempt to eavesdrop on a QKD channel inevitably disturbs the quantum states, revealing the presence of interception to legitimate parties.",
      "Lattice-based cryptography relies on the hardness of Learning With Errors (LWE) and related lattice problems. These problems are believed to be resistant to quantum attacks, forming the basis for CRYSTALS-Kyber (key encapsulation) and CRYSTALS-Dilithium (digital signatures) selected by NIST.",
      "RSA encryption derives its security from the difficulty of factoring large composite numbers into prime factors. Classical computers cannot factor 2048-bit RSA moduli in feasible time, but Shor's algorithm on a fault-tolerant quantum computer with sufficient logical qubits could break RSA-2048 in hours.",
      "Elliptic curve cryptography (ECC) provides equivalent security to RSA with smaller key sizes, making it widely used in TLS certificates, blockchain wallets, and modern messaging protocols. ECC security depends on the elliptic curve discrete logarithm problem, which Shor's algorithm can also solve efficiently.",
      "Quantum error correction is essential for building fault-tolerant quantum computers capable of running Shor's algorithm at scale. Surface codes and concatenated codes detect and correct errors in quantum states, but require thousands of physical qubits to encode a single logical qubit.",
    ],
  },
  {
    query: "What ethical considerations arise from using AI in medical diagnosis?",
    bestMatch: 5,
    documents: [
      "Algorithmic bias in medical AI systems can perpetuate and amplify existing healthcare disparities. Training data that underrepresents certain demographic groups leads to less accurate diagnoses for those populations, potentially widening the gap in healthcare outcomes across racial, ethnic, and socioeconomic lines.",
      "FDA clearance and regulatory frameworks for medical AI require validation studies demonstrating safety and effectiveness. However, the pace of AI advancement often outstrips regulatory processes, creating a gap between available technology and established guidelines for clinical deployment.",
      "Patient data privacy under HIPAA and GDPR imposes strict requirements on how medical AI systems handle protected health information. Training and inference must implement technical safeguards including encryption, access controls, and data minimization to prevent unauthorized disclosure of sensitive medical records.",
      "Explainable AI (XAI) methods attempt to make black-box model decisions interpretable to clinicians. Techniques like SHAP values, LIME, and attention visualization provide post-hoc explanations of model predictions, helping physicians understand why an AI system recommended a particular diagnosis or treatment plan.",
      "Transfer learning enables medical AI models pre-trained on large general datasets to be fine-tuned for specific clinical tasks with limited labeled data. This approach reduces the data collection burden for rare diseases but raises questions about whether pre-training data distributions match the target clinical population.",
      "Liability frameworks for AI-assisted diagnosis remain unsettled: when an AI system misdiagnoses a condition, responsibility may fall on the developer, the deploying hospital, the supervising physician, or be shared across multiple actors. Clear legal standards are needed as AI becomes more autonomous in clinical decision-making.",
      "Large language models like GPT-4 have demonstrated passing medical licensing examinations and generating clinical notes, but they also exhibit hallucination risks where the model produces confident but factually incorrect medical information. Verification mechanisms and human oversight remain critical for patient safety.",
      "Informed consent processes need to evolve when AI systems participate in medical decision-making. Patients should understand when an AI is involved in their diagnosis, what data the AI uses, how accurate it is, and their right to seek a second opinion from a human physician who made the assessment independently.",
    ],
  },
  {
    query: "How do modern vector databases differ from traditional relational databases for similarity search?",
    bestMatch: 0,
    documents: [
      "Vector databases index high-dimensional embeddings using approximate nearest neighbor (ANN) algorithms like HNSW, IVF, and PQ. These indexes trade a small amount of recall for dramatic speed improvements, enabling similarity searches over millions of vectors in milliseconds rather than scanning the entire dataset linearly.",
      "Relational databases use B-tree and hash indexes optimized for exact lookups, range queries, and joins on structured data. They excel at ACID transactions, referential integrity, and complex query planning but cannot efficiently perform cosine similarity or euclidean distance comparisons across high-dimensional vectors.",
      "Hierarchical Navigable Small World (HNSW) graphs build a multi-layer index where higher layers have fewer nodes connected by longer edges for coarse navigation, and lower layers have dense connections for fine-grained search. This structure enables logarithmic search complexity for approximate nearest neighbor queries.",
      "PostgreSQL with pgvector extension adds vector similarity search capabilities to a traditional relational database. It supports IVFFlat and HNSW indexes directly in SQL, allowing hybrid queries that combine vector similarity with structured filters like WHERE clauses, JOINs, and aggregations in a single query.",
      "Product Quantization (PQ) compresses high-dimensional vectors by splitting them into sub-vectors and quantizing each sub-vector independently. This reduces memory usage by 8-16x compared to storing raw float32 vectors, making it feasible to index billions of vectors on a single server, though with some accuracy loss.",
      "ACID compliance in relational databases guarantees atomic, consistent, isolated, and durable transactions essential for financial systems and inventory management. Vector databases typically relax these guarantees to prioritize query speed and horizontal scalability for read-heavy similarity workloads.",
      "Hybrid search combining vector similarity with keyword matching (BM25) and metadata filtering is increasingly common in modern retrieval systems. Dense vectors capture semantic meaning while sparse methods handle exact term matching, and metadata filters restrict the search space by categorical or numeric attributes.",
      "Distributed vector databases like Milvus and Qdrant shard indexes across multiple nodes and replicate data for fault tolerance. They support horizontal scaling by partitioning the vector space or using consistent hashing, enabling search across hundreds of billions of vectors in production RAG pipelines.",
    ],
  },
];

interface EmbeddingScenario {
  query: string;
  candidates: string[];
  bestMatch: number;
}

const EMBEDDING_SCENARIOS: EmbeddingScenario[] = [
  {
    query: "Deep learning frameworks",
    bestMatch: 0,
    candidates: [
      "TensorFlow architecture and graph computation",
      "Sushi rice preparation techniques",
      "PyTorch dynamic computation graphs",
      "Eiffel Tower height and construction history",
      "JAX accelerated linear algebra library",
    ],
  },
  {
    query: "Renaissance art history",
    bestMatch: 0,
    candidates: [
      "Mona Lisa painting by Leonardo da Vinci",
      "Supervised machine learning classification",
      "Sistine Chapel ceiling frescoes by Michelangelo",
      "Quantum entanglement and Bell's theorem",
      "Rome St. Peter's Basilica dome architecture",
    ],
  },
  {
    query: "Cloud computing infrastructure",
    bestMatch: 2,
    candidates: [
      "Shakespearean tragedy themes and analysis",
      "Sourdough bread starter fermentation process",
      "Kubernetes container orchestration and scaling",
      "AWS Lambda serverless function deployment",
      "Terraform infrastructure-as-code configuration",
    ],
  },
  {
    query: "High-performance computing hardware",
    bestMatch: 0,
    candidates: [
      "GPU parallel processing and CUDA programming",
      "Mediterranean diet nutritional composition",
      "MPI distributed memory message passing",
      "Baroque music instrumentation and harmony",
      "SIMD vectorization techniques in modern CPUs",
    ],
  },
  {
    query: "Natural language processing techniques",
    bestMatch: 3,
    candidates: [
      "Convolutional neural networks for computer vision",
      "Support vector machine kernel functions",
      "Transformer-based language model pre-training",
      "Word embedding and distributional semantic similarity",
      "K-nearest neighbors distance-based classification",
    ],
  },
];

interface PickedScenario {
  scenario: RerankScenario;
  expectedTopLine: number;
}

function pickRerankScenario(): PickedScenario {
  const idx = Math.floor(Math.random() * RERANK_SCENARIOS.length);
  const scenario = RERANK_SCENARIOS[idx]!;
  const pickedDocs: string[] = [];
  const pickedOrigIndices: number[] = [];
  const shuffled = scenario.documents
    .map((doc, i) => ({ doc, origIdx: i }))
    .toSorted(() => Math.random() - 0.5);
  for (let i = 0; i < 5 && i < shuffled.length; i++) {
    pickedDocs.push(shuffled[i]!.doc);
    pickedOrigIndices.push(shuffled[i]!.origIdx);
  }
  const expectedTop = pickedOrigIndices.indexOf(scenario.bestMatch);
  return {
    scenario: { query: scenario.query, documents: pickedDocs, bestMatch: expectedTop >= 0 ? expectedTop : 0 },
    expectedTopLine: expectedTop >= 0 ? expectedTop + 1 : 1,
  };
}

interface PickedEmbeddingScenario {
  query: string;
  candidates: string[];
  expectedLine: number;
}

function pickEmbeddingScenario(): PickedEmbeddingScenario {
  const idx = Math.floor(Math.random() * EMBEDDING_SCENARIOS.length);
  const scenario = EMBEDDING_SCENARIOS[idx]!;
  const shuffled = scenario.candidates
    .map((c, i) => ({ text: c, origIdx: i }))
    .toSorted(() => Math.random() - 0.5);
  const picked = shuffled.slice(0, 5);
  const bestNewIdx = picked.findIndex((p) => p.origIdx === scenario.bestMatch);
  return {
    query: scenario.query,
    candidates: picked.map((p) => p.text),
    expectedLine: bestNewIdx + 1,
  };
}

function cosineSimilarity(a: number[], b: number[]): number {
  let dot = 0, normA = 0, normB = 0;
  const len = Math.min(a.length, b.length);
  for (let i = 0; i < len; i++) {
    dot += a[i]! * b[i]!;
    normA += a[i]! * a[i]!;
    normB += b[i]! * b[i]!;
  }
  return dot / (Math.sqrt(normA) * Math.sqrt(normB));
}

function modelCategory(item: ModelInventoryItem): PlaygroundMode {
  const metadata = item.model?.metadata as Record<string, unknown> | undefined;
  const id = (item.model?.id ?? "").toLowerCase();

  const categories = metadata?.categories as string[] | undefined;
  if (categories && categories.length > 0) {
    if (categories.includes("embedding")) return "embeddings";
    if (categories.includes("rerank")) return "rerank";
    return "chat";
  }

  const modes = metadata?.modes as string[] | undefined;
  if (modes && modes.length > 0) {
    const mode = modes[0];
    if (mode === "embedding") return "embeddings";
    if (mode === "rerank") return "rerank";
    if (mode === "chat" || mode === "completion" || mode === "responses") return "chat";
    if (mode === "image_generation" || mode === "image_edit" || mode === "audio_transcription" || mode === "audio_speech" || mode === "video_generation") return "chat";
  }

  if (id.includes("rerank") || id.includes("colbert")) return "rerank";
  if (id.includes("embedding") || id.includes("embed") || id.includes("clip")) return "embeddings";
  if (id.includes("whisper") || id.includes("audio") || id.includes("speech")) return "chat";

  return "chat";
}

const DEFAULT_SYSTEM = "You are a concise assistant. answer user's answer briefly and clearly. use emojis when appropriate.";
const DEFAULT_PROMPT = "Say hello from aurora and mention which model you are!";

export function PlaygroundPage(): JSX.Element {
  const models = useModels();

  const grouped = useMemo(() => {
    const groups: Record<PlaygroundMode, ModelInventoryItem[]> = { chat: [], embeddings: [], rerank: [] };
    for (const item of models.data ?? []) {
      const cat = modelCategory(item);
      if (groups[cat]) groups[cat].push(item);
    }
    return groups;
  }, [models.data]);

  const options = useMemo(() => {
    const cats: PlaygroundMode[] = ["chat", "embeddings", "rerank"];
    return cats.flatMap((cat) =>
      (grouped[cat] ?? []).map((item) => ({
        value: modelDisplayName(item),
        label: modelDisplayName(item),
        description: modelSecondaryName(item),
        category: cat,
      })),
    );
  }, [grouped]);

  const [responseView, setResponseView] = useState<'preview' | 'raw'>('preview');
  const [model, setModel] = useState("");
  const [systemPrompt, setSystemPrompt] = useState(DEFAULT_SYSTEM);
  const [prompt, setPrompt] = useState(DEFAULT_PROMPT);
  const [temperature, setTemperature] = useState<number>(0.7);
  const [maxTokens, setMaxTokens] = useState<number>(4096);
  const [topP, setTopP] = useState<number>(1);
  const [frequencyPenalty, setFrequencyPenalty] = useState<number>(0);
  const [presencePenalty, setPresencePenalty] = useState<number>(0);
  const [seed, setSeed] = useState<number | undefined>(undefined);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [responseText, setResponseText] = useState("");
  const [reasoningText, setReasoningText] = useState("");
  const [stats, setStats] = useState<PlaygroundStats | null>(null);

  const selectedModel = model || options[0]?.value || "";
  const selectedOpt = options.find((o) => o.value === selectedModel);
  const mode: PlaygroundMode = selectedOpt?.category ?? "chat";

  const canSubmit = Boolean(selectedModel) && !loading;

  // Embeddings state
  const [embQuery, setEmbQuery] = useState("");
  const [embInputs, setEmbInputs] = useState("Hello from Aurora");
  const [embExpectedLine, setEmbExpectedLine] = useState<number | null>(null);
  // Rerank state
  const [rerankQuery, setRerankQuery] = useState("");
  const [rerankDocs, setRerankDocs] = useState("");
  const [rerankTopN, setRerankTopN] = useState(5);
  const [rerankExpectedLine, setRerankExpectedLine] = useState<number | null>(null);

  // Results for embeddings / rerank
  const [embResult, setEmbResult] = useState<EmbeddingResponse | null>(null);
  const [rerankResult, setRerankResult] = useState<RerankResponse | null>(null);
  const [embSimilarities, setEmbSimilarities] = useState<Array<{ line: number; text: string; similarity: number }> | null>(null);

  const prevMode = useRef<PlaygroundMode>("chat");
  useEffect(() => {
    if (mode !== prevMode.current) {
      if (mode === "rerank") {
        const picked = pickRerankScenario();
        setRerankQuery(picked.scenario.query);
        setRerankExpectedLine(picked.expectedTopLine);
        setRerankDocs(picked.scenario.documents.map((doc, i) => `${i + 1}. ${doc}`).join("\n"));
      } else if (mode === "embeddings") {
        const picked = pickEmbeddingScenario();
        setEmbQuery(picked.query);
        setEmbExpectedLine(picked.expectedLine);
        setEmbInputs(picked.candidates.map((c, i) => `${i + 1}. ${c}`).join("\n"));
      }
    }
    prevMode.current = mode;
  }, [mode]);

  const prevModel = useRef("");
  useEffect(() => {
    if (selectedModel && selectedModel !== prevModel.current) {
      if (mode === "rerank") {
        const picked = pickRerankScenario();
        setRerankQuery(picked.scenario.query);
        setRerankExpectedLine(picked.expectedTopLine);
        setRerankDocs(picked.scenario.documents.map((doc, i) => `${i + 1}. ${doc}`).join("\n"));
      } else if (mode === "embeddings") {
        const picked = pickEmbeddingScenario();
        setEmbQuery(picked.query);
        setEmbExpectedLine(picked.expectedLine);
        setEmbInputs(picked.candidates.map((c, i) => `${i + 1}. ${c}`).join("\n"));
      }
    }
    prevModel.current = selectedModel;
  }, [selectedModel, mode]);

  const handleModelChange = useCallback((value: string) => {
    setModel(value);
    setResponseText("");
    setReasoningText("");
    setStats(null);
    setError("");
    setEmbResult(null);
    setEmbSimilarities(null);
    setRerankResult(null);
  }, []);

  async function submitChat(): Promise<void> {
    if (!canSubmit) return;
    setLoading(true);
    setError("");
    setResponseText("");
    setReasoningText("");
    setStats(null);
    setEmbResult(null);
    setRerankResult(null);
    const startedAt = performance.now();
    const accContent: string[] = [];
    const accReasoning: string[] = [];
    try {
      const requestBody = {
        ...buildPlaygroundRequestBody({ model: selectedModel, systemPrompt, userPrompt: prompt, stream: true }),
        temperature,
        max_tokens: maxTokens,
        top_p: topP,
        frequency_penalty: frequencyPenalty,
        presence_penalty: presencePenalty,
        ...(seed !== undefined ? { seed } : {}),
      };
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        "Accept": "text/event-stream",
        "X-Aurora-Timezone": Intl.DateTimeFormat().resolvedOptions().timeZone,
      };
      const accessToken = getAccessToken();
      const apiKey = getApiKey();
      if (accessToken) headers["Authorization"] = `Bearer ${accessToken}`;
      else if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;
      const res = await fetch(gatewayEndpoint("/v1/chat/completions"), {
        method: "POST",
        headers,
        body: JSON.stringify(requestBody),
      });
      if (!res.ok) {
        const errBody = await res.text().catch(() => "");
        setError(`HTTP ${res.status}: ${res.statusText}${errBody ? ` — ${errBody}` : ""}`);
        setLoading(false);
        return;
      }
      const contentType = res.headers.get("Content-Type") ?? "";
      if (!contentType.includes("text/event-stream")) {
        const json = await res.json() as ChatCompletionPayload;
        const elapsedMs = Math.max(0, performance.now() - startedAt);
        const text = json.choices?.[0]?.message?.content?.trim() || JSON.stringify(json, null, 2);
        setReasoningText(json.choices?.[0]?.message?.reasoning?.trim() ?? "");
        setResponseText(text);
        const u = json.usage ?? {};
        setStats({
          promptTokens: Number(u.prompt_tokens ?? u.input_tokens ?? 0),
          completionTokens: Number(u.completion_tokens ?? u.output_tokens ?? 0),
          totalTokens: Number(u.total_tokens ?? 0),
          latencyMs: Math.round(elapsedMs),
          tokensPerSecond: 0,
        });
        setLoading(false);
        return;
      }
      const reader = res.body!.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      let usage: ChatCompletionPayload["usage"] | undefined;
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() ?? "";
        let hasUpdates = false;
        for (const line of lines) {
          if (!line.startsWith("data: ")) continue;
          const data = line.slice(6).trim();
          if (data === "[DONE]") continue;
          try {
            const parsed = JSON.parse(data) as { choices?: Array<{ delta?: { content?: string; reasoning?: string }; finish_reason?: string }>; usage?: ChatCompletionPayload["usage"] };
            const delta = parsed.choices?.[0]?.delta;
            if (delta?.content) {
              accContent.push(delta.content);
              hasUpdates = true;
            }
            if (delta?.reasoning) {
              accReasoning.push(delta.reasoning);
              hasUpdates = true;
            }
            if (parsed.usage) usage = parsed.usage;
          } catch {
            // skip malformed chunks
          }
        }
        if (hasUpdates) {
          await new Promise<void>((resolve) => {
            setResponseText(accContent.join(""));
            setReasoningText(accReasoning.join(""));
            setTimeout(resolve, 0);
          });
        }
      }
      const elapsedMs = Math.max(0, performance.now() - startedAt);
      const u = usage ?? {};
      const totalTokens = Number(u.total_tokens ?? 0);
      const promptTokens = Number(u.prompt_tokens ?? u.input_tokens ?? 0);
      const completionTokens = Number(u.completion_tokens ?? u.output_tokens ?? 0);
      setStats({
        promptTokens,
        completionTokens,
        totalTokens,
        latencyMs: Math.round(elapsedMs),
        tokensPerSecond: elapsedMs > 0 && totalTokens > 0 ? totalTokens / (elapsedMs / 1000) : 0,
      });
      if (!accContent.length) {
        setResponseText("(empty response)");
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Chat request failed.");
    } finally {
      setLoading(false);
    }
  }

  async function submitEmbeddings(): Promise<void> {
    if (!canSubmit) return;
    setLoading(true);
    setError("");
    setResponseText("");
    setReasoningText("");
    setStats(null);
    setEmbResult(null);
    setEmbSimilarities(null);
    setRerankResult(null);
    const startedAt = performance.now();
    try {
      const candidates = embInputs.split("\n").map((l) => l.trim()).filter(Boolean).map((l) => l.replace(/^\d+\.\s*/, ''));
      const query = embQuery || (candidates[0] ?? "");
      const inputs = [query, ...candidates];
      const requestBody = buildEmbeddingRequestBody({ model: selectedModel, input: inputs });
      const payload = await apiFetch<EmbeddingResponse>("/v1/embeddings", {
        method: "POST",
        json: requestBody,
      });
      const elapsedMs = Math.max(0, performance.now() - startedAt);
      setEmbResult(payload);
      const usage = payload.usage ?? {};
      const pt = usage.prompt_tokens ?? 0;
      setStats({
        promptTokens: pt,
        completionTokens: 0,
        totalTokens: usage.total_tokens ?? pt,
        latencyMs: Math.round(elapsedMs),
        tokensPerSecond: 0,
      });
      if (payload.data && payload.data.length >= 2 && payload.data[0]?.embedding) {
        const data = payload.data as EmbeddingResult[];
        const queryEmb = data[0]!.embedding!;
        const sims = inputs.slice(1).map((text, i) => ({
          line: i + 1,
          text,
          similarity: cosineSimilarity(queryEmb, data[i + 1]?.embedding ?? []),
        }));
        sims.sort((a, b) => b.similarity - a.similarity);
        setEmbSimilarities(sims);
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Embeddings request failed.");
    } finally {
      setLoading(false);
    }
  }

  async function submitRerank(): Promise<void> {
    if (!canSubmit) return;
    setLoading(true);
    setError("");
    setResponseText("");
    setReasoningText("");
    setStats(null);
    setRerankResult(null);
    const startedAt = performance.now();
    try {
      const docs = rerankDocs.split("\n").map((l) => l.trim()).filter(Boolean).map((l) => l.replace(/^\d+\.\s*/, ''));
      const requestBody = buildRerankRequestBody({
        model: selectedModel,
        query: rerankQuery || prompt,
        documents: docs,
        top_n: rerankTopN,
      });
      const payload = await apiFetch<RerankResponse>("/v1/rerank", {
        method: "POST",
        json: requestBody,
      });
      const elapsedMs = Math.max(0, performance.now() - startedAt);
      setRerankResult(payload);
      const usage = payload.usage ?? {};
      setStats({
        promptTokens: usage.prompt_tokens ?? 0,
        completionTokens: 0,
        totalTokens: usage.total_tokens ?? 0,
        latencyMs: Math.round(elapsedMs),
        tokensPerSecond: 0,
      });
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Rerank request failed.");
    } finally {
      setLoading(false);
    }
  }

  const submit = mode === "chat" ? submitChat : mode === "embeddings" ? submitEmbeddings : submitRerank;

  const generatedCurl = useMemo(() => {
    if (mode === "chat") {
      const body: Record<string, unknown> = {
        ...buildPlaygroundRequestBody({ model: selectedModel, systemPrompt, userPrompt: prompt, stream: true }),
        temperature,
        max_tokens: maxTokens,
        top_p: topP,
        frequency_penalty: frequencyPenalty,
        presence_penalty: presencePenalty,
      };
      if (seed !== undefined) body.seed = seed;
      return gatewayCurlExample(body);
    }
    if (mode === "embeddings") {
      const candidates = embInputs.split("\n").map((l) => l.trim()).filter(Boolean).map((l) => l.replace(/^\d+\.\s*/, ''));
      const query = embQuery || (candidates[0] ?? "");
      const inputs = [query, ...candidates];
      return embeddingCurlExample(buildEmbeddingRequestBody({ model: selectedModel, input: inputs }));
    }
    const docs = rerankDocs.split("\n").map((l) => l.trim()).filter(Boolean).map((l) => l.replace(/^\d+\.\s*/, ''));
    return rerankCurlExample(buildRerankRequestBody({
      model: selectedModel,
      query: rerankQuery || prompt,
      documents: docs,
      top_n: rerankTopN,
    }));
  }, [mode, selectedModel, systemPrompt, prompt, temperature, maxTokens, topP, frequencyPenalty, presencePenalty, seed, embInputs, rerankQuery, rerankDocs, rerankTopN]);

  const endpointPath = mode === "chat"
    ? "/v1/chat/completions"
    : mode === "embeddings"
      ? "/v1/embeddings"
      : "/v1/rerank";

  const submitLabel = mode === "chat"
    ? "Send message"
    : mode === "embeddings"
      ? "Generate Embeddings"
      : "Rerank";

  return (
    <div className="flex flex-col gap-6 h-[calc(100vh-2rem)]">
      <header className="flex flex-col sm:flex-row sm:items-end justify-between gap-4 pb-4 pt-4 border-b border-border/60 shrink-0">
        <div className="min-w-0 flex-1">
          <h1 className="font-serif text-[34px] font-normal leading-tight tracking-tight text-foreground">Playground</h1>
          <p className="mt-1.5 text-[15px] text-muted-foreground">Test chat, embeddings, and rerank models through the gateway. Select a model and the playground adapts to its capability.</p>
        </div>
      </header>

      <div className="grid grid-cols-1 gap-5 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)] min-h-[850px] xl:min-h-[calc(100vh-140px)]">
        <Surface className="p-0 overflow-hidden flex flex-col h-full border border-border">
          <form
            className="flex flex-col h-full"
            onSubmit={(event) => {
              event.preventDefault();
              void submit();
            }}
          >
            <div className="border-b bg-muted/20 p-5 shrink-0 flex items-center justify-between">
              <div className="flex flex-col gap-1">
                <div className="flex items-center gap-2">
                  <div className="h-6 w-1  bg-accent"></div>
                  <h3 className="font-semibold text-lg tracking-tight">Prompt</h3>
                </div>
                <p className="text-sm text-muted-foreground ml-3">Select a model to start testing.</p>
              </div>
              <div className="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider bg-background px-3 py-1.5  border">
                <Settings2 className="w-3.5 h-3.5" />
                Settings
              </div>
            </div>

            <div className="p-5 flex-1 space-y-5 overflow-y-auto">
              {/* Model + Temperature row */}
              <div className="grid grid-cols-[1fr_auto] gap-4 items-end">
                <label className="block space-y-2">
                  <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Model</span>
                  <select
                    value={model}
                    onChange={(event) => handleModelChange(event.target.value)}
                    className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm font-semibold text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all"
                  >
                    <option value="">{options[0] ? `Auto: ${options[0].label}` : "No models loaded"}</option>
                    {(["chat", "embeddings", "rerank"] as PlaygroundMode[]).map((cat) => {
                      const catOptions = options.filter((o) => o.category === cat);
                      if (catOptions.length === 0) return null;
                      return (
                        <optgroup key={cat} label={MODE_LABELS[cat]}>
                          {catOptions.map((option) => (
                            <option key={option.value} value={option.value}>
                              {option.label}{option.description ? ` — ${option.description}` : ""}
                            </option>
                          ))}
                        </optgroup>
                      );
                    })}
                  </select>
                  {options.length === 0 ? (
                    <span className="text-[11px] text-destructive block mt-2">No models loaded yet. Check provider configuration or refresh runtime models.</span>
                  ) : null}
                </label>
                {mode === "chat" && (
                  <label className="block space-y-2 w-[80px]">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Temp</span>
                    <input
                      type="number"
                      min="0"
                      max="2"
                      step="0.1"
                      value={temperature}
                      onChange={(e) => setTemperature(parseFloat(e.target.value))}
                      className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm font-mono text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all"
                    />
                  </label>
                )}
              </div>

              {/* Advanced inference parameters */}
              {mode === "chat" && (
                <div className="grid grid-cols-5 gap-3">
                  <label className="block space-y-2">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Max tokens</span>
                    <input
                      type="number" min="1" max="131072" step="1"
                      value={maxTokens}
                      onChange={(e) => setMaxTokens(Math.max(1, parseInt(e.target.value, 10) || 1))}
                      className="h-9 w-full rounded-md border border-border bg-background px-3 text-xs font-mono text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all"
                    />
                  </label>
                  <label className="block space-y-2">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Top P</span>
                    <input
                      type="number" min="0" max="1" step="0.05"
                      value={topP}
                      onChange={(e) => setTopP(Math.min(1, Math.max(0, parseFloat(e.target.value) || 0)))}
                      className="h-9 w-full rounded-md border border-border bg-background px-3 text-xs font-mono text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all"
                    />
                  </label>
                  <label className="block space-y-2">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Freq penalty</span>
                    <input
                      type="number" min="-2" max="2" step="0.1"
                      value={frequencyPenalty}
                      onChange={(e) => setFrequencyPenalty(Math.min(2, Math.max(-2, parseFloat(e.target.value) || 0)))}
                      className="h-9 w-full rounded-md border border-border bg-background px-3 text-xs font-mono text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all"
                    />
                  </label>
                  <label className="block space-y-2">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Pres penalty</span>
                    <input
                      type="number" min="-2" max="2" step="0.1"
                      value={presencePenalty}
                      onChange={(e) => setPresencePenalty(Math.min(2, Math.max(-2, parseFloat(e.target.value) || 0)))}
                      className="h-9 w-full rounded-md border border-border bg-background px-3 text-xs font-mono text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all"
                    />
                  </label>
                  <label className="block space-y-2">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Seed</span>
                    <input
                      type="number" min="0" max="999999" step="1"
                      value={seed ?? ""}
                      onChange={(e) => setSeed(e.target.value ? parseInt(e.target.value, 10) : undefined)}
                      className="h-9 w-full rounded-md border border-border bg-background px-3 text-xs font-mono text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all"
                      placeholder="—"
                    />
                  </label>
                </div>
              )}

              {/* Capability mode indicator */}
              <div className="flex items-center gap-2 text-[11px] font-bold uppercase tracking-wider">
                <span className="text-muted-foreground">Mode:</span>
                <span className={`inline-flex items-center gap-1.5  border px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-wider ${mode === "chat" ? "border-accent/30 bg-accent/10 text-accent" :
                    mode === "embeddings" ? "border-indigo-400/30 bg-indigo-500/10 text-indigo-400" :
                      "border-amber-400/30 bg-amber-500/10 text-amber-400"
                  }`}>
                  {mode === "chat" ? <Code className="h-3 w-3" /> : mode === "embeddings" ? <FileText className="h-3 w-3" /> : <ListOrdered className="h-3 w-3" />}
                  {MODE_LABELS[mode]}
                </span>
              </div>

              {/* Chat mode inputs */}
              {mode === "chat" && (
                <>
                  <label className="block space-y-2">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">System message</span>
                    <textarea
                      rows={4}
                      value={systemPrompt}
                      onChange={(event) => setSystemPrompt(event.target.value)}
                      className="w-full resize-y rounded-md border border-border bg-background px-4 py-3 text-sm text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all shadow-inner font-mono text-[13px]"
                    />
                  </label>

                  <label className="flex flex-col space-y-2 flex-1 min-h-[250px]">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">User message</span>
                    <textarea
                      value={prompt}
                      onChange={(event) => setPrompt(event.target.value)}
                      className="w-full flex-1 resize-y rounded-md border border-border bg-background px-4 py-3 text-sm text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all shadow-inner font-mono text-[13px]"
                    />
                  </label>
                </>
              )}

              {/* Embeddings mode inputs */}
              {mode === "embeddings" && (
                <>
                  <label className="block space-y-2">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Search query</span>
                    <input
                      value={embQuery}
                      onChange={(event) => setEmbQuery(event.target.value)}
                      placeholder="Deep learning frameworks"
                      className="w-full rounded-md border border-border bg-background px-4 py-3 text-sm text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all shadow-inner font-mono text-[13px]"
                    />
                  </label>
                  <label className="flex flex-col space-y-2 flex-1 min-h-[250px]">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Candidate texts <span className="font-normal normal-case tracking-normal text-muted-foreground/70">(one per line)</span>
                      {embExpectedLine != null && (
                        <span className="ml-2 text-[10px] font-bold px-1.5 py-0.5  bg-amber-500/10 text-amber-500 border border-amber-500/30">Expected nearest neighbor: Line {embExpectedLine}</span>
                      )}
                    </span>
                    <textarea
                      value={embInputs}
                      onChange={(event) => setEmbInputs(event.target.value)}
                      placeholder="TensorFlow architecture and graph computation"
                      className="w-full flex-1 resize-y rounded-md border border-border bg-background px-4 py-3 text-sm text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all shadow-inner font-mono text-[13px]"
                    />
                  </label>
                </>
              )}

              {/* Rerank mode inputs */}
              {mode === "rerank" && (
                <>
                  <label className="block space-y-2">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Search query</span>
                    <input
                      value={rerankQuery || prompt}
                      onChange={(event) => setRerankQuery(event.target.value)}
                      placeholder="What is the capital of France?"
                      className="w-full rounded-md border border-border bg-background px-4 py-3 text-sm text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all shadow-inner font-mono text-[13px]"
                    />
                  </label>

                  <label className="flex flex-col space-y-2 flex-1 min-h-[250px]">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Documents <span className="font-normal normal-case tracking-normal text-muted-foreground/70">(one per line)</span>
                      {rerankExpectedLine != null && (
                        <span className="ml-2 text-[10px] font-bold px-1.5 py-0.5  bg-amber-500/10 text-amber-500 border border-amber-500/30">Expected best match: Line {rerankExpectedLine}</span>
                      )}
                    </span>
                    <textarea
                      value={rerankDocs}
                      onChange={(event) => setRerankDocs(event.target.value)}
                      placeholder="Paris is the capital of France."
                      className="w-full flex-1 resize-y rounded-md border border-border bg-background px-4 py-3 text-sm text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all shadow-inner font-mono text-[13px] leading-relaxed"
                    />
                  </label>

                  <label className="block space-y-2 w-32">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Top N</span>
                    <input
                      type="number"
                      min="1"
                      max="100"
                      value={rerankTopN}
                      onChange={(e) => setRerankTopN(parseInt(e.target.value) || 5)}
                      className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm font-mono text-foreground outline-none focus:ring-2 focus:ring-ring focus:border-ring transition-all"
                    />
                  </label>
                </>
              )}

              {/* API curl example */}
              <details className="group mt-4 border rounded-md shrink-0">
                <summary className="cursor-pointer px-4 py-2 font-semibold text-[11px] bg-muted/20 hover:bg-muted/30 transition-colors list-none flex justify-between items-center outline-none uppercase tracking-wider text-muted-foreground">
                  <span className="flex items-center gap-2"><Code className="w-3.5 h-3.5" /> View API Request Code</span>
                  <span className="text-muted-foreground transition-transform duration-200 group-open:rotate-180">▼</span>
                </summary>
                <div className="p-0 border-t bg-black/90 relative group/copy">
                  <pre className="text-[11px] text-accent/80 p-4 font-mono overflow-x-auto whitespace-pre-wrap leading-relaxed select-all">
                    {generatedCurl}
                  </pre>
                  <button
                    type="button"
                    className="absolute top-2 right-2 p-1.5 rounded-md bg-white/10 hover:bg-white/20 text-white opacity-0 group-hover/copy:opacity-100 transition-opacity"
                    onClick={(e) => {
                      e.preventDefault();
                      navigator.clipboard.writeText(generatedCurl);
                    }}
                    title="Copy to clipboard"
                  >
                    <Terminal className="w-3.5 h-3.5" />
                  </button>
                </div>
              </details>
            </div>

            <div className="border-t bg-muted/30 p-4 flex flex-wrap items-center justify-between gap-3 mt-auto shrink-0">
              <span className="break-all font-mono text-xs text-muted-foreground bg-background px-2 py-1 rounded border">
                {gatewayEndpoint(endpointPath)}
              </span>
              <Button type="submit" disabled={!canSubmit} className="min-w-[140px]">
                {loading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Send className="mr-2 h-4 w-4" />}
                {loading ? "Sending..." : submitLabel}
              </Button>
            </div>
          </form>
        </Surface>

        <Surface className="p-0 overflow-hidden flex flex-col h-full border border-border">
          <div className="border-b bg-muted/20 p-5 shrink-0 flex items-center justify-between">
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-2">
                <div className="h-6 w-1  bg-success"></div>
                <h3 className="font-semibold text-lg tracking-tight">Response</h3>
              </div>
              <p className="text-sm text-muted-foreground ml-3">Inspect output payload and compute latency.</p>
            </div>
            {selectedModel && <span className="text-[10px] uppercase tracking-wider font-semibold text-muted-foreground bg-background border px-3 py-1.5 ">{selectedModel}</span>}
          </div>

          <div className="p-5 flex-1 flex flex-col space-y-4 overflow-y-auto">
            {/* Stats bar */}
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 shrink-0">
              <div className="rounded-lg border border-border bg-background/35 p-4 flex flex-col gap-1 shadow-inner">
                <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-wider font-bold text-muted-foreground">
                  <Code className="w-3.5 h-3.5" />
                  {mode === "rerank" ? "Prompt tokens" : "Tokens"}
                </div>
                <div className="mt-1 text-sm font-semibold text-foreground font-mono">
                  {stats
                    ? <span className="flex items-center justify-between">
                      <span title="Prompt">{formatTokens(stats.promptTokens)} <span className="text-muted-foreground font-sans text-xs font-normal">in</span></span>
                      {mode === "chat" && (
                        <>
                          <span className="text-muted-foreground/30">/</span>
                          <span title="Completion">{formatTokens(stats.completionTokens)} <span className="text-muted-foreground font-sans text-xs font-normal">out</span></span>
                        </>
                      )}
                      <span className="text-muted-foreground/30">/</span>
                      <span className="text-accent" title="Total">{formatTokens(stats.totalTokens)} <span className="text-muted-foreground font-sans text-xs font-normal">tot</span></span>
                    </span>
                    : <span className="text-muted-foreground/50">
                      {loading && mode === "chat" ? <>Streaming<span className="ml-1 inline-flex"><span className="animate-pulse">.</span><span className="animate-pulse delay-100">.</span><span className="animate-pulse delay-200">.</span></span></> : "No usage recorded"}
                    </span>}
                </div>
              </div>
              <div className="rounded-lg border border-border bg-background/35 p-4 flex flex-col gap-1 shadow-inner">
                <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-wider font-bold text-muted-foreground">
                  <Activity className="w-3.5 h-3.5" />
                  Throughput
                </div>
                <div className="mt-1 text-sm font-semibold text-foreground font-mono">
                  {stats
                    ? <span className="flex items-center justify-between">
                      <span className="text-success"><Clock className="w-3 h-3 inline mr-1 opacity-50" />{stats.latencyMs} <span className="text-muted-foreground font-sans text-[10px] font-normal">ms</span></span>
                      {stats.tokensPerSecond > 0 && (
                        <>
                          <span className="text-muted-foreground/30">|</span>
                          <span className="text-accent">{stats.tokensPerSecond.toFixed(1)} <span className="text-muted-foreground font-sans text-[10px] font-normal">tok/s</span></span>
                        </>
                      )}
                    </span>
                    : <span className="text-muted-foreground/50">Awaiting benchmark</span>}
                </div>
              </div>
            </div>

            {error ? <div className="rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive shrink-0">{error}</div> : null}
            {loading && !(responseText || reasoningText) ? (
              <div className="flex-1 flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border bg-muted/5 p-8 text-sm text-muted-foreground min-h-[300px]">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
                {mode === "chat" ? "Waiting for model response..." : mode === "embeddings" ? "Generating embeddings..." : "Reranking documents..."}
              </div>
            ) : null}

            {/* Chat result */}
            {mode === "chat" && (responseText || reasoningText) && (
              <div className="relative flex-1 flex flex-col min-h-[300px] group/resp">
                <div className="absolute top-3 left-3 z-10">
                  <div className="flex bg-background/80 backdrop-blur-sm border border-border/40 p-1">
                    <button
                      type="button"
                      onClick={() => setResponseView("preview")}
                      className={`px-3 py-1 text-[11px] font-semibold tracking-wider uppercase rounded-md transition-colors ${responseView === "preview" ? "bg-accent/15 text-accent" : "text-muted-foreground hover:bg-surface-hover/80 hover:text-foreground"}`}
                    >
                      Preview
                    </button>
                    <button
                      type="button"
                      onClick={() => setResponseView("raw")}
                      className={`px-3 py-1 text-[11px] font-semibold tracking-wider uppercase rounded-md transition-colors ${responseView === "raw" ? "bg-accent/15 text-accent" : "text-muted-foreground hover:bg-surface-hover/80 hover:text-foreground"}`}
                    >
                      Raw
                    </button>
                  </div>
                </div>

                <div className="flex-1 overflow-auto border border-border/60 bg-surface/50 p-5 pt-16 shadow-inner text-[14px] leading-relaxed text-foreground">
                  {reasoningText && (
                    <details open={loading} className="group mb-4 rounded-md border border-border/40 bg-muted/10">
                      <summary className="cursor-pointer px-3 py-2 text-[11px] font-bold uppercase tracking-wider text-muted-foreground hover:text-foreground transition-colors list-none flex items-center justify-between select-none">
                        <span className="flex items-center gap-2"><span className="w-1.5 h-1.5  bg-amber-500" /> Reasoning{loading ? " (streaming...)" : ""}</span>
                        <span className="text-muted-foreground/60 transition-transform duration-200 group-open:rotate-180">▼</span>
                      </summary>
                      <div className="px-3 pb-3 text-[13px] text-muted-foreground/90 border-t border-border/30 pt-2 italic whitespace-pre-wrap">
                        {reasoningText}
                        {loading && <span className="inline-block w-1 h-4 bg-amber-500/50 animate-pulse ml-0.5 align-middle" />}
                      </div>
                    </details>
                  )}
                  {responseText && responseView === "preview" ? (
                    <div className="prose prose-sm dark:prose-invert max-w-none prose-pre:bg-background/80 prose-pre:border prose-pre:border-border/40 prose-pre:shadow-inner prose-a:text-accent prose-headings:tracking-tight font-sans">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{responseText}</ReactMarkdown>
                      {loading && <span className="inline-block w-1.5 h-5 bg-foreground/50 animate-pulse ml-0.5 align-text-bottom" />}
                    </div>
                  ) : responseText && responseView === "raw" ? (
                    <pre className="font-mono text-[13px] whitespace-pre-wrap">{responseText}{loading && <span className="inline-block w-1.5 h-5 bg-foreground/50 animate-pulse ml-0.5" />}</pre>
                  ) : null}
                </div>

                <button
                  type="button"
                  className="absolute top-3 right-3 p-1.5 bg-background/80 hover:bg-background border text-foreground opacity-0 group-hover/resp:opacity-100 transition-opacity backdrop-blur-sm z-10"
                  onClick={() => navigator.clipboard.writeText(reasoningText ? `Reasoning:\n${reasoningText}\n\nResponse:\n${responseText}` : responseText)}
                  title="Copy response"
                >
                  <Terminal className="w-3.5 h-3.5" />
                </button>
              </div>
            )}

            {/* Embeddings result */}
            {mode === "embeddings" && embResult && !loading && (
              <div className="flex-1 flex flex-col min-h-[300px] gap-3">
                <div className="flex items-center gap-2 text-[12px] font-semibold text-muted-foreground">
                  <FileText className="h-4 w-4" />
                  {embResult.data?.length ?? 0} vector{(embResult.data?.length ?? 0) !== 1 ? "s" : ""} generated
                  {embResult.data?.[0]?.embedding && (
                    <span className="text-muted-foreground/60">
                      · {embResult.data[0].embedding.length} dimensions
                    </span>
                  )}
                  {embExpectedLine != null && embSimilarities?.[0] != null && (
                    <span className={`ml-auto text-[10px] font-bold px-1.5 py-0.5  border ${embSimilarities[0].line === embExpectedLine ? "bg-success/10 text-success border-success/30" : "bg-destructive/10 text-destructive border-destructive/30"}`}>
                      {embSimilarities[0].line === embExpectedLine ? "✓ Correct" : `✗ Expected Line ${embExpectedLine}`}
                    </span>
                  )}
                </div>
                {embSimilarities && embSimilarities.length > 0 ? (
                  <div className="flex-1 overflow-auto border border-border/60 bg-surface/50 p-4 shadow-inner space-y-3">
                    {embSimilarities.map((item, idx) => {
                      const isTop = idx === 0;
                      const scorePct = Math.round(item.similarity * 100);
                      const barWidth = Math.max(4, scorePct);
                      return (
                        <div key={idx} className={`border p-3 ${isTop ? "border-accent/40 bg-accent/[0.04]" : "border-border/40 bg-background/40"}`}>
                          <div className="flex items-center justify-between mb-2">
                            <span className="flex items-center gap-2">
                              <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">{isTop ? "Best match" : `Line ${item.line}`}</span>
                              {isTop && <span className="text-[10px] font-bold px-1.5 py-0.5  bg-accent/10 text-accent border border-accent/30">Top</span>}
                            </span>
                            <span className="font-mono text-[13px] font-bold text-foreground">{scorePct}%</span>
                          </div>
                          <div className="h-2  bg-background/60 border border-border/30 overflow-hidden mb-2">
                            <div
                              className={`h-full  transition-all ${isTop ? "bg-accent" : "bg-accent/60"}`}
                              style={{ width: `${barWidth}%` }}
                            />
                          </div>
                          <p className="text-[13px] text-foreground/80 leading-relaxed">{item.text}</p>
                        </div>
                      );
                    })}
                  </div>
                ) : (
                  <div className="flex-1 overflow-auto border border-border/60 bg-surface/50 shadow-inner">
                    <pre className="font-mono text-[12px] p-4 whitespace-pre-wrap text-foreground/90">
                      {JSON.stringify(embResult, null, 2)}
                    </pre>
                  </div>
                )}
              </div>
            )}

            {/* Rerank result */}
            {mode === "rerank" && rerankResult && !loading && (
              <div className="flex-1 flex flex-col min-h-[300px] gap-3">
                <div className="flex items-center gap-2 text-[12px] font-semibold text-muted-foreground">
                  <ListOrdered className="h-4 w-4" />
                  {rerankResult.results?.length ?? 0} result{(rerankResult.results?.length ?? 0) !== 1 ? "s" : ""} ranked
                  {rerankExpectedLine != null && rerankResult.results?.[0] != null && (
                    <span className={`ml-auto text-[10px] font-bold px-1.5 py-0.5  border ${rerankResult.results[0].index + 1 === rerankExpectedLine ? "bg-success/10 text-success border-success/30" : "bg-destructive/10 text-destructive border-destructive/30"}`}>
                      {rerankResult.results[0].index + 1 === rerankExpectedLine ? "✓ Correct" : `✗ Expected Line ${rerankExpectedLine}`}
                    </span>
                  )}
                </div>
                <div className="flex-1 overflow-auto border border-border/60 bg-surface/50 p-4 shadow-inner space-y-3">
                  {(rerankResult.results ?? []).map((result, idx) => {
                    const scorePct = Math.round(result.relevance_score * 100);
                    const barWidth = Math.max(4, scorePct);
                    const isTop = idx === 0;
                    const originalLine = (rerankDocs.split("\n").map((l) => l.trim()).filter(Boolean)[result.index] ?? "").replace(/^\d+\.\s*/, '');
                    return (
                      <div key={idx} className={`border p-3 ${isTop ? "border-accent/40 bg-accent/[0.04]" : "border-border/40 bg-background/40"}`}>
                        <div className="flex items-center justify-between mb-2">
                          <span className="flex items-center gap-2">
                            <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">{isTop ? "Best match" : `Line ${result.index + 1}`}</span>
                            {isTop && <span className="text-[10px] font-bold px-1.5 py-0.5  bg-accent/10 text-accent border border-accent/30">Top</span>}
                          </span>
                          <span className="font-mono text-[13px] font-bold text-foreground">{scorePct}%</span>
                        </div>
                        <div className="h-2  bg-background/60 border border-border/30 overflow-hidden mb-2">
                          <div
                            className={`h-full  transition-all ${isTop ? "bg-accent" : "bg-accent/60"}`}
                            style={{ width: `${barWidth}%` }}
                          />
                        </div>
                        {(result.document?.text || originalLine) && (
                          <p className="text-[13px] text-foreground/80 leading-relaxed">{result.document?.text || originalLine}</p>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            )}

            {/* Empty state */}
            {!loading && !error && !responseText && !embResult && !rerankResult ? (
              <div className="flex-1 flex items-center justify-center rounded-lg border border-dashed border-border bg-muted/5 p-8 min-h-[300px]">
                <EmptyState title={`Ready to test ${mode === "chat" ? "a chat" : mode === "embeddings" ? "an embeddings" : "a rerank"} model`}>
                  {mode === "chat"
                    ? "Select a chat model, write a prompt, and send to see the response with token usage, latency, and throughput."
                    : mode === "embeddings"
                      ? "Select an embedding model, provide a query and candidate texts, and see which candidate has the most similar embedding."
                      : "Select a rerank model, provide a query and documents, and see relevance scores."}
                </EmptyState>
              </div>
            ) : null}
          </div>
        </Surface>
      </div>
    </div>
  );
}

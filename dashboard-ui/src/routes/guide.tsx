import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { Copy, ExternalLink, Layers, Loader2, Play, RefreshCw, Terminal } from "lucide-react";
import { SiClaude, SiOpenaigym } from "react-icons/si";
import ampImage from "@/assets/providers/amp.png";
import claudeImage from "@/assets/providers/claude.png";
import clineImage from "@/assets/providers/cline.png";
import codexImage from "@/assets/providers/codex.png";
import continueImage from "@/assets/providers/continue.png";
import cursorImage from "@/assets/providers/cursor.png";
import deepseekTUIImage from "@/assets/providers/deepseek-tui.png";
import droidImage from "@/assets/providers/droid.png";
import hermesImage from "@/assets/providers/hermes.png";
import jcodeImage from "@/assets/providers/jcode.png";
import kilocodeImage from "@/assets/providers/kilocode.png";
import openclawImage from "@/assets/providers/openclaw.png";
import opencodeImage from "@/assets/providers/opencode.png";
import qwenImage from "@/assets/providers/qwen.png";
import rooImage from "@/assets/providers/roo.png";
import { CLIModelFieldGrid } from "@/components/cli-tools/CLIModelFieldGrid";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { PageHeader, Kicker } from "@/components/ui/page-header";
import { CodeBlock, Pill, SectionHeader, Surface } from "@/components/ui/surface";
import { DataTable, TableWrap, Td, Th } from "@/components/ui/data-table";
import { RequirePermission } from "@/components/shell/RequirePermission";
import { useClipboardButton } from "@/lib/clipboard/useClipboardButton";
import { applyCLITool, fetchCLITools, previewCLITool, type CLIPreviewResponse, type CLITool } from "@/lib/api/cli-tools";
import {
  ANTHROPIC_ENDPOINT_ROWS,
  buildPlaygroundRequestBody,
  buildPoolEmbeddingsBody,
  gatewayCurlExample,
  gatewayEndpoint,
  GATEWAY_FEATURE_ROWS,
  INFRA_ENDPOINT_ROWS,
  OPENAI_ENDPOINT_ROWS,
  POOL_CALL_STYLE_DOCS,
  POOL_USAGE_NOTES,
  poolEmbeddingsCurlExample,
} from "@/lib/gateway/guide";
import { usePools } from "@/lib/api/usePools";
import { useModels } from "@/lib/api/useModels";
import type { PoolSnapshot } from "@/lib/api/pools-types";
import { buildCLIPreviewRequest } from "@/lib/cli-tools/model-fields";
import { modelDisplayName, type ModelInventoryItem } from "@/lib/api/models-types";

const DEFAULT_CURL_BODY = buildPlaygroundRequestBody({
  model: "gpt-4o-mini",
  systemPrompt: "You are a concise assistant.",
  userPrompt: "Say hello from aurora and mention the provider you used.",
});

const POOL_MODEL_PLACEHOLDER = "your-model-id";

interface CLIFormState {
  base_url: string;
  api_key: string;
  model: string;
  model_overrides: Record<string, string>;
  selectedModels?: string[];
}

const TOOL_IMAGE_BY_ID: Record<string, string> = {
  "claude-code": claudeImage,
  amp: ampImage,
  cline: clineImage,
  codex: codexImage,
  continue: continueImage,
  cursor: cursorImage,
  "deepseek-tui": deepseekTUIImage,
  droid: droidImage,
  hermes: hermesImage,
  jcode: jcodeImage,
  kilo: kilocodeImage,
  openclaw: openclawImage,
  opencode: opencodeImage,
  qwen: qwenImage,
  roo: rooImage,
};

function toolImageSrc(tool: CLITool): string {
  return TOOL_IMAGE_BY_ID[tool.id] ?? "";
}

function toolIcon(tool: CLITool): JSX.Element {
  const key = `${tool.id} ${tool.name}`.toLowerCase();
  if (key.includes("claude") || key.includes("anthropic")) {
    return <SiClaude className="h-7 w-7 text-accent" />;
  }
  if (key.includes("openai") || key.includes("gpt")) {
    return <SiOpenaigym className="h-7 w-7 text-accent" />;
  }
  return <Terminal className="h-7 w-7 text-accent" />;
}

function ToolMark({ tool }: { tool: CLITool }): JSX.Element {
  const [failed, setFailed] = useState(false);
  const imageSrc = toolImageSrc(tool);
  if (imageSrc && !failed) {
    return <img src={imageSrc} alt="" className="h-9 w-9 rounded object-contain" loading="lazy" onError={() => setFailed(true)} />;
  }
  return toolIcon(tool);
}

function findPoolExampleModel(pool: PoolSnapshot, models: ModelInventoryItem[] | undefined): string {
  if (!models || models.length === 0) {
    return POOL_MODEL_PLACEHOLDER;
  }
  const knownNames = new Set([
    ...pool.members.map((member) => member.provider_name.trim()).filter(Boolean),
    pool.name.trim(),
  ]);
  for (const item of models) {
    const providerName = (item.provider_name ?? "").trim();
    const modelId = (item.model?.id ?? "").trim();
    if (providerName && modelId && knownNames.has(providerName)) {
      return modelId;
    }
  }
  return POOL_MODEL_PLACEHOLDER;
}

interface PoolCallExampleProps {
  pool: PoolSnapshot;
  exampleModelId: string;
}

function PoolCallExamples({ pool, exampleModelId }: PoolCallExampleProps): JSX.Element {
  const prefixedCopy = useClipboardButton({ logPrefix: "Failed to copy pool prefixed example:" });
  const providerHintCopy = useClipboardButton({ logPrefix: "Failed to copy pool provider-hint example:" });

  const prefixedCurl = poolEmbeddingsCurlExample(buildPoolEmbeddingsBody("prefixed", pool.name, exampleModelId));
  const providerHintCurl = poolEmbeddingsCurlExample(buildPoolEmbeddingsBody("provider_hint", pool.name, exampleModelId));

  const [prefixedDoc, providerHintDoc] = POOL_CALL_STYLE_DOCS;
  if (!prefixedDoc || !providerHintDoc) {
    return <p className="text-sm text-muted-foreground">Pool usage docs are unavailable.</p>;
  }

  const examples: Array<{
    style: (typeof POOL_CALL_STYLE_DOCS)[number]["style"];
    label: string;
    description: string;
    curl: string;
    copy: ReturnType<typeof useClipboardButton>;
  }> = [
      {
        style: prefixedDoc.style,
        label: prefixedDoc.label,
        description: prefixedDoc.description,
        curl: prefixedCurl,
        copy: prefixedCopy,
      },
      {
        style: providerHintDoc.style,
        label: providerHintDoc.label,
        description: providerHintDoc.description,
        curl: providerHintCurl,
        copy: providerHintCopy,
      },
    ];

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
      {examples.map((example) => (
        <div key={example.style} className="flex flex-col gap-2 rounded-lg border border-border bg-background/30 p-4">
          <div className="flex items-center gap-2">
            <Pill tone="accent" className="font-mono">
              {example.style}
            </Pill>
            <strong className="text-sm text-foreground">{example.label}</strong>
          </div>
          <p className="text-sm leading-6 text-muted-foreground">{example.description}</p>
          <CodeBlock>{example.curl}</CodeBlock>
          <div>
            <Button variant="secondary" onClick={() => example.copy.copy(example.curl)}>
              <Copy className="h-4 w-4" />
              {example.copy.copied ? "Copied" : "Copy cURL"}
            </Button>
          </div>
        </div>
      ))}
    </div>
  );
}

interface PoolCardProps {
  pool: PoolSnapshot;
  exampleModelId: string;
}

function PoolCard({ pool, exampleModelId }: PoolCardProps): JSX.Element {
  const healthyMembers = pool.members.filter((member) => member.healthy).length;
  const exampleIsPlaceholder = exampleModelId === POOL_MODEL_PLACEHOLDER;
  return (
    <div className="flex flex-col gap-4 rounded-lg border border-border bg-background/20 p-4">
      <div className="flex flex-wrap items-center gap-2">
        <Layers className="h-4 w-4 text-accent" />
        <code className="rounded bg-background/60 px-2 py-1 font-mono text-sm text-foreground">{pool.name}</code>
        <Pill tone="accent" className="font-mono text-xs">
          {pool.strategy}
        </Pill>
        <span className="text-xs text-muted-foreground">
          {healthyMembers}/{pool.members.length} healthy members
        </span>
      </div>
      {exampleIsPlaceholder ? (
        <p className="text-xs leading-5 text-muted-foreground">
          No model id from this pool was auto-detected. Replace <code className="font-mono">{POOL_MODEL_PLACEHOLDER}</code> in the examples below with a model id that the pool&apos;s members serve — check the Models page or the provider&apos;s documentation for available model ids.
        </p>
      ) : (
        <p className="text-xs leading-5 text-muted-foreground">
          Examples use <code className="font-mono">{exampleModelId}</code>, which is served by this pool. Swap it for any model id that the pool&apos;s members serve.
        </p>
      )}
      <PoolCallExamples pool={pool} exampleModelId={exampleModelId} />
    </div>
  );
}

function CLIToolsGuideSection(): JSX.Element {
  const [tools, setTools] = useState<CLITool[]>([]);
  const [selected, setSelected] = useState("");
  const [configOpen, setConfigOpen] = useState(false);
  const [form, setForm] = useState<CLIFormState>({ base_url: window.location.origin, api_key: "", model: "", model_overrides: {} });
  const [formVersion, setFormVersion] = useState(0);
  const [preview, setPreview] = useState<CLIPreviewResponse | null>(null);
  const [previewKey, setPreviewKey] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const models = useModels();
  const modelOptions = useMemo(() => Array.from(new Set((models.data ?? []).map(modelDisplayName).filter(Boolean))).sort(), [models.data]);

  function invalidatePreview(): void {
    setFormVersion((currentVersion) => currentVersion + 1);
    setPreview(null);
    setPreviewKey("");
  }

  function updateForm(patch: Partial<CLIFormState>): void {
    setForm((currentForm) => ({ ...currentForm, ...patch }));
    invalidatePreview();
  }

  function updateModelOverride(key: string, value: string): void {
    setForm((currentForm) => ({
      ...currentForm,
      model_overrides: { ...currentForm.model_overrides, [key]: value },
    }));
    invalidatePreview();
  }

  function selectTool(toolID: string): void {
    setSelected(toolID);
    setForm((currentForm) => {
      const reset = { ...currentForm, model_overrides: {} };
      delete (reset as Record<string, unknown>).selectedModels;
      return reset;
    });
    setConfigOpen(true);
    invalidatePreview();
  }

  const load = useCallback(async (): Promise<void> => {
    try {
      setLoading(true);
      setError("");
      const list = await fetchCLITools();
      setTools(list);
      setSelected((current) => current || list[0]?.id || "");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load CLI tools.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    if (form.model || modelOptions.length === 0) return;
    setForm((currentForm) => ({ ...currentForm, model: modelOptions[0] ?? "" }));
  }, [form.model, modelOptions]);

  const current = tools.find((tool) => tool.id === selected) ?? null;
  const currentPreviewKey = `${selected}:${formVersion}`;
  const canApply = Boolean(current?.can_apply && preview && previewKey === currentPreviewKey);
  const applyDisabledReason = !current
    ? "Select a CLI tool first."
    : !current.can_apply
      ? "Host apply is disabled for this tool or this gateway. Enable CLI_TOOLS_APPLY_ENABLED=true to allow supported tools to write config on the gateway host."
      : !preview
        ? "Preview this configuration before applying it on the gateway host."
        : previewKey !== currentPreviewKey
          ? "Preview again after changing the URL, key, or model."
          : "";

  async function runPreview(): Promise<void> {
    if (!selected) return;
    try {
      setBusy(true);
      setError("");
      setNotice("");
      setPreview(await previewCLITool(selected, buildCLIPreviewRequest(form)));
      setPreviewKey(currentPreviewKey);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to preview tool config.");
    } finally {
      setBusy(false);
    }
  }

  async function runApply(): Promise<void> {
    if (!selected) return;
    if (!canApply) {
      setError("Preview this exact CLI configuration before applying it on the gateway host.");
      return;
    }
    if (!window.confirm("Apply this Aurora CLI config on the gateway host?")) return;
    try {
      setBusy(true);
      setError("");
      setNotice("");
      const resp = await applyCLITool(selected, buildCLIPreviewRequest(form));
      updateForm({ api_key: "" });
      setNotice(`Applied to ${resp.path}${resp.backup_path ? ` (backup: ${resp.backup_path})` : ""}.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to apply tool config.");
    } finally {
      setBusy(false);
    }
  }

  async function copySnippet(value: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(value);
      setNotice("Copied snippet to clipboard.");
    } catch {
      setError("Unable to copy snippet. Select and copy it manually.");
    }
  }

  return (
    <Surface className="min-w-0 overflow-hidden p-5 xl:col-span-2">
      <SectionHeader
        title="CLI tools configurator"
        subtitle="Generate and optionally apply Aurora Gateway configuration for developer CLI clients."
        className="mb-4"
      />
      <div className="mb-4 flex justify-end">
        <Button variant="secondary" onClick={() => void load()} disabled={loading}>
          {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
          Refresh
        </Button>
      </div>
      {error ? <CLIAlert tone="warning">{error}</CLIAlert> : null}
      {notice ? <CLIAlert tone="success">{notice}</CLIAlert> : null}
      {loading ? (
        <div className="flex items-center gap-2 rounded-lg border border-border bg-background/25 p-4 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading CLI tools...
        </div>
      ) : tools.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border bg-background/20 p-4 text-sm text-muted-foreground">
          No CLI tools are available. Enable CLI tools in gateway config.
        </div>
      ) : (
        <div className="flex min-w-0 flex-col gap-5">
          <div className="grid min-w-0 gap-3 md:grid-cols-2 xl:grid-cols-3">
            {tools.map((tool) => {
              const isSelected = selected === tool.id;
              return (
                <button
                  key={tool.id}
                  type="button"
                  onClick={() => selectTool(tool.id)}
                  className={`flex min-h-40 min-w-0 flex-col justify-between rounded-xl border p-4 text-left transition ${isSelected ? "border-accent bg-accent/10 shadow-sm" : "border-border bg-background/35 hover:border-accent/50 hover:bg-surface-hover"}`}
                >
                  <span className="flex min-w-0 flex-col gap-3">
                    <span className="flex min-w-0 items-start justify-between gap-3">
                      <span className="flex min-w-0 items-center gap-3">
                        <span className="grid h-14 w-14 shrink-0 place-items-center rounded-xl border border-border/50 bg-muted/20">
                          <ToolMark tool={tool} />
                        </span>
                        <span className="min-w-0">
                          <span className="block truncate font-semibold text-foreground">{tool.name}</span>
                          {tool.config_type ? <span className="mt-1 block text-xs uppercase tracking-wide text-muted-foreground">{tool.config_type}</span> : null}
                        </span>
                      </span>
                      {tool.can_apply ? <Pill tone="accent">Apply</Pill> : <Pill tone="muted">Preview</Pill>}
                    </span>
                    <span className="break-words text-xs leading-5 text-muted-foreground">{tool.description}</span>
                  </span>
                  <span className="mt-4 flex min-w-0 flex-wrap items-center gap-2 text-xs text-muted-foreground">
                    {tool.model_fields?.length ? <Pill tone="accent">{tool.model_fields.length} model fields</Pill> : <Pill tone="muted">Fallback model</Pill>}
                    {tool.default_command ? <Pill tone="muted">{tool.default_command}</Pill> : null}
                    <span className="ml-auto text-accent">{isSelected ? "Selected" : "Configure"}</span>
                  </span>
                </button>
              );
            })}
          </div>

          <Dialog open={Boolean(current && configOpen)} onOpenChange={setConfigOpen}>
            <DialogContent className="max-h-[90vh] min-w-0 max-w-[calc(100vw-2rem)] overflow-y-auto sm:max-w-[920px]">
              {current ? (
                <div className="flex min-w-0 flex-col gap-5">
                  <DialogHeader className="min-w-0">
                    <div className="flex min-w-0 items-start gap-3 pr-8">
                      <span className="grid h-16 w-16 shrink-0 place-items-center rounded-xl border border-border/50 bg-muted/20">
                        <ToolMark tool={current} />
                      </span>
                      <div className="min-w-0 space-y-2">
                        <DialogTitle className="break-words">Configure {current.name}</DialogTitle>
                        <DialogDescription className="break-words">{current.description}</DialogDescription>
                        <div className="flex min-w-0 flex-wrap items-center gap-2">
                          {current.config_type ? <Pill tone="muted">{current.config_type}</Pill> : null}
                          {current.default_command ? <Pill tone="accent">{current.default_command}</Pill> : null}
                          {current.can_apply ? <Pill tone="accent">Host apply available</Pill> : <Pill tone="muted">Preview only</Pill>}
                          {current.docs_url ? (
                            <a className="inline-flex items-center gap-1 text-xs text-accent hover:underline" href={current.docs_url} target="_blank" rel="noreferrer">
                              Docs
                              <ExternalLink className="h-3 w-3" />
                            </a>
                          ) : null}
                        </div>
                        {current.config_path ? <p className="min-w-0 break-words text-xs text-muted-foreground">Apply path: <code className="break-all font-mono text-foreground">{current.config_path}</code></p> : null}
                      </div>
                    </div>
                  </DialogHeader>

                  <div className="grid min-w-0 gap-4 rounded-xl border border-border bg-background/25 p-4 md:grid-cols-3">
                    <CLIField label="Gateway base URL">
                      <input className="field-input min-w-0 font-mono" value={form.base_url} onChange={(e) => updateForm({ base_url: e.target.value })} />
                    </CLIField>
                    <CLIField label="API key">
                      <input type="password" autoComplete="new-password" className="field-input min-w-0 font-mono" value={form.api_key} onChange={(e) => updateForm({ api_key: e.target.value })} placeholder="Only sent when applying" />
                    </CLIField>
                    <CLIField label={current.model_fields?.length ? "Fallback model" : "Model or combo"}>
                      <select className="field-input min-w-0 max-w-full truncate font-mono" value={form.model} onChange={(e) => updateForm({ model: e.target.value })}>
                        <option value={form.model}>{form.model}</option>
                        {modelOptions.filter((model) => model !== form.model).map((model) => <option key={model} value={model}>{model}</option>)}
                      </select>
                    </CLIField>
                    {current.model_fields?.length ? (
                      <CLIModelFieldGrid
                        modelFields={current.model_fields}
                        modelOverrides={form.model_overrides}
                        fallbackModel={form.model}
                        models={models.data ?? []}
                        onChange={updateModelOverride}
                        selectedModels={form.selectedModels}
                        onSelectedModelsChange={(models) => setForm((f) => ({ ...f, selectedModels: models, model: models[0] || f.model }))}
                      />
                    ) : null}
                    {current.notes?.length ? (
                      <div className="min-w-0 rounded-lg border border-border bg-background/25 p-3 md:col-span-3">
                        <ul className="list-disc space-y-1 break-words pl-5 text-xs leading-5 text-muted-foreground">
                          {current.notes.map((note) => <li key={note}>{note}</li>)}
                        </ul>
                      </div>
                    ) : null}
                    {current.can_apply && preview && !canApply ? (
                      <p className="min-w-0 break-words text-xs text-warning md:col-span-3">Preview again after changing the tool, URL, key, or model before applying on the host.</p>
                    ) : null}
                    <div className="flex min-w-0 flex-wrap items-center gap-2 md:col-span-3">
                      <Button onClick={() => void runPreview()} disabled={busy || !current}>
                        {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
                        Preview
                      </Button>
                      <Button variant="secondary" onClick={() => void runApply()} disabled={busy || !canApply}>Apply on host</Button>
                      {!canApply && applyDisabledReason ? <span className="min-w-0 flex-1 break-words text-xs leading-5 text-muted-foreground">{applyDisabledReason}</span> : null}
                    </div>
                  </div>

                  {preview ? (
                    <div className="grid min-w-0 gap-4">
                      {Object.entries(preview.snippets).map(([kind, value]) => (
                        <div key={kind} className="min-w-0 overflow-hidden rounded-lg border border-border bg-background/20">
                          <div className="flex min-w-0 items-center justify-between gap-2 border-b border-border/40 px-4 py-2">
                            <span className="min-w-0 truncate text-sm font-medium uppercase tracking-wide text-muted-foreground">{kind}</span>
                            <Button variant="ghost" size="sm" onClick={() => void copySnippet(value)} className="shrink-0">
                              <Copy className="h-4 w-4" />
                              Copy
                            </Button>
                          </div>
                          <pre className="max-h-80 max-w-full overflow-auto p-4 text-xs text-foreground"><code>{value}</code></pre>
                        </div>
                      ))}
                    </div>
                  ) : null}
                </div>
              ) : null}
            </DialogContent>
          </Dialog>
        </div>
      )}
    </Surface>
  );
}

function CLIField({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return <label className="block min-w-0 space-y-2"><span className="text-xs font-medium text-muted-foreground">{label}</span>{children}</label>;
}

function CLIAlert({ children, tone }: { children: string; tone: "warning" | "success" }): JSX.Element {
  return <div className={`mb-4 min-w-0 break-words rounded-lg border px-4 py-3 text-sm ${tone === "warning" ? "border-warning/30 bg-warning/15 text-warning" : "border-success/30 bg-success/15 text-success"}`}>{children}</div>;
}

function PoolUsageSection(): JSX.Element {
  const pools = usePools();
  const models = useModels();

  const poolsList = pools.data?.pools ?? [];

  let body: JSX.Element;
  if (pools.isLoading) {
    body = <p className="text-sm text-muted-foreground">Loading configured pools�</p>;
  } else if (pools.error) {
    body = (
      <p className="text-sm text-destructive">
        Failed to load pools: {pools.error.message}
      </p>
    );
  } else if (poolsList.length === 0) {
    body = (
      <div className="rounded-lg border border-dashed border-border bg-background/20 p-4 text-sm text-muted-foreground">
        No provider pools are configured on this gateway. Pools group two or more configured providers that share an upstream and can serve the same models � define them under <code className="font-mono">pools</code> in the gateway config to enable load balancing and automatic failover. See the docs for examples.
      </div>
    );
  } else {
    body = (
      <div className="flex flex-col gap-4">
        {poolsList.map((pool) => (
          <PoolCard key={pool.name} pool={pool} exampleModelId={findPoolExampleModel(pool, models.data)} />
        ))}
        <ul className="list-disc space-y-2 pl-5 text-xs leading-5 text-muted-foreground">
          {POOL_USAGE_NOTES.map((note) => (
            <li key={note}>{note}</li>
          ))}
        </ul>
      </div>
    );
  }

  return (
    <Surface className="p-5 xl:col-span-2">
      <SectionHeader
        title="Provider pools"
        subtitle="Load-balanced groups of providers. Address them via a pool-prefixed model or a separate provider field."
        className="mb-4"
      />
      {body}
    </Surface>
  );
}

export function GuidePage(): JSX.Element {
  const baseCopy = useClipboardButton({ logPrefix: "Failed to copy base URL:" });
  const curlCopy = useClipboardButton({ logPrefix: "Failed to copy cURL:" });
  const baseUrl = gatewayEndpoint("/v1");
  const curl = gatewayCurlExample(DEFAULT_CURL_BODY as unknown as Record<string, unknown>);

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Gateway Guide"
        subtitle="Configured URLs and request patterns for this running gateway."
      />

      <Surface className="relative overflow-hidden p-6" variant="elevated">
        <div className="absolute inset-y-0 right-0 w-1/2 bg-gradient-to-l from-accent/10 to-transparent" />
        <div className="relative flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div className="min-w-0 space-y-2">
            <Kicker>Configured base URL</Kicker>
            <h3 className="break-all font-mono text-2xl font-semibold tracking-tight text-foreground">
              {baseUrl}
            </h3>
            <p className="max-w-2xl text-sm leading-6 text-muted-foreground">
              Use this as the OpenAI- and Anthropic-compatible base URL in SDKs and tools. The endpoint reflects the current host and dashboard base path.
            </p>
          </div>
          <Button onClick={() => baseCopy.copy(baseUrl)} className="shrink-0">
            <Copy className="h-4 w-4" />
            {baseCopy.copied ? "Copied" : "Copy base URL"}
          </Button>
        </div>
      </Surface>

      <div className="grid grid-cols-1 gap-5 xl:grid-cols-2">
        <Surface className="p-5 xl:col-span-2">
          <SectionHeader title="Endpoints" subtitle="Routes exposed by the gateway from this deployment." className="mb-4" />
          <div className="mb-6">
            <h4 className="mb-3 text-sm font-bold uppercase tracking-wider text-accent">OpenAI-compatible</h4>
            <TableWrap>
              <DataTable>
                <thead>
                  <tr>
                    <Th>Method</Th>
                    <Th>Endpoint</Th>
                    <Th>Feature</Th>
                    <Th>Description</Th>
                  </tr>
                </thead>
                <tbody>
                  {OPENAI_ENDPOINT_ROWS.map((row) => (
                    <tr key={`openai:${row.method}:${row.path}`} className="hover:bg-surface-hover/50">
                      <Td>
                        <Pill tone="accent" className="font-mono">{row.method}</Pill>
                      </Td>
                      <Td>
                        <code className="break-all rounded bg-background/60 px-2 py-1 font-mono text-xs text-foreground">
                          {gatewayEndpoint(row.path)}
                        </code>
                      </Td>
                      <Td className="font-medium">{row.label}</Td>
                      <Td className="text-muted-foreground">{row.description}</Td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
            </TableWrap>
          </div>

          <div className="mb-6">
            <h4 className="mb-3 text-sm font-bold uppercase tracking-wider text-accent">Anthropic-compatible</h4>
            <p className="mb-2 text-xs leading-5 text-muted-foreground">
              The <code className="font-mono">/v1/messages</code> endpoint accepts native Anthropic-format request bodies, translates them to the gateway&apos;s internal format, dispatches to <strong>any configured provider</strong> (OpenAI, Groq, etc.), and converts the response back to Anthropic format. This allows Anthropic SDKs and Claude Code to route through the gateway to any upstream provider.
            </p>
            <p className="mb-2 text-xs leading-5 text-muted-foreground">
              Authentication accepts either <code className="font-mono">Authorization: Bearer &lt;key&gt;</code> or <code className="font-mono">x-api-key: &lt;key&gt;</code>. Set the <code className="font-mono">anthropic-version: 2023-06-01</code> header as required by the Anthropic API spec.
            </p>
            <p className="mb-3 text-xs leading-5 text-muted-foreground">
              Enable by setting <code className="font-mono">ENABLE_ANTHROPIC_INGRESS=true</code> in your <code className="font-mono">.env</code> or <code className="font-mono">enable_anthropic_ingress: true</code> in your config YAML.
            </p>

            <TableWrap>
              <DataTable>
                <thead>
                  <tr>
                    <Th>Method</Th>
                    <Th>Endpoint</Th>
                    <Th>Feature</Th>
                    <Th>Description</Th>
                  </tr>
                </thead>
                <tbody>
                  {ANTHROPIC_ENDPOINT_ROWS.map((row) => (
                    <tr key={`anthropic:${row.method}:${row.path}`} className="hover:bg-surface-hover/50">
                      <Td>
                        <Pill tone="accent" className="font-mono">{row.method}</Pill>
                      </Td>
                      <Td>
                        <code className="break-all rounded bg-background/60 px-2 py-1 font-mono text-xs text-foreground">
                          {gatewayEndpoint(row.path)}
                        </code>
                      </Td>
                      <Td className="font-medium">{row.label}</Td>
                      <Td className="text-muted-foreground">{row.description}</Td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
            </TableWrap>
          </div>

          <div>
            <h4 className="mb-3 text-sm font-bold uppercase tracking-wider text-accent">Infrastructure &amp; shared</h4>
            <p className="mb-2 text-xs leading-5 text-muted-foreground">
              Endpoints shared across SDK types. <code className="font-mono">/v1/models</code> serves both OpenAI-format and Anthropic-format model lists depending on the client.
            </p>
            <TableWrap>
              <DataTable>
                <thead>
                  <tr>
                    <Th>Method</Th>
                    <Th>Endpoint</Th>
                    <Th>Feature</Th>
                    <Th>Description</Th>
                  </tr>
                </thead>
                <tbody>
                  {INFRA_ENDPOINT_ROWS.map((row) => (
                    <tr key={`infra:${row.method}:${row.path}`} className="hover:bg-surface-hover/50">
                      <Td>
                        <Pill tone={row.method === "ANY" ? "warning" : "accent"} className="font-mono">{row.method}</Pill>
                      </Td>
                      <Td>
                        <code className="break-all rounded bg-background/60 px-2 py-1 font-mono text-xs text-foreground">
                          {gatewayEndpoint(row.path)}
                        </code>
                      </Td>
                      <Td className="font-medium">{row.label}</Td>
                      <Td className="text-muted-foreground">{row.description}</Td>
                    </tr>
                  ))}
                </tbody>
              </DataTable>
            </TableWrap>
          </div>
        </Surface>

        <Surface className="p-5">
          <SectionHeader
            title="OpenAI SDK setup"
            subtitle="Point SDKs at the gateway and keep using familiar model calls."
            className="mb-4"
          />
          <CodeBlock>{`baseURL: ${baseUrl}\napiKey: master key or managed API key`}</CodeBlock>
        </Surface>

        <Surface className="p-5">
          <SectionHeader
            title="Request features"
            subtitle="Headers and dashboard-managed features that affect requests."
            className="mb-4"
          />
          <div className="divide-y divide-border overflow-hidden rounded-lg border border-border">
            {GATEWAY_FEATURE_ROWS.map((feature) => (
              <div key={feature.label} className="grid gap-1 bg-background/25 px-4 py-3 sm:grid-cols-[10rem_1fr]">
                <strong className="text-sm text-foreground">{feature.label}</strong>
                <span className="text-sm leading-6 text-muted-foreground">{feature.value}</span>
              </div>
            ))}
          </div>
        </Surface>

        <PoolUsageSection />

        <Surface className="p-5 xl:col-span-2">
          <SectionHeader
            title="Chat completion example"
            subtitle="This example uses the same request shape as the Playground."
            className="mb-4"
          />
          <CodeBlock className="mb-4">{curl}</CodeBlock>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={() => curlCopy.copy(curl)}>
              <Copy className="h-4 w-4" />
              {curlCopy.copied ? "Copied" : "Copy cURL"}
            </Button>
            <Button asChild>
              <Link to="/admin/dashboard/playground">
                Open Playground
                <ExternalLink className="h-4 w-4" />
              </Link>
            </Button>
          </div>
        </Surface>
        <RequirePermission resource="admin/cli-tools">
          <CLIToolsGuideSection />
        </RequirePermission>
      </div>
    </div>
  );
}

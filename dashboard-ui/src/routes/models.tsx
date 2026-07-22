import * as React from "react";
import {
  Brain,
  Braces,
  Database,
  Edit3,
  ExternalLink,
  Eye,
  FileJson,
  FileText,
  Film,
  Globe,
  Loader2,
  LineChartIcon,
  MessageSquare,
  Mic,
  Monitor,
  Plus,
  Save,
  Search,
  Target,
  Trash2,
  Volume2,
  Wrench,
  X,
  Zap,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { PageHeader } from "@/components/ui/page-header";
import { EmptyState, Pill, Surface } from "@/components/ui/surface";
import { DataTable, TableWrap, Td, Th } from "@/components/ui/data-table";
import { useModels } from "@/lib/api/useModels";
import { modelDisplayName, type ModelInventoryItem } from "@/lib/api/models-types";
import type { AliasView } from "@/lib/api/aliases-types";
import type { ModelOverrideView } from "@/lib/api/model-overrides-types";
import type { ModelPricing, ModelPricingView } from "@/lib/api/model-pricing-types";
import {
  useAliases,
  useDeleteAlias,
  useDeleteModelOverride,
  useDeleteModelPricing,
  useModelOverrides,
  useModelPricing,
  useUpsertAlias,
  useUpsertModelOverride,
  useUpsertModelPricing,
} from "@/lib/api/useModelManagement";
import { cn } from "@/lib/utils";

interface DisplayRow {
  key: string;
  displayName: string;
  secondaryName: string;
  providerName: string;
  providerType: string;
  model: ModelInventoryItem;
  alias: AliasView | null;
  isAlias: boolean;
  access: ModelInventoryItem["access"] | null;
  maskingAlias: AliasView | null;
}

interface DisplayGroup {
  key: string;
  displayName: string;
  typeLabel: string;
  rows: DisplayRow[];
  selector: string;
  access: ModelOverrideView | null;
}

interface AliasFormState {
  mode: "create" | "edit";
  originalName: string;
  name: string;
  target_model: string;
  target_provider: string;
  description: string;
  enabled: boolean;
}

interface OverrideFormState {
  title: string;
  selector: string;
  enabled: boolean;
  user_paths: string;
  hasExisting: boolean;
  defaultEnabled: boolean;
  effectiveEnabled: boolean;
}

interface PricingFormState {
  selector: string;
  title: string;
  pricing: ModelPricing;
  view: ModelPricingView;
}

interface CapabilitySpec {
  key: string;
  label: string;
  Icon: LucideIcon;
}

interface ScalarBadge {
  key: string;
  label: string;
  tooltip: string;
}

const PRICING_FIELDS: Array<{ key: keyof ModelPricing; label: string; hint: string }> = [
  { key: "input_per_mtok", label: "Input / MTok", hint: "Text input tokens" },
  { key: "output_per_mtok", label: "Output / MTok", hint: "Text output tokens" },
  { key: "cached_input_per_mtok", label: "Cached input / MTok", hint: "Prompt cache reads" },
  { key: "cache_write_per_mtok", label: "Cache write / MTok", hint: "Prompt cache writes" },
  { key: "reasoning_output_per_mtok", label: "Reasoning / MTok", hint: "Thinking/reasoning output" },
  { key: "batch_input_per_mtok", label: "Batch input / MTok", hint: "Batch input tokens" },
  { key: "batch_output_per_mtok", label: "Batch output / MTok", hint: "Batch output tokens" },
  { key: "per_request", label: "Per request", hint: "Flat request fee" },
  { key: "per_image", label: "Per image", hint: "Generated image fee" },
  { key: "per_page", label: "Per page", hint: "Document page fee" },
  { key: "per_second_input", label: "Input / second", hint: "Audio/video input" },
  { key: "per_second_output", label: "Output / second", hint: "Audio/video output" },
  { key: "per_character_input", label: "Input / character", hint: "Character-metered input" },
];

const CAPABILITY_SPECS: CapabilitySpec[] = [
  { key: "vision", Icon: Eye, label: "Vision / image input" },
  { key: "function_calling", Icon: Wrench, label: "Function calling" },
  { key: "tool_choice", Icon: Target, label: "Tool choice" },
  { key: "parallel_function_calling", Icon: Zap, label: "Parallel function calls" },
  { key: "reasoning", Icon: Brain, label: "Reasoning / thinking" },
  { key: "response_schema", Icon: Braces, label: "Response schema" },
  { key: "structured_output", Icon: FileJson, label: "Structured output" },
  { key: "prompt_caching", Icon: Database, label: "Prompt caching" },
  { key: "system_messages", Icon: MessageSquare, label: "System messages" },
  { key: "pdf_input", Icon: FileText, label: "PDF input" },
  { key: "web_search", Icon: Globe, label: "Web search" },
  { key: "audio_input", Icon: Mic, label: "Audio input" },
  { key: "audio_output", Icon: Volume2, label: "Audio output" },
  { key: "video_input", Icon: Film, label: "Video input" },
  { key: "computer_use", Icon: Monitor, label: "Computer use" },
];

function metadata(item: ModelInventoryItem): Record<string, unknown> {
  const m = item.model?.metadata;
  return m && typeof m === "object" ? (m as Record<string, unknown>) : {};
}

function pricing(item: ModelInventoryItem): Record<string, unknown> {
  const p = metadata(item).pricing;
  return p && typeof p === "object" ? (p as Record<string, unknown>) : {};
}

function price(value: unknown, digits = 2): string {
  const n = Number(value);
  if (!Number.isFinite(n)) return "-";
  if (n === 0) return "Free";
  return `$${n.toFixed(digits)}`;
}

function priceFine(value: unknown): string {
  const n = Number(value);
  if (!Number.isFinite(n)) return "-";
  if (n === 0) return "Free";
  return `$${n < 0.01 ? n.toFixed(6) : n.toFixed(4)}`;
}

function pricingSourceTone(source: string): "success" | "danger" | "warning" | "muted" | "accent" {
  if (source === "user_override") return "accent";
  if (source === "missing") return "warning";
  return "muted";
}

function pricingSourceLabel(source: string): string {
  if (source === "user_override") return "Override";
  if (source === "missing") return "Missing";
  return "Registry";
}

function pricingValue(view: ModelPricingView | null, item: ModelInventoryItem): Record<string, unknown> {
  return (view?.effective_pricing as Record<string, unknown> | undefined) ?? pricing(item);
}

function pricingFormInitial(view: ModelPricingView): ModelPricing {
  return {
    currency: view.override_pricing?.currency ?? view.effective_pricing?.currency ?? view.base_pricing?.currency ?? "USD",
    ...view.override_pricing,
  };
}

function setPricingField(pricing: ModelPricing, key: keyof ModelPricing, value: string): ModelPricing {
  if (key === "currency") return { ...pricing, currency: value };
  const trimmed = value.trim();
  if (trimmed === "") {
    const next = { ...pricing };
    delete next[key];
    return next;
  }
  return { ...pricing, [key]: Number(trimmed) };
}

function pricingInputValue(pricing: ModelPricing, key: keyof ModelPricing): string | number {
  const value = pricing[key];
  return typeof value === "number" ? value : "";
}

function anyInputPrice(p: Record<string, unknown>): string {
  if (p.input_per_mtok !== undefined) return price(p.input_per_mtok);
  if (p.per_second_input !== undefined) return `${priceFine(p.per_second_input)}/s`;
  if (p.per_character_input !== undefined) return `${priceFine(p.per_character_input)}/ch`;
  if (p.input_per_image !== undefined) return `${priceFine(p.input_per_image)}/img`;
  if (p.per_image !== undefined) return `${priceFine(p.per_image)}/img`;
  if (p.per_page !== undefined) return `${priceFine(p.per_page)}/pg`;
  if (p.per_request !== undefined) return `${priceFine(p.per_request)}/req`;
  return "-";
}

function anyOutputPrice(p: Record<string, unknown>): string {
  if (p.output_per_mtok !== undefined) return price(p.output_per_mtok);
  if (p.per_second_output !== undefined) return `${priceFine(p.per_second_output)}/s`;
  return "-";
}

function modelModes(item: ModelInventoryItem): string {
  const modes = metadata(item).modes;
  return Array.isArray(modes) ? modes.map(String).join(", ") || "-" : "-";
}

function capabilityIcons(item: ModelInventoryItem): CapabilitySpec[] {
  const caps = metadata(item).capabilities;
  if (!caps || typeof caps !== "object") return [];
  const record = caps as Record<string, unknown>;
  return CAPABILITY_SPECS.filter((spec) => record[spec.key] === true);
}

function formatTokenBadge(value: unknown): string {
  const n = Number(value);
  if (!Number.isFinite(n) || n <= 0) return "";
  if (n >= 1_000_000) {
    const v = n / 1_000_000;
    return `${Number.isInteger(v) ? v.toFixed(0) : v.toFixed(1)}M`;
  }
  if (n >= 1000) {
    const v = n / 1000;
    return `${Number.isInteger(v) ? v.toFixed(0) : v.toFixed(1)}K`;
  }
  return String(n);
}

function scalarBadges(item: ModelInventoryItem): ScalarBadge[] {
  const m = metadata(item);
  const out: ScalarBadge[] = [];
  if (m.context_window) {
    out.push({
      key: "ctx",
      label: `${formatTokenBadge(m.context_window)} ctx`,
      tooltip: `Context window: ${Number(m.context_window).toLocaleString()} tokens`,
    });
  }
  if (m.max_output_tokens) {
    out.push({
      key: "out",
      label: `${formatTokenBadge(m.max_output_tokens)} out`,
      tooltip: `Max output: ${Number(m.max_output_tokens).toLocaleString()} tokens`,
    });
  }
  const rankings = m.rankings && typeof m.rankings === "object" ? (m.rankings as Record<string, unknown>) : {};
  const arena = rankings.chatbot_arena && typeof rankings.chatbot_arena === "object" ? (rankings.chatbot_arena as Record<string, unknown>) : null;
  if (arena?.elo) {
    out.push({
      key: "elo",
      label: `Elo ${Math.round(Number(arena.elo))}`,
      tooltip: `Chatbot Arena Elo: ${arena.elo}${arena.rank ? ` (rank ${arena.rank})` : ""}`,
    });
  }
  if (m.output_vector_size) {
    out.push({
      key: "vec",
      label: `${m.output_vector_size}-d`,
      tooltip: "Embedding dimension",
    });
  }
  return out;
}

function modalitySummary(item: ModelInventoryItem): string {
  const modalities = metadata(item).modalities;
  if (!modalities || typeof modalities !== "object") return "";
  const record = modalities as { input?: unknown[]; output?: unknown[] };
  const glyph: Record<string, string> = { text: "T", image: "I", audio: "A", video: "V" };
  const render = (arr: unknown[] | undefined) =>
    (arr ?? []).map((m) => glyph[String(m)] ?? String(m).slice(0, 1).toUpperCase()).filter(Boolean).join("+");
  const input = render(record.input);
  const output = render(record.output);
  return input || output ? `${input || "-"} -> ${output || "-"}` : "";
}

function metadataAliases(item: ModelInventoryItem): string[] {
  const aliases = metadata(item).aliases;
  return Array.isArray(aliases) ? aliases.map(String).filter(Boolean) : [];
}

function pricingDetails(item: ModelInventoryItem): string[] {
  const p = pricing(item);
  const details: string[] = [];
  const fields: Array<[string, string]> = [
    ["cached_input_per_mtok", "cached"],
    ["per_second_input", "in/s"],
    ["per_second_output", "out/s"],
    ["per_character_input", "in/ch"],
    ["input_per_image", "in/img"],
    ["per_image", "image"],
    ["per_page", "page"],
    ["per_request", "request"],
  ];
  for (const [key, label] of fields) {
    if (p[key] !== undefined && p[key] !== null) details.push(`${label}: ${priceFine(p[key])}`);
  }
  return details;
}

function aliasTargetLabel(alias: AliasView): string {
  if (alias.target_provider) return `${alias.target_provider}/${alias.target_model}`;
  return alias.resolved_model || alias.target_model;
}

function modelIdentifierKeys(item: ModelInventoryItem): string[] {
  const keys = new Set<string>();
  const id = String(item.model?.id ?? "").trim();
  const provider = String(item.provider_name ?? "").trim();
  const type = String(item.provider_type ?? "").trim();
  const display = modelDisplayName(item);
  for (const value of [id, provider && id ? `${provider}/${id}` : "", type && id ? `${type}/${id}` : "", display]) {
    const key = value.trim().toLowerCase();
    if (key) keys.add(key);
  }
  return Array.from(keys);
}

function buildRows(models: ModelInventoryItem[], aliases: AliasView[], aliasesAvailable: boolean): DisplayRow[] {
  const rows: DisplayRow[] = models.map((model) => ({
    key: `model:${modelDisplayName(model)}`,
    displayName: modelDisplayName(model),
    secondaryName: "",
    providerName: String(model.provider_name ?? ""),
    providerType: String(model.provider_type ?? ""),
    model,
    alias: null,
    isAlias: false,
    access: model.access ?? null,
    maskingAlias: null,
  }));

  if (!aliasesAvailable) return rows;

  const maskingAliases = new Map<string, AliasView>();
  for (const alias of aliases) {
    const name = alias.name.trim().toLowerCase();
    if (name && alias.enabled !== false && alias.valid) maskingAliases.set(name, alias);
  }
  const concreteByDisplay = new Map<string, DisplayRow>();
  for (const row of rows) {
    concreteByDisplay.set(row.displayName.toLowerCase(), row);
    for (const key of modelIdentifierKeys(row.model)) {
      if (maskingAliases.has(key)) row.maskingAlias = maskingAliases.get(key) ?? null;
    }
  }

  for (const alias of aliases) {
    const target = concreteByDisplay.get(aliasTargetLabel(alias).toLowerCase())?.model ?? null;
    rows.push({
      key: `alias:${alias.name}`,
      displayName: alias.name,
      secondaryName: aliasTargetLabel(alias),
      providerName: target ? String(target.provider_name ?? "") : "",
      providerType: target ? String(target.provider_type ?? alias.provider_type ?? "") : String(alias.provider_type ?? ""),
      model: target ?? ({ model: { id: alias.name, metadata: {} } } as ModelInventoryItem),
      alias,
      isAlias: true,
      access: null,
      maskingAlias: null,
    });
  }

  return [...rows].toSorted((a: DisplayRow, b: DisplayRow) => {
    if (a.isAlias !== b.isAlias) return a.isAlias ? -1 : 1;
    return a.displayName.localeCompare(b.displayName);
  });
}

function matchesFilter(row: DisplayRow, filter: string): boolean {
  const needle = filter.trim().toLowerCase();
  if (!needle) return true;
  const fields = [
    row.displayName,
    row.secondaryName,
    row.providerName,
    row.providerType,
    row.model.model?.id,
    row.model.model?.owned_by,
    metadata(row.model).display_name,
    metadata(row.model).description,
    row.alias?.description,
    row.alias ? aliasTargetLabel(row.alias) : "",
  ];
  return fields.some((field) => String(field ?? "").toLowerCase().includes(needle));
}

function buildGroups(rows: DisplayRow[], overrides: ModelOverrideView[]): DisplayGroup[] {
  const overridesBySelector = new Map(overrides.map((view) => [view.selector, view]));
  const groups = new Map<string, DisplayGroup>();
  for (const row of rows) {
    const key = row.providerName || row.providerType || "unassigned";
    if (!groups.has(key)) {
      const selector = row.providerName ? `${row.providerName}/` : "";
      groups.set(key, {
        key,
        displayName: row.providerName || row.providerType || "Unassigned",
        typeLabel: row.providerName && row.providerType ? row.providerType : "",
        rows: [],
        selector,
        access: selector ? overridesBySelector.get(selector) ?? null : null,
      });
    }
    groups.get(key)?.rows.push(row);
  }
  return Array.from(groups.values()).toSorted((a: DisplayGroup, b: DisplayGroup) =>
    a.displayName.localeCompare(b.displayName),
  );
}

/* ------------------------------------------------------------------ */
/*  Client-side model categorisation                                   */
/* ------------------------------------------------------------------ */

const MODE_TO_CAT: Record<string, string> = {
  chat: "text_generation",
  completion: "text_generation",
  responses: "text_generation",
  embedding: "embedding",
  rerank: "rerank",
  image_generation: "image",
  image_edit: "image",
  audio_transcription: "audio",
  audio_speech: "audio",
  video_generation: "video",
  moderation: "utility",
  ocr: "utility",
  search: "utility",
};

const CATEGORY_DISPLAY: Record<string, string> = {
  all: "All",
  text_generation: "Text Generation",
  embedding: "Embeddings",
  rerank: "Rerankers",
  image: "Image",
  audio: "Audio",
  video: "Video",
  utility: "Utility",
};

const CATEGORY_ORDER = ["all", "text_generation", "embedding", "rerank", "image", "audio", "video", "utility"];

function computeModelCategories(item: ModelInventoryItem): string[] {
  const meta = metadata(item);
  const metaCats = meta.categories;
  if (Array.isArray(metaCats) && metaCats.length > 0) {
    return metaCats.map(String);
  }

  const modes = meta.modes;
  if (Array.isArray(modes) && modes.length > 0) {
    const cats = new Set<string>();
    for (const m of modes) {
      const c = MODE_TO_CAT[String(m).toLowerCase()];
      if (c) cats.add(c);
    }
    if (cats.size > 0) return Array.from(cats);
  }

  const id = String(item.model?.id ?? "").toLowerCase();
  const ptype = String(item.provider_type ?? "").toLowerCase();

  if (ptype === "reranker") return ["rerank"];
  if (id.includes("reranker") || id.includes("rerank")) return ["rerank"];
  if (id.includes("embedding") || id.includes("embed")) return ["embedding"];
  if (id.includes("whisper") || id.includes("audio") || id.includes("speech")) return ["audio"];
  if (id.includes("tts")) return ["audio"];
  if (id.includes("image") || id.includes("dall-e")) return ["image"];
  if (id.includes("video")) return ["video"];
  if (id.includes("moderat") || id.includes("ocr") || id.includes("search")) return ["utility"];

  return ["text_generation"];
}

interface LocalCategory {
  key: string;
  displayName: string;
  count: number;
}

function computeLocalCategories(items: ModelInventoryItem[]): LocalCategory[] {
  const counts = new Map<string, number>();
  let total = 0;
  for (const item of items) {
    total++;
    const cats = computeModelCategories(item);
    for (const c of cats) {
      counts.set(c, (counts.get(c) ?? 0) + 1);
    }
  }
  const result: LocalCategory[] = [{ key: "all", displayName: "All", count: total }];
  for (const key of CATEGORY_ORDER) {
    if (key === "all") continue;
    const count = counts.get(key) ?? 0;
    if (count > 0) {
      result.push({ key, displayName: CATEGORY_DISPLAY[key] ?? key, count });
    }
  }

  return result;
}

function categoryMatches(item: ModelInventoryItem, categoryKey: string): boolean {
  if (categoryKey === "all") return true;
  return computeModelCategories(item).includes(categoryKey);
}

function parseUserPaths(value: string): string[] {
  return value
    .split(/\r?\n|,/)
    .map((part) => part.trim())
    .filter(Boolean);
}

function accessTone(access: DisplayRow["access"] | null): "success" | "danger" | "warning" {
  if (!access) return "warning";
  return access.effective_enabled ? "success" : "danger";
}

function accessText(access: DisplayRow["access"] | null): string {
  if (!access) return "Alias";
  if (!access.effective_enabled) return "Disabled";
  const paths = access.user_paths ?? [];
  return paths.length > 0 ? `${paths.length} paths` : "Enabled";
}

export function ModelsPage(): JSX.Element {
  const [category, setCategory] = React.useState("all");
  const [filter, setFilter] = React.useState("");
  const [aliasForm, setAliasForm] = React.useState<AliasFormState | null>(null);
  const [overrideForm, setOverrideForm] = React.useState<OverrideFormState | null>(null);
  const [pricingForm, setPricingForm] = React.useState<PricingFormState | null>(null);
  const [notice, setNotice] = React.useState("");
  const [localError, setLocalError] = React.useState("");

  const models = useModels();
  const aliases = useAliases();
  const overrides = useModelOverrides();
  const modelPricing = useModelPricing();
  const saveAlias = useUpsertAlias();
  const removeAlias = useDeleteAlias();
  const saveOverride = useUpsertModelOverride();
  const removeOverride = useDeleteModelOverride();
  const savePricing = useUpsertModelPricing();
  const removePricing = useDeleteModelPricing();

  const aliasesAvailable = !(aliases.error && "status" in aliases.error && (aliases.error as { status: number }).status === 503);
  const overridesAvailable = !(overrides.error && "status" in overrides.error && (overrides.error as { status: number }).status === 503);
  const pricingAvailable = !(modelPricing.error && "status" in modelPricing.error && (modelPricing.error as { status: number }).status === 503);

  const allModels = models.data ?? [];
  const pricingBySelector = React.useMemo(
    () => new Map((modelPricing.data ?? []).map((view) => [view.selector, view])),
    [modelPricing.data],
  );

  const tabs = React.useMemo(() => computeLocalCategories(allModels), [allModels]);

  const rows = React.useMemo(
    () => buildRows(allModels, aliases.data ?? [], aliasesAvailable),
    [allModels, aliases.data, aliasesAvailable],
  );
  const filteredByCategory = React.useMemo(
    () => rows.filter((row) => categoryMatches(row.model, category)),
    [rows, category],
  );
  const filteredRows = React.useMemo(
    () => filteredByCategory.filter((row) => matchesFilter(row, filter)),
    [filteredByCategory, filter],
  );
  const groups = React.useMemo(() => buildGroups(filteredRows, overrides.data ?? []), [filteredRows, overrides.data]);
  const countText = filter ? `${filteredRows.length} / ${rows.length}` : String(rows.length);
  const modelOptions = React.useMemo(() => allModels.map(modelDisplayName).filter(Boolean), [allModels]);
  const busy = models.isLoading || aliases.isLoading || overrides.isLoading || modelPricing.isLoading;

  function openAliasCreate(): void {
    setLocalError("");
    setAliasForm({ mode: "create", originalName: "", name: "", target_model: "", target_provider: "", description: "", enabled: true });
  }

  function openAliasCreateForModel(modelName: string): void {
    setLocalError("");
    setAliasForm({ mode: "create", originalName: "", name: "", target_model: modelName, target_provider: "", description: "", enabled: true });
  }

  function openAliasEdit(alias: AliasView): void {
    setLocalError("");
    setAliasForm({
      mode: "edit",
      originalName: alias.name,
      name: alias.name,
      target_model: alias.target_model,
      target_provider: alias.target_provider ?? "",
      description: alias.description ?? "",
      enabled: alias.enabled !== false,
    });
  }

  async function submitAlias(): Promise<void> {
    if (!aliasForm) return;
    setLocalError("");
    const name = aliasForm.name.trim();
    const target = aliasForm.target_model.trim();
    if (!name || !target) {
      setLocalError("Alias name and target model are required.");
      return;
    }
    try {
      await saveAlias.mutateAsync({ name, target_model: target, target_provider: aliasForm.target_provider ?? "", description: aliasForm.description, enabled: aliasForm.enabled });
      if (aliasForm.mode === "edit" && aliasForm.originalName && aliasForm.originalName !== name) {
        await removeAlias.mutateAsync(aliasForm.originalName);
      }
      setAliasForm(null);
      setNotice(aliasForm.mode === "edit" ? "Alias saved." : "Alias created.");
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "Unable to save alias.");
    }
  }

  async function toggleAlias(alias: AliasView): Promise<void> {
    setLocalError("");
    try {
      await saveAlias.mutateAsync({
        name: alias.name,
        target_model: alias.target_model,
        target_provider: alias.target_provider,
        description: alias.description,
        enabled: alias.enabled === false,
      });
      setNotice(alias.enabled === false ? "Alias enabled." : "Alias disabled.");
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "Unable to update alias.");
    }
  }

  async function confirmDeleteAlias(alias: AliasView): Promise<void> {
    setLocalError("");
    try {
      await removeAlias.mutateAsync(alias.name);
      setNotice("Alias deleted.");
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "Unable to delete alias.");
    }
  }

  function openOverride(row: DisplayRow): void {
    const access = row.access;
    const override = access?.override as { user_paths?: string[]; enabled?: boolean } | undefined;
    setLocalError("");
    setOverrideForm({
      title: row.displayName,
      selector: access?.selector ?? row.displayName,
      enabled: access?.effective_enabled ?? true,
      user_paths: (override?.user_paths ?? access?.user_paths ?? ["/"]).join("\n"),
      hasExisting: Boolean(access?.override),
      defaultEnabled: access?.default_enabled ?? true,
      effectiveEnabled: access?.effective_enabled ?? true,
    });
  }

  function openProviderOverride(group: DisplayGroup): void {
    setLocalError("");
    setOverrideForm({
      title: `Provider access for ${group.displayName}`,
      selector: group.selector,
      enabled: group.access?.enabled ?? true,
      user_paths: (group.access?.user_paths?.length ? group.access.user_paths : ["/"]).join("\n"),
      hasExisting: Boolean(group.access),
      defaultEnabled: true,
      effectiveEnabled: group.access?.enabled ?? true,
    });
  }

  async function submitOverride(): Promise<void> {
    if (!overrideForm) return;
    setLocalError("");
    try {
      await saveOverride.mutateAsync({
        selector: overrideForm.selector,
        enabled: overrideForm.enabled,
        user_paths: overrideForm.enabled ? parseUserPaths(overrideForm.user_paths) : [],
      });
      setOverrideForm(null);
      setNotice("Model availability saved.");
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "Unable to save model availability.");
    }
  }

  async function deleteOverride(): Promise<void> {
    if (!overrideForm) return;
    setLocalError("");
    try {
      await removeOverride.mutateAsync(overrideForm.selector);
      setOverrideForm(null);
      setNotice("Model availability override removed.");
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "Unable to remove model availability override.");
    }
  }

  function openPricing(row: DisplayRow): void {
    const view = pricingBySelector.get(row.displayName);
    if (!view) {
      setLocalError("Pricing controls are not available for this model yet.");
      return;
    }
    setLocalError("");
    setPricingForm({
      selector: view.selector,
      title: row.displayName,
      pricing: pricingFormInitial(view),
      view,
    });
  }

  async function submitPricing(): Promise<void> {
    if (!pricingForm) return;
    setLocalError("");
    try {
      const updated = await savePricing.mutateAsync({ selector: pricingForm.selector, pricing: pricingForm.pricing });
      setPricingForm({ ...pricingForm, view: updated, pricing: pricingFormInitial(updated) });
      setNotice("Pricing override saved. Recalculate usage pricing to update historical costs.");
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "Unable to save pricing override.");
    }
  }

  async function deletePricing(): Promise<void> {
    if (!pricingForm) return;
    setLocalError("");
    try {
      await removePricing.mutateAsync(pricingForm.selector);
      setPricingForm(null);
      setNotice("Pricing override removed.");
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "Unable to remove pricing override.");
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Registered Models"
        subtitle="View and manage registered AI models, their capabilities, and configuration overrides."
        actions={<div className="border border-border px-3 py-1 text-sm text-muted-foreground">{countText} models</div>}
      />

      {!aliasesAvailable ? <Alert tone="warning">Aliases feature is unavailable.</Alert> : null}
      {aliases.error && aliasesAvailable ? <Alert tone="warning">{aliases.error.message}</Alert> : null}
      {!overridesAvailable ? <Alert tone="warning">Model overrides feature is unavailable.</Alert> : null}
      {overrides.error && overridesAvailable ? <Alert tone="warning">{overrides.error.message}</Alert> : null}
      {!pricingAvailable ? <Alert tone="warning">Model pricing controls are unavailable.</Alert> : null}
      {modelPricing.error && pricingAvailable ? <Alert tone="warning">{modelPricing.error.message}</Alert> : null}
      {localError ? <Alert tone="warning">{localError}</Alert> : null}
      {notice ? <Alert tone="success">{notice}</Alert> : null}

      <div className="flex flex-wrap gap-2">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            type="button"
            onClick={() => setCategory(tab.key)}
            className={cn(
              "inline-flex items-center gap-2 border px-3.5 py-1.5 text-[13px] font-medium transition-all duration-200",
              category === tab.key
                ? "border-accent bg-accent text-accent-foreground"
                : "border-border/60 bg-surface/80 text-muted-foreground hover:bg-surface-hover hover:text-foreground backdrop-blur-sm",
            )}
          >
            <span>{tab.displayName}</span>
            <span className={cn(
              "px-1.5 text-[11px] font-semibold",
              category === tab.key ? "bg-accent-foreground/20" : "bg-background/50"
            )}>{tab.count}</span>
          </button>
        ))}
      </div>

      <Surface className="p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <label className="relative min-w-0 flex-1 md:max-w-xl">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <input
              value={filter}
              onChange={(event) => setFilter(event.target.value)}
              placeholder="Filter by provider, provider/model, alias, or owner..."
              className="h-10 w-full rounded-md border border-border bg-background pl-9 pr-3 text-sm text-foreground outline-none focus:ring-2 focus:ring-ring"
            />
          </label>
          {aliasesAvailable ? (
            <Button onClick={openAliasCreate}>
              <Plus className="h-4 w-4" />
              Create Alias
            </Button>
          ) : null}
        </div>
      </Surface>

      {busy ? (
        <Surface className="flex items-center gap-2 p-6 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading models...
        </Surface>
      ) : models.error ? (
        <Alert tone="warning">{models.error.message}</Alert>
      ) : groups.length === 0 ? (
        <EmptyState title={filter ? "No models match your filter" : category === "all" ? "No models registered" : "No models in this category"}>
          {filter ? "Clear the search field or try filtering by provider, owner, alias, or model ID." : "Configure providers and refresh runtime models to populate this inventory."}
        </EmptyState>
      ) : (
        <TableWrap>
          <DataTable>
            <thead>
              <tr>
                <Th>Model</Th>
                <Th>Modes</Th>
                <Th>Input $/MTok</Th>
                <Th>Output $/MTok</Th>
                <Th>Cached $/MTok</Th>
                <Th>Availability</Th>
                <Th className="text-right">Actions</Th>
              </tr>
            </thead>
            {groups.map((group) => (
              <tbody key={group.key}>
                <tr className="bg-surface-hover/30 backdrop-blur-sm border-t border-border/40 first:border-t-0">
                  <Td colSpan={7}>
                    <div className="flex flex-wrap items-center justify-between gap-3 py-1">
                      <div>
                        <div className="font-mono text-[14px] font-bold tracking-tight text-foreground">
                          {group.displayName} {group.typeLabel ? <span className="text-muted-foreground font-medium">({group.typeLabel})</span> : null}
                          <span className="ml-3 text-[12px] font-medium uppercase tracking-wider text-muted-foreground">{group.rows.length} models</span>
                        </div>
                        {group.access ? <div className="mt-1 text-[12px] text-muted-foreground">Override: {group.access.enabled === false ? "disabled" : "enabled"}</div> : null}
                      </div>
                      {overridesAvailable && group.selector ? (
                        <Button variant="ghost" size="sm" onClick={() => openProviderOverride(group)}>
                          <Edit3 className="h-4 w-4" />
                          Provider access
                        </Button>
                      ) : null}
                    </div>
                  </Td>
                </tr>
                {group.rows.map((row) => {
                  const pricingView = row.isAlias ? null : pricingBySelector.get(row.displayName) ?? null;
                  const p = pricingValue(pricingView, row.model);
                  const m = metadata(row.model);
                  const caps = row.isAlias ? [] : capabilityIcons(row.model);
                  const scalars = row.isAlias ? [] : scalarBadges(row.model);
                  const modalities = row.isAlias ? "" : modalitySummary(row.model);
                  const aliases = row.isAlias ? [] : metadataAliases(row.model);
                  const priceDetails = row.isAlias ? [] : pricingDetails(row.model);
                  const sourceURL = typeof m.source_url === "string" ? m.source_url : "";
                  const family = typeof m.family === "string" ? m.family : "";
                  const owner = String(m.owned_by ?? row.model.model?.owned_by ?? "");
                  const display = typeof m.display_name === "string" ? m.display_name : "";
                  const modes = modelModes(row.model).split(", ").filter((mode) => mode && mode !== "-");
                  return (
                    <tr key={row.key} className={cn("hover:bg-surface-hover/40", row.isAlias && "bg-accent/5")}>
                      <Td>
                        <div className="space-y-2">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="font-mono text-sm font-medium">{row.displayName}</span>
                            {row.isAlias ? <Pill tone="accent">Alias</Pill> : null}
                            {row.alias && row.alias.enabled === false ? <Pill tone="danger">Disabled</Pill> : null}
                            {row.alias && !row.alias.valid ? <Pill tone="warning">Invalid target</Pill> : null}
                            {row.maskingAlias ? <Pill tone="warning">Masked by {row.maskingAlias.name}</Pill> : null}
                            {!row.isAlias && pricingView ? <Pill tone={pricingSourceTone(pricingView.source)}>{pricingSourceLabel(pricingView.source)}</Pill> : null}
                          </div>
                          {!row.isAlias && (caps.length > 0 || scalars.length > 0 || modalities) ? (
                            <div className="flex flex-wrap items-center gap-1.5">
                              {caps.map(({ key, Icon, label }) => (
                                <span
                                  key={key}
                                  title={label}
                                  aria-label={label}
                                  className="inline-flex h-6 w-6 items-center justify-center rounded-md border border-border bg-background/45 text-muted-foreground"
                                >
                                  <Icon className="h-3.5 w-3.5" />
                                </span>
                              ))}
                              {scalars.map((badge) => (
                                <span
                                  key={badge.key}
                                  title={badge.tooltip}
                                  className="rounded-md border border-border bg-background/45 px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground"
                                >
                                  {badge.label}
                                </span>
                              ))}
                              {modalities ? (
                                <span
                                  title="Modalities: input -> output"
                                  className="rounded-md border border-accent/25 bg-accent/10 px-1.5 py-0.5 font-mono text-[10px] text-accent"
                                >
                                  {modalities}
                                </span>
                              ) : null}
                            </div>
                          ) : null}
                          {row.isAlias ? (
                            <div className="text-xs text-muted-foreground">Targets <span className="font-mono">{row.secondaryName}</span></div>
                          ) : m.description ? (
                            <div className="line-clamp-2 max-w-xl text-xs leading-5 text-muted-foreground">{String(m.description)}</div>
                          ) : null}
                          {!row.isAlias && (display || owner || family || sourceURL || aliases.length > 0 || priceDetails.length > 0) ? (
                            <div className="flex max-w-4xl flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
                              {display ? <span>{display}</span> : null}
                              {owner ? <span>Owner: <strong className="text-foreground">{owner}</strong></span> : null}
                              {family ? <span>Family: <strong className="text-foreground">{family}</strong></span> : null}
                              {priceDetails.slice(0, 4).map((detail) => (
                                <span key={detail} className="rounded border border-border bg-background/35 px-1.5 py-0.5 font-mono">
                                  {detail}
                                </span>
                              ))}
                              {aliases.slice(0, 3).map((alias) => (
                                <span key={alias} className="rounded border border-border bg-background/35 px-1.5 py-0.5 font-mono">
                                  {alias}
                                </span>
                              ))}
                              {aliases.length > 3 ? <span>+ {aliases.length - 3} aliases</span> : null}
                              {sourceURL ? (
                                <a
                                  href={sourceURL}
                                  target="_blank"
                                  rel="noreferrer"
                                  className="inline-flex items-center gap-1 text-accent hover:text-accent-hover"
                                >
                                  Docs <ExternalLink className="h-3 w-3" />
                                </a>
                              ) : null}
                            </div>
                          ) : null}
                        </div>
                      </Td>
                      <Td>
                        {row.isAlias || modes.length === 0 ? "-" : (
                          <div className="flex flex-wrap gap-1">
                            {modes.map((mode) => <Pill key={mode} tone="muted">{mode}</Pill>)}
                          </div>
                        )}
                      </Td>
                      <Td className="font-mono text-xs">{anyInputPrice(p)}</Td>
                      <Td className="font-mono text-xs">{anyOutputPrice(p)}</Td>
                      <Td className="font-mono text-xs">{price(p.cached_input_per_mtok)}</Td>
                      <Td><Pill tone={accessTone(row.access)}>{accessText(row.access)}</Pill></Td>
                      <Td>
                        <div className="flex justify-end gap-2">
                          {row.alias ? (
                            <>
                              <Button variant="secondary" size="sm" disabled={saveAlias.isPending} onClick={() => void toggleAlias(row.alias!)}>
                                {row.alias.enabled === false ? "Enable" : "Disable"}
                              </Button>
                              <Button variant="ghost" size="icon" aria-label={`Edit alias ${row.alias.name}`} onClick={() => openAliasEdit(row.alias!)}>
                                <Edit3 className="h-4 w-4" />
                              </Button>
                              <Button variant="ghost" size="icon" aria-label={`Delete alias ${row.alias.name}`} disabled={removeAlias.isPending} onClick={() => void confirmDeleteAlias(row.alias!)}>
                                <Trash2 className="h-4 w-4 text-destructive" />
                              </Button>
                            </>
                          ) : (
                            <>
                              <Button variant="ghost" size="sm" onClick={() => openAliasCreateForModel(row.displayName)}>
                                <Plus className="h-4 w-4" />
                                Alias
                              </Button>
                              {pricingAvailable ? (
                                <Button variant="ghost" size="sm" onClick={() => openPricing(row)}>
                                  <LineChartIcon className="h-4 w-4" />
                                  Pricing
                                </Button>
                              ) : null}
                              {overridesAvailable ? (
                                <Button variant="ghost" size="sm" onClick={() => openOverride(row)}>
                                  <Edit3 className="h-4 w-4" />
                                  Access
                                </Button>
                              ) : null}
                            </>
                          )}
                        </div>
                      </Td>
                    </tr>
                  );
                })}
              </tbody>
            ))}
          </DataTable>
        </TableWrap>
      )}

      <AliasDialog
        form={aliasForm}
        modelOptions={modelOptions}
        pending={saveAlias.isPending || removeAlias.isPending}
        error={localError}
        onChange={setAliasForm}
        onClose={() => setAliasForm(null)}
        onSubmit={() => void submitAlias()}
      />
      <OverrideDialog
        form={overrideForm}
        pending={saveOverride.isPending || removeOverride.isPending}
        error={localError}
        onChange={setOverrideForm}
        onClose={() => setOverrideForm(null)}
        onSubmit={() => void submitOverride()}
        onDelete={() => void deleteOverride()}
      />
      <PricingDialog
        form={pricingForm}
        pending={savePricing.isPending || removePricing.isPending}
        error={localError}
        onChange={setPricingForm}
        onClose={() => setPricingForm(null)}
        onSubmit={() => void submitPricing()}
        onDelete={() => void deletePricing()}
      />
    </div>
  );
}

function Alert({ children, tone }: { children: string; tone: "warning" | "success" }): JSX.Element {
  return (
    <div
      className={cn(
        "border px-4 py-3 text-[14px]",
        tone === "warning" && "border-warning/30 bg-warning/15 text-warning",
        tone === "success" && "border-success/30 bg-success/15 text-success",
      )}
    >
      {children}
    </div>
  );
}

function AliasDialog({
  form,
  modelOptions,
  pending,
  error,
  onChange,
  onClose,
  onSubmit,
}: {
  form: AliasFormState | null;
  modelOptions: string[];
  pending: boolean;
  error: string;
  onChange: (form: AliasFormState | null) => void;
  onClose: () => void;
  onSubmit: () => void;
}): JSX.Element {
  return (
    <Dialog open={Boolean(form)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-2xl">
        {form ? (
          <>
            <DialogHeader>
              <DialogTitle>{form.mode === "edit" ? "Edit alias" : "Create alias"}</DialogTitle>
              <DialogDescription>Aliases are not supported on pass-through endpoints.</DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Alias name">
                <input className="field-input font-mono" value={form.name} onChange={(e) => onChange({ ...form, name: e.target.value })} />
              </Field>
              <Field label="Target model">
                <input className="field-input font-mono" list="model-options" value={form.target_model} onChange={(e) => onChange({ ...form, target_model: e.target.value })} />
                <datalist id="model-options">
                  {modelOptions.map((model) => <option key={model} value={model} />)}
                </datalist>
              </Field>
              <Field label="Target provider (optional)">
                <input className="field-input font-mono" placeholder="e.g. openai" value={form.target_provider} onChange={(e) => onChange({ ...form, target_provider: e.target.value })} />
              </Field>
            </div>
            <Field label="Description">
              <textarea className="field-input min-h-24" value={form.description} onChange={(e) => onChange({ ...form, description: e.target.value })} />
            </Field>
            <label className="flex items-center gap-2 text-sm text-foreground">
              <input type="checkbox" checked={form.enabled} onChange={(e) => onChange({ ...form, enabled: e.target.checked })} />
              Alias is enabled
            </label>
            {form.mode === "edit" ? <p className="text-xs text-muted-foreground">Renaming an alias creates the new name first, then removes the old one.</p> : null}
            {error ? <p className="text-sm text-warning">{error}</p> : null}
            <DialogFooter>
              <Button variant="secondary" onClick={onClose}>Cancel</Button>
              <Button onClick={onSubmit} disabled={pending}>
                {pending ? <Loader2 className="h-4 w-4 animate-spin" /> : form.mode === "edit" ? <Save className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
                {pending ? "Saving..." : form.mode === "edit" ? "Save Alias" : "Create Alias"}
              </Button>
            </DialogFooter>
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function OverrideDialog({
  form,
  pending,
  error,
  onChange,
  onClose,
  onSubmit,
  onDelete,
}: {
  form: OverrideFormState | null;
  pending: boolean;
  error: string;
  onChange: (form: OverrideFormState | null) => void;
  onClose: () => void;
  onSubmit: () => void;
  onDelete: () => void;
}): JSX.Element {
  return (
    <Dialog open={Boolean(form)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-2xl">
        {form ? (
          <>
            <DialogHeader>
              <DialogTitle>{form.title}</DialogTitle>
              <DialogDescription>
                Default enabled: {form.defaultEnabled ? "yes" : "no"} • Effective now: {form.effectiveEnabled ? "yes" : "no"}
              </DialogDescription>
            </DialogHeader>
            <Field label="Selector">
              <input className="field-input font-mono" value={form.selector} disabled />
            </Field>
            <label className="flex items-center gap-3 rounded-lg border border-border bg-background/35 p-3 text-sm text-foreground">
              <input type="checkbox" checked={form.enabled} onChange={(e) => onChange({ ...form, enabled: e.target.checked })} />
              {form.enabled ? "Enabled for matching user paths" : "Disabled for every user path"}
            </label>
            <Field label="User Paths">
              <textarea
                className="field-input min-h-28 font-mono"
                value={form.user_paths}
                disabled={!form.enabled}
                onChange={(e) => onChange({ ...form, user_paths: e.target.value })}
                placeholder="/\n/team/alpha"
              />
            </Field>
            <p className="text-xs leading-5 text-muted-foreground">Turn off Enabled to disable this selector. Keep Enabled on and use / to allow every user path, or enter team paths to restrict access.</p>
            {error ? <p className="text-sm text-warning">{error}</p> : null}
            <DialogFooter>
              <Button variant="secondary" onClick={onClose}>Cancel</Button>
              {form.hasExisting ? <Button variant="outline" onClick={onDelete} disabled={pending}><X className="h-4 w-4" />Remove Override</Button> : null}
              <Button onClick={onSubmit} disabled={pending}>{pending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}Save Availability</Button>
            </DialogFooter>
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function PricingDialog({
  form,
  pending,
  error,
  onChange,
  onClose,
  onSubmit,
  onDelete,
}: {
  form: PricingFormState | null;
  pending: boolean;
  error: string;
  onChange: (form: PricingFormState | null) => void;
  onClose: () => void;
  onSubmit: () => void;
  onDelete: () => void;
}): JSX.Element {
  const effective = form?.view.effective_pricing;
  const base = form?.view.base_pricing;
  const override = form?.view.override_pricing;
  return (
    <Dialog open={Boolean(form)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-4xl">
        {form ? (
          <>
            <DialogHeader>
              <DialogTitle>Pricing for {form.title}</DialogTitle>
              <DialogDescription>
                Effective source: {pricingSourceLabel(form.view.source)} • Override key: {form.view.override_selector || form.selector}
              </DialogDescription>
            </DialogHeader>

            <div className="grid gap-3 md:grid-cols-3">
              <PricingSummary title="Base" pricing={base} />
              <PricingSummary title="Override" pricing={override} empty="No local override" />
              <PricingSummary title="Effective" pricing={effective} />
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Currency">
                <input
                  className="field-input font-mono"
                  value={form.pricing.currency ?? "USD"}
                  onChange={(e) => onChange({ ...form, pricing: { ...form.pricing, currency: e.target.value } })}
                />
              </Field>
              <div className="rounded-lg border border-border bg-background/35 p-3 text-xs leading-5 text-muted-foreground">
                Blank numeric fields inherit from registry/base pricing. Saved values override registry pricing until removed.
              </div>
            </div>

            <div className="grid max-h-[45vh] gap-3 overflow-y-auto pr-1 sm:grid-cols-2">
              {PRICING_FIELDS.map((field) => (
                <Field key={field.key} label={field.label}>
                  <div className="flex gap-2">
                    <input
                      className="field-input font-mono"
                      type="number"
                      min={0}
                      step="any"
                      placeholder={priceFine(base?.[field.key])}
                      value={pricingInputValue(form.pricing, field.key)}
                      onChange={(e) => onChange({ ...form, pricing: setPricingField(form.pricing, field.key, e.target.value) })}
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => onChange({ ...form, pricing: setPricingField(form.pricing, field.key, "") })}
                    >
                      Inherit
                    </Button>
                  </div>
                  <div className="text-[11px] text-muted-foreground">
                    {field.hint} • base {priceFine(base?.[field.key])} • effective {priceFine(effective?.[field.key])}
                  </div>
                </Field>
              ))}
            </div>

            {form.view.overridden_fields.length > 0 ? (
              <div className="flex flex-wrap gap-1.5 text-xs text-muted-foreground">
                Overrides: {form.view.overridden_fields.map((field) => <Pill key={field} tone="accent">{field}</Pill>)}
              </div>
            ) : null}
            {error ? <p className="text-sm text-warning">{error}</p> : null}
            <DialogFooter>
              <Button variant="secondary" onClick={onClose}>Close</Button>
              {form.view.has_override ? <Button variant="outline" onClick={onDelete} disabled={pending}><X className="h-4 w-4" />Remove Override</Button> : null}
              <Button onClick={onSubmit} disabled={pending}>{pending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}Save Pricing</Button>
            </DialogFooter>
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function PricingSummary({ title, pricing, empty }: { title: string; pricing: ModelPricing | undefined; empty?: string }): JSX.Element {
  return (
    <div className="rounded-lg border border-border bg-background/35 p-3">
      <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">{title}</div>
      {pricing ? (
        <div className="mt-2 space-y-1 font-mono text-xs text-foreground">
          <div>in: {priceFine(pricing.input_per_mtok)}</div>
          <div>out: {priceFine(pricing.output_per_mtok)}</div>
          <div>cached: {priceFine(pricing.cached_input_per_mtok)}</div>
          <div>currency: {pricing.currency || "USD"}</div>
        </div>
      ) : (
        <div className="mt-2 text-xs text-muted-foreground">{empty ?? "No pricing found"}</div>
      )}
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }): JSX.Element {
  return (
    <label className="block space-y-2">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      {children}
    </label>
  );
}

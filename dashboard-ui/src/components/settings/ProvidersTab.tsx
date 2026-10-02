import { Surface, SectionHeader, Pill } from "@/components/ui/surface";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { RuntimeStatusBadge, useSettings, StatusChip } from "./SettingsContext";
import { ServerIcon, RefreshCwIcon, PlusIcon, Edit3Icon, Trash2Icon, SaveIcon, XIcon, CheckIcon, SquareIcon, CheckSquareIcon, MinusIcon, KeyIcon } from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { fetchProviderStatus, createProvider, createProviders, updateProvider, deleteProvider, setProviderEnabled, refreshRuntime, type ProviderFormData, type ProviderStatusResponse } from "@/lib/api/providers";
import { withBasePath } from "@/lib/basepath";
import { useState, useCallback, useMemo, useRef, type ReactNode } from "react";
import { cn } from "@/lib/utils";
import { ExternalAuthDialog } from "./ExternalAuthDialog";
import { ProviderEgressPanel } from "./ProviderEgressPanel";
import { IpListEditor } from "./IpListEditor";
import { IpInventory } from "./IpInventory";
import { ExternalIpsPanel } from "./ExternalIpsPanel";
import { fetchExternalAuthProviders } from "@/lib/api/external-auth";
import { fetchExtensions, type Extension } from "@/lib/api/extensions";
import { useEgressStatus, useToggleEgressExit } from "@/lib/api/useEgress";
import { serversBoundTo, useEgressServerLists } from "@/lib/api/egress-servers";
import { exitDestination, exitForIp, exitState, toneDotClass } from "@/lib/egress-view";
import type { ProviderStatusItem } from "@/lib/api/providers-types";
import { filterToText, hasAdvancedConditions, resolveAutofetchFilter } from "@/lib/autofetch-filter";
import { parseProviderNames } from "@/lib/provider-names";

/** "Claude OAuth" / "OpenCode Auth" / "Zen Device" → "Link … account". */
function linkAccountLabel(name: string): string {
  const subject = name.replace(/\s+(oauth|auth|device|account)\b.*$/i, "").trim();
  return subject ? `Link ${subject} account` : "Link account";
}

const PROVIDER_LOGOS: Record<string, string> = {
  alicode: "alicode.png",
  anthropic: "anthropic.png",
  antigravity: "antigravity.png",
  azure: "azure.png",
  bedrock: "aws-polly.png",
  brave: "brave-search.png",
  cerebras: "cerebras.png",
  chutes: "chutes.png",
  cloudflare: "cloudflare-ai.png",
  cohere: "cohere.png",
  deepgram: "deepgram.png",
  deepseek: "deepseek.png",
  elevenlabs: "elevenlabs.png",
  exa: "exa.png",
  fal: "fal-ai.png",
  fireworks: "fireworks.png",
  gemini: "gemini.png",
  github: "github.png",
  glm: "glm.png",
  google: "gemini.png",
  groq: "groq.png",
  huggingface: "huggingface.png",
  jina: "jina-ai.png",
  kimi: "kimi.png",
  mistral: "mistral.png",
  nebius: "nebius.png",
  nvidia: "nvidia.png",
  ollama: "ollama.png",
  openai: "openai.png",
  openrouter: "openrouter.png",
  perplexity: "perplexity.png",
  qwen: "qwen.png",
  siliconflow: "siliconflow.png",
  stability: "stability-ai.png",
  together: "together.png",
  togetherai: "together.png",
  vertex: "vertex.png",
  voyage: "voyage-ai.png",
  xai: "xai.png",
};

function providerLogo(provider: { name: string; type?: string; config?: { type?: string } }): string | undefined {
  const keys = [provider.type, provider.config?.type, provider.name]
    .map(value => value?.toLowerCase().replace(/[^a-z0-9-]/g, ""))
    .filter(Boolean) as string[];
  const fileName = keys.map(key => PROVIDER_LOGOS[key]).find(Boolean);
  return fileName ? withBasePath(`/admin/static/providers/${fileName}`) : undefined;
}

function ProviderMark({ provider }: { provider: { name: string; type?: string; config?: { type?: string } } }): JSX.Element {
  const logo = providerLogo(provider);
  if (logo) {
    return <img src={logo} alt="" className="h-7 w-7 rounded object-contain" loading="lazy" />;
  }
  return (
    <div className="flex h-7 w-7 items-center justify-center border border-border/40 bg-background/70 text-[10px] font-bold uppercase text-muted-foreground">
      {(provider.name || provider.type || "P").slice(0, 1)}
    </div>
  );
}

function ConfigSourceBadge({ source }: { source: string | undefined }): JSX.Element {
  const colorMap: Record<string, string> = {
    config_file: "border-blue-500/30 bg-blue-500/10 text-blue-400",
    env_var: "border-purple-500/30 bg-purple-500/10 text-purple-400",
    ui: "border-green-500/30 bg-green-500/10 text-green-400",
    static: "border-border/40 bg-background/50 text-muted-foreground",
  };
  const labelMap: Record<string, string> = {
    config_file: "Config File",
    env_var: "Env Var",
    ui: "UI Created",
    static: "Static",
  };
  const cls = colorMap[source ?? ""] ?? colorMap.static;
  const label = labelMap[source ?? ""] ?? labelMap.static;
  return (
    <span className={`inline-flex items-center border px-2 py-0.5 text-[9px] font-bold uppercase tracking-widest ${cls}`}>
      {label}
    </span>
  );
}

/** Short human label for the tier an exit sits in (mirrors the egress panel). */
/** Live count of what the Name field will create, once it holds more than one. */
function bulkNamePreview(value: string): JSX.Element | null {
  if (value.trim() === "") return null;
  const { names, invalid } = parseProviderNames(value);
  if (invalid.length > 0) {
    return (
      <p className="text-[11px] font-medium text-destructive">
        Not a valid name: {invalid.join(", ")}
      </p>
    );
  }
  if (names.length > 1) {
    return (
      <p className="text-[11px] font-medium text-accent">
        {names.length} providers will be created with this form.
      </p>
    );
  }
  return null;
}

function tierName(tier: number): string {
  if (tier < 0) return "preferred";
  if (tier === 0) return "rotation";
  return "fallback";
}

/**
 * The endpoint lists an extension (VPN) contributes to this provider, one
 * checkbox each. Only rendered when such exits exist, so a plain provider
 * never sees the section.
 */
function ExtensionEndpoints({ provider }: { provider: string }): JSX.Element | null {
  const { data } = useEgressStatus();
  const toggle = useToggleEgressExit();
  const lists = useEgressServerLists();
  const status = data?.providers.find((entry) => entry.provider === provider);
  const exits = (status?.exits ?? []).filter((exit) => exit.source === "extension");
  const bound = serversBoundTo(lists, provider);
  if (!provider || (exits.length === 0 && bound.servers.length === 0)) return null;

  const total = bound.lists.reduce((sum, list) => sum + list.payload.servers.length, 0);
  const coreKind = bound.lists[0]?.payload.core_kind ?? "";

  // Nothing is routed yet: the extension has endpoints bound here but no
  // tunnel core to send them through, so there are no exits to toggle.
  if (exits.length === 0) {
    return (
      <div className="flex flex-col gap-2 border border-accent/25 bg-accent/5 px-3 py-2.5">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-[11px] font-bold uppercase tracking-wider text-accent">Extension endpoints</span>
          <Pill tone="accent">{bound.servers.length}</Pill>
          {bound.lists.map((list) => (
            <Pill key={list.id} tone="muted">{list.name}</Pill>
          ))}
        </div>
        <div className="flex flex-wrap gap-1.5">
          {bound.servers.map((server) => (
            <span
              key={`${server.host}:${server.port}`}
              title={[server.name, server.protocol, server.alive === false ? "not answering" : "answering"].filter(Boolean).join(" · ")}
              className={
                server.alive === false
                  ? "border border-border/30 bg-background/40 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground line-through"
                  : "border border-accent/30 bg-accent/10 px-1.5 py-0.5 font-mono text-[11px] text-accent"
              }
            >
              {server.host}:{server.port}
            </span>
          ))}
        </div>
        <p className="text-[11px] leading-relaxed text-muted-foreground">
          {total > bound.servers.length ? `${bound.servers.length} of ${total} endpoints kept · ` : ""}
          {coreKind === "none"
            ? "report-only: no tunnel core is configured, so traffic does not use them yet."
            : "bound, but no exit is registered for this provider yet."}
        </p>
      </div>
    );
  }

  const active = exits.filter((exit) => !exit.disabled).length;
  return (
    <div className="flex flex-col gap-2 border border-accent/25 bg-accent/5 px-3 py-2.5">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-[11px] font-bold uppercase tracking-wider text-accent">Extension endpoints</span>
        <Pill tone="accent">{active} of {exits.length} in use</Pill>
      </div>
      <ul className="flex max-h-52 flex-col gap-1 overflow-y-auto">
        {exits.map((exit) => (
          <li
            key={exit.name}
            className={`flex items-center gap-2 border border-border/30 bg-background/40 px-2.5 py-1.5 ${exit.disabled ? "opacity-60" : ""}`}
          >
            <button
              type="button"
              role="checkbox"
              aria-checked={!exit.disabled}
              aria-label={`${exit.disabled ? "Use" : "Stop using"} ${exit.name}`}
              disabled={toggle.isPending}
              onClick={() => toggle.mutate({ provider, exit: exit.name, disabled: !exit.disabled })}
              className="shrink-0 p-0.5 hover:bg-border/20 transition-colors disabled:opacity-50"
              title={exit.disabled ? "Add this endpoint to the rotation" : "Keep this endpoint out of the rotation"}
            >
              {exit.disabled ? (
                <SquareIcon className="h-4 w-4 text-muted-foreground" />
              ) : (
                <CheckSquareIcon className="h-4 w-4 text-accent" />
              )}
            </button>
            <span className={`min-w-0 flex-1 truncate text-[12px] text-foreground ${exit.disabled ? "line-through" : ""}`} title={exit.name}>
              {exit.name.replace(/^vpn:/, "")}
            </span>
            <Pill tone="muted">{tierName(exit.tier)}</Pill>
          </li>
        ))}
      </ul>
      <p className="text-[11px] leading-relaxed text-muted-foreground">
        Endpoint lists contributed by an extension. Uncheck one to keep it out of this provider's rotation; the extension keeps refreshing the rest.
      </p>
    </div>
  );
}

/** One grouped block of the provider form: a titled container so the modal
 *  reads as Connection / Models / Egress instead of one flat field list. */
function Section({
  title,
  hint,
  aside,
  children,
}: {
  title: string;
  hint?: string;
  aside?: ReactNode;
  children: ReactNode;
}): JSX.Element {
  return (
    <section className="flex flex-col gap-3 border border-border/40 bg-background/20 px-3 py-3 sm:px-4 sm:py-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="flex flex-col gap-1">
          <h4 className="text-[11px] font-bold uppercase tracking-wider text-accent">{title}</h4>
          {hint ? <p className="max-w-prose text-[11px] leading-relaxed text-muted-foreground">{hint}</p> : null}
        </div>
        {aside}
      </div>
      {children}
    </section>
  );
}

/**
 * The at-a-glance address strip on a provider card: one chip per configured
 * source IP, coloured by its live state, clickable to take it out of (or back
 * into) the rotation immediately — no save round-trip. Extension endpoints are
 * summarised as a count and managed in the Egress drill-down below.
 */
function ProviderIpStrip({ name, onManage }: { name: string; onManage: () => void }): JSX.Element | null {
  const { data } = useEgressStatus();
  const toggle = useToggleEgressExit();
  const exits = data?.providers.find((entry) => entry.provider === name)?.exits ?? [];
  if (exits.length === 0) return null;

  const configExits = exits.filter((exit) => exit.source !== "extension");
  const extensionCount = exits.length - configExits.length;
  const shown = configExits.slice(0, 8);
  const hidden = configExits.length - shown.length;

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">IPs</span>
      {shown.map((exit) => {
        const state = exitState(exit);
        return (
          <button
            key={exit.name}
            type="button"
            disabled={toggle.isPending}
            onClick={() => toggle.mutate({ provider: name, exit: exit.name, disabled: !exit.disabled })}
            className={cn(
              "inline-flex items-center gap-1.5 border px-1.5 py-0.5 font-mono text-[10px] transition-colors disabled:opacity-60",
              exit.disabled
                ? "border-border/40 text-muted-foreground hover:border-accent/40"
                : "border-border/60 text-foreground hover:border-accent/50",
            )}
            title={`${exitDestination(exit)} — ${state.label}${exit.requests ? `, ${exit.ok}/${exit.requests} ok` : ""}. Click to ${exit.disabled ? "put it back into" : "take it out of"} the rotation.`}
            aria-label={`${exit.disabled ? "Resume" : "Pause"} ${exit.name} for ${name}`}
          >
            <span className={cn("h-1.5 w-1.5 shrink-0 rounded-full", toneDotClass(state.tone))} />
            <span className={cn(exit.disabled && "line-through")}>{exitDestination(exit)}</span>
          </button>
        );
      })}
      {hidden > 0 ? <span className="text-[10px] text-muted-foreground">+{hidden} more</span> : null}
      {extensionCount > 0 ? <Pill tone="accent">{extensionCount} extension exits</Pill> : null}
      <button
        type="button"
        onClick={onManage}
        className="text-[10px] font-bold uppercase tracking-wider text-accent transition-colors hover:text-accent-hover"
        title="Open this provider's settings"
      >
        Manage IPs
      </button>
    </div>
  );
}

interface ProviderModalProps {
  mode: "add" | "edit";
  initial: (ProviderFormData & { originalName?: string; apiKeySet?: boolean }) | undefined;
  onClose: () => void;
  onSaved: () => void;
}
function ProviderModal({ mode, initial, onClose, onSaved }: ProviderModalProps): JSX.Element {
  const [form, setForm] = useState<ProviderFormData>(initial ?? { name: "", type: "", base_url: "", api_version: "", api_key: "", models: "", bind_ip: "", pool_only: false, user_agent: "", disable_api_key: false, auto_fetch_models: true });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [presetConfirm, setPresetConfirm] = useState<{ preset: { name: string; type: string; base_url: string; auth_method?: string; key_optional?: boolean; description: string }; field: string } | null>(null);

  // Live egress rows for the provider being edited: every source IP then
  // carries its state, its traffic, and a pause/resume control that applies
  // immediately — the rotation toggle is a runtime switch, not a form field.
  const egressProvider = initial?.originalName ?? form.name;
  const { data: egressData } = useEgressStatus();
  const exitToggle = useToggleEgressExit();
  const egressExits = (egressData?.providers.find((entry) => entry.provider === egressProvider)?.exits ?? []).filter(
    (exit) => exit.source !== "extension",
  );
  const egressOff = egressExits.filter((exit) => exit.disabled).length;

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      const { autofetch_filter, autofetch_filter_text, ...rest } = form;
      const payload = {
        ...rest,
        autofetch_filter: resolveAutofetchFilter(autofetch_filter ?? initial?.autofetch_filter, autofetch_filter_text ?? ""),
      };
      if (mode === "add") {
        const { names, invalid } = parseProviderNames(form.name);
        if (names.length === 0) {
          setError("Enter at least one provider name.");
          return;
        }
        if (invalid.length > 0) {
          setError(
            `Not a valid provider name: ${invalid.join(", ")}. Use letters, digits, dots, dashes or underscores.`,
          );
          return;
        }
        if (names.length === 1) {
          await createProvider({ ...payload, name: names[0] ?? "" });
        } else {
          // A batch: create them one at a time so a single failure leaves the
          // rest standing, then keep only the failures in the field so a
          // second attempt retries exactly those.
          const { created, failed } = await createProviders(payload, names);
          if (failed.length > 0) {
            setForm({ ...form, name: failed.map((entry) => entry.name).join(", ") });
            setError(
              `Created ${created.length} of ${names.length}. Failed: ${failed
                .map((entry) => `${entry.name} — ${entry.error}`)
                .join("; ")}`,
            );
            onSaved();
            return;
          }
        }
      } else {
        const originalName = initial?.originalName ?? form.name;
        if (originalName && form.name !== originalName) {
          await updateProvider(originalName, { ...payload, new_name: form.name });
        } else {
          await updateProvider(originalName, payload);
        }
      }
      onSaved();
      onClose();
    } catch (err: any) {
      setError(err?.message || "Failed to save provider");
    } finally {
      setSaving(false);
    }
  };

  const providerTypes = ["openai", "anthropic", "gemini", "azure", "deepseek", "groq", "minimax", "ollama", "vllm", "openrouter", "oracle", "xai", "zai", "reranker", "custom"];

  // Detect preset when type or base_url changes
  const detectPresetDebounced = useCallback(async (type: string, baseUrl: string) => {
    if (mode !== "add" || !type) return;
    try {
      const { detectProviderPreset } = await import("@/lib/api/providers");
      const result = await detectProviderPreset(form.name, type, baseUrl);
      if (result.matched && result.preset) {
        setPresetConfirm({ preset: result.preset, field: result.preset.name });
      }
    } catch {
      // Ignore detection errors
    }
  }, [mode, form.name]);

  const handleTypeChange = (newType: string) => {
    setForm({ ...form, type: newType });
    detectPresetDebounced(newType, form.base_url);
  };

  const handleBaseUrlChange = (newUrl: string) => {
    setForm({ ...form, base_url: newUrl });
    detectPresetDebounced(form.type, newUrl);
  };

  const applyPreset = (preset: { type: string; base_url: string; auth_method?: string; key_optional?: boolean }) => {
    setForm({
      ...form,
      type: preset.type,
      base_url: preset.base_url,
    });
    setPresetConfirm(null);
  };

  const dismissPreset = () => {
    setPresetConfirm(null);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/50 backdrop-blur-sm" onClick={onClose}>
      <div className="w-full sm:max-w-2xl max-h-[90vh] overflow-y-auto border border-border/60 bg-surface shadow-2xl" onClick={(e) => e.stopPropagation()}>
        <div className="sticky top-0 z-10 flex items-center justify-between border-b border-border/50 bg-surface px-4 py-3 sm:px-6">
          <div className="flex flex-col gap-0.5">
            <h3 className="font-semibold text-[15px] tracking-tight text-foreground">{mode === "add" ? "Add Provider" : "Edit Provider"}</h3>
            {form.name ? <span className="font-mono text-[11px] text-muted-foreground">{form.name}</span> : null}
          </div>
          <button onClick={onClose} aria-label="Close" className="p-1 hover:bg-border/20 transition-colors"><XIcon className="h-4 w-4 text-muted-foreground" /></button>
        </div>
        <div className="flex flex-col gap-4 px-4 py-4 sm:px-6 sm:py-5">
          <Section title="Connection" hint="Where the gateway reaches this provider.">
            <div className="flex flex-col gap-1.5">
              <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Name</label>
              <Input type="text" placeholder={mode === "add" ? "my-provider, backup-1 backup-2" : "my-provider"} value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })} />
              {mode === "edit" && (
                <div className="text-[11px] text-muted-foreground">Rename by editing this value. Pools that referenced the old name are updated automatically.</div>
              )}
              {mode === "add" && (
                <div className="text-[11px] text-muted-foreground">
                  One name per provider. Separate several with a comma or a space to create them all at once — everything else on this form applies to each.
                </div>
              )}
              {mode === "add" && bulkNamePreview(form.name)}
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Type</label>
                <select className="field-input w-full" value={form.type}
                  onChange={(e) => handleTypeChange(e.target.value)}>
                  <option value="">Select type...</option>
                  {providerTypes.map((t) => <option key={t} value={t}>{t}</option>)}
                </select>
              </div>
              <div className="flex flex-col gap-1.5">
                <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">API Version</label>
                <Input type="text" placeholder="2024-01-01" value={form.api_version}
                  onChange={(e) => setForm({ ...form, api_version: e.target.value })} />
              </div>
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Base URL</label>
              <Input type="text" placeholder="https://api.openai.com/v1" value={form.base_url}
                onChange={(e) => handleBaseUrlChange(e.target.value)} />
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">API Key</label>
              <Input type="password" placeholder={mode === "edit" && initial?.apiKeySet ? "Leave empty to keep existing key" : "sk-..."} value={form.api_key}
                onChange={(e) => setForm({ ...form, api_key: e.target.value })} />
              {mode === "edit" && initial?.apiKeySet && (
                <div className="text-[11px] text-success">A key is already set for this provider. Leave empty to keep it.</div>
              )}
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">User Agent</label>
              <Input type="text" placeholder="my-app/1.0" value={form.user_agent ?? ""}
                onChange={(e) => setForm({ ...form, user_agent: e.target.value })} />
              <div className="text-[11px] text-muted-foreground">Custom User-Agent header sent to the upstream provider</div>
            </div>
          </Section>

          <Section title="Models" hint="An explicit list wins over discovery; auto-fetch keeps the inventory in sync with the upstream /models endpoint.">
            <div className="flex flex-col gap-1.5">
              <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Models</label>
              <Input type="text" placeholder="gpt-4, gpt-3.5-turbo" value={form.models}
                onChange={(e) => setForm({ ...form, models: e.target.value })} />
              <div className="text-[11px] text-muted-foreground">Comma separated model IDs</div>
            </div>

            <div className="flex items-center justify-between gap-3 border border-border/40 bg-background/30 px-3 py-2.5">
              <div className="flex flex-col gap-1 pr-2">
                <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Auto-fetch models</label>
                <div className="text-[11px] text-muted-foreground">Automatically discover available models via the provider's /models endpoint.</div>
              </div>
              <Switch
                checked={form.auto_fetch_models ?? true}
                size="sm"
                onCheckedChange={(v) => setForm({ ...form, auto_fetch_models: v })}
                aria-label="Auto-fetch models"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Auto-fetch filter</label>
              <div className="relative">
                <Input type="text" placeholder="free, flash  (empty = keep all)" value={form.autofetch_filter_text ?? ""}
                  onChange={(e) => setForm({ ...form, autofetch_filter_text: e.target.value })}
                  className="pr-8" />
                {(form.autofetch_filter_text ?? "") !== "" && (
                  <button type="button" onClick={() => setForm({ ...form, autofetch_filter_text: "" })}
                    className="absolute right-2 top-1/2 -translate-y-1/2 p-0.5 hover:bg-border/30 rounded transition-colors"
                    title="Clear filter">
                    <XIcon className="h-3.5 w-3.5 text-muted-foreground" />
                  </button>
                )}
              </div>
              <div className="text-[11px] text-muted-foreground">Comma-separated substrings. Only models whose ID contains every value are kept. Leave empty to keep all.</div>
              {initial?.autofetch_filter && filterToText(initial.autofetch_filter) !== "" && (form.autofetch_filter_text ?? "") === "" && (
                <div className="text-[11px] text-warning">Warning: {hasAdvancedConditions(initial.autofetch_filter) ? "This provider has an advanced filter set." : "This provider has an autofetch filter set."} Clearing the text will remove it.</div>
              )}
              {initial?.autofetch_filter && hasAdvancedConditions(initial.autofetch_filter) && (form.autofetch_filter_text ?? "").trim() !== filterToText(initial.autofetch_filter).trim() && (
                <div className="text-[11px] text-warning">Warning: editing this text replaces the advanced rules (not_contains, regex, price) with plain substrings.</div>
              )}
            </div>
          </Section>

          <Section
            title="Source IPs & egress"
            hint="Checked addresses are registered as this provider's exits and the first one stays its bind_ip. Pause / Resume flips an address in the live rotation immediately; the checkbox only decides what is saved."
            aside={
              <div className="flex flex-wrap items-center gap-1.5">
                <Pill tone="muted">{form.egress_strategy || "round_robin"}</Pill>
                {mode === "edit" && egressExits.length > 0 ? (
                  <Pill tone={egressOff > 0 ? "warning" : "success"}>
                    {egressExits.length - egressOff} of {egressExits.length} in rotation
                  </Pill>
                ) : null}
              </div>
            }
          >
            <IpListEditor
              value={
                form.bind_ips && form.bind_ips.length > 0
                  ? form.bind_ips
                  : form.bind_ip
                    ? [form.bind_ip]
                    : []
              }
              onChange={(next) => setForm({ ...form, bind_ips: next, bind_ip: next[0] ?? "" })}
              exits={egressExits}
              onExitToggle={(ip, disabled) => {
                const exit = exitForIp(egressExits, ip);
                exitToggle.mutate({ provider: egressProvider, exit: exit?.name ?? ip, disabled });
              }}
              togglePending={exitToggle.isPending}
            />
            <div className="flex flex-col gap-1.5">
              <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Egress strategy</label>
              <select
                className="field-input w-full"
                value={form.egress_strategy ?? ""}
                onChange={(e) => setForm({ ...form, egress_strategy: e.target.value })}
              >
                <option value="">round_robin (default)</option>
                <option value="round_robin">round_robin</option>
                <option value="random">random</option>
                <option value="weighted">weighted</option>
                <option value="first">first</option>
              </select>
              <div className="text-[11px] text-muted-foreground">How one exit is picked from the set: rotate in order, at random, weighted by weight, or always the first.</div>
            </div>
            {mode === "edit" && <ExtensionEndpoints provider={initial?.originalName ?? form.name} />}
          </Section>

          <Section title="Advanced" hint="Optional behaviour switches.">
            <div className="flex items-center justify-between gap-3 border border-border/40 bg-background/30 px-3 py-2.5">
              <div className="flex flex-col gap-1 pr-2">
                <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Pool only</label>
                <div className="text-[11px] text-muted-foreground">Hide from model list; reachable only through a pool.</div>
              </div>
              <Switch
                checked={form.pool_only ?? false}
                size="sm"
                onCheckedChange={(v) => setForm({ ...form, pool_only: v })}
                aria-label="Pool only"
              />
            </div>
            <div className="flex items-center justify-between gap-3 border border-border/40 bg-background/30 px-3 py-2.5">
              <div className="flex flex-col gap-1 pr-2">
                <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Disable API Key</label>
                <div className="text-[11px] text-muted-foreground">Stop sending the static API key upstream. Use when an auth token supersedes it.</div>
              </div>
              <Switch
                checked={form.disable_api_key ?? false}
                size="sm"
                onCheckedChange={(v) => setForm({ ...form, disable_api_key: v })}
                aria-label="Disable API key"
              />
            </div>
          </Section>
        </div>
        {presetConfirm && (
          <div className="mx-4 mt-4 border border-accent/30 bg-accent/5 p-3 rounded sm:mx-6">
            <div className="flex items-start gap-2">
              <div className="flex-1">
                <div className="text-[12px] font-semibold text-accent">Preset detected: {presetConfirm.preset.name}</div>
                <div className="text-[11px] text-muted-foreground mt-1">{presetConfirm.preset.description}</div>
                <div className="text-[11px] text-muted-foreground mt-1">
                  Base URL: <span className="font-mono text-foreground">{presetConfirm.preset.base_url}</span>
                </div>
                {presetConfirm.preset.auth_method && (
                  <div className="text-[11px] text-muted-foreground">
                    Auth: <span className="font-mono text-foreground">{presetConfirm.preset.auth_method}</span>
                  </div>
                )}
              </div>
            </div>
            <div className="flex items-center gap-2 mt-2">
              <button
                onClick={() => applyPreset(presetConfirm.preset)}
                className="px-3 py-1 text-[11px] font-medium bg-accent text-accent-foreground hover:bg-accent/80 transition-colors"
              >
                Apply Preset
              </button>
              <button
                onClick={dismissPreset}
                className="px-3 py-1 text-[11px] font-medium text-muted-foreground hover:text-foreground transition-colors"
              >
                Dismiss
              </button>
            </div>
          </div>
        )}
        {error && <div className="px-4 pt-1 text-[13px] font-medium text-destructive sm:px-6">{error}</div>}
        <div className="sticky bottom-0 flex items-center gap-3 border-t border-border/50 bg-surface px-4 py-3 sm:px-6">
          <Button onClick={handleSave} disabled={saving}>
            <SaveIcon className="mr-1.5 h-3.5 w-3.5" />
            {saving ? "Saving..." : mode === "add" ? "Create Provider" : "Update Provider"}
          </Button>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          {mode === "edit" && egressExits.length > 0 ? (
            <span className="ml-auto hidden text-[11px] text-muted-foreground sm:block">
              Pause / Resume on an address applies right away — only the checkbox waits for a save.
            </span>
          ) : null}
        </div>
      </div>
    </div>
  );
}

export function ProvidersTab(): JSX.Element {
  const { config } = useSettings();
  const runtimeSettings = config?.settings as any;
  const queryClient = useQueryClient();

  const { data: providerStatus, isLoading, refetch } = useQuery({
    queryKey: ["provider-status"],
    queryFn: fetchProviderStatus,
  });

  // Manual model sync button per provider: refetches every provider catalog
  // immediately (models removed upstream drop out of the registry), even
  // when auto-fetch is enabled and also when it is disabled for that provider.
  const [syncingProvider, setSyncingProvider] = useState<string | null>(null);
  const syncModels = async (name: string) => {
    if (syncingProvider) return;
    setSyncingProvider(name);
    try {
      await refreshRuntime();
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["models"] }),
        queryClient.invalidateQueries({ queryKey: ["provider-status"] }),
        queryClient.invalidateQueries({ queryKey: ["pools"] }),
      ]);
    } catch {
      // Spinner stops; the models list simply keeps the previous snapshot.
    } finally {
      setSyncingProvider(null);
    }
  };

  // External auth is extension-only: the gateway answers 404 on
  // /external-auth/* until an extension providing the external_auth feature
  // is applied. A failed fetch hides every auth control in this tab.
  const { data: externalAuthProviders } = useQuery({
    queryKey: ["external-auth-providers"],
    queryFn: fetchExternalAuthProviders,
    retry: false,
  });
  const externalAuthEnabled = Array.isArray(externalAuthProviders);

  // Applied extensions that provide the external_auth feature (device-flow
  // auth, …). Each carries its own accent so the link control can render one
  // colored key per enabled flow.
  const { data: externalAuthExtensions = [] } = useQuery({
    queryKey: ["extensions"],
    queryFn: fetchExtensions,
    enabled: externalAuthEnabled,
    retry: false,
    select: (list: Extension[]) =>
      list.filter((e) => Boolean(e.applied) && (e.provides?.features ?? []).includes("external_auth")),
  });
  const externalAuthKeys = useMemo(
    () =>
      externalAuthExtensions
        .map((e) => ({
          id: e.id,
          name: e.name,
          // "Claude OAuth" / "OpenCode Auth" → "Link Claude account"
          label: linkAccountLabel(e.name),
          color: e.ui?.accent || undefined,
        }))
        .sort((a, b) => a.id.localeCompare(b.id)),
    [externalAuthExtensions],
  );

  const [modalOpen, setModalOpen] = useState<"add" | "edit" | null>(null);
  const [editingProvider, setEditingProvider] = useState<ProviderFormData & { originalName: string; apiKeySet?: boolean } | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkConfirm, setBulkConfirm] = useState<"enable" | "disable" | "auto-fetch-on" | "auto-fetch-off" | "delete" | null>(null);
  const [bulkEditField, setBulkEditField] = useState<"bind_ip" | "user_agent" | "autofetch_filter" | null>(null);
  const [bulkEditValue, setBulkEditValue] = useState("");
  const [bulkEditSaving, setBulkEditSaving] = useState(false);
  const [externalAuthDialogProvider, setExternalAuthDialogProvider] = useState<string | null>(null);

  const deleteMutation = useMutation({
    mutationFn: (name: string) => deleteProvider(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["provider-status"] });
      setDeleteConfirm(null);
    },
  });

  const toggleMutation = useMutation({
    mutationFn: ({ name, enabled }: { name: string; enabled: boolean }) => setProviderEnabled(name, enabled),
    onMutate: async ({ name, enabled }) => {
      await queryClient.cancelQueries({ queryKey: ["provider-status"] });
      const previous = queryClient.getQueryData<ProviderStatusResponse>(["provider-status"]);
      queryClient.setQueryData<ProviderStatusResponse>(["provider-status"], (old: ProviderStatusResponse | undefined) => {
        if (!old) return old;
        return {
          ...old,
          providers: old.providers.map((p) =>
            p.name === name ? { ...p, config: { ...p.config, enabled } } : p
          ),
        };
      });
      return previous ? { previous } : {};
    },
    onError: (_err: Error, _variables, context: { previous?: ProviderStatusResponse } | undefined) => {
      if (context?.previous) {
        queryClient.setQueryData(["provider-status"], context.previous);
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ["provider-status"] });
    },
  });

  const bulkMutation = useMutation({
    mutationFn: async ({ names, action }: { names: string[]; action: string }) => {
      for (const name of names) {
        if (action === "enable") {
          await updateProvider(name, { enabled: true });
        } else if (action === "disable") {
          await updateProvider(name, { enabled: false });
        } else if (action === "auto-fetch-on") {
          await updateProvider(name, { auto_fetch_models: true });
        } else if (action === "auto-fetch-off") {
          await updateProvider(name, { auto_fetch_models: false });
        } else if (action === "delete") {
          await deleteProvider(name);
        }
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ["provider-status"] });
      setSelected(new Set());
      setBulkConfirm(null);
    },
  });

  const handleBulkEdit = async () => {
    if (!bulkEditField || selected.size === 0) return;
    setBulkEditSaving(true);
    try {
      for (const name of selected) {
        const patch: Record<string, any> = {};
        if (bulkEditField === "bind_ip") patch.bind_ip = bulkEditValue;
        else if (bulkEditField === "user_agent") patch.user_agent = bulkEditValue;
        else if (bulkEditField === "autofetch_filter") {
          const text = bulkEditValue.trim();
          patch.autofetch_filter = { mode: "all", conditions: text === "" ? [] : text.split(",").map((v) => ({ contains: v.trim() })).filter((c) => c.contains) };
        }
        await updateProvider(name, patch);
      }
      queryClient.invalidateQueries({ queryKey: ["provider-status"] });
      setSelected(new Set());
      setBulkEditField(null);
      setBulkEditValue("");
    } finally {
      setBulkEditSaving(false);
    }
  };

  const providers = providerStatus?.providers ?? [];
  const summary = providerStatus?.summary;

  // External auth is an extension feature (provides.features includes
  // "external_auth"); show the link control only while an extension provides
  // it and for providers with auth_method=external or the extension-provided
  // CLI emulation type.
  const supportsExternalAuth = (provider: any) => {
    if (!externalAuthEnabled) return false;
    const auth = provider.config?.auth_method || "";
    if (auth === "external") return true;
    return provider.type === "cli-emulation";
  };

  // One place that turns a live provider row into a form payload, so the card
  // and the "Manage IPs" shortcut both open the same edit form.
  const openEdit = (provider: ProviderStatusItem): void => {
    const bindIps =
      provider.config?.bind_ips && provider.config.bind_ips.length > 0
        ? provider.config.bind_ips
        : provider.config?.bind_ip
          ? [provider.config.bind_ip]
          : [];
    setEditingProvider({
      name: provider.name,
      originalName: provider.name,
      type: provider.config?.type || provider.type || "",
      base_url: provider.config?.base_url || "",
      api_version: provider.config?.api_version || "",
      api_key: provider.config?.api_key || "",
      models: provider.config?.models?.join(", ") || "",
      bind_ip: bindIps[0] ?? "",
      bind_ips: bindIps,
      egress_strategy: provider.config?.egress_strategy || "",
      pool_only: provider.config?.pool_only ?? false,
      user_agent: provider.config?.user_agent || "",
      disable_api_key: provider.config?.disable_api_key ?? false,
      auto_fetch_models: provider.config?.auto_fetch_models ?? true,
      autofetch_filter: provider.config?.autofetch_filter ?? null,
      autofetch_filter_text: filterToText(provider.config?.autofetch_filter),
      apiKeySet: provider.config?.api_key_set ?? false,
    });
    setModalOpen("edit");
  };

  const allSelected = providers.length > 0 && providers.every((p) => selected.has(p.name));  const someSelected = providers.some((p) => selected.has(p.name)) && !allSelected;

  const allIds = useMemo(() => providers.map((p) => p.name), [providers]);
  const lastClickedRef = useRef<string | null>(null);

  const toggleSelectAll = useCallback((_e?: React.MouseEvent) => {
    if (allSelected) {
      setSelected(new Set());
    } else {
      setSelected(new Set(providers.map((p) => p.name)));
    }
    lastClickedRef.current = null;
  }, [allSelected, providers]);

  const toggleSelect = useCallback((name: string, e?: React.MouseEvent) => {
    if (e?.shiftKey && lastClickedRef.current !== null && lastClickedRef.current !== name) {
      const fromIdx = allIds.indexOf(lastClickedRef.current);
      const toIdx = allIds.indexOf(name);
      if (fromIdx !== -1 && toIdx !== -1) {
        const start = Math.min(fromIdx, toIdx);
        const end = Math.max(fromIdx, toIdx);
        const range = allIds.slice(start, end + 1);
        setSelected((prev) => {
          const next = new Set(prev);
          for (const rid of range) next.add(rid);
          return next;
        });
        lastClickedRef.current = name;
        return;
      }
    }
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(name)) {
        next.delete(name);
      } else {
        next.add(name);
      }
      return next;
    });
    lastClickedRef.current = name;
  }, [allIds]);


  const handleBulkAction = () => {
    if (!bulkConfirm || selected.size === 0) return;
    bulkMutation.mutate({ names: Array.from(selected), action: bulkConfirm });
  };

  return (
    <div className="flex flex-col gap-6">
      <Surface id="provider-config" className="p-6 scroll-mt-20">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="border border-border/40 bg-background/80 p-2">
                <ServerIcon className="h-4 w-4 text-accent" />
              </div>
              <SectionHeader
                title="Provider Configuration"
                subtitle="Active providers, pool assignments, and current provider health."
              />
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <RuntimeStatusBadge featureKey="providers" />
              <RuntimeStatusBadge featureKey="pools" />
              <RuntimeStatusBadge featureKey="models" />
            </div>
          </div>

          {summary && (
            <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
              <div className="border border-border/60 bg-surface-hover/20 p-3 text-center">
                <div className="text-[20px] font-bold text-foreground">{summary.total}</div>
                <div className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground mt-1">Total</div>
              </div>
              <div className="border border-success/20 bg-success/5 p-3 text-center">
                <div className="text-[20px] font-bold text-success">{summary.healthy}</div>
                <div className="text-[10px] font-bold uppercase tracking-wider text-success/70 mt-1">Healthy</div>
              </div>
              <div className="border border-warning/20 bg-warning/5 p-3 text-center">
                <div className="text-[20px] font-bold text-warning">{summary.degraded}</div>
                <div className="text-[10px] font-bold uppercase tracking-wider text-warning/70 mt-1">Degraded</div>
              </div>
              <div className="border border-destructive/20 bg-destructive/5 p-3 text-center">
                <div className="text-[20px] font-bold text-destructive">{summary.unhealthy}</div>
                <div className="text-[10px] font-bold uppercase tracking-wider text-destructive/70 mt-1">Unhealthy</div>
              </div>
              <div className="border border-border/40 bg-background/40 p-3 text-center">
                <div className="text-[20px] font-bold text-muted-foreground">{summary.disabled ?? 0}</div>
                <div className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground/70 mt-1">Disabled</div>
              </div>
            </div>
          )}

          <IpInventory />

          <ExternalIpsPanel providerNames={allIds} />

          <div className="flex items-center gap-2">
            <Button onClick={() => { setEditingProvider(null); setModalOpen("add"); }}>
              <PlusIcon className="mr-1.5 h-4 w-4" /> Add Provider
            </Button>
            <Button variant="outline" onClick={() => refetch()} disabled={isLoading}>
              <RefreshCwIcon className={`mr-1.5 h-3.5 w-3.5 ${isLoading ? "animate-spin" : ""}`} />
              Refresh
            </Button>
          </div>

          {/* Bulk actions toolbar */}
          {selected.size > 0 && (
            <div className="border border-accent/30 bg-accent/5 px-3 sm:px-4 py-2.5">
              <div className="flex items-center gap-3 mb-2">
                <span className="text-[12px] font-medium text-accent">{selected.size} selected</span>
                <div className="ml-auto">
                  <Button size="sm" variant="ghost" onClick={() => setSelected(new Set())} className="text-[11px] h-7">
                    Clear
                  </Button>
                </div>
              </div>
              <div className="flex items-center gap-1.5 overflow-x-auto pb-1 -mb-1">
                <Button size="sm" variant="outline" onClick={() => setBulkConfirm("enable")} className="text-[11px] h-7 shrink-0">
                  <CheckIcon className="mr-1 h-3 w-3" /> Enable
                </Button>
                <Button size="sm" variant="outline" onClick={() => setBulkConfirm("disable")} className="text-[11px] h-7 shrink-0">
                  <MinusIcon className="mr-1 h-3 w-3" /> Disable
                </Button>
                <Button size="sm" variant="outline" onClick={() => setBulkConfirm("auto-fetch-on")} className="text-[11px] h-7 shrink-0">
                  Auto-fetch On
                </Button>
                <Button size="sm" variant="outline" onClick={() => setBulkConfirm("auto-fetch-off")} className="text-[11px] h-7 shrink-0">
                  Auto-fetch Off
                </Button>
                <Button size="sm" variant="outline" onClick={() => { setBulkEditField("bind_ip"); setBulkEditValue(""); }} className="text-[11px] h-7 shrink-0">
                  Set Bind IP
                </Button>
                <Button size="sm" variant="outline" onClick={() => { setBulkEditField("user_agent"); setBulkEditValue(""); }} className="text-[11px] h-7 shrink-0">
                  Set User Agent
                </Button>
                <Button size="sm" variant="outline" onClick={() => { setBulkEditField("autofetch_filter"); setBulkEditValue(""); }} className="text-[11px] h-7 shrink-0">
                  Set Filter
                </Button>
                <Button size="sm" variant="outline" onClick={() => setBulkConfirm("delete")} className="text-[11px] h-7 shrink-0 text-destructive hover:bg-destructive/10">
                  <Trash2Icon className="mr-1 h-3 w-3" /> Delete
                </Button>
              </div>
            </div>
          )}

          {providers.length > 0 ? (
            <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
              {/* Select-all header */}
              <div className="col-span-full flex items-center gap-2 px-1">
                <button
                  onClick={(e: React.MouseEvent) => toggleSelectAll(e)}
                  className="flex items-center gap-1.5 text-[11px] text-muted-foreground hover:text-foreground transition-colors"
                >
                  {allSelected ? (
                    <CheckSquareIcon className="h-4 w-4 text-accent" />
                  ) : someSelected ? (
                    <div className="h-4 w-4 border-2 border-accent bg-accent/20 flex items-center justify-center"><MinusIcon className="h-2.5 w-2.5 text-accent" /></div>
                  ) : (
                    <SquareIcon className="h-4 w-4" />
                  )}
                  {allSelected ? "Deselect all" : someSelected ? `Select all (${providers.length})` : `Select all (${providers.length})`}
                </button>
              </div>

              {providers.map((provider) => (
                <div
                  key={provider.name}
                  className={`border bg-surface p-3 sm:p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30 ${
                    selected.has(provider.name)
                      ? "border-accent/50 bg-accent/5"
                      : provider.config?.enabled === false
                        ? "border-border/40 opacity-60"
                        : "border-border/40"
                  }`}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex min-w-0 items-center gap-2 sm:gap-3">
                      <button
                        onClick={(e: React.MouseEvent) => toggleSelect(provider.name, e)}
                        className="shrink-0 p-0.5 hover:bg-border/20 transition-colors"
                        title={selected.has(provider.name) ? "Deselect" : "Select"}
                      >
                        {selected.has(provider.name) ? (
                          <CheckSquareIcon className="h-4 w-4 text-accent" />
                        ) : (
                          <SquareIcon className="h-4 w-4 text-muted-foreground" />
                        )}
                      </button>
                      <ProviderMark provider={provider} />
                      <div className="flex min-w-0 flex-col gap-1">
                        <div className="flex min-w-0 flex-wrap items-center gap-1.5 sm:gap-2">
                          <span className="truncate text-[13px] sm:text-[14px] font-semibold text-foreground">{provider.name}</span>
                          <ConfigSourceBadge source={provider.config_source} />
                          {provider.config?.enabled === false && <Pill tone="muted">disabled</Pill>}
                          {provider.config?.pool_only && <Pill tone="accent">pool-only</Pill>}
                          {provider.config?.auto_fetch_models === false && <Pill tone="muted">no auto-fetch</Pill>}
                          {provider.config?.auto_fetch_models !== false && filterToText(provider.config?.autofetch_filter) && (
                            <Pill tone="accent" className="max-w-[14rem] truncate">filter: {filterToText(provider.config?.autofetch_filter)}</Pill>
                          )}
                          {provider.config?.user_agent && <Pill tone="accent">custom UA</Pill>}
                          {provider.config?.disable_api_key && <Pill tone="warning">key off</Pill>}
                          {(provider.config?.bind_ips?.length ?? 0) > 1 ? (
                            <Pill tone="muted">bind: {provider.config?.bind_ips?.length} ips</Pill>
                          ) : provider.config?.bind_ip ? (
                            <Pill tone="muted">bind: {provider.config.bind_ip}</Pill>
                          ) : null}
                          {supportsExternalAuth(provider) && provider.external_auth_status?.has_token && !provider.external_auth_status?.expired && (
                            <Pill tone="success">linked</Pill>
                          )}
                        </div>
                        <span className="text-[11px] text-muted-foreground">{provider.type || provider.config?.type || "custom"}</span>
                      </div>
                    </div>
                    <div className="flex items-center gap-0.5 sm:gap-1 shrink-0">
                      <button
                        onClick={() => void syncModels(provider.name)}
                        className="p-1.5 hover:bg-accent/10 transition-colors disabled:opacity-50"
                        disabled={syncingProvider !== null}
                        title="Sync models — fetch the model list now"
                        aria-label={`Sync models for ${provider.name}`}
                      >
                        <RefreshCwIcon className={cn("h-3.5 w-3.5 text-muted-foreground", syncingProvider === provider.name && "animate-spin")} />
                      </button>
                      <Switch
                        checked={provider.config?.enabled !== false}
                        size="sm"
                        disabled={toggleMutation.isPending}
                        onCheckedChange={(enabled) => toggleMutation.mutate({ name: provider.name, enabled })}
                        aria-label={`Toggle provider ${provider.name}`}
                        title={`${provider.config?.enabled === false ? "Enable" : "Disable"} ${provider.name}`}
                      />
                      <button onClick={() => openEdit(provider)} className="p-1.5 hover:bg-border/20 transition-colors" title="Edit provider">
                        <Edit3Icon className="h-3.5 w-3.5 text-muted-foreground" />
                      </button>
                      {supportsExternalAuth(provider) &&
                        (externalAuthKeys.length > 0
                          ? externalAuthKeys.map((key) => (
                              <button
                                key={key.id}
                                onClick={() => setExternalAuthDialogProvider(provider.name)}
                                className="p-1.5 hover:bg-accent/10 transition-colors"
                                style={key.color ? { color: key.color } : undefined}
                                title={key.label}
                                aria-label={`${key.label} for ${provider.name}`}
                              >
                                <KeyIcon className="h-3.5 w-3.5" />
                              </button>
                            ))
                          : (
                              <button
                                onClick={() => setExternalAuthDialogProvider(provider.name)}
                                className="p-1.5 hover:bg-accent/10 transition-colors text-accent"
                                title="Link account"
                                aria-label={`Link account for ${provider.name}`}
                              >
                                <KeyIcon className="h-3.5 w-3.5" />
                              </button>
                            ))}
                      <button onClick={() => setDeleteConfirm(provider.name)} className="p-1.5 hover:bg-destructive/10 transition-colors" title="Delete provider">
                        <Trash2Icon className="h-3.5 w-3.5 text-destructive/70" />
                      </button>
                    </div>
                  </div>
                  <div className="flex items-center gap-3 text-[11px] sm:text-[12px] text-muted-foreground">
                    <StatusChip enabled={provider.status === "healthy"} />
                    <span className="font-mono">{provider.runtime?.discovered_model_count ?? 0} models</span>
                  </div>
                   {provider.config?.base_url && (
                     <div className="text-[10px] sm:text-[11px] text-muted-foreground font-mono truncate" title={provider.config.base_url}>
                       {provider.config.base_url}
                     </div>
                   )}
                   <ProviderIpStrip name={provider.name} onManage={() => openEdit(provider)} />
                   <ProviderEgressPanel provider={provider.name} />
                  {provider.config?.models && provider.config.models.length > 0 && (
                    <div className="flex flex-wrap gap-1 mt-1">
                      {provider.config.models.slice(0, 3).map((m) => (
                        <span key={m} className="inline-flex items-center border border-border/30 bg-background/30 px-1.5 sm:px-2 py-0.5 text-[9px] sm:text-[10px] font-medium text-muted-foreground">{m}</span>
                      ))}
                      {provider.config.models.length > 3 && <span className="text-[9px] sm:text-[10px] text-muted-foreground self-center">+{provider.config.models.length - 3} more</span>}
                    </div>
                  )}
                </div>
              ))}
            </div>
          ) : (
            <p className="text-[13px] leading-relaxed text-foreground/80">No provider data available. Add a provider or check the config file.</p>
          )}

          {runtimeSettings?.client?.enabled_passthrough_providers && (
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Pool assignments</div>
              <div className="mt-2 text-[14px] font-medium text-foreground">Passthrough providers: {(runtimeSettings.client.enabled_passthrough_providers as string[])?.join(", ") || "None"}</div>
            </div>
          )}
        </div>
      </Surface>

      {modalOpen && (
        <ProviderModal
          mode={modalOpen}
          initial={editingProvider ?? undefined}
          onClose={() => { setModalOpen(null); setEditingProvider(null); }}
          onSaved={() => { queryClient.invalidateQueries({ queryKey: ["provider-status"] }); }}
        />
      )}

      {deleteConfirm && (
        <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/50 backdrop-blur-sm" onClick={() => setDeleteConfirm(null)}>
          <div className="w-full sm:max-w-sm border border-border/60 bg-surface p-4 sm:p-6 shadow-2xl" onClick={(e) => e.stopPropagation()}>
            <h3 className="font-semibold text-[15px] tracking-tight text-foreground mb-2">Delete Provider</h3>
            <p className="text-[13px] text-foreground/80 mb-4">Are you sure you want to delete provider <strong>{deleteConfirm}</strong>? This will remove any UI-created overrides for this provider.</p>
            <div className="flex items-center gap-3">
              <Button onClick={() => deleteMutation.mutate(deleteConfirm)} disabled={deleteMutation.isPending} className="bg-destructive hover:bg-destructive/90">
                {deleteMutation.isPending ? "Deleting..." : "Delete"}
              </Button>
              <Button variant="outline" onClick={() => setDeleteConfirm(null)}>Cancel</Button>
            </div>
            {deleteMutation.isError && <div className="mt-3 text-[13px] font-medium text-destructive">{deleteMutation.error?.message}</div>}
          </div>
        </div>
      )}

      {bulkConfirm && (
        <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/50 backdrop-blur-sm" onClick={() => setBulkConfirm(null)}>
          <div className="w-full sm:max-w-sm border border-border/60 bg-surface p-4 sm:p-6 shadow-2xl" onClick={(e) => e.stopPropagation()}>
            <h3 className="font-semibold text-[15px] tracking-tight text-foreground mb-2">Bulk Action</h3>
            <p className="text-[13px] text-foreground/80 mb-4">
              {bulkConfirm === "enable" && <>Enable <strong>{selected.size}</strong> selected provider{selected.size !== 1 ? "s" : ""}?</>}
              {bulkConfirm === "disable" && <>Disable <strong>{selected.size}</strong> selected provider{selected.size !== 1 ? "s" : ""}?</>}
              {bulkConfirm === "auto-fetch-on" && <>Turn on auto-fetch for <strong>{selected.size}</strong> provider{selected.size !== 1 ? "s" : ""}?</>}
              {bulkConfirm === "auto-fetch-off" && <>Turn off auto-fetch for <strong>{selected.size}</strong> provider{selected.size !== 1 ? "s" : ""}?</>}
              {bulkConfirm === "delete" && <>Delete <strong>{selected.size}</strong> selected provider{selected.size !== 1 ? "s" : ""}? This cannot be undone.</>}
            </p>
            <div className="flex items-center gap-3">
              <Button
                onClick={handleBulkAction}
                disabled={bulkMutation.isPending}
                className={bulkConfirm === "delete" ? "bg-destructive hover:bg-destructive/90" : ""}
              >
                {bulkMutation.isPending ? "Processing..." : "Confirm"}
              </Button>
              <Button variant="outline" onClick={() => setBulkConfirm(null)}>Cancel</Button>
            </div>
            {bulkMutation.isError && <div className="mt-3 text-[13px] font-medium text-destructive">{bulkMutation.error?.message}</div>}
          </div>
        </div>
      )}

      {externalAuthDialogProvider && (
        <ExternalAuthDialog
          providerName={externalAuthDialogProvider}
          open={!!externalAuthDialogProvider}
          onOpenChange={(open) => {
            if (!open) setExternalAuthDialogProvider(null);
          }}
          onComplete={() => {
            queryClient.invalidateQueries({ queryKey: ["provider-status"] });
          }}
        />
      )}

      {bulkEditField && (
        <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/50 backdrop-blur-sm" onClick={() => setBulkEditField(null)}>
          <div className="w-full sm:max-w-md border border-border/60 bg-surface p-4 sm:p-6 shadow-2xl" onClick={(e) => e.stopPropagation()}>
            <h3 className="font-semibold text-[15px] tracking-tight text-foreground mb-1">
              {bulkEditField === "bind_ip" && "Set Bind IP"}
              {bulkEditField === "user_agent" && "Set User Agent"}
              {bulkEditField === "autofetch_filter" && "Set Auto-fetch Filter"}
            </h3>
            <p className="text-[12px] text-muted-foreground mb-4">
              Applied to <strong>{selected.size}</strong> selected provider{selected.size !== 1 ? "s" : ""}.
              {bulkEditField === "autofetch_filter" ? " Comma-separated substrings. Leave empty to clear." : " Leave empty to clear."}
            </p>
            <div className="relative">
              <Input
                type="text"
                placeholder={bulkEditField === "bind_ip" ? "203.0.113.10" : bulkEditField === "user_agent" ? "my-app/1.0" : "free, flash  (empty = clear)"}
                value={bulkEditValue}
                onChange={(e) => setBulkEditValue(e.target.value)}
                autoFocus
                className="pr-8"
              />
              {bulkEditValue !== "" && (
                <button type="button" onClick={() => setBulkEditValue("")}
                  className="absolute right-2 top-1/2 -translate-y-1/2 p-0.5 hover:bg-border/30 rounded transition-colors"
                  title="Clear">
                  <XIcon className="h-3.5 w-3.5 text-muted-foreground" />
                </button>
              )}
            </div>
            <div className="flex items-center gap-3 mt-4">
              <Button onClick={handleBulkEdit} disabled={bulkEditSaving}>
                {bulkEditSaving ? "Applying..." : "Apply to All"}
              </Button>
              <Button variant="outline" onClick={() => setBulkEditField(null)}>Cancel</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

import { Surface, SectionHeader } from "@/components/ui/surface";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { RuntimeStatusBadge, useSettings, StatusChip } from "./SettingsContext";
import { ServerIcon, RefreshCwIcon, PlusIcon, Edit3Icon, Trash2Icon, SaveIcon, XIcon } from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { fetchProviderStatus, createProvider, updateProvider, deleteProvider, type ProviderFormData } from "@/lib/api/providers";
import { withBasePath } from "@/lib/basepath";
import { useState } from "react";

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

interface ProviderModalProps {
  mode: "add" | "edit";
  initial: (ProviderFormData & { originalName?: string }) | undefined;
  onClose: () => void;
  onSaved: () => void;
}

function ProviderModal({ mode, initial, onClose, onSaved }: ProviderModalProps): JSX.Element {
  const [form, setForm] = useState<ProviderFormData>(initial ?? { name: "", type: "", base_url: "", api_version: "", api_key: "", models: "" });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      if (mode === "add") {
        await createProvider(form);
      } else {
        await updateProvider(initial?.originalName ?? form.name, form);
      }
      onSaved();
      onClose();
    } catch (err: any) {
      setError(err?.message || "Failed to save provider");
    } finally {
      setSaving(false);
    }
  };

  const providerTypes = ["openai", "anthropic", "google", "azure", "aws-bedrock", "mistral", "cohere", "jina", "togetherai", "openrouter", "deepseek", "xai", "custom"];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" onClick={onClose}>
      <div className="w-full max-w-lg  border border-border/60 bg-surface p-6 shadow-2xl" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-semibold text-[15px] tracking-tight text-foreground">{mode === "add" ? "Add Provider" : "Edit Provider"}</h3>
          <button onClick={onClose} className="p-1 hover:bg-border/20 transition-colors"><XIcon className="h-4 w-4 text-muted-foreground" /></button>
        </div>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Name</label>
            <Input type="text" placeholder="my-provider" value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              disabled={mode === "edit"} className={mode === "edit" ? "opacity-60" : ""} />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Type</label>
            <select className="field-input w-full" value={form.type}
              onChange={(e) => setForm({ ...form, type: e.target.value })}>
              <option value="">Select type...</option>
              {providerTypes.map((t) => <option key={t} value={t}>{t}</option>)}
            </select>
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Base URL</label>
            <Input type="text" placeholder="https://api.openai.com/v1" value={form.base_url}
              onChange={(e) => setForm({ ...form, base_url: e.target.value })} />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">API Version</label>
            <Input type="text" placeholder="2024-01-01" value={form.api_version}
              onChange={(e) => setForm({ ...form, api_version: e.target.value })} />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">API Key</label>
            <Input type="password" placeholder="sk-..." value={form.api_key}
              onChange={(e) => setForm({ ...form, api_key: e.target.value })} />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Models</label>
            <Input type="text" placeholder="gpt-4, gpt-3.5-turbo" value={form.models}
              onChange={(e) => setForm({ ...form, models: e.target.value })} />
            <div className="text-[11px] text-muted-foreground">Comma separated model IDs</div>
          </div>
        </div>
        {error && <div className="mt-3 text-[13px] font-medium text-destructive">{error}</div>}
        <div className="flex items-center gap-3 mt-4 pt-3 border-t border-border/50">
          <Button onClick={handleSave} disabled={saving}>
            <SaveIcon className="mr-1.5 h-3.5 w-3.5" />
            {saving ? "Saving..." : mode === "add" ? "Create Provider" : "Update Provider"}
          </Button>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
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

  const [modalOpen, setModalOpen] = useState<"add" | "edit" | null>(null);
  const [editingProvider, setEditingProvider] = useState<ProviderFormData & { originalName: string } | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  const deleteMutation = useMutation({
    mutationFn: (name: string) => deleteProvider(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["provider-status"] });
      setDeleteConfirm(null);
    },
  });

  const providers = providerStatus?.providers ?? [];
  const summary = providerStatus?.summary;

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
            <div className="grid grid-cols-4 gap-3">
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
            </div>
          )}

          <div className="flex items-center gap-2">
            <Button onClick={() => { setEditingProvider(null); setModalOpen("add"); }} size="sm">
              <PlusIcon className="mr-1.5 h-3.5 w-3.5" /> Add Provider
            </Button>
          </div>

          {providers.length > 0 ? (
            <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
              {providers.map((provider) => (
                <div key={provider.name} className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30.5">
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex min-w-0 items-center gap-3">
                      <ProviderMark provider={provider} />
                      <div className="flex min-w-0 flex-col gap-1">
                        <div className="flex min-w-0 flex-wrap items-center gap-2">
                          <span className="truncate text-[14px] font-semibold text-foreground">{provider.name}</span>
                          <ConfigSourceBadge source={provider.config_source} />
                        </div>
                        <span className="text-[11px] text-muted-foreground">{provider.type || provider.config?.type || "custom"}</span>
                      </div>
                    </div>
                    <div className="flex items-center gap-1">
                      <button onClick={() => { setEditingProvider({ name: provider.name, originalName: provider.name, type: provider.config?.type || provider.type || "", base_url: provider.config?.base_url || "", api_version: provider.config?.api_version || "", api_key: "", models: provider.config?.models?.join(", ") || "" }); setModalOpen("edit"); }} className="p-1.5 hover:bg-border/20 transition-colors" title="Edit provider">
                        <Edit3Icon className="h-3.5 w-3.5 text-muted-foreground" />
                      </button>
                      <button onClick={() => setDeleteConfirm(provider.name)} className="p-1.5 hover:bg-destructive/10 transition-colors" title="Delete provider">
                        <Trash2Icon className="h-3.5 w-3.5 text-destructive/70" />
                      </button>
                    </div>
                  </div>
                  <div className="flex items-center gap-3 text-[12px] text-muted-foreground">
                    <StatusChip enabled={provider.status === "healthy"} />
                    <span className="font-mono">{provider.runtime?.discovered_model_count ?? 0} models</span>
                  </div>
                  {provider.config?.base_url && (
                    <div className="text-[11px] text-muted-foreground font-mono truncate" title={provider.config.base_url}>
                      {provider.config.base_url}
                    </div>
                  )}
                  {provider.config?.models && provider.config.models.length > 0 && (
                    <div className="flex flex-wrap gap-1 mt-1">
                      {provider.config.models.slice(0, 5).map((m) => (
                        <span key={m} className="inline-flex items-center border border-border/30 bg-background/30 px-2 py-0.5 text-[10px] font-medium text-muted-foreground">{m}</span>
                      ))}
                      {provider.config.models.length > 5 && <span className="text-[10px] text-muted-foreground self-center">+{provider.config.models.length - 5} more</span>}
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

          <div className="flex items-center gap-3 mt-2 border-t border-border/50 pt-4">
            <Button onClick={() => refetch()} disabled={isLoading}>
              <RefreshCwIcon className={`mr-2 h-4 w-4 ${isLoading ? "animate-spin" : ""}`} />
              {isLoading ? "Refreshing..." : "Refresh Provider Health"}
            </Button>
          </div>
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
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" onClick={() => setDeleteConfirm(null)}>
          <div className="w-full max-w-sm  border border-border/60 bg-surface p-6 shadow-2xl" onClick={(e) => e.stopPropagation()}>
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
    </div>
  );
}

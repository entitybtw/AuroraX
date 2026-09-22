import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Surface, SectionHeader } from "@/components/ui/surface";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ToggleField } from "@/components/ui/toggle-field";
import {
  KeyIcon,
  GlobeIcon,
  BoxesIcon,
  RefreshCwIcon,
  PlusIcon,
  Trash2Icon,
  CheckCircleIcon,
  XCircleIcon,
  ZapIcon,
  ShieldAlertIcon,
  PencilIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  AlertTriangleIcon,
  SparklesIcon,
} from "lucide-react";
import { apiFetch } from "@/lib/api/client";
import { usePools } from "@/lib/api/usePools";
import { fetchProviderStatus } from "@/lib/api/providers";
import {
  HeaderRulesEditor,
  openCodeSessionRule,
  type HeaderRule,
} from "@/components/settings/HeaderRulesEditor";

type TargetKind = "pool" | "provider" | "fallback" | "all";

// --- API hooks ---

function useServerProviders() {
  return useQuery({
    queryKey: ["provider-status"],
    queryFn: async () => {
      const res = await fetchProviderStatus();
      return res;
    },
    staleTime: 10_000,
  });
}

// --- Types ---

interface SessionHubStatus {
  total_mappings: number;
  by_provider: Record<string, number>;
  configured_providers: number;
  enabled_providers: number;
  storage_mode: string;
}

interface ProviderRule {
  name: string;
  enabled: boolean;
  headers: HeaderRule[];
}

interface MappingEntry {
  inbound_value: string;
  outbound_value: string;
  provider: string;
  created_at: string;
}

// --- API hooks ---

function useSessionHubStatus() {
  return useQuery({
    queryKey: ["sessionhub", "status"],
    queryFn: () => apiFetch<{ status: string; data: SessionHubStatus }>("/admin/api/v1/sessionhub/status").then(r => r.data),
    refetchInterval: 5000,
  });
}

function useSessionHubProviders() {
  return useQuery({
    queryKey: ["sessionhub", "providers"],
    queryFn: () => apiFetch<{ status: string; data: ProviderRule[] }>("/admin/api/v1/sessionhub/providers").then(r => r.data),
  });
}

function useSessionHubMappings() {
  return useQuery({
    queryKey: ["sessionhub", "mappings"],
    queryFn: () => apiFetch<{ status: string; data: { mappings: MappingEntry[]; total: number } }>("/admin/api/v1/sessionhub/mappings").then(r => r.data),
    refetchInterval: 5000,
  });
}

function useSessionHubMutations() {
  const qc = useQueryClient();
  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ["sessionhub"] });
  };

  const createProvider = useMutation({
    mutationFn: (data: { name: string; rule: ProviderRule }) =>
      apiFetch("/admin/api/v1/sessionhub/providers", { method: "POST", json: data }),
    onSuccess: invalidate,
  });

  const deleteProvider = useMutation({
    mutationFn: (name: string) =>
      apiFetch(`/admin/api/v1/sessionhub/providers/${encodeURIComponent(name)}`, { method: "DELETE" }),
    onSuccess: invalidate,
  });

  const updateProvider = useMutation({
    mutationFn: ({ name, rule }: { name: string; rule: ProviderRule }) =>
      apiFetch(`/admin/api/v1/sessionhub/providers/${encodeURIComponent(name)}`, { method: "PUT", json: rule }),
    onSuccess: invalidate,
  });

  const ensureOpenCode = useMutation({
    mutationFn: (name: string) =>
      apiFetch(`/admin/api/v1/sessionhub/providers/${encodeURIComponent(name)}/ensure-opencode`, {
        method: "POST",
      }),
    onSuccess: invalidate,
  });

  const clearMappings = useMutation({
    mutationFn: () => apiFetch("/admin/api/v1/sessionhub/mappings", { method: "DELETE" }),
    onSuccess: invalidate,
  });

  const setStorageMode = useMutation({
    mutationFn: (mode: "memory" | "disk") =>
      apiFetch("/admin/api/v1/sessionhub/storage", { method: "PUT", json: { mode } }),
    onSuccess: invalidate,
  });

  return {
    createProvider,
    deleteProvider,
    updateProvider,
    ensureOpenCode,
    clearMappings,
    setStorageMode,
  };
}

// --- Helpers ---

/** A header rule is "zen-safe" when it produces ses_/msg_ + 26 hex chars. */
function isZenSafe(hr: HeaderRule): boolean {
  if (hr.mode !== "generate" && hr.mode !== "map" && hr.mode !== "map_or_generate") return true;
  const charset = (hr.charset || "alphanumeric").toLowerCase();
  return hr.length === 26 && charset === "hex" && (hr.prefix === "ses_" || hr.prefix === "msg_");
}

function ruleHasWarnings(rule: ProviderRule): boolean {
  return rule.headers.some((h) => !isZenSafe(h));
}

function StatusBadge({ healthy, label }: { healthy: boolean; label: string }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] font-medium ${
        healthy
          ? "border-success/30 bg-success/10 text-success"
          : "border-destructive/30 bg-destructive/10 text-destructive"
      }`}
    >
      {healthy ? <CheckCircleIcon className="h-3 w-3" /> : <XCircleIcon className="h-3 w-3" />}
      {label}
    </span>
  );
}

function ModeChip({ header }: { header: HeaderRule }) {
  const warn = !isZenSafe(header);
  return (
    <span
      className={`inline-flex items-center gap-1 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium ${
        warn ? "text-amber-600 dark:text-amber-400" : "text-muted-foreground"
      }`}
      title={warn ? "Not free-tier safe (use length 26 + hex)" : undefined}
    >
      {warn && <AlertTriangleIcon className="h-2.5 w-2.5" />}
      {header.name || "(unnamed)"}: {header.mode}
    </span>
  );
}

// --- Editable provider card ---

function ProviderCard({
  provider,
  onDelete,
  onSave,
  onEnsure,
  saving,
  ensuring,
}: {
  provider: ProviderRule;
  onDelete: (name: string) => void;
  onSave: (name: string, rule: ProviderRule) => void;
  onEnsure: (name: string) => void;
  saving: boolean;
  ensuring: boolean;
}) {
  const [expanded, setExpanded] = useState(false);
  const [enabled, setEnabled] = useState(provider.enabled);
  const [headers, setHeaders] = useState<HeaderRule[]>(provider.headers);
  const warn = ruleHasWarnings(provider);
  const dirty = enabled !== provider.enabled || JSON.stringify(headers) !== JSON.stringify(provider.headers);

  const reset = () => {
    setEnabled(provider.enabled);
    setHeaders(provider.headers);
  };

  return (
    <div className="overflow-hidden rounded-lg border border-border/40 bg-surface transition-colors">
      {/* Header row */}
      <div className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="flex min-w-0 flex-1 items-center gap-3 text-left"
          aria-expanded={expanded}
        >
          <div className={`h-2.5 w-2.5 shrink-0 rounded-full ${provider.enabled ? "bg-success" : "bg-muted-foreground/40"}`} />
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span className="truncate font-mono text-[13px] font-medium text-foreground">{provider.name}</span>
              {warn && (
                <span className="inline-flex shrink-0 items-center gap-1 rounded-full border border-amber-500/30 bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-medium text-amber-600 dark:text-amber-400">
                  <AlertTriangleIcon className="h-2.5 w-2.5" />
                  non-default
                </span>
              )}
            </div>
            <div className="mt-0.5 text-[12px] text-muted-foreground">
              {provider.headers.length} header rule{provider.headers.length !== 1 ? "s" : ""}
            </div>
          </div>
          <span className="ml-auto shrink-0 text-muted-foreground">
            {expanded ? <ChevronUpIcon className="h-4 w-4" /> : <ChevronDownIcon className="h-4 w-4" />}
          </span>
        </button>

        <div className="flex flex-wrap items-center gap-2">
          {provider.headers.map((h) => (
            <ModeChip key={h.name} header={h} />
          ))}
          <button
            type="button"
            onClick={() => onDelete(provider.name)}
            className="rounded-lg border border-border/40 p-2 text-muted-foreground transition-colors hover:border-destructive/40 hover:bg-destructive/10 hover:text-destructive"
            aria-label={`Delete rule for ${provider.name}`}
          >
            <Trash2Icon className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Expanded editor */}
      {expanded && (
        <div className="border-t border-border/40 bg-background/40 p-4 flex flex-col gap-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <ToggleField
              label="Rule enabled"
              checked={enabled}
              onCheckedChange={setEnabled}
              className="max-w-xs"
            />
            <Button
              variant="outline"
              size="sm"
              onClick={() => onEnsure(provider.name)}
              disabled={ensuring}
              className="self-start sm:self-auto"
            >
              <SparklesIcon className="mr-1.5 h-3.5 w-3.5" />
              {ensuring ? "Applying..." : "Add missing OpenCode headers"}
            </Button>
          </div>

          <div>
            <div className="mb-2 text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
              Header rules
            </div>
            <HeaderRulesEditor headers={headers} onChange={setHeaders} compact />
          </div>

          <div className="flex flex-col gap-2 sm:flex-row">
            <Button
              size="sm"
              onClick={() => onSave(provider.name, { name: provider.name, enabled, headers })}
              disabled={!dirty || saving}
            >
              {saving ? "Saving..." : "Save changes"}
            </Button>
            <Button size="sm" variant="outline" onClick={reset} disabled={!dirty || saving}>
              Reset
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

function MappingRow({ mapping }: { mapping: MappingEntry }) {
  return (
    <div className="flex items-center gap-3 rounded-lg border border-border/40 bg-surface p-3 transition-colors hover:bg-surface-hover/30">
      <div className="h-2.5 w-2.5 shrink-0 rounded-full bg-success" />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="shrink-0 text-[12px] text-muted-foreground">in:</span>
          <span className="truncate font-mono text-[12px] text-foreground">{mapping.inbound_value}</span>
        </div>
        <div className="mt-1 flex min-w-0 items-center gap-2">
          <span className="shrink-0 text-[12px] text-muted-foreground">out:</span>
          <span className="truncate font-mono text-[12px] font-medium text-accent">{mapping.outbound_value}</span>
        </div>
      </div>
      <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
        {mapping.provider}
      </span>
    </div>
  );
}

export function SessionHubTab(): JSX.Element {
  const { data: status } = useSessionHubStatus();
  const { data: providers } = useSessionHubProviders();
  const { data: mappingsData } = useSessionHubMappings();
  const mutations = useSessionHubMutations();

  const [showAdd, setShowAdd] = useState(false);
  const [addTargetKind, setAddTargetKind] = useState<TargetKind>("pool");
  const [newSelectedPool, setNewSelectedPool] = useState("");
  const [newSelectedProvider, setNewSelectedProvider] = useState("");
  const [newName, setNewName] = useState("");
  const [newEnabled, setNewEnabled] = useState(true);
  const [newHeaders, setNewHeaders] = useState<HeaderRule[]>([openCodeSessionRule()]);

  const poolQuery = usePools();
  const provStatusQuery = useServerProviders();
  const pools = poolQuery.data?.pools ?? [];
  const serverProviders =
    (provStatusQuery.data as { providers?: Array<{ name: string }> } | undefined)?.providers ?? [];
  const boundTargets = providers ?? [];

  const resetAddForm = () => {
    setNewName("");
    setNewEnabled(true);
    setNewHeaders([openCodeSessionRule()]);
  };

  const handleAdd = () => {
    if (!newName) return;
    mutations.createProvider.mutate(
      { name: newName, rule: { name: newName, enabled: newEnabled, headers: newHeaders } },
      {
        onSuccess: () => {
          resetAddForm();
          setShowAdd(false);
        },
      }
    );
  };

  const openAddFor = (kind: TargetKind, name: string) => {
    setShowAdd(true);
    setAddTargetKind(kind);
    setNewName(name);
    if (kind === "pool") setNewSelectedPool(name);
    if (kind === "provider") setNewSelectedProvider(name);
  };

  return (
    <div className="flex flex-col gap-6">
      {/* Risk notice */}
      <div className="flex items-start gap-3 rounded-lg border border-amber-500/30 bg-amber-500/5 p-4">
        <ShieldAlertIcon className="mt-0.5 h-5 w-5 shrink-0 text-amber-500" />
        <div className="text-sm">
          <p className="font-medium text-amber-600 dark:text-amber-400">Use at your own risk</p>
          <p className="mt-1 leading-snug text-muted-foreground">
            The Session Hub rewrites upstream headers to impersonate official clients. This may
            violate a provider&apos;s terms of service, and upstream fingerprint hardening can
            break it at any time. For OpenCode Zen free tier, the session must be exactly 26 hex
            characters (charset <span className="font-mono">hex</span>, length <span className="font-mono">26</span>);
            the dashboard flags rules that are not free-tier safe.
          </p>
        </div>
      </div>

      {/* Status */}
      <Surface id="sessionhub-status" className="p-4 sm:p-6 scroll-mt-20">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="border border-border/40 bg-background/80 p-2">
                <KeyIcon className="h-4 w-4 text-accent" />
              </div>
              <SectionHeader
                title="Session Hub"
                subtitle="Header transformation engine. Maps inbound session IDs to unique outbound values per provider, preventing upstream detection of shared clients."
              />
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {status && (
                <StatusBadge
                  healthy={status.enabled_providers > 0}
                  label={`${status.enabled_providers}/${status.configured_providers} active`}
                />
              )}
              {status && (
                <span className="inline-flex items-center gap-1.5 rounded-full border border-border/40 bg-surface px-2.5 py-1 text-[11px] font-medium text-muted-foreground">
                  {status.total_mappings} mappings
                </span>
              )}
            </div>
          </div>

          {status && (
            <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
              <div className="border border-border/40 bg-surface p-4">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Total mappings</div>
                <div className="mt-2 font-mono text-[20px] font-medium text-foreground">{status.total_mappings}</div>
              </div>
              <div className="border border-border/40 bg-surface p-4">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Providers</div>
                <div className="mt-2 font-mono text-[20px] font-medium text-foreground">{status.configured_providers}</div>
              </div>
              <div className="border border-border/40 bg-surface p-4">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Active</div>
                <div className="mt-2 font-mono text-[20px] font-medium text-success">{status.enabled_providers}</div>
              </div>
              <div className="col-span-2 border border-border/40 bg-surface p-4 xl:col-span-1">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">By provider</div>
                <div className="mt-2 flex flex-wrap gap-1">
                  {Object.entries(status.by_provider).length === 0 ? (
                    <span className="text-[12px] text-muted-foreground/60">none</span>
                  ) : (
                    Object.entries(status.by_provider).map(([prov, count]) => (
                      <span key={prov} className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                        {prov}: {count}
                      </span>
                    ))
                  )}
                </div>
              </div>
            </div>
          )}
        </div>
      </Surface>

      {/* Provider Rules */}
      <Surface id="sessionhub-providers" className="p-4 sm:p-6 scroll-mt-20">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="border border-border/40 bg-background/80 p-2">
                <ZapIcon className="h-4 w-4 text-accent" />
              </div>
              <SectionHeader
                title="Provider Rules"
                subtitle="Give each pool or provider its own client identity. Click a rule to edit it in place."
              />
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setShowAdd((v) => !v);
                if (showAdd) resetAddForm();
              }}
              className="self-start"
            >
              <PlusIcon className="mr-1.5 h-3.5 w-3.5" />
              {showAdd ? "Close" : "Add rule"}
            </Button>
          </div>

          {providers && providers.length > 0 ? (
            <div className="grid grid-cols-1 gap-3">
              {providers.map((p) => (
                <ProviderCard
                  key={p.name}
                  provider={p}
                  onDelete={(name) => mutations.deleteProvider.mutate(name)}
                  onSave={(name, rule) => mutations.updateProvider.mutate({ name, rule })}
                  onEnsure={(name) => mutations.ensureOpenCode.mutate(name)}
                  saving={mutations.updateProvider.isPending}
                  ensuring={mutations.ensureOpenCode.isPending}
                />
              ))}
            </div>
          ) : (
            <div className="border border-dashed border-border/60 bg-surface/50 p-8 text-center">
              <KeyIcon className="mx-auto h-8 w-8 text-muted-foreground/40" />
              <p className="mt-2 text-[13px] text-muted-foreground">
                No provider rules configured. Add one below to start transforming headers.
              </p>
            </div>
          )}

          {/* Binding overview */}
          {(pools.length > 0 || serverProviders.length > 0) && (
            <div>
              <div className="mb-2 text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
                Binding overview
              </div>
              <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
                {pools.map((p) => {
                  const bound = boundTargets.some((b) => b.name === p.name);
                  return (
                    <button
                      key={`pool-${p.name}`}
                      type="button"
                      onClick={() => {
                        if (!bound) openAddFor("pool", p.name);
                      }}
                      className={`flex min-h-[44px] items-center gap-2 rounded-lg border px-3 py-2 text-left transition-colors ${
                        bound
                          ? "border-success/30 bg-success/10"
                          : "border-border/40 bg-surface hover:border-accent/40 hover:bg-surface-hover/30"
                      }`}
                      title={bound ? `"${p.name}" already has a rule` : `Bind a rule to pool "${p.name}"`}
                    >
                      <BoxesIcon className="h-3.5 w-3.5 shrink-0 text-accent" />
                      <span className="truncate text-[13px] font-medium text-foreground">{p.name}</span>
                      <span className="ml-auto shrink-0">
                        {bound ? (
                          <CheckCircleIcon className="h-3.5 w-3.5 text-success" />
                        ) : (
                          <PlusIcon className="h-3.5 w-3.5 text-muted-foreground" />
                        )}
                      </span>
                    </button>
                  );
                })}
                {serverProviders.map((sp) => {
                  const bound = boundTargets.some((b) => b.name === sp.name);
                  return (
                    <button
                      key={`prov-${sp.name}`}
                      type="button"
                      onClick={() => {
                        if (!bound) openAddFor("provider", sp.name);
                      }}
                      className={`flex min-h-[44px] items-center gap-2 rounded-lg border px-3 py-2 text-left transition-colors ${
                        bound
                          ? "border-success/30 bg-success/10"
                          : "border-border/40 bg-surface hover:border-accent/40 hover:bg-surface-hover/30"
                      }`}
                      title={bound ? `"${sp.name}" already has a rule` : `Bind a rule to provider "${sp.name}"`}
                    >
                      <GlobeIcon className="h-3.5 w-3.5 shrink-0 text-accent" />
                      <span className="truncate text-[13px] font-medium text-foreground">{sp.name}</span>
                      <span className="ml-auto shrink-0">
                        {bound ? (
                          <CheckCircleIcon className="h-3.5 w-3.5 text-success" />
                        ) : (
                          <PlusIcon className="h-3.5 w-3.5 text-muted-foreground" />
                        )}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          {/* Add form */}
          {showAdd && (
            <div className="flex flex-col gap-4 rounded-lg border border-accent/30 bg-accent/5 p-4">
              <div className="flex items-center gap-2">
                <PencilIcon className="h-4 w-4 text-accent" />
                <span className="text-[13px] font-semibold text-foreground">New rule</span>
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <label className="flex flex-col gap-1">
                  <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Apply to</span>
                  <select
                    value={addTargetKind}
                    onChange={(e) => {
                      setAddTargetKind(e.target.value as TargetKind);
                      setNewName("");
                    }}
                    className="h-11 md:h-9 rounded-lg border border-border/60 bg-surface px-3 text-base md:text-[13px] text-foreground"
                  >
                    <option value="pool">Pool</option>
                    <option value="provider">Provider</option>
                    <option value="fallback">Fallback</option>
                    <option value="all">All (wildcard)</option>
                  </select>
                </label>

                {addTargetKind === "pool" && (
                  <label className="flex flex-col gap-1">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Target pool</span>
                    <select
                      value={newSelectedPool}
                      onChange={(e) => {
                        setNewSelectedPool(e.target.value);
                        setNewName(e.target.value);
                      }}
                      className="h-11 md:h-9 rounded-lg border border-border/60 bg-surface px-3 text-base md:text-[13px] text-foreground"
                    >
                      <option value="">Select a pool...</option>
                      {pools.map((p) => (
                        <option key={p.name} value={p.name}>{p.name}</option>
                      ))}
                    </select>
                  </label>
                )}

                {addTargetKind === "provider" && (
                  <label className="flex flex-col gap-1">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Target provider</span>
                    <select
                      value={newSelectedProvider}
                      onChange={(e) => {
                        setNewSelectedProvider(e.target.value);
                        setNewName(e.target.value);
                      }}
                      className="h-11 md:h-9 rounded-lg border border-border/60 bg-surface px-3 text-base md:text-[13px] text-foreground"
                    >
                      <option value="">Select a provider...</option>
                      {serverProviders.map((p) => (
                        <option key={p.name} value={p.name}>{p.name}</option>
                      ))}
                    </select>
                  </label>
                )}

                {(addTargetKind === "fallback" || addTargetKind === "all") && (
                  <label className="flex flex-col gap-1">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Target key</span>
                    <Input
                      placeholder={addTargetKind === "all" ? "* (all providers)" : "fallback target name"}
                      value={newName}
                      onChange={(e) => setNewName(e.target.value)}
                    />
                  </label>
                )}
              </div>

              <ToggleField
                label="Rule enabled"
                checked={newEnabled}
                onCheckedChange={setNewEnabled}
                className="max-w-xs"
              />

              <div className="text-[12px] text-muted-foreground">
                {newName ? (
                  <>Rule will be bound to <span className="font-mono text-foreground">{newName}</span></>
                ) : (
                  "Select a pool or provider above to bind this rule."
                )}
              </div>

              <div>
                <div className="mb-2 text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
                  Header rules
                </div>
                <HeaderRulesEditor headers={newHeaders} onChange={setNewHeaders} />
              </div>

              <div className="flex flex-col gap-2 sm:flex-row">
                <Button onClick={handleAdd} disabled={!newName || mutations.createProvider.isPending}>
                  {mutations.createProvider.isPending ? "Creating..." : "Create rule"}
                </Button>
                <Button
                  variant="outline"
                  onClick={() => {
                    setShowAdd(false);
                    resetAddForm();
                  }}
                >
                  Cancel
                </Button>
              </div>
            </div>
          )}
        </div>
      </Surface>

      {/* Live Mappings */}
      <Surface id="sessionhub-mappings" className="p-4 sm:p-6 scroll-mt-20">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="border border-border/40 bg-background/80 p-2">
                <RefreshCwIcon className="h-4 w-4 text-accent" />
              </div>
              <SectionHeader
                title="Live Mappings"
                subtitle="Active inbound-to-outbound session mappings from each provider."
              />
            </div>
            <div className="flex w-full flex-col gap-2 lg:w-auto lg:items-end">
              <ToggleField
                label="Persist to disk"
                description={
                  status?.storage_mode === "disk"
                    ? "Mappings are saved to disk and restored after restart."
                    : "Mappings are kept in memory and reset on restart."
                }
                checked={status?.storage_mode === "disk"}
                onCheckedChange={(checked) => mutations.setStorageMode.mutate(checked ? "disk" : "memory")}
                disabled={mutations.setStorageMode.isPending}
                size="sm"
                aria-label="Toggle session mapping persistence"
                className="w-full max-w-sm lg:max-w-xs"
              />
              <Button
                variant="outline"
                size="sm"
                onClick={() => mutations.clearMappings.mutate()}
                disabled={mutations.clearMappings.isPending}
                className="self-start lg:self-end"
              >
                <Trash2Icon className="mr-1.5 h-3.5 w-3.5" />
                Clear all
              </Button>
            </div>
          </div>

          {mappingsData && mappingsData.mappings.length > 0 ? (
            <div className="grid max-h-[400px] grid-cols-1 gap-2 overflow-y-auto sm:grid-cols-2">
              {mappingsData.mappings.slice(0, 50).map((m, idx) => (
                <MappingRow key={idx} mapping={m} />
              ))}
              {mappingsData.mappings.length > 50 && (
                <div className="py-2 text-center text-[12px] text-muted-foreground sm:col-span-2">
                  Showing 50 of {mappingsData.mappings.length} mappings
                </div>
              )}
            </div>
          ) : (
            <div className="border border-dashed border-border/60 bg-surface/50 p-8 text-center">
              <RefreshCwIcon className="mx-auto h-8 w-8 text-muted-foreground/40" />
              <p className="mt-2 text-[13px] text-muted-foreground">
                No active mappings yet. Mappings appear when requests flow through the gateway.
              </p>
            </div>
          )}
        </div>
      </Surface>
    </div>
  );
}

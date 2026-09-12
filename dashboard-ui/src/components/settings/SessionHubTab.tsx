import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Surface, SectionHeader } from "@/components/ui/surface";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ToggleField } from "@/components/ui/toggle-field";
import { KeyIcon, GlobeIcon, BoxesIcon, RefreshCwIcon, PlusIcon, Trash2Icon, CheckCircleIcon, XCircleIcon, ZapIcon } from "lucide-react";
import { apiFetch } from "@/lib/api/client";
import { usePools } from "@/lib/api/usePools";
import { fetchProviderStatus } from "@/lib/api/providers";

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

interface HeaderRule {
  name: string;
  mode: string;
  prefix: string;
  length: number;
  value: string;
  values: string[];
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
      apiFetch("/admin/api/v1/sessionhub/providers", {
        method: "POST",
        json: data,
      }),
    onSuccess: invalidate,
  });

  const deleteProvider = useMutation({
    mutationFn: (name: string) =>
      apiFetch(`/admin/api/v1/sessionhub/providers/${encodeURIComponent(name)}`, {
        method: "DELETE",
      }),
    onSuccess: invalidate,
  });

  const updateProvider = useMutation({
    mutationFn: ({ name, rule }: { name: string; rule: ProviderRule }) =>
      apiFetch(`/admin/api/v1/sessionhub/providers/${encodeURIComponent(name)}`, {
        method: "PUT",
        json: rule,
      }),
    onSuccess: invalidate,
  });

  const clearMappings = useMutation({
    mutationFn: () => apiFetch("/admin/api/v1/sessionhub/mappings", { method: "DELETE" }),
    onSuccess: invalidate,
  });

  const clearProviderMappings = useMutation({
    mutationFn: (provider: string) =>
      apiFetch(`/admin/api/v1/sessionhub/mappings/${encodeURIComponent(provider)}`, {
        method: "DELETE",
      }),
    onSuccess: invalidate,
  });

  const setStorageMode = useMutation({
    mutationFn: (mode: "memory" | "disk") =>
      apiFetch("/admin/api/v1/sessionhub/storage", {
        method: "PUT",
        json: { mode },
      }),
    onSuccess: invalidate,
  });

  return {
    createProvider,
    deleteProvider,
    updateProvider,
    clearMappings,
    clearProviderMappings,
    setStorageMode,
  };
}

// --- Components ---

function StatusBadge({ healthy, label }: { healthy: boolean; label: string }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] font-medium ${
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

function ProviderCard({
  provider,
  onDelete,
}: {
  provider: ProviderRule;
  onDelete: (name: string) => void;
}) {
  return (
    <div className="border border-border/40 bg-surface p-4 transition-colors hover:bg-surface-hover/30">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className={`h-2.5 w-2.5 rounded-full ${provider.enabled ? "bg-success" : "bg-muted-foreground/40"}`} />
          <div>
            <span className="font-mono text-[13px] font-medium text-foreground">{provider.name}</span>
            <div className="mt-1 text-[12px] text-muted-foreground">
              {provider.headers.length} header rule{provider.headers.length !== 1 ? "s" : ""}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {provider.headers.map((h) => (
            <span
              key={h.name}
              className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground"
            >
              {h.name}: {h.mode}
            </span>
          ))}
          <button
            onClick={() => onDelete(provider.name)}
            className="rounded p-1.5 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
          >
            <Trash2Icon className="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>
  );
}

function MappingRow({ mapping }: { mapping: MappingEntry }) {
  return (
    <div className="flex items-center gap-4 border border-border/40 bg-surface p-3 transition-colors hover:bg-surface-hover/30">
      <div className="h-2.5 w-2.5 rounded-full bg-success" />
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-[12px] text-muted-foreground">inbound:</span>
          <span className="font-mono text-[13px] text-foreground truncate">{mapping.inbound_value}</span>
        </div>
        <div className="flex items-center gap-2 mt-1">
          <span className="text-[12px] text-muted-foreground">outbound ({mapping.provider}):</span>
          <span className="font-mono text-[13px] font-medium text-accent truncate">{mapping.outbound_value}</span>
        </div>
      </div>
      <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
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
  const [newTargetKind, setNewTargetKind] = useState<TargetKind>("pool");
  const [newSelectedPool, setNewSelectedPool] = useState("");
  const [newSelectedProvider, setNewSelectedProvider] = useState("");
  const [newName, setNewName] = useState("");
  const [newEnabled, setNewEnabled] = useState(true);
  const [newHeaders, setNewHeaders] = useState<HeaderRule[]>([
    { name: "x-opencode-session", mode: "map_or_generate", prefix: "ses_", length: 28, value: "", values: [] },
  ]);

  const poolQuery = usePools();
  const provStatusQuery = useServerProviders();
  const pools = poolQuery.data?.pools ?? [];
  const serverProviders =
    (provStatusQuery.data as { providers?: Array<{ name: string }> } | undefined)?.providers ?? [];
  const boundTargets = providers ?? [];

  // Auto-set the rule "Name" from the selected target so binding is unambiguous.
  const applyTargetName = (name: string) => {
    if (name) setNewName(name);
  };

  const handleAdd = () => {
    if (!newName) return;
    mutations.createProvider.mutate(
      {
        name: newName,
        rule: { name: newName, enabled: newEnabled, headers: newHeaders },
      },
      {
        onSuccess: () => {
          setNewName("");
          setNewEnabled(true);
          setNewHeaders([{ name: "x-opencode-session", mode: "map_or_generate", prefix: "ses_", length: 28, value: "", values: [] }]);
          setShowAdd(false);
        },
      }
    );
  };

  const addHeaderRule = () => {
    setNewHeaders([...newHeaders, { name: "", mode: "passthrough", prefix: "", length: 0, value: "", values: [] }]);
  };

  const updateHeaderRule = (index: number, field: keyof HeaderRule, value: string | number | string[]) => {
    const updated = [...newHeaders];
    const existing = updated[index];
    if (!existing) return;
    updated[index] = {
      name: existing.name,
      mode: existing.mode,
      prefix: existing.prefix,
      length: existing.length,
      value: existing.value,
      values: existing.values,
      [field]: value,
    };
    setNewHeaders(updated);
  };

  const removeHeaderRule = (index: number) => {
    setNewHeaders(newHeaders.filter((_, i) => i !== index));
  };

  return (
    <div className="flex flex-col gap-6">
      {/* Status */}
      <Surface id="sessionhub-status" className="p-6 scroll-mt-20">
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
                <span className="inline-flex items-center gap-1.5 rounded-full border border-border/40 bg-surface px-2.5 py-0.5 text-[11px] font-medium text-muted-foreground">
                  {status.total_mappings} mappings
                </span>
              )}
            </div>
          </div>

          {status && (
            <div className="grid grid-cols-1 gap-3 xl:grid-cols-4">
              <div className="border border-border/40 bg-surface p-4 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Total Mappings</div>
                <div className="mt-2 font-mono text-[20px] font-medium text-foreground">{status.total_mappings}</div>
              </div>
              <div className="border border-border/40 bg-surface p-4 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Providers</div>
                <div className="mt-2 font-mono text-[20px] font-medium text-foreground">{status.configured_providers}</div>
              </div>
              <div className="border border-border/40 bg-surface p-4 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Active</div>
                <div className="mt-2 font-mono text-[20px] font-medium text-success">{status.enabled_providers}</div>
              </div>
              <div className="border border-border/40 bg-surface p-4 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">By Provider</div>
                <div className="mt-2 flex flex-wrap gap-1">
                  {Object.entries(status.by_provider).map(([prov, count]) => (
                    <span key={prov} className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                      {prov}: {count}
                    </span>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>
      </Surface>

      {/* Provider Rules */}
      <Surface id="sessionhub-providers" className="p-6 scroll-mt-20">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="border border-border/40 bg-background/80 p-2">
                <ZapIcon className="h-4 w-4 text-accent" />
              </div>
              <SectionHeader
                title="Provider Rules"
                subtitle="Define per-provider header transformation rules. Each provider can generate unique session IDs, rewrite headers, or inject custom values."
              />
            </div>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={() => setShowAdd(!showAdd)}>
                <PlusIcon className="mr-1.5 h-3.5 w-3.5" />
                Add Rule
              </Button>
            </div>
          </div>

          {providers && providers.length > 0 ? (
            <div className="grid grid-cols-1 gap-3">
              {providers.map((p) => (
                <ProviderCard
                  key={p.name}
                  provider={p}
                  onDelete={(name) => mutations.deleteProvider.mutate(name)}
                />
              ))}
            </div>
          ) : (
            <div className="border border-dashed border-border/60 bg-surface/50 p-8 text-center">
              <KeyIcon className="mx-auto h-8 w-8 text-muted-foreground/40" />
              <p className="mt-2 text-[13px] text-muted-foreground">
                No provider rules configured. Add a provider to start transforming headers.
              </p>
            </div>
          )}

          {/* Server target overview: shows which pools/providers have a rule bound */}
          {(pools.length > 0 || serverProviders.length > 0) && (
            <div className="mt-1">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground mb-2">
                Binding overview
              </div>
              <div className="grid grid-cols-1 gap-2 md:grid-cols-2 xl:grid-cols-3">
                {pools.map((p) => {
                  const bound = boundTargets.some((b) => b.name === p.name);
                  return (
                    <button
                      key={p.name}
                      onClick={() => {
                        if (bound) return;
                        setShowAdd(true);
                        setNewTargetKind("pool");
                        setNewSelectedPool(p.name);
                        setNewName(p.name);
                      }}
                      className={`flex items-center gap-2 border text-left px-3 py-2 rounded transition-colors ${
                        bound
                          ? "border-success/30 bg-success/10"
                          : "border-border/40 bg-surface hover:bg-surface-hover/30"
                      }`}
                      title={bound ? `"${p.name}" already has a rule` : `Bind a rule to pool "${p.name}"`}
                    >
                      <BoxesIcon className="h-3.5 w-3.5 shrink-0 text-accent" />
                      <span className="text-[13px] font-medium text-foreground truncate">{p.name}</span>
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
                      key={sp.name}
                      onClick={() => {
                        if (bound) return;
                        setShowAdd(true);
                        setNewTargetKind("provider");
                        setNewSelectedProvider(sp.name);
                        setNewName(sp.name);
                      }}
                      className={`flex items-center gap-2 border text-left px-3 py-2 rounded transition-colors ${
                        bound
                          ? "border-success/30 bg-success/10"
                          : "border-border/40 bg-surface hover:bg-surface-hover/30"
                      }`}
                      title={bound ? `"${sp.name}" already has a rule` : `Bind a rule to provider "${sp.name}"`}
                    >
                      <GlobeIcon className="h-3.5 w-3.5 shrink-0 text-accent" />
                      <span className="text-[13px] font-medium text-foreground truncate">{sp.name}</span>
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
            <div className="border border-border/40 bg-surface p-4">
              <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
                <div>
                  <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground mb-1">Apply to (target)</div>
                  <select
                    value={newTargetKind}
                    onChange={(e) => {
                      const k = e.target.value as TargetKind;
                      setNewTargetKind(k);
                      setNewName("");
                    }}
                    className="w-full border border-border/60 bg-surface px-3 py-2 text-[13px] text-foreground rounded"
                  >
                    <option value="pool">Pool</option>
                    <option value="provider">Provider</option>
                    <option value="fallback">Fallback</option>
                    <option value="all">All (wildcard)</option>
                  </select>
                </div>

                {newTargetKind === "pool" && (
                  <div>
                    <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground mb-1">Pool to attach rule to</div>
                    <select
                      value={newSelectedPool}
                      onChange={(e) => {
                        setNewSelectedPool(e.target.value);
                        applyTargetName(e.target.value);
                      }}
                      className="w-full border border-border/60 bg-surface px-3 py-2 text-[13px] text-foreground rounded"
                    >
                      <option value="">Select a pool…</option>
                      {pools.map((p) => (
                        <option key={p.name} value={p.name}>{p.name}</option>
                      ))}
                    </select>
                  </div>
                )}

                {newTargetKind === "provider" && (
                  <div>
                    <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground mb-1">Provider to attach rule to</div>
                    <select
                      value={newSelectedProvider}
                      onChange={(e) => {
                        setNewSelectedProvider(e.target.value);
                        applyTargetName(e.target.value);
                      }}
                      className="w-full border border-border/60 bg-surface px-3 py-2 text-[13px] text-foreground rounded"
                    >
                      <option value="">Select a provider…</option>
                      {serverProviders.map((p) => (
                        <option key={p.name} value={p.name}>{p.name}</option>
                      ))}
                    </select>
                  </div>
                )}

                {(newTargetKind === "fallback" || newTargetKind === "all") && (
                  <div>
                    <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground mb-1">Target key</div>
                    <Input
                      placeholder={newTargetKind === "all" ? "* (all providers)" : "fallback target name"}
                      value={newName}
                      onChange={(e) => setNewName(e.target.value)}
                    />
                  </div>
                )}

                <div className="flex items-end">
                  <ToggleField
                    label="Enabled"
                    checked={newEnabled}
                    onCheckedChange={setNewEnabled}
                  />
                </div>
              </div>

              <div className="mt-2 text-[12px] text-muted-foreground/70">
                {newName
                  ? <>Rule will be bound to <span className="font-mono text-foreground">{newName}</span></>
                  : "Select a pool/provider above to bind this rule."
                }
              </div>

              <div className="mt-4">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground mb-2">Header Rules</div>
                <div className="flex flex-col gap-3">
                  {newHeaders.map((hr, idx) => (
                    <div key={idx} className="border border-border/40 bg-background/50 p-3 rounded">
                      <div className="flex items-center gap-2 mb-2">
                        <Input
                          placeholder="Header name (e.g. x-opencode-session)"
                          value={hr.name}
                          onChange={(e) => updateHeaderRule(idx, "name", e.target.value)}
                          className="flex-1"
                        />
                        <select
                          value={hr.mode}
                          onChange={(e) => updateHeaderRule(idx, "mode", e.target.value)}
                          className="border border-border/60 bg-surface px-3 py-2 text-[13px] text-foreground rounded"
                        >
                          <option value="map_or_generate">Map or Generate (Recommended)</option>
                          <option value="map">Map (unique per provider)</option>
                          <option value="generate">Generate (fresh each time)</option>
                          <option value="passthrough">Passthrough</option>
                          <option value="static">Static value</option>
                          <option value="random_from_list">Random from list</option>
                          <option value="remove">Remove</option>
                        </select>
                        <button
                          onClick={() => removeHeaderRule(idx)}
                          className="rounded p-1.5 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                        >
                          <Trash2Icon className="h-4 w-4" />
                        </button>
                      </div>

                      {/* Mode-specific options */}
                      {(hr.mode === "map" || hr.mode === "generate" || hr.mode === "map_or_generate") && (
                        <div className="flex items-center gap-2 text-[12px]">
                          <div className="flex items-center gap-1">
                            <span className="text-muted-foreground">Prefix:</span>
                            <Input
                              value={hr.prefix || "ses_"}
                              onChange={(e) => updateHeaderRule(idx, "prefix", e.target.value)}
                              className="w-20 h-7 text-[12px]"
                            />
                          </div>
                          <div className="flex items-center gap-1">
                            <span className="text-muted-foreground">Length:</span>
                            <Input
                              type="number"
                              min={4}
                              max={64}
                              value={hr.length || 28}
                              onChange={(e) => updateHeaderRule(idx, "length", parseInt(e.target.value) || 28)}
                              className="w-16 h-7 text-[12px]"
                            />
                          </div>
                          <span className="text-muted-foreground/60">
                            → {hr.prefix || "ses_"}{"{"}{hr.length || 28}{"}"}alphanumeric
                          </span>
                        </div>
                      )}

                      {hr.mode === "static" && (
                        <div className="flex items-center gap-1 text-[12px]">
                          <span className="text-muted-foreground">Value:</span>
                          <Input
                            placeholder="Fixed value to send"
                            value={hr.value || ""}
                            onChange={(e) => updateHeaderRule(idx, "value", e.target.value)}
                            className="flex-1 h-7 text-[12px]"
                          />
                        </div>
                      )}

                      {hr.mode === "random_from_list" && (
                        <div className="text-[12px]">
                          <span className="text-muted-foreground">Values (comma-separated):</span>
                          <Input
                            placeholder="Mozilla/5.0..., OpenCode/1.0"
                            value={(hr.values || []).join(", ")}
                            onChange={(e) => updateHeaderRule(idx, "values", e.target.value.split(",").map(s => s.trim()).filter(Boolean))}
                            className="mt-1 h-7 text-[12px]"
                          />
                        </div>
                      )}

                      {hr.mode === "passthrough" && (
                        <div className="text-[12px] text-muted-foreground/60">
                          Original value from client is forwarded unchanged
                        </div>
                      )}

                      {hr.mode === "remove" && (
                        <div className="text-[12px] text-muted-foreground/60">
                          Header will be stripped before sending upstream
                        </div>
                      )}
                    </div>
                  ))}
                </div>
                <Button variant="outline" size="sm" className="mt-2" onClick={addHeaderRule}>
                  <PlusIcon className="mr-1.5 h-3 w-3" />
                  Add Rule
                </Button>
              </div>

              <div className="flex gap-2 mt-4">
                <Button size="sm" onClick={handleAdd} disabled={!newName || mutations.createProvider.isPending}>
                  {mutations.createProvider.isPending ? "Creating..." : "Create Rule"}
                </Button>
                <Button size="sm" variant="outline" onClick={() => setShowAdd(false)}>
                  Cancel
                </Button>
              </div>
            </div>
          )}
        </div>
      </Surface>

      {/* Live Mappings */}
      <Surface id="sessionhub-mappings" className="p-6 scroll-mt-20">
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
            <div className="flex w-full flex-col items-stretch gap-2 md:w-auto md:items-end">
              <ToggleField
                label="Persist to disk"
                description={
                  status?.storage_mode === "disk"
                    ? "Mappings are saved to disk and restored after restart."
                    : "Mappings are kept in memory and reset on restart."
                }
                checked={status?.storage_mode === "disk"}
                onCheckedChange={(checked) =>
                  mutations.setStorageMode.mutate(checked ? "disk" : "memory")
                }
                disabled={mutations.setStorageMode.isPending}
                size="sm"
                aria-label="Toggle session mapping persistence"
                className="w-full max-w-sm md:max-w-xs"
              />
              <Button
                variant="outline"
                size="sm"
                onClick={() => mutations.clearMappings.mutate()}
                disabled={mutations.clearMappings.isPending}
                className="self-start md:self-end"
              >
                <Trash2Icon className="mr-1.5 h-3.5 w-3.5" />
                Clear All
              </Button>
            </div>
          </div>

          {mappingsData && mappingsData.mappings.length > 0 ? (
            <div className="grid grid-cols-1 gap-2 max-h-[400px] overflow-y-auto">
              {mappingsData.mappings.slice(0, 50).map((m, idx) => (
                <MappingRow key={idx} mapping={m} />
              ))}
              {mappingsData.mappings.length > 50 && (
                <div className="text-center text-[12px] text-muted-foreground py-2">
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

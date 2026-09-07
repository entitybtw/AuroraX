import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Surface, SectionHeader } from "@/components/ui/surface";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ToggleField } from "@/components/ui/toggle-field";
import { KeyIcon, RefreshCwIcon, PlusIcon, Trash2Icon, CheckCircleIcon, XCircleIcon, ZapIcon } from "lucide-react";
import { apiFetch } from "@/lib/api/client";

// --- Types ---

interface SessionHubStatus {
  total_mappings: number;
  by_provider: Record<string, number>;
  configured_providers: number;
  enabled_providers: number;
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
        body: JSON.stringify(data),
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
        body: JSON.stringify(rule),
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

  return { createProvider, deleteProvider, updateProvider, clearMappings, clearProviderMappings };
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
  const [newName, setNewName] = useState("");
  const [newEnabled, setNewEnabled] = useState(true);
  const [newHeaders, setNewHeaders] = useState<HeaderRule[]>([
    { name: "x-opencode-session", mode: "map", prefix: "ses_", length: 32, value: "", values: [] },
  ]);

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
          setNewHeaders([{ name: "x-opencode-session", mode: "map", prefix: "ses_", length: 32, value: "", values: [] }]);
          setShowAdd(false);
        },
      }
    );
  };

  const addHeaderRule = () => {
    setNewHeaders([...newHeaders, { name: "", mode: "passthrough", prefix: "", length: 0, value: "", values: [] }]);
  };

  const updateHeaderRule = (index: number, field: keyof HeaderRule, value: string | number) => {
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

          {/* Add form */}
          {showAdd && (
            <div className="border border-border/40 bg-surface p-4">
              <div className="grid grid-cols-1 gap-3 xl:grid-cols-3">
                <div>
                  <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground mb-1">Name</div>
                  <Input placeholder="opencode-zen" value={newName} onChange={(e) => setNewName(e.target.value)} />
                </div>
                <div className="flex items-end">
                  <ToggleField
                    label="Enabled"
                    checked={newEnabled}
                    onCheckedChange={setNewEnabled}
                  />
                </div>
              </div>

              <div className="mt-4">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground mb-2">Header Rules</div>
                <div className="flex flex-col gap-2">
                  {newHeaders.map((hr, idx) => (
                    <div key={idx} className="flex items-center gap-2">
                      <Input
                        placeholder="Header name"
                        value={hr.name}
                        onChange={(e) => updateHeaderRule(idx, "name", e.target.value)}
                        className="flex-1"
                      />
                      <select
                        value={hr.mode}
                        onChange={(e) => updateHeaderRule(idx, "mode", e.target.value)}
                        className="border border-border/60 bg-surface px-3 py-2 text-[13px] text-foreground rounded"
                      >
                        <option value="map">Map (unique per provider)</option>
                        <option value="generate">Generate (fresh each time)</option>
                        <option value="passthrough">Passthrough</option>
                        <option value="static">Static value</option>
                        <option value="random_from_list">Random from list</option>
                        <option value="remove">Remove</option>
                      </select>
                      {hr.mode === "static" && (
                        <Input
                          placeholder="Static value"
                          value={hr.value || ""}
                          onChange={(e) => updateHeaderRule(idx, "value", e.target.value)}
                          className="flex-1"
                        />
                      )}
                      {(hr.mode === "map" || hr.mode === "generate") && (
                        <Input
                          placeholder="Prefix"
                          value={hr.prefix || "ses_"}
                          onChange={(e) => updateHeaderRule(idx, "prefix", e.target.value)}
                          className="w-24"
                        />
                      )}
                      <button
                        onClick={() => removeHeaderRule(idx)}
                        className="rounded p-1.5 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                      >
                        <Trash2Icon className="h-4 w-4" />
                      </button>
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
                subtitle="Active inbound-to-outbound session mappings. These are stored in memory and reset on restart."
              />
            </div>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => mutations.clearMappings.mutate()}
                disabled={mutations.clearMappings.isPending}
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

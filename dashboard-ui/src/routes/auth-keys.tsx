import React, { useMemo, useState } from "react";
import { PlusIcon, CopyIcon, CheckIcon, ShieldAlertIcon, BarChart3Icon, ChevronLeftIcon, ChevronRightIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { Surface, EmptyState } from "@/components/ui/surface";
import { PageHeader } from "@/components/ui/page-header";
import { DocsCollapsible } from "@/components/ui/docs-collapsible";
import { DataTable, TableWrap, Td, Th } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { useAuthKeys, useAuthKeyStats } from "@/lib/api/useAuthKeys";
import type {
  AuthKey,
  AuthKeyBucketCount,
  AuthKeyRateLimitWindow,
  AuthKeyStats,
  CreateAuthKeyInput,
  AuthKeyIssued,
} from "@/lib/api/auth-keys-types";
import { useModels } from "@/lib/api/useModels";
import { usePools } from "@/lib/api/usePools";
import { useProviderStatus } from "@/lib/api/useProviders";
import { format } from "date-fns";

interface AuthKeyFormState extends Omit<CreateAuthKeyInput, "provider_pool_id"> {
  provider_pool_id: string;
}

function defaultAuthKeyForm(): AuthKeyFormState {
  return {
    name: "",
    description: "",
    user_path: "",
    tenant_id: "default",
    allowed_providers: [],
    allowed_models: [],
    denied_models: [],
    provider_pool_id: "",
    expires_at: "",
    requests_per_minute: undefined,
    requests_per_day: undefined,
    tokens_per_minute: undefined,
    tokens_per_day: undefined,
  };
}

function formatPolicyList(values: string[] | undefined, fallback: string) {
  const filtered = Array.isArray(values) ? values.filter(Boolean) : [];
  return filtered.length > 0 ? filtered.join(", ") : fallback;
}

function authKeyRoutingSummary(key: AuthKey) {
  const parts: string[] = [];
  if (key.provider_pool_id) parts.push(`pool=${key.provider_pool_id}`);
  parts.push(`providers=${formatPolicyList(key.allowed_providers, "all")}`);
  parts.push(`models=${formatPolicyList(key.allowed_models, "all")}`);
  if (key.denied_models.length > 0) {
    parts.push(`denied=${formatPolicyList(key.denied_models, "none")}`);
  }
  return parts.join(" · ");
}

function toggleSelection(values: string[], value: string) {
  return values.includes(value)
    ? values.filter((item) => item !== value)
    : [...values, value];
}

function uniqueSorted(values: Array<string | undefined>) {
  return Array.from(new Set(values.map((value) => String(value || "").trim()).filter(Boolean))).toSorted();
}

export function AuthKeysPage(): JSX.Element {
  const { data: keys = [], isLoading, createMutation, deactivateMutation } = useAuthKeys();
  const { data: modelInventory = [] } = useModels();
  const { data: providerStatus } = useProviderStatus();
  const { data: pools } = usePools();

  const [formOpen, setFormOpen] = useState(false);
  const [issuedKey, setIssuedKey] = useState<AuthKeyIssued | null>(null);
  const [copied, setCopied] = useState(false);
  const [copyError, setCopyError] = useState<string | null>(null);
  const [revealIssuedKey, setRevealIssuedKey] = useState(false);

  // Form State
  const [formData, setFormData] = useState<AuthKeyFormState>(defaultAuthKeyForm);

  const [deactivateId, setDeactivateId] = useState<string | null>(null);
  const [statsKey, setStatsKey] = useState<AuthKey | null>(null);
  const [page, setPage] = useState(1);
  const pageSize = 10;
  const totalPages = Math.max(1, Math.ceil(keys.length / pageSize));
  const safePage = Math.min(page, totalPages);
  const paginatedKeys = keys.slice((safePage - 1) * pageSize, safePage * pageSize);
  React.useEffect(() => { setPage(1); }, [keys.length]);
  const providerOptions = uniqueSorted([
    ...(providerStatus?.providers || []).map((provider) => provider.name),
    ...(pools?.pools || []).map((pool) => pool.name),
  ]);
  const poolOptions = uniqueSorted((pools?.pools || []).map((pool) => pool.name));
  const modelOptions = uniqueSorted(modelInventory.map((item) => item.model?.id));

  // Handlers
  const handleOpenForm = () => {
    setFormData(defaultAuthKeyForm());
    setIssuedKey(null);
    setCopied(false);
    setCopyError(null);
    setRevealIssuedKey(false);
    setFormOpen(true);
  };

  const handleCopy = async () => {
    if (!issuedKey) return;
    try {
      await navigator.clipboard.writeText(issuedKey.value);
      setCopied(true);
      setCopyError(null);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
      setCopyError("Clipboard access failed. Reveal and copy manually.");
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    createMutation.mutate({
      ...formData,
      allowed_providers: formData.allowed_providers,
      allowed_models: formData.allowed_models,
      denied_models: formData.denied_models,
      provider_pool_id: formData.provider_pool_id?.trim() || undefined,
    }, {
      onSuccess: (data) => {
        setIssuedKey(data);
      },
    });
  };

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="API Keys"
        subtitle="Issue scoped API keys for services, teams, or deployments that need gateway access."
        actions={
          <Button onClick={handleOpenForm}>
            <PlusIcon className="mr-2 h-4 w-4" />
            Create API Key
          </Button>
        }
      />

      <DocsCollapsible
        title="How API keys work"
        manual="API keys let your backend programmatically issue gateway credentials for each customer. When a user signs up on your platform, call POST /admin/api/v1/auth-keys to create a key scoped to their user path. Revoke it with the deactivate endpoint when they leave. Keys can also be managed manually through this dashboard for testing and debugging."
        authNote='All endpoints require an Authorization: Bearer &lt;master-key&gt; header. The master key is set via the MASTER_KEY environment variable.'
        endpoints={[
          { method: "POST", path: "/admin/api/v1/auth-keys", description: "Issue a scoped key — call this when a new user signs up to grant them gateway access" },
          { method: "POST", path: "/admin/api/v1/auth-keys/:id/deactivate", description: "Revoke a key — call this when a user is removed or leaves your platform" },
          { method: "GET", path: "/admin/api/v1/auth-keys", description: "List all keys to find a specific key's ID or review active keys" },
          { method: "GET", path: "/admin/api/v1/auth-keys/:id/stats", description: "Check usage stats (requests, tokens) for a specific key" },
        ]}
      />

      {isLoading ? (
        <Surface variant="subtle" className="py-12">
          <EmptyState
            title="Loading API keys..."
            description="Fetching your access keys."
          />
        </Surface>
      ) : keys.length === 0 ? (
        <Surface variant="subtle" className="py-12">
          <EmptyState
            title="No API keys yet"
            description="Create a managed key for a service, team, or deployment that needs gateway access."
            action={
              <Button onClick={handleOpenForm}>
                <PlusIcon className="mr-2 h-4 w-4" />
                Create API Key
              </Button>
            }
          />
        </Surface>
      ) : (
        <Surface>
          <TableWrap>
            <DataTable>
              <thead className="bg-surface-hover/30 backdrop-blur-sm">
                <tr>
                  <Th>Name & Description</Th>
                  <Th>User Path</Th>
				  <Th>Routing</Th>
                  <Th className="text-right">Rate Limits</Th>
                  <Th>Token</Th>
                  <Th>Status</Th>
                  <Th>Expires</Th>
                  <Th className="text-right">Actions</Th>
                </tr>
              </thead>
              <tbody>
                {paginatedKeys.map((key) => {
                  const limits = [];
                  const rl = key.rate_limits || key;
                  if (rl.requests_per_minute) limits.push(`${rl.requests_per_minute} rpm`);
                  if (rl.requests_per_day) limits.push(`${rl.requests_per_day} rpd`);
                  if (rl.tokens_per_minute) limits.push(`${rl.tokens_per_minute} tpm`);
                  if (rl.tokens_per_day) limits.push(`${rl.tokens_per_day} tpd`);


                  return (
                    <tr key={key.id} className="hover:bg-surface-hover/40 transition-colors border-b border-border/40 last:border-0">
                      <Td>
                        <div className="flex flex-col gap-1">
                          <span className="font-semibold text-[14px] text-foreground">{key.name}</span>
                          {key.description && <span className="text-[12px] text-muted-foreground max-w-[200px] truncate" title={key.description}>{key.description}</span>}
                        </div>
                      </Td>
                      <Td><code className="border border-border/40 bg-background/50 px-2 py-1 font-mono text-[12px]">{key.user_path || "global"}</code></Td>
                      <Td>
                        <div className="max-w-[260px] truncate text-[12px] text-muted-foreground" title={authKeyRoutingSummary(key)}>
                          {authKeyRoutingSummary(key)}
                        </div>
                      </Td>
                      <Td className="text-right">
                        <div className="flex flex-wrap justify-end gap-1.5 max-w-[220px] ml-auto">
                          {limits.length > 0 ? limits.map((limit) => (
                            <span key={limit} className="inline-flex items-center rounded bg-muted/30 border border-border/40 px-1.5 py-0.5 text-[10px] font-mono text-muted-foreground">{limit}</span>
                          )) : <span className="text-muted-foreground text-[12px] font-medium">Unlimited</span>}
                        </div>
                      </Td>
                      <Td><code className="border border-border/40 bg-surface/50 px-2.5 py-1 font-mono text-[12px] tracking-widest">{key.redacted_value}</code></Td>
                      <Td>
                        <span
                          className={`inline-flex items-center border px-2.5 py-0.5 text-[10px] font-bold tracking-wider uppercase ${key.active
                              ? "border-success/30 bg-success/10 text-success"
                              : "border-border/60 bg-surface/60 text-muted-foreground"
                            }`}
                        >
                          {key.active ? "Active" : "Inactive"}
                        </span>
                      </Td>
                      <Td className="whitespace-nowrap text-muted-foreground text-[12px] font-medium">
                        {formatOptionalDate(key.expires_at)}
                      </Td>
                      <Td className="text-right">
                        <div className="flex justify-end gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setStatsKey(key)}
                          >
                            <BarChart3Icon className="mr-1.5 h-3.5 w-3.5" />
                            View
                          </Button>
                          {key.active && (
                            <Button
                              variant="outline"
                              size="sm"
                              className="text-destructive hover:bg-destructive/10 hover:text-destructive border-destructive/20"
                              disabled={deactivateMutation.isPending}
                              onClick={() => setDeactivateId(key.id)}
                            >
                              Deactivate
                            </Button>
                          )}
                        </div>
                      </Td>
                    </tr>
                  );
                })}
              </tbody>
            </DataTable>
          </TableWrap>
          {totalPages > 1 && (
            <div className="flex items-center justify-between border-t border-border/40 px-4 py-3">
              <span className="text-[12px] text-muted-foreground">
                {keys.length} key{keys.length !== 1 ? "s" : ""}
              </span>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={safePage <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  <ChevronLeftIcon className="h-4 w-4" />
                </Button>
                <span className="text-[12px] text-muted-foreground font-mono tabular-nums min-w-[4rem] text-center">
                  {safePage} / {totalPages}
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={safePage >= totalPages}
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                >
                  <ChevronRightIcon className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}
        </Surface>
      )}

      <Dialog open={deactivateId !== null} onOpenChange={(open) => { if (!open) setDeactivateId(null) }}>
        <DialogContent className="sm:max-w-[450px]">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-destructive">
              <ShieldAlertIcon className="h-5 w-5" />
              Deactivate API Key
            </DialogTitle>
            <DialogDescription className="text-[14px] leading-relaxed pt-3">
              Are you sure you want to deactivate this key?
              <br /><br />
              <strong className="text-foreground">This is a one-time permanent action.</strong> The key will immediately stop authenticating gateway requests. If you or your services need access again, you must create a brand new API Key and rotate it.
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-3 pt-4 border-t border-border/40 mt-2">
            <Button variant="outline" onClick={() => setDeactivateId(null)}>Cancel</Button>
            <Button
              variant="destructive"
              disabled={deactivateMutation.isPending}
              onClick={() => {
                if (deactivateId) {
                  deactivateMutation.mutate(deactivateId, {
                    onSuccess: () => setDeactivateId(null)
                  });
                }
              }}
            >
              {deactivateMutation.isPending ? "Deactivating..." : "Yes, permanently deactivate"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <AuthKeyStatsDrawer
        keyEntry={statsKey}
        onClose={() => setStatsKey(null)}
      />

      <Dialog
        open={formOpen}
        onOpenChange={(open) => {
          setFormOpen(open);
          if (!open) {
            setIssuedKey(null);
            setCopied(false);
            setCopyError(null);
            setRevealIssuedKey(false);
          }
        }}
      >
        <DialogContent className="sm:max-w-[1200px]">
          {issuedKey ? (
            <>
              <DialogHeader>
                <DialogTitle>API Key Created</DialogTitle>
                <DialogDescription className="text-warning font-medium">
                  Store this key securely — it won't be shown again.
                </DialogDescription>
              </DialogHeader>
              <div className="flex flex-col gap-4 py-4">
                <div className="flex items-center gap-2 rounded-md border bg-muted p-3">
                  <code className="flex-1 break-all font-mono text-sm">
                    {revealIssuedKey ? issuedKey.value : issuedKey.redacted_value || "••••••••••••••••"}
                  </code>
                  <Button type="button" variant="outline" onClick={() => setRevealIssuedKey((value) => !value)}>
                    {revealIssuedKey ? "Hide" : "Reveal"}
                  </Button>
                  <Button size="icon" variant="ghost" onClick={handleCopy} aria-label="Copy API key">
                    {copied ? <CheckIcon className="h-4 w-4 text-success" /> : <CopyIcon className="h-4 w-4" />}
                  </Button>
                </div>
                {copyError && <p className="text-sm text-destructive">{copyError}</p>}
              </div>
              <div className="flex justify-end">
                <Button onClick={() => setFormOpen(false)}>Done</Button>
              </div>
            </>
          ) : (
            <form onSubmit={handleSubmit}>
              <DialogHeader>
                <DialogTitle>Create API Key</DialogTitle>
                <DialogDescription>
                  Generate a new access token for the gateway.
                </DialogDescription>
              </DialogHeader>
              <div className="flex flex-col gap-4 py-4">
                <div className="flex flex-col gap-2">
                  <label htmlFor="name" className="text-sm font-medium">Name <span className="text-muted-foreground text-xs font-normal">(required)</span></label>
                  <Input
                    id="name"
                    required
                    placeholder="e.g. ci-deploy"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  />
                </div>
                <div className="flex flex-col gap-2">
                  <label htmlFor="user_path" className="text-sm font-medium">User Path <span className="text-muted-foreground text-xs font-normal">(optional)</span></label>
                  <Input
                    id="user_path"
                    placeholder="e.g. /department1/team-a"
                    value={formData.user_path}
                    onChange={(e) => setFormData({ ...formData, user_path: e.target.value })}
                  />
                  <p className="text-xs text-muted-foreground">Overrides X-aurora-User-Path for downstream routing and audit logging.</p>
                </div>
                <input type="hidden" name="tenant_id" value="default" />
                <div className="flex flex-col gap-2">
                  <label htmlFor="description" className="text-sm font-medium">Description <span className="text-muted-foreground text-xs font-normal">(optional)</span></label>
                  <Input
                    id="description"
                    placeholder="What is this key used for?"
                    value={formData.description}
                    onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  />
                </div>
                <div className=" border border-border/60 bg-muted/15 p-4">
                  <div className="mb-3">
                    <h3 className="text-sm font-semibold text-foreground">Routing Restrictions</h3>
                    <p className="mt-1 text-xs text-muted-foreground">
                      Optional Bifrost-style virtual-key controls. Select from configured providers, pools, and discovered models. Leave blank to allow all.
                    </p>
                  </div>
                  <div className="grid gap-4 lg:grid-cols-3">
                    <div className="flex flex-col gap-1.5 lg:col-span-3">
                      <label htmlFor="provider_pool_id" className="text-xs font-medium text-muted-foreground">Default provider pool</label>
                      <select
                        id="provider_pool_id"
                        className="field-input"
                        value={formData.provider_pool_id || ""}
                        onChange={(e) => setFormData({ ...formData, provider_pool_id: e.target.value })}
                      >
                        <option value="">No default pool</option>
                        {poolOptions.map((pool) => (
                          <option key={pool} value={pool}>{pool}</option>
                        ))}
                      </select>
                      {poolOptions.length === 0 && <p className="text-[11px] text-muted-foreground">No pools are configured.</p>}
                    </div>
                    <div className="flex flex-col gap-2">
                      <span className="text-xs font-medium text-muted-foreground">Allowed providers and pools</span>
                      <div className="max-h-60 overflow-y-auto rounded-lg border border-border/60 bg-background/45 p-2">
                        {providerOptions.length > 0 ? providerOptions.map((provider) => (
                          <label key={provider} className="flex items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-muted/40">
                            <input
                              type="checkbox"
                              checked={formData.allowed_providers?.includes(provider) || false}
                              onChange={() => setFormData({ ...formData, allowed_providers: toggleSelection(formData.allowed_providers || [], provider) })}
                            />
                            <span className="font-mono">{provider}</span>
                          </label>
                        )) : <p className="px-2 py-1.5 text-xs text-muted-foreground">No providers loaded.</p>}
                      </div>
                    </div>
                    <div className="flex flex-col gap-2">
                      <span className="text-xs font-medium text-muted-foreground">Allowed models</span>
                      <div className="max-h-60 overflow-y-auto rounded-lg border border-border/60 bg-background/45 p-2">
                        <label className="flex items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-muted/40">
                          <input
                            type="checkbox"
                            checked={formData.allowed_models?.includes("*") || false}
                            onChange={() => setFormData({ ...formData, allowed_models: toggleSelection(formData.allowed_models || [], "*") })}
                          />
                          <span className="font-mono">*</span>
                          <span className="text-muted-foreground">all models</span>
                        </label>
                        <div className="max-h-52 overflow-y-auto">
                          {modelOptions.map((model) => (
                            <label key={model} className="flex items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-muted/40">
                              <input
                                type="checkbox"
                                checked={formData.allowed_models?.includes(model) || false}
                                onChange={() => setFormData({ ...formData, allowed_models: toggleSelection(formData.allowed_models || [], model) })}
                              />
                              <span className="font-mono break-all">{model}</span>
                            </label>
                          ))}
                        </div>
                      </div>
                    </div>
                    <div className="flex flex-col gap-2">
                      <span className="text-xs font-medium text-muted-foreground">Denied models</span>
                      <div className="max-h-60 overflow-y-auto rounded-lg border border-border/60 bg-background/45 p-2">
                        {modelOptions.length > 0 ? modelOptions.map((model) => (
                          <label key={model} className="flex items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-muted/40">
                            <input
                              type="checkbox"
                              checked={formData.denied_models?.includes(model) || false}
                              onChange={() => setFormData({ ...formData, denied_models: toggleSelection(formData.denied_models || [], model) })}
                            />
                            <span className="font-mono break-all">{model}</span>
                          </label>
                        )) : <p className="px-2 py-1.5 text-xs text-muted-foreground">No models loaded.</p>}
                      </div>
                    </div>
                  </div>
                  <p className="mt-3 text-[11px] leading-relaxed text-muted-foreground">
                    Denied models win over allowed models. Use <code className="rounded bg-background/60 px-1 py-0.5">*</code> to explicitly allow all in an allowlist.
                  </p>
                </div>
                <div className="flex flex-col gap-2">
                  <label className="text-sm font-medium">Rate Limits <span className="text-muted-foreground text-xs font-normal">(optional)</span></label>
                  <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
                    <div className="flex flex-col gap-1">
                      <span className="text-xs text-muted-foreground">Requests / min</span>
                      <Input type="number" min="0" placeholder="Unlimited" value={formData.requests_per_minute || ""} onChange={e => setFormData({ ...formData, requests_per_minute: parseInt(e.target.value) || undefined })} />
                    </div>
                    <div className="flex flex-col gap-1">
                      <span className="text-xs text-muted-foreground">Requests / day</span>
                      <Input type="number" min="0" placeholder="Unlimited" value={formData.requests_per_day || ""} onChange={e => setFormData({ ...formData, requests_per_day: parseInt(e.target.value) || undefined })} />
                    </div>
                    <div className="flex flex-col gap-1">
                      <span className="text-xs text-muted-foreground">Tokens / min</span>
                      <Input type="number" min="0" placeholder="Unlimited" value={formData.tokens_per_minute || ""} onChange={e => setFormData({ ...formData, tokens_per_minute: parseInt(e.target.value) || undefined })} />
                    </div>
                    <div className="flex flex-col gap-1">
                      <span className="text-xs text-muted-foreground">Tokens / day</span>
                      <Input type="number" min="0" placeholder="Unlimited" value={formData.tokens_per_day || ""} onChange={e => setFormData({ ...formData, tokens_per_day: parseInt(e.target.value) || undefined })} />
                    </div>
                  </div>
                  <p className="text-[11px] leading-relaxed text-muted-foreground">
                    Rate limits are enforced in-memory by default. Configure Redis (<code className="rounded bg-background/60 px-1 py-0.5">REDIS_URL</code>) to share counters across replicas for distributed rate limiting.
                  </p>
                </div>
                <div className="flex flex-col gap-2">
                  <label htmlFor="expires_at" className="text-sm font-medium">Expires At <span className="text-muted-foreground text-xs font-normal">(optional)</span></label>
                  <Input
                    id="expires_at"
                    type="date"
                    value={formData.expires_at}
                    onChange={(e) => setFormData({ ...formData, expires_at: e.target.value })}
                  />
                </div>
                {createMutation.isError && (
                  <p className="text-sm text-destructive">{createMutation.error.message}</p>
                )}
              </div>
              <div className="flex justify-end gap-2">
                <Button type="button" variant="outline" onClick={() => setFormOpen(false)}>Cancel</Button>
                <Button type="submit" disabled={createMutation.isPending}>
                  {createMutation.isPending ? "Creating..." : "Create API Key"}
                </Button>
              </div>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

const STATS_WINDOW_OPTIONS: ReadonlyArray<{ days: number; label: string }> = [
  { days: 7, label: "7d" },
  { days: 30, label: "30d" },
  { days: 90, label: "90d" },
];

interface AuthKeyStatsDrawerProps {
  keyEntry: AuthKey | null;
  onClose: () => void;
}

function AuthKeyStatsDrawer({ keyEntry, onClose }: AuthKeyStatsDrawerProps): JSX.Element {
  const [days, setDays] = useState<number>(30);
  const id = keyEntry?.id ?? null;
  const { data, isLoading, isError, error } = useAuthKeyStats(id, days);

  return (
    <Dialog open={keyEntry !== null} onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="sm:max-w-[1400px] max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <BarChart3Icon className="h-5 w-5" />
            {keyEntry?.name ?? "API Key"} — Stats
          </DialogTitle>
          <DialogDescription>
            Live rate-limit consumption and recent activity for this key.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-3 py-3 border-b border-border/40 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-wrap items-center gap-2 text-[12px] text-muted-foreground">
            {keyEntry?.redacted_value && (
              <code className="border border-border/40 bg-surface/50 px-2 py-0.5 font-mono tracking-widest">
                {keyEntry.redacted_value}
              </code>
            )}
            <span className="font-mono">path={keyEntry?.user_path || "global"}</span>
            {data?.window && (
              <span>
                {data.window.start_date} → {data.window.end_date} ({data.window.timezone})
              </span>
            )}
            {data?.last_used_at && <span>last used {formatDateTime(data.last_used_at)}</span>}
          </div>
          <div className="flex items-center gap-1 rounded-md border border-border/60 bg-background/40 p-0.5 self-start lg:self-auto">
            {STATS_WINDOW_OPTIONS.map((option) => (
              <button
                key={option.days}
                type="button"
                onClick={() => setDays(option.days)}
                className={`px-2.5 py-1 text-[11px] font-semibold rounded ${days === option.days
                    ? "bg-primary/15 text-primary"
                    : "text-muted-foreground hover:bg-muted/40"
                  }`}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>

        {isLoading && (
          <div className="py-12">
            <EmptyState title="Loading stats…" description="Fetching current consumption and recent traffic." />
          </div>
        )}
        {isError && (
          <div className="py-8">
            <EmptyState
              title="Couldn't load stats"
              description={error?.message ?? "An unknown error occurred."}
            />
          </div>
        )}

        {data && (
          <div className="flex flex-col gap-5 pt-4">
            <RateLimitSection snapshot={data.rate_limit_status ?? null} />
            <SummaryGrid stats={data} />
            <RequestBreakdown stats={data} />
            <UsageSection stats={data} />
            <div className="grid gap-4 lg:grid-cols-3">
              <BucketList title="Top models" buckets={data.top_models} />
              <BucketList title="Top providers" buckets={data.top_providers} />
              <BucketList title="Top errors" buckets={data.top_errors} emptyHint="No errors in window" />
            </div>
            <DailySeries series={data.daily} />
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

interface RateLimitSectionProps {
  snapshot: AuthKeyStats["rate_limit_status"] | null;
}

function RateLimitSection({ snapshot }: RateLimitSectionProps): JSX.Element {
  const windows: Array<{ key: string; label: string; window: AuthKeyRateLimitWindow }> = [];
  if (snapshot?.requests_per_minute) windows.push({ key: "rpm", label: "Requests / min", window: snapshot.requests_per_minute });
  if (snapshot?.requests_per_day) windows.push({ key: "rpd", label: "Requests / day", window: snapshot.requests_per_day });
  if (snapshot?.tokens_per_minute) windows.push({ key: "tpm", label: "Tokens / min", window: snapshot.tokens_per_minute });
  if (snapshot?.tokens_per_day) windows.push({ key: "tpd", label: "Tokens / day", window: snapshot.tokens_per_day });

  return (
    <section>
      <h3 className="text-[13px] font-semibold uppercase tracking-wide text-muted-foreground mb-2">Rate Limits</h3>
      {windows.length === 0 ? (
        <Surface variant="subtle" className="py-4">
          <p className="text-center text-[12px] text-muted-foreground">No rate limits configured for this key.</p>
        </Surface>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          {windows.map((entry) => (
            <RateLimitBar key={entry.key} label={entry.label} window={entry.window} />
          ))}
        </div>
      )}
    </section>
  );
}

function RateLimitBar({ label, window }: { label: string; window: AuthKeyRateLimitWindow }): JSX.Element {
  const safeLimit = window.limit > 0 ? window.limit : 1;
  const pct = Math.min(100, Math.max(0, (window.used / safeLimit) * 100));
  const tone = pct >= 90 ? "bg-destructive" : pct >= 70 ? "bg-warning" : "bg-primary";
  const resetAt = useMemo(() => {
    try {
      return new Date(window.reset_at).toLocaleTimeString();
    } catch {
      return window.reset_at;
    }
  }, [window.reset_at]);

  return (
    <div className="rounded-lg border border-border/60 bg-background/40 px-3 py-2.5">
      <div className="flex items-baseline justify-between gap-2 mb-1.5">
        <span className="text-[12px] font-medium text-foreground">{label}</span>
        <span className="text-[11px] font-mono text-muted-foreground">
          {window.used.toLocaleString()} / {window.limit.toLocaleString()}
        </span>
      </div>
      <div className="h-1.5 bg-muted/50 overflow-hidden">
        <div className={`h-full ${tone} transition-all`} style={{ width: `${pct}%` }} />
      </div>
      <div className="mt-1.5 flex items-center justify-between text-[10px] text-muted-foreground">
        <span>{window.remaining.toLocaleString()} remaining</span>
        <span>resets {resetAt}</span>
      </div>
    </div>
  );
}

interface SummaryGridProps {
  stats: {
    requests: { total: number; success_rate: number; error_rate: number; stream_count: number };
    cache: { hit_rate: number; total_hits: number; misses: number };
    latency: { avg_ns: number; max_ns: number };
  };
}

function SummaryGrid({ stats }: SummaryGridProps): JSX.Element {
  const cards = [
    { label: "Requests", value: stats.requests.total.toLocaleString(), hint: `${stats.requests.stream_count.toLocaleString()} streamed` },
    { label: "Success rate", value: formatPct(stats.requests.success_rate), hint: `${formatPct(stats.requests.error_rate)} errors` },
    { label: "Cache hit rate", value: formatPct(stats.cache.hit_rate), hint: `${stats.cache.total_hits.toLocaleString()} hits / ${stats.cache.misses.toLocaleString()} misses` },
    { label: "Avg latency", value: formatDurationNs(stats.latency.avg_ns), hint: `max ${formatDurationNs(stats.latency.max_ns)}` },
  ];

  return (
    <section>
      <h3 className="text-[13px] font-semibold uppercase tracking-wide text-muted-foreground mb-2">Activity</h3>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {cards.map((card) => (
          <Surface key={card.label} variant="subtle" className="px-3 py-2.5">
            <div className="text-[11px] uppercase tracking-wide text-muted-foreground">{card.label}</div>
            <div className="mt-1 text-[20px] font-semibold text-foreground tabular-nums">{card.value}</div>
            <div className="text-[11px] text-muted-foreground">{card.hint}</div>
          </Surface>
        ))}
      </div>
    </section>
  );
}

function RequestBreakdown({ stats }: { stats: AuthKeyStats }): JSX.Element {
  const requestRows = [
    ["2xx success", stats.requests.success_count],
    ["3xx redirect", stats.requests.redirect_count],
    ["4xx client errors", stats.requests.client_error_count],
    ["5xx server errors", stats.requests.server_error_count],
  ] as const;
  const cacheRows = [
    ["Exact hits", stats.cache.exact_hits],
    ["Semantic hits", stats.cache.semantic_hits],
    ["Misses", stats.cache.misses],
  ] as const;
  const latencyRows = [
    ["Min", formatDurationNs(stats.latency.min_ns)],
    ["Avg", formatDurationNs(stats.latency.avg_ns)],
    ["Max", formatDurationNs(stats.latency.max_ns)],
  ] as const;

  return (
    <section className="grid gap-4 lg:grid-cols-3">
      <BreakdownCard title="Status breakdown" rows={requestRows} />
      <BreakdownCard title="Cache breakdown" rows={cacheRows} />
      <BreakdownCard title="Latency" rows={latencyRows} />
    </section>
  );
}

function BreakdownCard({ title, rows }: { title: string; rows: ReadonlyArray<readonly [string, number | string]> }): JSX.Element {
  return (
    <Surface variant="subtle" className="px-3 py-2.5">
      <h3 className="text-[12px] font-semibold uppercase tracking-wide text-muted-foreground mb-2">{title}</h3>
      <dl className="flex flex-col gap-1.5">
        {rows.map(([label, value]) => (
          <div key={label} className="flex items-center justify-between gap-3 text-[12px]">
            <dt className="text-muted-foreground">{label}</dt>
            <dd className="font-mono tabular-nums text-foreground">
              {typeof value === "number" ? value.toLocaleString() : value}
            </dd>
          </div>
        ))}
      </dl>
    </Surface>
  );
}

interface UsageSectionProps {
  stats: AuthKeyStats;
}

function UsageSection({ stats }: UsageSectionProps): JSX.Element {
  const { usage } = stats;
  return (
    <section>
      <h3 className="text-[13px] font-semibold uppercase tracking-wide text-muted-foreground mb-2">Usage</h3>
      <Surface variant="subtle" className="px-4 py-3">
        <div className="grid gap-4 sm:grid-cols-3">
          <UsageMetric label="Input tokens" value={usage.input_tokens.toLocaleString()} cost={usage.input_cost} />
          <UsageMetric label="Output tokens" value={usage.output_tokens.toLocaleString()} cost={usage.output_cost} />
          <UsageMetric label="Total tokens" value={usage.total_tokens.toLocaleString()} cost={usage.total_cost} bold />
        </div>
        {usage.note_user_path_tie && (
          <p className="mt-3 border border-info/30 bg-info/10 px-3 py-2 text-[11px] text-info">
            Token and cost figures are aggregated by user_path, so they may include other keys using the same path.
          </p>
        )}
      </Surface>
    </section>
  );
}

function UsageMetric({ label, value, cost, bold }: { label: string; value: string; cost?: number | null | undefined; bold?: boolean }): JSX.Element {
  return (
    <div>
      <div className="text-[11px] uppercase tracking-wide text-muted-foreground">{label}</div>
      <div className={`mt-0.5 tabular-nums ${bold ? "text-[18px] font-semibold" : "text-[16px] font-medium"} text-foreground`}>{value}</div>
      {cost != null && (
        <div className="text-[11px] text-muted-foreground">${cost.toFixed(4)}</div>
      )}
    </div>
  );
}

function BucketList({ title, buckets, emptyHint }: { title: string; buckets: AuthKeyBucketCount[]; emptyHint?: string }): JSX.Element {
  const max = buckets.reduce((acc, b) => Math.max(acc, b.count), 0);
  return (
    <section>
      <h3 className="text-[13px] font-semibold uppercase tracking-wide text-muted-foreground mb-2">{title}</h3>
      <Surface variant="subtle" className="px-3 py-2">
        {buckets.length === 0 ? (
          <p className="py-3 text-center text-[12px] text-muted-foreground">{emptyHint ?? "No data"}</p>
        ) : (
          <ul className="flex flex-col gap-1.5">
            {buckets.map((bucket) => {
              const pct = max > 0 ? (bucket.count / max) * 100 : 0;
              return (
                <li key={bucket.label} className="flex flex-col gap-0.5">
                  <div className="flex items-center justify-between text-[12px]">
                    <span className="truncate font-mono" title={bucket.label}>{bucket.label}</span>
                    <span className="tabular-nums text-muted-foreground">{bucket.count.toLocaleString()}</span>
                  </div>
                  <div className="h-1 rounded bg-muted/40 overflow-hidden">
                    <div className="h-full bg-primary/60 rounded" style={{ width: `${pct}%` }} />
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </Surface>
    </section>
  );
}

interface DailySeriesProps {
  series: Array<{ date: string; requests: number; errors: number; cache_hits: number }>;
}

function DailySeries({ series }: DailySeriesProps): JSX.Element {
  const totalRequests = series.reduce((acc, day) => acc + day.requests, 0);
  if (series.length === 0 || totalRequests === 0) {
    return (
      <section>
        <h3 className="text-[13px] font-semibold uppercase tracking-wide text-muted-foreground mb-2">Daily activity</h3>
        <Surface variant="subtle" className="py-6">
          <p className="text-center text-[12px] text-muted-foreground">No requests in window.</p>
        </Surface>
      </section>
    );
  }

  const totalErrors = series.reduce((acc, day) => acc + day.errors, 0);
  const totalCacheHits = series.reduce((acc, day) => acc + day.cache_hits, 0);
  const max = series.reduce((acc, day) => Math.max(acc, day.requests), 0) || 1;
  return (
    <section>
      <div className="mb-2 flex items-center justify-between gap-3">
        <h3 className="text-[13px] font-semibold uppercase tracking-wide text-muted-foreground">Daily activity</h3>
        <span className="text-[11px] text-muted-foreground">
          {totalRequests.toLocaleString()} requests · {totalErrors.toLocaleString()} errors · {totalCacheHits.toLocaleString()} cache hits
        </span>
      </div>
      <Surface variant="subtle" className="px-4 py-3">
        <div className="flex items-end gap-1 h-32">
          {series.map((day) => {
            const reqHeight = (day.requests / max) * 100;
            const errPct = day.requests > 0 ? (day.errors / day.requests) * 100 : 0;
            return (
              <div
                key={day.date}
                className="flex-1 flex flex-col justify-end group relative h-full"
                title={`${day.date}: ${day.requests.toLocaleString()} req · ${day.errors.toLocaleString()} err · ${day.cache_hits.toLocaleString()} cache`}
              >
                <div
                  className="w-full bg-primary/40 rounded-t group-hover:bg-primary/60 transition-colors relative"
                  style={{ height: `${reqHeight}%` }}
                >
                  {errPct > 0 && (
                    <div
                      className="absolute bottom-0 left-0 right-0 bg-destructive/70 rounded-t"
                      style={{ height: `${errPct}%` }}
                    />
                  )}
                </div>
              </div>
            );
          })}
        </div>
        <div className="mt-2 flex justify-between text-[10px] text-muted-foreground font-mono">
          <span>{series[0]?.date}</span>
          <span>{series[series.length - 1]?.date}</span>
        </div>
      </Surface>
    </section>
  );
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return format(date, "MMM d, yyyy h:mm a");
}

function formatOptionalDate(value: string | null | undefined): string {
  if (!value) return "Never";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return format(date, "MMM d, yyyy");
}

function formatPct(value: number): string {
  if (!Number.isFinite(value)) return "—";
  return `${(value * 100).toFixed(1)}%`;
}

function formatDurationNs(ns: number): string {
  if (!ns || !Number.isFinite(ns)) return "—";
  if (ns < 1_000) return `${ns} ns`;
  if (ns < 1_000_000) return `${(ns / 1_000).toFixed(1)} µs`;
  if (ns < 1_000_000_000) return `${(ns / 1_000_000).toFixed(1)} ms`;
  return `${(ns / 1_000_000_000).toFixed(2)} s`;
}

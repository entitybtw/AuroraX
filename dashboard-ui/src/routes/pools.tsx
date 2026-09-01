import * as React from "react";
import {
  Shuffle,
  Users,
  Activity,
  HeartPulse,
  Orbit,
  Loader2,
  Plus,
  Pencil,
  Trash2,
  X,
  Server,
  ShieldCheck,
  Weight,
  Globe,
  Database,
} from "lucide-react";
import { PageHeader } from "@/components/ui/page-header";
import { Button } from "@/components/ui/button";
import { ToggleField } from "@/components/ui/toggle-field";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { EmptyState, Pill, Surface } from "@/components/ui/surface";
import { usePools } from "@/lib/api/usePools";
import type { PoolSnapshot, PoolConfigPayload } from "@/lib/api/pools-types";
import {
  createPool,
  deletePool,
  fetchPoolOptions,
  updatePool,
} from "@/lib/api/pools";
import type { PoolOptions } from "@/lib/api/pools-types";
import { cn } from "@/lib/utils";
import { formatRequests } from "@/lib/format/numbers";

interface MemberForm {
  name: string;
  type: string;
  weight: number;
}

interface PoolForm {
  mode: "create" | "edit";
  originalName: string;
  name: string;
  strategy: "round_robin" | "weighted";
  healthAware: boolean;
  userAgent: string;
  autoFetchModels: boolean;
  members: MemberForm[];
}

export function PoolsPage(): JSX.Element {
  const pools = usePools();
  const [selectedIdx, setSelectedIdx] = React.useState(0);
  const [form, setForm] = React.useState<PoolForm | null>(null);
  const [options, setOptions] = React.useState<PoolOptions>({ providers: [] });
  const [error, setError] = React.useState("");
  const [notice, setNotice] = React.useState("");

  const poolList = pools.data?.pools ?? [];
  const selected = poolList[selectedIdx] ?? null;

  React.useEffect(() => {
    if (selectedIdx >= poolList.length) setSelectedIdx(0);
  }, [poolList.length, selectedIdx]);

  const loadOptions = React.useCallback(async () => {
    try {
      setOptions(await fetchPoolOptions());
    } catch {
      setOptions({ providers: [] });
    }
  }, []);

  const openCreate = () => {
    setError("");
    setForm({
      mode: "create",
      originalName: "",
      name: "",
      strategy: "round_robin",
      healthAware: true,
      userAgent: "",
      autoFetchModels: true,
      members: [],
    });
    void loadOptions();
  };

  const openEdit = (pool: PoolSnapshot) => {
    setError("");
    setForm({
      mode: "edit",
      originalName: pool.name,
      name: pool.name,
      strategy: (pool.strategy as PoolForm["strategy"]) || "round_robin",
      healthAware: pool.health_aware,
      userAgent: pool.user_agent ?? "",
      autoFetchModels: pool.auto_fetch_models ?? true,
      members: pool.members.map((m) => ({
        name: m.provider_name,
        type: "",
        weight: m.weight > 0 ? m.weight : 1,
      })),
    });
    void loadOptions();
  };

  async function submit(): Promise<void> {
    if (!form) return;
    if (!form.name.trim()) {
      setError("Pool name is required.");
      return;
    }
    if (form.members.length < 1) {
      setError("Add at least one provider member.");
      return;
    }
    const payload: PoolConfigPayload = {
      name: form.name.trim(),
      members: form.members.map((m) => m.name),
      strategy: form.strategy,
      health_aware: form.healthAware,
      ...(form.userAgent ? { user_agent: form.userAgent } : {}),
      auto_fetch_models: form.autoFetchModels,
      ...(form.strategy === "weighted"
        ? {
            weights: Object.fromEntries(
              form.members.map((m) => [m.name, m.weight]),
            ),
          }
        : {}),
    };
    try {
      setError("");
      if (form.mode === "edit") await updatePool(form.originalName, payload);
      else await createPool(payload);
      setForm(null);
      setNotice(form.mode === "edit" ? "Pool saved." : "Pool created.");
      await pools.refetch();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to save pool.");
    }
  }

  async function remove(pool: PoolSnapshot): Promise<void> {
    if (!window.confirm(`Delete pool ${pool.name}? Traffic routed through it will fall back to direct provider/model routing.`))
      return;
    try {
      setError("");
      await deletePool(pool.name);
      setNotice("Pool deleted.");
      await pools.refetch();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to delete pool.");
    }
  }

  if (pools.isLoading && !pools.data) return <LoadingState />;
  if (pools.error) return <ErrorState message={pools.error.message} />;

  return (
    <div className="flex flex-col gap-5">
      <PageHeader
        title="Provider Pools"
        subtitle="Load-balanced groups of providers. Address a pool like a model — changes apply without a restart."
        actions={
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4" />
            Create Pool
          </Button>
        }
      />

      {error ? <Alert tone="warning">{error}</Alert> : null}
      {notice ? <Alert tone="success">{notice}</Alert> : null}

      {poolList.length === 0 ? (
        <EmptyState title="No provider pools configured">
          Group two or more provider instances (e.g. multiple accounts / IPs of the
          same upstream) into a pool to balance load and fail over automatically.
        </EmptyState>
      ) : (
        <>
          <PoolSelector
            pools={poolList}
            selectedIdx={selectedIdx}
            onSelect={setSelectedIdx}
            onCreate={openCreate}
          />
          {selected && (
            <PoolView pool={selected} onEdit={openEdit} onDelete={remove} />
          )}
          {pools.data && (
            <div className="flex flex-wrap items-center gap-3">
              <SummaryBadge
                icon={Orbit}
                label="Total pools"
                value={String(pools.data.summary.total)}
              />
              <SummaryBadge
                icon={HeartPulse}
                label="Members online"
                value={`${pools.data.summary.healthy_members} / ${pools.data.summary.total_members}`}
                highlight={
                  pools.data.summary.healthy_members ===
                  pools.data.summary.total_members
                }
              />
            </div>
          )}
        </>
      )}

      <PoolDialog
        form={form}
        options={options}
        error={error}
        onChange={setForm}
        onClose={() => setForm(null)}
        onSubmit={() => void submit()}
      />
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Pool selector tabs                                                 */
/* ------------------------------------------------------------------ */

function PoolSelector({
  pools,
  selectedIdx,
  onSelect,
  onCreate,
}: {
  pools: PoolSnapshot[];
  selectedIdx: number;
  onSelect: (i: number) => void;
  onCreate: () => void;
}) {
  return (
    <div className="flex gap-1 overflow-x-auto border border-border/40 bg-surface p-1">
      {pools.map((pool, i) => {
        const isActive = selectedIdx === i;
        const allHealthy = pool.members.every((m) => m.healthy);
        return (
          <button
            key={pool.name}
            type="button"
            onClick={() => onSelect(i)}
            className={cn(
              "group flex shrink-0 items-center gap-2 px-3 py-2 text-xs font-medium transition-all duration-200",
              isActive
                ? "bg-accent/12 text-accent"
                : "text-muted-foreground hover:bg-surface-hover/60 hover:text-foreground",
            )}
          >
            <span
              className={cn(
                "h-1.5 w-1.5 transition-colors",
                allHealthy ? "bg-success" : "bg-warning",
              )}
            />
            <span className="font-semibold">{pool.name}</span>
            <span className="rounded bg-background/60 px-1.5 py-0.5 text-[10px] tabular-nums text-muted-foreground">
              {pool.members.length}
            </span>
            {pool.user_agent && (
              <span className="rounded bg-accent/10 px-1.5 py-0.5 text-[9px] tabular-nums text-accent" title="Custom User-Agent">
                <Globe className="h-2.5 w-2.5 inline" />
              </span>
            )}
            {pool.auto_fetch_models === false && (
              <span className="rounded bg-muted/10 px-1.5 py-0.5 text-[9px] tabular-nums text-muted-foreground" title="Auto-fetch disabled">
                <Database className="h-2.5 w-2.5 inline opacity-50" />
              </span>
            )}
          </button>
        );
      })}
      <button
        type="button"
        onClick={onCreate}
        className="flex shrink-0 items-center gap-1.5 px-3 py-2 text-xs font-medium text-muted-foreground transition-colors hover:text-accent"
      >
        <Plus className="h-3.5 w-3.5" />
        New
      </button>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Pool view                                                          */
/* ------------------------------------------------------------------ */

function PoolView({
  pool,
  onEdit,
  onDelete,
}: {
  pool: PoolSnapshot;
  onEdit: (p: PoolSnapshot) => void;
  onDelete: (p: PoolSnapshot) => void;
}) {
  const totalActive = pool.members.reduce((s, m) => s + m.active_requests, 0);

  const health =
    pool.members.every((m) => m.healthy)
      ? { label: "All healthy", className: "text-success", dot: "bg-success" }
      : pool.members.some((m) => m.healthy)
        ? { label: "Degraded", className: "text-warning", dot: "bg-warning" }
        : { label: "Unhealthy", className: "text-destructive", dot: "bg-destructive" };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <h2 className="font-mono text-lg font-semibold">{pool.name}</h2>
          <Pill tone={pool.source === "ui" ? "accent" : "muted"}>
            {pool.source === "ui" ? "UI-managed" : "config"}
          </Pill>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => onEdit(pool)}>
            <Pencil className="h-3.5 w-3.5" />
            Edit
          </Button>
          <Button variant="ghost" size="sm" onClick={() => void onDelete(pool)}>
            <Trash2 className="h-3.5 w-3.5 text-destructive" />
            Delete
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
        <StatCard icon={Shuffle} label="Strategy" value={pool.strategy.replace(/_/g, " ")} sub="Load-balancing" />
        <StatCard icon={Users} label="Members" value={String(pool.members.length)} sub="Providers" />
        <StatCard icon={Activity} label="Active" value={String(totalActive)} sub="In-flight" accent />
        <StatCard
          icon={ShieldCheck}
          label="Health-aware"
          value={pool.health_aware ? "On" : "Off"}
          sub="Skip unhealthy"
        />
        <StatCard
          icon={HeartPulse}
          label="Health"
          value={health.label}
          valueClass={health.className}
          dot={health.dot}
          sub={`${pool.members.filter((m) => m.healthy).length}/${pool.members.length} online`}
        />
        <StatCard
          icon={Weight}
          label="Weighted"
          value={pool.strategy === "weighted" ? "Yes" : "No"}
          sub="Per-member weights"
        />
        <StatCard
          icon={Globe}
          label="User Agent"
          value={pool.user_agent || "default"}
          sub="Override for pool"
        />
        <StatCard
          icon={Database}
          label="Auto-fetch models"
          value={pool.auto_fetch_models !== false ? "On" : "Off"}
          sub="Discover via /models"
        />
      </div>

      <MemberTable members={pool.members} strategy={pool.strategy} />

      <Surface className="p-4">
        <div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          <Server className="h-3.5 w-3.5" />
          Call the pool as a model
        </div>
        <p className="mb-3 text-xs text-muted-foreground">
          Address the pool by prefixing any model its members serve with the pool
          name. Requests are balanced across healthy members.
        </p>
        <pre className="overflow-x-auto rounded bg-background/60 p-3 font-mono text-xs text-foreground">
          {`curl \${BASE_PATH}/v1/chat/completions \\\n  -H "Authorization: Bearer $KEY" \\\n  -d '{"model":"${pool.name}/<model-id>","messages":[{"role":"user","content":"hi"}]}'`}
        </pre>
      </Surface>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Member table                                                       */
/* ------------------------------------------------------------------ */

function MemberTable({
  members,
  strategy,
}: {
  members: PoolSnapshot["members"];
  strategy: string;
}) {
  return (
    <div className="overflow-hidden border border-border/40 bg-surface">
      <div className="flex items-center justify-between border-b border-border/40 px-4 py-2.5">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          Member breakdown
        </h3>
        <span className="text-[10px] text-muted-foreground">
          {members.length} member{members.length !== 1 ? "s" : ""}
        </span>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-border/30 bg-background/30 text-left text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
              <th className="px-4 py-2.5">Provider</th>
              <th className="px-4 py-2.5">Status</th>
              <th className="px-4 py-2.5 text-right">Active</th>
              <th className="px-4 py-2.5 text-right">Total</th>
              <th className="px-4 py-2.5 text-right">Errors</th>
              <th className="px-4 py-2.5 text-right">Latency</th>
              {strategy === "weighted" && <th className="px-4 py-2.5 text-right">Weight</th>}
            </tr>
          </thead>
          <tbody>
            {members.map((m) => (
              <tr
                key={m.provider_name}
                className="border-b border-border/20 transition-colors duration-150 hover:bg-background/40"
              >
                <td className="px-4 py-2.5">
                  <div className="flex items-center gap-2">
                    <span className={cn("h-1.5 w-1.5 shrink-0", m.healthy ? "bg-success" : "bg-destructive")} />
                    <span className="font-mono font-medium text-foreground">{m.provider_name}</span>
                  </div>
                </td>
                <td className="px-4 py-2.5">
                  <span
                    className={cn(
                      "inline-flex items-center gap-1 px-2 py-0.5 text-[10px] font-semibold",
                      m.healthy ? "bg-success/10 text-success" : "bg-destructive/10 text-destructive",
                    )}
                  >
                    {m.healthy ? "Healthy" : "Unhealthy"}
                  </span>
                </td>
                <td className="px-4 py-2.5 text-right font-mono tabular-nums text-foreground">{m.active_requests}</td>
                <td className="px-4 py-2.5 text-right font-mono tabular-nums text-foreground">{formatRequests(m.total_requests)}</td>
                <td className={cn("px-4 py-2.5 text-right font-mono tabular-nums", m.total_errors > 0 ? "text-destructive" : "text-foreground")}>
                  {formatRequests(m.total_errors)}
                </td>
                <td className="px-4 py-2.5 text-right font-mono tabular-nums text-foreground">{formatLatency(m.latency_ewma_us)}</td>
                {strategy === "weighted" && (
                  <td className="px-4 py-2.5 text-right font-mono tabular-nums text-foreground">{m.weight ?? "-"}</td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Create / edit dialog                                               */
/* ------------------------------------------------------------------ */

function PoolDialog({
  form,
  options,
  error,
  onChange,
  onClose,
  onSubmit,
}: {
  form: PoolForm | null;
  options: PoolOptions;
  error: string;
  onChange: (form: PoolForm | null) => void;
  onClose: () => void;
  onSubmit: () => void;
}) {
  const [selectedAdd, setSelectedAdd] = React.useState("");

  if (!form) return <Dialog open={false} onOpenChange={() => undefined} />;

  // Once a member is picked, restrict the candidate list to the same provider type.
  const lockedType = form.members[0]?.type || null;
  const candidates = options.providers.filter((p) => {
    if (form.members.some((m) => m.name === p.name)) return false;
    if (lockedType && p.type !== lockedType) return false;
    return true;
  });

  const addMember = () => {
    const pick = candidates.find((c) => c.name === selectedAdd) ?? candidates[0];
    if (!pick) return;
    onChange({
      ...form,
      members: [...form.members, { name: pick.name, type: pick.type, weight: 1 }],
    });
    setSelectedAdd("");
  };

  const removeMember = (name: string) =>
    onChange({ ...form, members: form.members.filter((m) => m.name !== name) });

  const setWeight = (name: string, weight: number) =>
    onChange({
      ...form,
      members: form.members.map((m) => (m.name === name ? { ...m, weight } : m)),
    });

  return (
    <Dialog open={Boolean(form)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-2xl w-full">
        <DialogHeader>
          <DialogTitle>{form.mode === "edit" ? "Edit pool" : "Create pool"}</DialogTitle>
          <DialogDescription>
            Group provider instances that share an upstream and serve the same models.
            Changes apply live — no restart required.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <Field label="Pool name">
            <input
              className="field-input font-mono"
              value={form.name}
              disabled={form.mode === "edit"}
              placeholder="e.g. openai-multi-account"
              onChange={(e) => onChange({ ...form, name: e.target.value })}
            />
            {form.mode === "edit" && (
              <p className="text-[11px] text-muted-foreground">Pool name cannot be changed after creation.</p>
            )}
          </Field>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Strategy">
              <select
                className="field-input font-mono"
                value={form.strategy}
                onChange={(e) => onChange({ ...form, strategy: e.target.value as PoolForm["strategy"] })}
              >
                <option value="round_robin">round_robin</option>
                <option value="weighted">weighted</option>
              </select>
            </Field>
            <div className="flex items-end">
              <ToggleField
                size="sm"
                label="Health-aware routing"
                checked={form.healthAware}
                onCheckedChange={(checked) => onChange({ ...form, healthAware: checked })}
                aria-label="Health-aware routing"
              />
            </div>
            <Field label="User Agent">
              <input
                className="field-input font-mono"
                value={form.userAgent}
                placeholder="my-app/1.0"
                onChange={(e) => onChange({ ...form, userAgent: e.target.value })}
              />
              <p className="text-[11px] text-muted-foreground">Override User-Agent header for all members of this pool</p>
            </Field>
            <div className="flex items-end">
              <ToggleField
                size="sm"
                label="Auto-fetch models"
                checked={form.autoFetchModels}
                onCheckedChange={(checked) => onChange({ ...form, autoFetchModels: checked })}
                aria-label="Auto-fetch models"
              />
              <p className="text-[11px] ml-2 text-muted-foreground">Automatically discover models via /models endpoint for pool members</p>
            </div>
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium text-muted-foreground">Members</span>
              <span className="text-[10px] text-muted-foreground">
                {form.members.length} selected
              </span>
            </div>

            {form.members.length === 0 ? (
              <div className="border border-border bg-background/35 p-3 text-sm text-muted-foreground">
                No members yet. Add a provider below.
              </div>
            ) : (
              <div className="space-y-2">
                {form.members.map((m) => (
                  <div
                    key={m.name}
                    className="flex flex-wrap items-center gap-2 border border-border bg-background/35 px-3 py-2"
                  >
                    <span className="font-mono text-sm">{m.name}</span>
                    {m.type && <Pill tone="muted">{m.type}</Pill>}
                    {form.strategy === "weighted" && (
                      <label className="ml-auto flex items-center gap-1.5 text-xs text-muted-foreground">
                        weight
                        <input
                          type="number"
                          min={1}
                          value={m.weight}
                          onChange={(e) => setWeight(m.name, Math.max(1, Number(e.target.value) || 1))}
                          className="field-input w-20 py-1 text-center"
                        />
                      </label>
                    )}
                    <Button variant="ghost" size="sm" onClick={() => removeMember(m.name)}>
                      <X className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                ))}
              </div>
            )}

            <div className="flex flex-col gap-2 sm:flex-row">
              <select
                className="field-input font-mono"
                value={selectedAdd}
                onChange={(e) => setSelectedAdd(e.target.value)}
              >
                <option value="">Select a provider…</option>
                {candidates.map((c) => (
                  <option key={c.name} value={c.name}>
                    {c.name} ({c.type})
                  </option>
                ))}
              </select>
              <Button type="button" variant="secondary" onClick={addMember} disabled={candidates.length === 0}>
                <Plus className="h-3.5 w-3.5" />
                Add
              </Button>
            </div>
            {candidates.length === 0 && form.members.length > 0 && (
              <p className="text-[11px] text-muted-foreground">
                All providers of type <span className="font-mono">{lockedType}</span> are already members.
              </p>
            )}
          </div>

          {error ? <p className="text-sm text-warning">{error}</p> : null}
        </div>

        <DialogFooter>
          <Button variant="secondary" onClick={onClose}>
            <X className="h-4 w-4" />
            Cancel
          </Button>
          <Button onClick={onSubmit}>
            {form.mode === "edit" ? "Save changes" : "Create pool"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/* ------------------------------------------------------------------ */
/*  Shared bits                                                        */
/* ------------------------------------------------------------------ */

function StatCard({
  icon: Icon,
  label,
  value,
  sub,
  accent,
  valueClass,
  dot,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string;
  sub: string;
  accent?: boolean;
  valueClass?: string;
  dot?: string;
}) {
  return (
    <div className="group relative border border-border/40 bg-surface p-3 transition-all duration-200 hover:border-accent/20">
      <div className="flex items-center gap-2 text-muted-foreground">
        <Icon className="h-3.5 w-3.5" />
        <span className="text-[10px] font-semibold uppercase tracking-wider">{label}</span>
      </div>
      <div className="mt-1.5 flex items-center gap-1.5">
        {dot && <span className={cn("h-2 w-2", dot)} />}
        <span className={cn("font-mono text-sm font-bold", valueClass ?? (accent ? "text-accent" : "text-foreground"))}>
          {value}
        </span>
      </div>
      <div className="mt-0.5 text-[10px] text-muted-foreground">{sub}</div>
    </div>
  );
}

function SummaryBadge({
  icon: Icon,
  label,
  value,
  highlight,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string;
  highlight?: boolean;
}) {
  return (
    <div className="inline-flex items-center gap-2 border border-border/40 bg-surface px-3 py-1.5 text-xs">
      <Icon className="h-3 w-3 text-muted-foreground" />
      <span className="text-muted-foreground">{label}</span>
      <span className={cn("font-semibold", highlight ? "text-success" : "text-foreground")}>{value}</span>
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

function Alert({ children, tone }: { children: string; tone: "warning" | "success" }): JSX.Element {
  return (
    <div className={cn("border px-4 py-3 text-sm", tone === "warning" ? "border-warning/30 bg-warning/15 text-warning" : "border-success/30 bg-success/15 text-success")}>
      {children}
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Provider Pools" subtitle="Load-balanced groups of providers." />
      <div className="flex flex-col items-center justify-center gap-4 py-20 text-sm text-muted-foreground">
        <Loader2 className="h-6 w-6 animate-spin text-accent" />
        <span>Loading pools…</span>
      </div>
    </div>
  );
}

function ErrorState({ message }: { message: string }) {
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Provider Pools" subtitle="Load-balanced groups of providers." />
      <div className="rounded-lg border border-destructive/25 bg-destructive/10 px-4 py-6 text-sm text-destructive">
        Failed to load pools: {message}
      </div>
    </div>
  );
}

function formatLatency(micros: number | undefined): string {
  const v = Number(micros || 0);
  if (v <= 0) return "-";
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(2)}s`;
  if (v >= 1_000) return `${Math.round(v / 1_000)}ms`;
  return `${Math.round(v)}us`;
}

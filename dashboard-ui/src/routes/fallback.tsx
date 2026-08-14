import * as React from "react";
import { ArrowDown, ArrowUp, AlertTriangle, CheckCircle2, Loader2, Plus, Save, Trash2 } from "lucide-react";
import { PageHeader } from "@/components/ui/page-header";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { EmptyState, Pill, Surface } from "@/components/ui/surface";
import { createCombo, deleteCombo, fetchCombos, updateCombo, type ComboPayload, type ComboView } from "@/lib/api/combos";
import { useModels } from "@/lib/api/useModels";
import { modelDisplayName } from "@/lib/api/models-types";
import { useDashboardConfig } from "@/lib/api/useDashboardConfig";
import { cn } from "@/lib/utils";

interface ChainForm {
  mode: "create" | "edit";
  originalName: string;
  name: string;
  description: string;
  models: string[];
  enabled: boolean;
}

export function FallbackPage(): JSX.Element {
  const config = useDashboardConfig();
  const fallback = config.data?.fallback;
  const rules = fallback?.manual_rules ?? [];

  const [combos, setCombos] = React.useState<ComboView[]>([]);
  const [loadingCombos, setLoadingCombos] = React.useState(true);
  const [comboError, setComboError] = React.useState("");
  const [notice, setNotice] = React.useState("");
  const [form, setForm] = React.useState<ChainForm | null>(null);
  const [saving, setSaving] = React.useState(false);

  const models = useModels();
  const modelOptions = React.useMemo(
    () => (models.data ?? []).map(modelDisplayName).filter(Boolean),
    [models.data],
  );

  const loadCombos = React.useCallback(async () => {
    try {
      setComboError("");
      setCombos(await fetchCombos());
    } catch (err) {
      setComboError(err instanceof Error ? err.message : "Unable to load fallback chains.");
    } finally {
      setLoadingCombos(false);
    }
  }, []);

  React.useEffect(() => {
    void loadCombos();
  }, [loadCombos]);

  const openCreate = () =>
    setForm({
      mode: "create",
      originalName: "",
      name: "",
      description: "",
      models: [],
      enabled: true,
    });

  const openEdit = (view: ComboView) =>
    setForm({
      mode: "edit",
      originalName: view.combo.name,
      name: view.combo.name,
      description: view.combo.description ?? "",
      models: [...view.combo.models],
      enabled: view.combo.enabled,
    });

  async function submit(): Promise<void> {
    if (!form) return;
    const payload: ComboPayload = {
      name: form.name.trim(),
      enabled: form.enabled,
      models: form.models,
      ...(form.description.trim() ? { description: form.description.trim() } : {}),
    };
    if (!payload.name || payload.models.length < 2) {
      setComboError("Chain name and at least two models are required.");
      return;
    }
    setSaving(true);
    try {
      setComboError("");
      if (form.mode === "edit") {
        await updateCombo(form.originalName, payload);
      } else {
        await createCombo(payload);
      }
      setForm(null);
      setNotice(form.mode === "edit" ? "Chain updated." : "Chain created.");
      await loadCombos();
    } catch (err) {
      setComboError(err instanceof Error ? err.message : "Unable to save chain.");
    } finally {
      setSaving(false);
    }
  }

  async function toggleEnabled(view: ComboView): Promise<void> {
    const payload: ComboPayload = {
      name: view.combo.name,
      models: view.combo.models,
      enabled: !view.combo.enabled,
      ...(view.combo.description ? { description: view.combo.description } : {}),
    };
    try {
      setComboError("");
      await updateCombo(view.combo.name, payload);
      setNotice(view.combo.enabled ? "Chain disabled." : "Chain enabled.");
      await loadCombos();
    } catch (err) {
      setComboError(err instanceof Error ? err.message : "Unable to toggle chain.");
    }
  }

  async function remove(view: ComboView): Promise<void> {
    if (!window.confirm(`Delete chain "${view.combo.name}"?`)) return;
    try {
      setComboError("");
      await deleteCombo(view.combo.name);
      setNotice("Chain deleted.");
      await loadCombos();
    } catch (err) {
      setComboError(err instanceof Error ? err.message : "Unable to delete chain.");
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Fallback"
        subtitle="View config-defined fallback rules and manage fallback chains that route requests through a primary model with automatic failover."
      />

      {comboError ? (
        <Banner tone="warning">{comboError}</Banner>
      ) : null}
      {notice ? (
        <Banner tone="success">{notice}</Banner>
      ) : null}

      {/* ── Section 1: Config Rules ──────────────────────────────────── */}
      <section className="flex flex-col gap-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="font-display text-lg font-bold tracking-tight text-foreground">
              Config Rules
            </h3>
            <p className="mt-1 text-[13px] text-muted-foreground">
              Read-only rules loaded from fallback.json configuration.
            </p>
          </div>
        </div>

        {config.isLoading ? (
          <Surface className="flex items-center gap-2 p-6 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading config…
          </Surface>
        ) : config.isError ? (
          <Banner tone="warning">
            {config.error instanceof Error ? config.error.message : "Unable to load dashboard config."}
          </Banner>
        ) : (
          <>
            <Surface className="p-4 text-sm text-muted-foreground">
              When a request targets a fallback chain, the first model acts as
              primary. If it fails, Aurora retries the next model in the list,
              and so on, until a model succeeds or the chain is exhausted.
            </Surface>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="border border-border bg-surface p-5 flex flex-col gap-3">
                <div className="text-[10px] font-bold tracking-widest uppercase text-muted-foreground">
                  Fallback Mode
                </div>
                <div className="font-mono text-sm text-foreground">
                  {fallback?.mode ?? "not configured"}
                </div>
              </div>

              <div className="border border-border bg-surface p-5 flex flex-col gap-3">
                <div className="text-[10px] font-bold tracking-widest uppercase text-muted-foreground">
                  Manual Rules
                </div>
                <div className="flex items-center gap-2">
                  <Pill tone={fallback?.manual_rules_configured ? "success" : "muted"}>
                    {fallback?.manual_rules_configured ? "Configured" : "Not configured"}
                  </Pill>
                  <span className="text-xs text-muted-foreground">
                    {rules.length} rule{rules.length !== 1 ? "s" : ""}
                  </span>
                </div>
              </div>
            </div>

            {rules.length === 0 ? (
              <EmptyState
                title="No fallback rules configured"
                description="Add manual fallback rules to the fallback configuration to define ordered model chains with automatic failover."
              />
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {rules.map((rule, idx) => (
                  <RuleCard
                    key={idx}
                    source={rule.source ?? "unknown"}
                    targets={rule.targets ?? []}
                  />
                ))}
              </div>
            )}
          </>
        )}
      </section>

      {/* ── Section 2: Fallback Chains (combos) ──────────────────────── */}
      <section className="flex flex-col gap-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="font-display text-lg font-bold tracking-tight text-foreground">
              Fallback Chains
            </h3>
            <p className="mt-1 text-[13px] text-muted-foreground">
              Create and manage fallback chains that appear as selectable models.
            </p>
          </div>
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4" />
            Create Chain
          </Button>
        </div>

        {loadingCombos ? (
          <Surface className="flex items-center gap-2 p-6 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading chains…
          </Surface>
        ) : combos.length === 0 ? (
          <EmptyState
            title="No fallback chains"
            description="Create a fallback chain to expose an ordered model chain as a selectable model."
            action={<Button onClick={openCreate}><Plus className="h-4 w-4" />Create Chain</Button>}
          />
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {combos.map((view) => (
              <ChainCard
                key={view.combo.id || view.combo.name}
                view={view}
                onEdit={() => openEdit(view)}
                onToggle={() => void toggleEnabled(view)}
                onDelete={() => void remove(view)}
              />
            ))}
          </div>
        )}
      </section>

      {/* ── Dialog ───────────────────────────────────────────────────── */}
      <ChainDialog
        form={form}
        saving={saving}
        error={comboError}
        modelOptions={modelOptions}
        onChange={setForm}
        onClose={() => setForm(null)}
        onSubmit={() => void submit()}
      />
    </div>
  );
}

/* ── Rule Card (read-only from config) ─────────────────────────────── */

function RuleCard({
  source,
  targets,
}: {
  source: string;
  targets: string[];
}): JSX.Element {
  return (
    <div className="border border-border bg-surface p-5 flex flex-col gap-4">
      <div className="flex items-center gap-2 flex-wrap">
        <Pill tone="accent">source</Pill>
        <h3 className="font-mono text-sm font-semibold text-foreground truncate">
          {source}
        </h3>
      </div>

      {targets.length > 0 ? (
        <div className="border border-border/60 bg-background/40 p-3">
          <div className="text-[10px] font-bold tracking-widest uppercase text-muted-foreground mb-2">
            Fallback Chain
          </div>
          <div className="flex flex-col">
            {targets.map((target, idx) => (
              <React.Fragment key={`${target}-${idx}`}>
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground w-5 text-right shrink-0">
                    {idx + 1}.
                  </span>
                  <span className="font-mono text-sm text-foreground truncate">
                    {target}
                  </span>
                  <Pill tone={idx === 0 ? "accent" : "muted"} className="ml-auto shrink-0">
                    {idx === 0 ? "primary" : `fallback ${idx}`}
                  </Pill>
                </div>
                {idx < targets.length - 1 ? (
                  <div className="flex items-center justify-center py-0.5">
                    <ArrowDown className="h-3 w-3 text-muted-foreground" />
                  </div>
                ) : null}
              </React.Fragment>
            ))}
          </div>
        </div>
      ) : (
        <div className="text-xs text-muted-foreground flex items-center gap-1.5">
          <AlertTriangle className="h-3 w-3" />
          No targets configured
        </div>
      )}
    </div>
  );
}

/* ── Chain Card (writable combo) ───────────────────────────────────── */

function ChainCard({
  view,
  onEdit,
  onToggle,
  onDelete,
}: {
  view: ComboView;
  onEdit: () => void;
  onToggle: () => void;
  onDelete: () => void;
}): JSX.Element {
  return (
    <div className="border border-border bg-surface p-5 flex flex-col gap-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <h3 className="font-mono text-sm font-semibold text-foreground truncate">
              {view.combo.name}
            </h3>
            <Pill tone={view.combo.enabled ? "success" : "warning"}>
              {view.combo.enabled ? "Enabled" : "Disabled"}
            </Pill>
          </div>
          {view.combo.description ? (
            <p className="mt-1 text-xs text-muted-foreground">
              {view.combo.description}
            </p>
          ) : null}
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <Button variant="ghost" size="icon" onClick={onEdit} title="Edit chain">
            <Save className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={onToggle}
            title={view.combo.enabled ? "Disable chain" : "Enable chain"}
          >
            {view.combo.enabled ? (
              <ArrowDown className="h-4 w-4 text-warning" />
            ) : (
              <ArrowUp className="h-4 w-4 text-success" />
            )}
          </Button>
          <Button variant="ghost" size="icon" onClick={onDelete} title="Delete chain">
            <Trash2 className="h-4 w-4 text-destructive" />
          </Button>
        </div>
      </div>

      {/* Model chain */}
      <div className="border border-border/60 bg-background/40 p-3">
        <div className="text-[10px] font-bold tracking-widest uppercase text-muted-foreground mb-2">
          Model Chain
        </div>
        <div className="flex flex-col">
          {view.combo.models.map((model, idx) => (
            <React.Fragment key={`${model}-${idx}`}>
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground w-5 text-right shrink-0">
                  {idx + 1}.
                </span>
                <span className="font-mono text-sm text-foreground truncate">
                  {model}
                </span>
                <Pill tone={idx === 0 ? "accent" : "muted"} className="ml-auto shrink-0">
                  {idx === 0 ? "primary" : `fallback ${idx}`}
                </Pill>
              </div>
              {idx < view.combo.models.length - 1 ? (
                <div className="flex items-center justify-center py-0.5">
                  <ArrowDown className="h-3 w-3 text-muted-foreground" />
                </div>
              ) : null}
            </React.Fragment>
          ))}
        </div>
      </div>

      {/* Validation */}
      {view.valid ? (
        <div className="flex items-center gap-1.5 text-xs text-success">
          <CheckCircle2 className="h-3 w-3" />
          Valid chain
        </div>
      ) : (
        <div className="flex flex-col gap-1">
          {view.errors?.map((e) => (
            <div key={e} className="flex items-center gap-1.5 text-xs text-warning">
              <AlertTriangle className="h-3 w-3" />
              {e}
            </div>
          ))}
          {view.warnings?.map((w) => (
            <div key={w} className="flex items-center gap-1.5 text-xs text-muted-foreground">
              {w}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

/* ── Chain Dialog ──────────────────────────────────────────────────── */

function ChainDialog({
  form,
  saving,
  error,
  modelOptions,
  onChange,
  onClose,
  onSubmit,
}: {
  form: ChainForm | null;
  saving: boolean;
  error: string;
  modelOptions: string[];
  onChange: (form: ChainForm | null) => void;
  onClose: () => void;
  onSubmit: () => void;
}): JSX.Element {
  const [selectedModel, setSelectedModel] = React.useState(modelOptions[0] ?? "");

  React.useEffect(() => {
    setSelectedModel(modelOptions[0] ?? "");
  }, [modelOptions]);

  if (!form) {
    return <Dialog open={false} onOpenChange={() => undefined} />;
  }

  const addModel = () => {
    const model = selectedModel.trim();
    if (!model || form.models.includes(model)) return;
    onChange({ ...form, models: [...form.models, model] });
  };

  const removeModel = (model: string) => {
    onChange({ ...form, models: form.models.filter((m) => m !== model) });
  };

  const moveModel = (index: number, direction: -1 | 1) => {
    const newIndex = index + direction;
    if (newIndex < 0 || newIndex >= form.models.length) return;
    const next = [...form.models];
    const tmp = next[index] as string;
    next[index] = next[newIndex] as string;
    next[newIndex] = tmp;
    onChange({ ...form, models: next });
  };

  return (
    <Dialog open={Boolean(form)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {form.mode === "edit" ? "Edit Fallback Chain" : "Create Fallback Chain"}
          </DialogTitle>
          <DialogDescription>
            Select models from the live registry. The first model is primary; subsequent models are fallbacks.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          {/* Name */}
          <label className="block space-y-2">
            <span className="text-xs font-medium text-muted-foreground">Name</span>
            <input
              className="field-input font-mono"
              value={form.name}
              onChange={(e) => onChange({ ...form, name: e.target.value })}
              placeholder="my-fallback-chain"
            />
          </label>

          {/* Description */}
          <label className="block space-y-2">
            <span className="text-xs font-medium text-muted-foreground">Description</span>
            <input
              className="field-input"
              value={form.description}
              onChange={(e) => onChange({ ...form, description: e.target.value })}
              placeholder="Optional description"
            />
          </label>

          {/* Add model */}
          <label className="block space-y-2">
            <span className="text-xs font-medium text-muted-foreground">Add model</span>
            <div className="flex gap-2">
              <select
                className="field-input font-mono flex-1"
                value={selectedModel}
                onChange={(e) => setSelectedModel(e.target.value)}
              >
                {modelOptions.map((model) => (
                  <option key={model} value={model}>
                    {model}
                  </option>
                ))}
              </select>
              <Button type="button" variant="secondary" onClick={addModel}>
                Add
              </Button>
            </div>
          </label>

          {/* Model chain list */}
          <div className="space-y-2">
            <span className="text-xs font-medium text-muted-foreground">
              Fallback chain ({form.models.length} model{form.models.length !== 1 ? "s" : ""})
            </span>
            {form.models.length === 0 ? (
              <div className="border border-border bg-background/35 p-3 text-sm text-muted-foreground">
                No models added yet. Select a model above and click Add.
              </div>
            ) : (
              <div className="flex flex-col border border-border bg-background/35 divide-y divide-border/40">
                {form.models.map((model, idx) => (
                  <div
                    key={model}
                    className="flex items-center justify-between px-3 py-2 gap-2"
                  >
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <div className="flex flex-col gap-0.5 shrink-0">
                        <button
                          type="button"
                          disabled={idx === 0}
                          onClick={() => moveModel(idx, -1)}
                          className={cn(
                            "text-muted-foreground hover:text-foreground disabled:opacity-30 disabled:cursor-not-allowed",
                          )}
                          title="Move up"
                        >
                          <ArrowUp className="h-3 w-3" />
                        </button>
                        <button
                          type="button"
                          disabled={idx === form.models.length - 1}
                          onClick={() => moveModel(idx, 1)}
                          className={cn(
                            "text-muted-foreground hover:text-foreground disabled:opacity-30 disabled:cursor-not-allowed",
                          )}
                          title="Move down"
                        >
                          <ArrowDown className="h-3 w-3" />
                        </button>
                      </div>
                      <span className="font-mono text-sm truncate">
                        <span className="text-muted-foreground">
                          {idx === 0 ? "primary" : `fallback ${idx}`}:
                        </span>{" "}
                        {model}
                      </span>
                    </div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      onClick={() => removeModel(model)}
                      title="Remove model"
                    >
                      <Trash2 className="h-3.5 w-3.5 text-destructive" />
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Enabled toggle */}
          <label className="flex items-center gap-2 text-sm cursor-pointer select-none">
            <input
              type="checkbox"
              checked={form.enabled}
              onChange={(e) => onChange({ ...form, enabled: e.target.checked })}
              className="h-4 w-4 accent-[var(--accent)]"
            />
            Enabled
          </label>
        </div>

        {error ? <p className="text-sm text-warning">{error}</p> : null}

        <DialogFooter>
          <Button variant="secondary" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button onClick={onSubmit} disabled={saving}>
            {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            {form.mode === "edit" ? "Save Changes" : "Create Chain"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/* ── Helpers ────────────────────────────────────────────────────────── */

function Banner({
  children,
  tone,
}: {
  children: string;
  tone: "warning" | "success";
}): JSX.Element {
  return (
    <div
      className={cn(
        "border px-4 py-3 text-sm",
        tone === "warning"
          ? "border-warning/30 bg-warning/15 text-warning"
          : "border-success/30 bg-success/15 text-success",
      )}
    >
      {children}
    </div>
  );
}

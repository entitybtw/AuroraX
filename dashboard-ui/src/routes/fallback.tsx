import * as React from "react";
import { ArrowDown, ArrowUp, AlertTriangle, CheckCircle2, Loader2, Plus, Save, Trash2, ToggleLeft, ToggleRight } from "lucide-react";
import { PageHeader } from "@/components/ui/page-header";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { EmptyState, Pill, Surface } from "@/components/ui/surface";
import { createCombo, deleteCombo, fetchCombos, updateCombo, type ComboPayload, type ComboView } from "@/lib/api/combos";
import { useModels } from "@/lib/api/useModels";
import { modelDisplayName } from "@/lib/api/models-types";
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
  const [combos, setCombos] = React.useState<ComboView[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [form, setForm] = React.useState<ChainForm | null>(null);
  const [error, setError] = React.useState("");
  const [notice, setNotice] = React.useState("");
  const models = useModels();
  const modelOptions = React.useMemo(
    () => (models.data ?? []).map(modelDisplayName).filter(Boolean),
    [models.data],
  );

  const load = React.useCallback(async () => {
    try {
      setError("");
      setCombos(await fetchCombos());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load fallback chains.");
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  const openCreate = (): void =>
    setForm({
      mode: "create",
      originalName: "",
      name: "",
      description: "",
      models: [],
      enabled: true,
    });

  const openEdit = (view: ComboView): void =>
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
    };
    if (form.description.trim()) payload.description = form.description.trim();
    if (!payload.name || payload.models.length < 2) {
      setError("Chain name and at least two models are required.");
      return;
    }
    try {
      setError("");
      if (form.mode === "edit") {
        await updateCombo(form.originalName, payload);
      } else {
        await createCombo(payload);
      }
      setForm(null);
      setNotice(form.mode === "edit" ? "Fallback chain saved." : "Fallback chain created.");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to save fallback chain.");
    }
  }

  async function remove(view: ComboView): Promise<void> {
    if (!window.confirm(`Delete fallback chain ${view.combo.name}?`)) return;
    try {
      setError("");
      await deleteCombo(view.combo.name);
      setNotice("Fallback chain deleted.");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to delete fallback chain.");
    }
  }

  async function toggleEnabled(view: ComboView): Promise<void> {
    try {
      setError("");
      const payload: ComboPayload = {
        name: view.combo.name,
        description: view.combo.description ?? "",
        models: view.combo.models,
        enabled: !view.combo.enabled,
      };
      await updateCombo(view.combo.name, payload);
      setNotice(view.combo.enabled ? "Chain disabled." : "Chain enabled.");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to toggle chain.");
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Fallback Chains"
        subtitle="Define ordered model fallback chains that route requests through a primary model with automatic failover."
        actions={
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4" />
            Create Chain
          </Button>
        }
      />

      {error ? (
        <Banner tone="warning">{error}</Banner>
      ) : null}
      {notice ? (
        <Banner tone="success">{notice}</Banner>
      ) : null}

      <Surface className="p-4 text-sm text-muted-foreground">
        When a request targets a fallback chain name, the first model acts as
        primary. If it fails, Aurora retries the next model in the list, and so
        on, until a model succeeds or the chain is exhausted.
      </Surface>

      {loading ? (
        <Surface className="flex items-center gap-2 p-6 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading fallback chains...
        </Surface>
      ) : combos.length === 0 ? (
        <EmptyState
          title="No fallback chains configured"
          description="Create a chain to expose an ordered set of models as a single selectable name with automatic failover."
          action={
            <Button onClick={openCreate}>
              <Plus className="h-4 w-4" />
              Create Chain
            </Button>
          }
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

      <ChainDialog
        form={form}
        error={error}
        modelOptions={modelOptions}
        onChange={setForm}
        onClose={() => setForm(null)}
        onSubmit={() => void submit()}
      />
    </div>
  );
}

/* ── Chain Card ─────────────────────────────────────────────────────── */

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
  const { combo, valid, errors, warnings } = view;

  return (
    <div className="border border-border bg-surface p-5 flex flex-col gap-4">
      {/* Header row */}
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <Pill tone={combo.enabled ? "success" : "warning"}>
              {combo.enabled ? "Enabled" : "Disabled"}
            </Pill>
            <h3 className="font-mono text-sm font-semibold text-foreground truncate">
              {combo.name}
            </h3>
          </div>
          {combo.description ? (
            <p className="mt-1.5 text-xs text-muted-foreground leading-relaxed">
              {combo.description}
            </p>
          ) : null}
        </div>
      </div>

      {/* Model chain */}
      <div className="border border-border/60 bg-background/40 p-3">
        <div className="text-[10px] font-bold tracking-widest uppercase text-muted-foreground mb-2">
          Model Chain
        </div>
        <div className="flex flex-col">
          {combo.models.map((model, idx) => (
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
              {idx < combo.models.length - 1 ? (
                <div className="flex items-center justify-center py-0.5">
                  <ArrowDown className="h-3 w-3 text-muted-foreground" />
                </div>
              ) : null}
            </React.Fragment>
          ))}
        </div>
      </div>

      {/* Validation */}
      <div className="flex flex-col gap-1">
        {valid ? (
          <div className="flex items-center gap-1.5 text-xs text-success">
            <CheckCircle2 className="h-3 w-3" />
            Valid chain
          </div>
        ) : (
          <div className="flex items-center gap-1.5 text-xs text-warning">
            <AlertTriangle className="h-3 w-3" />
            Validation issues
          </div>
        )}
        {errors?.map((e) => (
          <div key={e} className="text-xs text-destructive pl-5">
            {e}
          </div>
        ))}
        {warnings?.map((w) => (
          <div key={w} className="text-xs text-muted-foreground pl-5">
            {w}
          </div>
        ))}
      </div>

      {/* Actions */}
      <div className="flex items-center gap-2 pt-2 border-t border-border/40">
        <Button variant="ghost" size="sm" onClick={onEdit} disabled={view.readonly}>
          Edit
        </Button>
        <Button variant="ghost" size="sm" onClick={onToggle} disabled={view.readonly}>
          {combo.enabled ? (
            <ToggleRight className="h-4 w-4 text-success" />
          ) : (
            <ToggleLeft className="h-4 w-4 text-muted-foreground" />
          )}
          {combo.enabled ? "Disable" : "Enable"}
        </Button>
        <div className="flex-1" />
        <Button variant="ghost" size="icon" onClick={onDelete} disabled={view.readonly}>
          <Trash2 className="h-4 w-4 text-destructive" />
        </Button>
      </div>
    </div>
  );
}

/* ── Chain Dialog ───────────────────────────────────────────────────── */

function ChainDialog({
  form,
  error,
  modelOptions,
  onChange,
  onClose,
  onSubmit,
}: {
  form: ChainForm | null;
  error: string;
  modelOptions: string[];
  onChange: (form: ChainForm | null) => void;
  onClose: () => void;
  onSubmit: () => void;
}): JSX.Element {
  const [selectedModel, setSelectedModel] = React.useState("");

  React.useEffect(() => {
    if (modelOptions.length > 0) {
      setSelectedModel((prev) => (prev && modelOptions.includes(prev) ? prev : modelOptions[0]!));
    }
  }, [modelOptions]);

  if (!form) return <Dialog open={false} onOpenChange={() => undefined} />;

  const addModel = (): void => {
    const model = selectedModel.trim();
    if (!model || form.models.includes(model)) return;
    onChange({ ...form, models: [...form.models, model] });
  };

  const removeModel = (model: string): void => {
    onChange({ ...form, models: form.models.filter((m) => m !== model) });
  };

  const moveModel = (index: number, direction: "up" | "down"): void => {
    const target = direction === "up" ? index - 1 : index + 1;
    if (target < 0 || target >= form.models.length) return;
    const next = [...form.models];
    const temp = next[index]!;
    next[index] = next[target]!;
    next[target] = temp;
    onChange({ ...form, models: next });
  };

  return (
    <Dialog open={Boolean(form)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {form.mode === "edit" ? "Edit fallback chain" : "Create fallback chain"}
          </DialogTitle>
          <DialogDescription>
            Define an ordered list of models. The first model is primary; subsequent
            models are used as automatic fallbacks on failure.
          </DialogDescription>
        </DialogHeader>

        {/* Name */}
        <Field label="Name">
          <input
            className="field-input font-mono"
            placeholder="e.g. fast-reliable"
            value={form.name}
            onChange={(e) => onChange({ ...form, name: e.target.value })}
          />
        </Field>

        {/* Description */}
        <Field label="Description">
          <input
            className="field-input"
            placeholder="Optional description"
            value={form.description}
            onChange={(e) => onChange({ ...form, description: e.target.value })}
          />
        </Field>

        {/* Add model */}
        <Field label="Add model to chain">
          <div className="flex gap-2">
            <select
              className="field-input font-mono"
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
        </Field>

        {/* Model chain builder */}
        <div className="space-y-2">
          <span className="text-xs font-medium text-muted-foreground">
            Fallback chain
          </span>
          {form.models.length === 0 ? (
            <div className="border border-border bg-background/35 p-3 text-sm text-muted-foreground">
              No models selected. Add at least two models.
            </div>
          ) : (
            <div className="flex flex-col">
              {form.models.map((model, idx) => (
                <React.Fragment key={`${model}-${idx}`}>
                  <div className="flex items-center gap-2 border border-border bg-background/35 px-3 py-2">
                    <span className="text-xs text-muted-foreground w-5 text-right shrink-0">
                      {idx + 1}.
                    </span>
                    <span className="font-mono text-sm text-foreground truncate flex-1">
                      {model}
                    </span>
                    <Pill tone={idx === 0 ? "accent" : "muted"} className="shrink-0">
                      {idx === 0 ? "primary" : `fallback ${idx}`}
                    </Pill>
                    <div className="flex items-center gap-0.5 shrink-0">
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        onClick={() => moveModel(idx, "up")}
                        disabled={idx === 0}
                      >
                        <ArrowUp className="h-3 w-3" />
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        onClick={() => moveModel(idx, "down")}
                        disabled={idx === form.models.length - 1}
                      >
                        <ArrowDown className="h-3 w-3" />
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        onClick={() => removeModel(model)}
                      >
                        <Trash2 className="h-3 w-3 text-destructive" />
                      </Button>
                    </div>
                  </div>
                  {idx < form.models.length - 1 ? (
                    <div className="flex items-center justify-center py-0.5">
                      <ArrowDown className="h-3 w-3 text-muted-foreground" />
                    </div>
                  ) : null}
                </React.Fragment>
              ))}
            </div>
          )}
        </div>

        {/* Enabled toggle */}
        <label className="flex items-center gap-2 text-sm cursor-pointer select-none">
          <button
            type="button"
            role="switch"
            aria-checked={form.enabled}
            className={cn(
              "relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors",
              form.enabled ? "bg-success" : "bg-muted",
            )}
            onClick={() => onChange({ ...form, enabled: !form.enabled })}
          >
            <span
              className={cn(
                "pointer-events-none inline-block h-4 w-4 rounded-full bg-foreground shadow-lg ring-0 transition-transform",
                form.enabled ? "translate-x-4" : "translate-x-0",
              )}
            />
          </button>
          Enabled
        </label>

        {/* Validation feedback */}
        {form.models.length > 0 && form.models.length < 2 ? (
          <p className="text-xs text-warning">
            A chain requires at least two models.
          </p>
        ) : null}

        {error ? <p className="text-sm text-warning">{error}</p> : null}

        <DialogFooter>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={onSubmit}>
            <Save className="h-4 w-4" />
            {form.mode === "edit" ? "Save Changes" : "Create Chain"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/* ── Helpers ────────────────────────────────────────────────────────── */

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}): JSX.Element {
  return (
    <label className="block space-y-2">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      {children}
    </label>
  );
}

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

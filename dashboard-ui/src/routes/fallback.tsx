import * as React from "react";
import {
  ArrowDown,
  ArrowUp,
  Edit3,
  Loader2,
  Plus,
  Save,
  Trash2,
} from "lucide-react";
import { PageHeader } from "@/components/ui/page-header";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { EmptyState, Pill, Surface } from "@/components/ui/surface";
import {
  fetchFallback,
  updateFallback,
  type FallbackRule,
} from "@/lib/api/fallback";
import { useModels } from "@/lib/api/useModels";
import { modelDisplayName } from "@/lib/api/models-types";
import { cn } from "@/lib/utils";

/* ── Poll interval (ms) ────────────────────────────────────────────── */
const POLL_MS = 10_000;

/* ── Banner helper ─────────────────────────────────────────────────── */

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

/* ── Info banner ───────────────────────────────────────────────────── */

function InfoBanner(): JSX.Element {
  return (
    <div className="border border-accent/30 bg-accent/10 px-4 py-3 text-sm text-accent">
      Rules are synced with <code className="font-mono">configs/fallback.json</code>.
      Changes take effect immediately.
    </div>
  );
}

/* ── Dialog form state ─────────────────────────────────────────────── */

interface DialogForm {
  source: string;
  targets: string[];
}

/* ── Main page ─────────────────────────────────────────────────────── */

export function FallbackPage(): JSX.Element {
  const [rules, setRules] = React.useState<FallbackRule[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");
  const [notice, setNotice] = React.useState("");
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [editIndex, setEditIndex] = React.useState<number | null>(null);
  const [form, setForm] = React.useState<DialogForm>({ source: "", targets: [] });
  const [saving, setSaving] = React.useState(false);

  const models = useModels();
  const modelOptions = React.useMemo(
    () => (models.data ?? []).map(modelDisplayName).filter(Boolean),
    [models.data],
  );

  /* ── Load rules ───────────────────────────────────────────────────── */

  const loadRules = React.useCallback(async () => {
    try {
      setError("");
      setRules(await fetchFallback());
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Unable to load fallback rules.",
      );
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void loadRules();
  }, [loadRules]);

  /* ── Auto-refresh ─────────────────────────────────────────────────── */

  React.useEffect(() => {
    const id = setInterval(() => {
      void loadRules();
    }, POLL_MS);
    return () => clearInterval(id);
  }, [loadRules]);

  /* ── Dialog helpers ───────────────────────────────────────────────── */

  const openCreate = () => {
    setEditIndex(null);
    setForm({ source: "", targets: [] });
    setDialogOpen(true);
  };

  const openEdit = (index: number) => {
    const rule = rules[index];
    if (!rule) return;
    setEditIndex(index);
    setForm({ source: rule.source, targets: [...rule.targets] });
    setDialogOpen(true);
  };

  const closeDialog = () => {
    setDialogOpen(false);
    setEditIndex(null);
  };

  /* ── Save all rules ───────────────────────────────────────────────── */

  const saveRule = async () => {
    const source = form.source.trim();
    if (!source || form.targets.length === 0) {
      setError("Source model and at least one target are required.");
      return;
    }
    const rule: FallbackRule = { source, targets: form.targets };

    setSaving(true);
    try {
      setError("");
      let next: FallbackRule[];
      if (editIndex !== null) {
        next = [...rules];
        next[editIndex] = rule;
      } else {
        next = [...rules, rule];
      }
      const updated = await updateFallback(next);
      setRules(updated);
      closeDialog();
      setNotice(editIndex !== null ? "Rule updated." : "Rule added.");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Unable to save rule.",
      );
    } finally {
      setSaving(false);
    }
  };

  /* ── Delete a rule ────────────────────────────────────────────────── */

  const deleteRule = async (index: number) => {
    const rule = rules[index];
    if (!rule) return;
    if (!window.confirm(`Delete fallback rule for "${rule.source}"?`)) return;

    try {
      setError("");
      const next = rules.filter((_, i) => i !== index);
      const updated = await updateFallback(next);
      setRules(updated);
      setNotice("Rule deleted.");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Unable to delete rule.",
      );
    }
  };

  /* ── Render ───────────────────────────────────────────────────────── */

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Fallback"
        subtitle="Configure real-time fallback routing between models."
        actions={
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4" />
            Add Rule
          </Button>
        }
      />

      <InfoBanner />

      {error ? <Banner tone="warning">{error}</Banner> : null}
      {notice ? <Banner tone="success">{notice}</Banner> : null}

      {loading ? (
        <Surface className="flex items-center gap-2 p-6 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading rules…
        </Surface>
      ) : rules.length === 0 ? (
        <EmptyState
          title="No fallback rules"
          description="Add a rule to configure fallback routing for a source model."
          action={
            <Button onClick={openCreate}>
              <Plus className="h-4 w-4" />
              Add Rule
            </Button>
          }
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {rules.map((rule, idx) => (
            <RuleCard
              key={`${rule.source}-${idx}`}
              rule={rule}
              onEdit={() => openEdit(idx)}
              onDelete={() => void deleteRule(idx)}
            />
          ))}
        </div>
      )}

      <RuleDialog
        open={dialogOpen}
        form={form}
        editIndex={editIndex}
        saving={saving}
        error={error}
        modelOptions={modelOptions}
        onChange={setForm}
        onClose={closeDialog}
        onSubmit={() => void saveRule()}
      />
    </div>
  );
}

/* ── Rule Card ─────────────────────────────────────────────────────── */

function RuleCard({
  rule,
  onEdit,
  onDelete,
}: {
  rule: FallbackRule;
  onEdit: () => void;
  onDelete: () => void;
}): JSX.Element {
  return (
    <Surface className="p-5 flex flex-col gap-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <h3 className="font-mono text-sm font-semibold text-foreground truncate">
              {rule.source}
            </h3>
            <Pill tone="accent">source</Pill>
          </div>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <Button
            variant="ghost"
            size="icon"
            onClick={onEdit}
            title="Edit rule"
          >
            <Edit3 className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={onDelete}
            title="Delete rule"
          >
            <Trash2 className="h-4 w-4 text-destructive" />
          </Button>
        </div>
      </div>

      <div className="border border-border/60 bg-background/40 p-3">
        <div className="text-[10px] font-bold tracking-widest uppercase text-muted-foreground mb-2">
          Target Chain
        </div>
        <div className="flex flex-col">
          {rule.targets.map((target, idx) => (
            <React.Fragment key={`${target}-${idx}`}>
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground w-5 text-right shrink-0">
                  {idx + 1}.
                </span>
                <span className="font-mono text-sm text-foreground truncate">
                  {target}
                </span>
                <Pill
                  tone={idx === 0 ? "success" : "muted"}
                  className="ml-auto shrink-0"
                >
                  {idx === 0 ? "primary" : `fallback ${idx}`}
                </Pill>
              </div>
              {idx < rule.targets.length - 1 ? (
                <div className="flex items-center justify-center py-0.5">
                  <ArrowDown className="h-3 w-3 text-muted-foreground" />
                </div>
              ) : null}
            </React.Fragment>
          ))}
        </div>
      </div>
    </Surface>
  );
}

/* ── Rule Dialog ───────────────────────────────────────────────────── */

function RuleDialog({
  open,
  form,
  editIndex,
  saving,
  error,
  modelOptions,
  onChange,
  onClose,
  onSubmit,
}: {
  open: boolean;
  form: DialogForm;
  editIndex: number | null;
  saving: boolean;
  error: string;
  modelOptions: string[];
  onChange: (form: DialogForm) => void;
  onClose: () => void;
  onSubmit: () => void;
}): JSX.Element {
  const [customTarget, setCustomTarget] = React.useState("");
  const [selectedTarget, setSelectedTarget] = React.useState(
    modelOptions[0] ?? "",
  );

  React.useEffect(() => {
    setSelectedTarget(modelOptions[0] ?? "");
  }, [modelOptions]);

  const addTarget = (value: string) => {
    const v = value.trim();
    if (!v || form.targets.includes(v)) return;
    onChange({ ...form, targets: [...form.targets, v] });
    setCustomTarget("");
  };

  const removeTarget = (index: number) => {
    onChange({
      ...form,
      targets: form.targets.filter((_, i) => i !== index),
    });
  };

  const moveTarget = (index: number, direction: -1 | 1) => {
    const newIndex = index + direction;
    if (newIndex < 0 || newIndex >= form.targets.length) return;
    const next = [...form.targets];
    const tmp = next[index] as string;
    next[index] = next[newIndex] as string;
    next[newIndex] = tmp;
    onChange({ ...form, targets: next });
  };

  const availableModels = modelOptions.filter(
    (m) => !form.targets.includes(m),
  );

  const isEdit = editIndex !== null;

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-2xl max-sm:mx-4">
        <DialogHeader>
          <DialogTitle>
            {isEdit ? "Edit Fallback Rule" : "Add Fallback Rule"}
          </DialogTitle>
          <DialogDescription>
            Define a source model and an ordered list of targets. When the
            source is unavailable, requests fall through to each target in order.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          <label className="block space-y-2">
            <span className="text-xs font-medium text-muted-foreground">
              Source model
            </span>
            <input
              className="field-input font-mono"
              value={form.source}
              onChange={(e) =>
                onChange({ ...form, source: e.target.value })
              }
              placeholder="e.g. openai/gpt-4o"
            />
          </label>

          <label className="block space-y-2">
            <span className="text-xs font-medium text-muted-foreground">
              Add target
            </span>
            <div className="flex gap-2">
              <select
                className="field-input font-mono flex-1"
                value={selectedTarget}
                onChange={(e) => setSelectedTarget(e.target.value)}
              >
                {availableModels.length === 0 ? (
                  <option value="">No available models</option>
                ) : (
                  availableModels.map((model) => (
                    <option key={model} value={model}>
                      {model}
                    </option>
                  ))
                )}
              </select>
              <Button
                type="button"
                variant="secondary"
                onClick={() => {
                  addTarget(selectedTarget);
                }}
                disabled={!selectedTarget.trim()}
              >
                Add
              </Button>
            </div>
            <div className="flex gap-2 mt-1">
              <input
                className="field-input font-mono flex-1"
                value={customTarget}
                onChange={(e) => setCustomTarget(e.target.value)}
                placeholder="Or type a custom model name…"
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    addTarget(customTarget);
                  }
                }}
              />
              <Button
                type="button"
                variant="secondary"
                onClick={() => addTarget(customTarget)}
                disabled={!customTarget.trim()}
              >
                Add
              </Button>
            </div>
          </label>

          <div className="space-y-2">
            <span className="text-xs font-medium text-muted-foreground">
              Target chain ({form.targets.length} model
              {form.targets.length !== 1 ? "s" : ""})
            </span>
            {form.targets.length === 0 ? (
              <div className="border border-border bg-background/35 p-3 text-sm text-muted-foreground">
                No targets added yet. Select or type a model above and click
                Add.
              </div>
            ) : (
              <div className="flex flex-col border border-border bg-background/35 divide-y divide-border/40">
                {form.targets.map((target, idx) => (
                  <div
                    key={`${target}-${idx}`}
                    className="flex items-center justify-between px-3 py-2 gap-2"
                  >
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <div className="flex flex-col gap-0.5 shrink-0">
                        <button
                          type="button"
                          disabled={idx === 0}
                          onClick={() => moveTarget(idx, -1)}
                          className={cn(
                            "text-muted-foreground hover:text-foreground disabled:opacity-30 disabled:cursor-not-allowed",
                          )}
                          title="Move up"
                        >
                          <ArrowUp className="h-3 w-3" />
                        </button>
                        <button
                          type="button"
                          disabled={idx === form.targets.length - 1}
                          onClick={() => moveTarget(idx, 1)}
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
                        {target}
                      </span>
                    </div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      onClick={() => removeTarget(idx)}
                      title="Remove target"
                    >
                      <Trash2 className="h-3.5 w-3.5 text-destructive" />
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {error ? <p className="text-sm text-warning">{error}</p> : null}

        <DialogFooter>
          <Button variant="secondary" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button onClick={onSubmit} disabled={saving}>
            {saving ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Save className="h-4 w-4" />
            )}
            {isEdit ? "Save Changes" : "Add Rule"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

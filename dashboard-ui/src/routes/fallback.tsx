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
import { Switch } from "@/components/ui/switch";
import {
  fetchFallback,
  updateFallback,
  type FallbackRule,
} from "@/lib/api/fallback";
import { useModels } from "@/lib/api/useModels";
import { modelDisplayName } from "@/lib/api/models-types";
import { cn } from "@/lib/utils";

const POLL_MS = 10_000;

function Banner({ children, tone }: { children: string; tone: "warning" | "success" }): JSX.Element {
  return (
    <div className={cn("border px-4 py-3 text-sm rounded-lg", tone === "warning" ? "border-warning/30 bg-warning/15 text-warning" : "border-success/30 bg-success/15 text-success")}>
      {children}
    </div>
  );
}

export function FallbackPage(): JSX.Element {
  const [rules, setRules] = React.useState<FallbackRule[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");
  const [notice, setNotice] = React.useState("");
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [editIdx, setEditIdx] = React.useState<number | null>(null);
  const [formSource, setFormSource] = React.useState("");
  const [formTargets, setFormTargets] = React.useState<string[]>([]);
  const [saving, setSaving] = React.useState(false);

  const models = useModels();
  const modelOptions = React.useMemo(() => (models.data ?? []).map(modelDisplayName).filter(Boolean), [models.data]);

  const loadRules = React.useCallback(async () => {
    try {
      setError("");
      const fetched = await fetchFallback();
      setRules(fetched);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load fallback rules.");
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => { void loadRules(); }, [loadRules]);
  React.useEffect(() => {
    const id = setInterval(() => { void loadRules(); }, POLL_MS);
    return () => clearInterval(id);
  }, [loadRules]);

  const openCreate = () => {
    setEditIdx(null);
    setFormSource("");
    setFormTargets([]);
    setDialogOpen(true);
  };

  const openEdit = (idx: number) => {
    const rule = rules[idx];
    if (!rule) return;
    setEditIdx(idx);
    setFormSource(rule.source);
    setFormTargets([...rule.targets]);
    setDialogOpen(true);
  };

  const closeDialog = () => {
    setDialogOpen(false);
    setEditIdx(null);
  };

  const saveRule = async () => {
    if (!formSource.trim() || formTargets.length === 0) {
      setError("Chain name and at least one target required.");
      return;
    }
    setSaving(true);
    try {
      setError("");
      const isEdit = editIdx !== null;
      const existing = isEdit ? rules[editIdx!] : undefined;
      const rule: FallbackRule = { source: formSource.trim(), targets: formTargets, enabled: existing?.enabled !== false };
      let next: FallbackRule[];
      if (isEdit) {
        next = [...rules];
        next[editIdx!] = rule;
      } else {
        next = [...rules, rule];
      }
      const updated = await updateFallback(next);
      setRules(updated);
      closeDialog();
      setNotice(isEdit ? "Rule updated." : "Rule added.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to save.");
    } finally {
      setSaving(false);
    }
  };

  const deleteRule = async (idx: number) => {
    const rule = rules[idx];
    if (!rule) return;
    if (!window.confirm(`Delete rule for "${rule.source}"?`)) return;
    setSaving(true);
    try {
      setError("");
      const updated = await updateFallback(rules.filter((_, i) => i !== idx));
      setRules(updated);
      setNotice("Rule deleted.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to delete.");
    } finally {
      setSaving(false);
    }
  };

  const toggleRule = async (idx: number) => {
    const rule = rules[idx];
    if (!rule) return;
    setSaving(true);
    try {
      setError("");
      const next = [...rules];
      next[idx] = { ...rule, enabled: !rule.enabled };
      const updated = await updateFallback(next);
      setRules(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to toggle rule.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex flex-col gap-4 md:gap-6">
      <PageHeader
        title="Fallback"
        subtitle="Configure fallback routing. Order from configs/fallback.json."
        actions={<Button onClick={openCreate}><Plus className="h-4 w-4" /> Add Rule</Button>}
      />

      {error ? <Banner tone="warning">{error}</Banner> : null}
      {notice ? <Banner tone="success">{notice}</Banner> : null}

      {loading ? (
        <Surface className="flex items-center gap-2 p-6 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" /> Loading rules…
        </Surface>
      ) : rules.length === 0 ? (
        <EmptyState title="No fallback rules" description="Add a rule to configure fallback routing." action={<Button onClick={openCreate}><Plus className="h-4 w-4" /> Add Rule</Button>} />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {rules.map((rule, idx) => (
            <Surface key={`${rule.source}-${idx}`} className={`p-4 flex flex-col gap-3 ${rule.enabled === false ? "opacity-60" : ""}`}>
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 flex-wrap">
                    <h3 className="font-mono text-sm font-semibold text-foreground truncate">{rule.source}</h3>
                    <Pill tone="accent">chain</Pill>
                    {rule.enabled === false && <Pill tone="muted">disabled</Pill>}
                  </div>
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  <Switch
                    checked={rule.enabled !== false}
                    size="sm"
                    disabled={saving}
                    onCheckedChange={() => void toggleRule(idx)}
                    aria-label={`Toggle rule for ${rule.source}`}
                    title={rule.enabled === false ? "Enable rule" : "Disable rule"}
                  />
                  <Button variant="ghost" size="icon" onClick={() => openEdit(idx)} title="Edit rule"><Edit3 className="h-4 w-4" /></Button>
                  <Button variant="ghost" size="icon" onClick={() => void deleteRule(idx)} title="Delete rule"><Trash2 className="h-4 w-4 text-destructive" /></Button>
                </div>
              </div>
              <div className="border border-border/60 bg-background/40 p-3">
                <div className="text-[10px] font-bold tracking-widest uppercase text-muted-foreground mb-2">Target Chain</div>
                <div className="flex flex-col">
                  {rule.targets.map((target, tIdx) => (
                    <React.Fragment key={`${target}-${tIdx}`}>
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-muted-foreground w-5 text-right shrink-0">{tIdx + 1}.</span>
                        <span className="font-mono text-sm text-foreground truncate">{target}</span>
                        <Pill tone={tIdx === 0 ? "success" : "muted"} className="ml-auto shrink-0">{tIdx === 0 ? "primary" : `fallback ${tIdx}`}</Pill>
                      </div>
                      {tIdx < rule.targets.length - 1 && <div className="flex items-center justify-center py-0.5"><ArrowDown className="h-3 w-3 text-muted-foreground" /></div>}
                    </React.Fragment>
                  ))}
                </div>
              </div>
            </Surface>
          ))}
        </div>
      )}

      <EditDialog
        open={dialogOpen}
        isEdit={editIdx !== null}
        source={formSource}
        targets={formTargets}
        modelOptions={modelOptions}
        saving={saving}
        error={error}
        onSourceChange={setFormSource}
        onTargetsChange={setFormTargets}
        onSave={() => void saveRule()}
        onClose={closeDialog}
      />
    </div>
  );
}

/* ── Edit Dialog ─────────────────────────────────────────────────── */

function EditDialog({
  open,
  isEdit,
  source,
  targets,
  modelOptions,
  saving,
  error,
  onSourceChange,
  onTargetsChange,
  onSave,
  onClose,
}: {
  open: boolean;
  isEdit: boolean;
  source: string;
  targets: string[];
  modelOptions: string[];
  saving: boolean;
  error: string;
  onSourceChange: (v: string) => void;
  onTargetsChange: (v: string[]) => void;
  onSave: () => void;
  onClose: () => void;
}): JSX.Element {
  const [targetInput, setTargetInput] = React.useState("");
  const available = modelOptions.filter((m) => !targets.includes(m));

  const addTarget = () => {
    const v = targetInput.trim();
    if (!v || targets.includes(v)) return;
    onTargetsChange([...targets, v]);
    setTargetInput("");
  };

  const removeTarget = (i: number) => onTargetsChange(targets.filter((_, idx) => idx !== i));

  const moveTarget = (i: number, dir: -1 | 1) => {
    const ni = i + dir;
    if (ni < 0 || ni >= targets.length) return;
    const next = [...targets];
    const value = next[i];
    next[i] = next[ni]!;
    next[ni] = value!;
    onTargetsChange(next);
  };

  const editTarget = (i: number, v: string) => {
    const next = [...targets];
    next[i] = v;
    onTargetsChange(next);
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="w-screen max-w-none sm:w-[calc(100vw-2rem)] sm:max-w-2xl h-screen max-h-none sm:h-auto sm:max-h-[90vh] overflow-y-auto overflow-x-hidden p-4 sm:p-6 rounded-none sm:rounded-xl">
        <DialogHeader>
          <DialogTitle className="text-lg">{isEdit ? "Edit Fallback Rule" : "Add Fallback Rule"}</DialogTitle>
          <DialogDescription>Define a chain name and ordered fallback targets. Clients call the chain by name.</DialogDescription>
        </DialogHeader>

        <div className="flex w-full min-w-0 flex-col gap-4">
          <label className="block w-full space-y-2">
            <span className="text-sm font-medium text-muted-foreground">Chain name</span>
            <input
              className="w-full h-12 sm:h-11 border border-border/50 bg-background/40 px-3 sm:px-4 font-mono text-base sm:text-sm focus:outline-none focus:border-accent/70 focus:ring-2 focus:ring-accent/15 rounded-lg"
              placeholder="e.g. deepseek-chain"
              value={source}
              onChange={(e) => onSourceChange(e.target.value)}
            />
            <div className="text-[12px] text-muted-foreground">Clients can call this fallback chain by name (e.g. <code>model="deepseek-chain"</code>).</div>
          </label>

          <label className="block w-full space-y-2">
            <span className="text-sm font-medium text-muted-foreground">Add target</span>
            <div className="flex min-w-0 w-full flex-col gap-2 sm:flex-row">
              <select className="min-w-0 flex-1 w-full h-12 sm:h-11 border border-border/50 bg-background/40 px-3 sm:px-4 font-mono text-base sm:text-sm focus:outline-none focus:border-accent/70 rounded-lg"
                value={targetInput} onChange={(e) => setTargetInput(e.target.value)}>
                <option value="">Select model…</option>
                {available.map((m) => <option key={m} value={m}>{m}</option>)}
              </select>
              <Button type="button" variant="secondary" onClick={addTarget} disabled={!targetInput.trim()} className="h-12 sm:h-11 px-5 shrink-0 w-full sm:w-auto">Add</Button>
            </div>
          </label>

          {targets.length > 0 && (
            <div className="w-full space-y-2">
              <span className="text-sm font-medium text-muted-foreground">Target chain ({targets.length})</span>
              <div className="border border-border/60 bg-background/40 divide-y divide-border/40 max-h-48 sm:max-h-60 overflow-y-auto rounded-lg">
                {targets.map((t, idx) => (
                  <div key={`${t}-${idx}`} className="flex min-w-0 w-full items-center gap-1.5 sm:gap-2 px-2 sm:px-3 py-2.5">
                    <div className="flex flex-col gap-0.5 shrink-0">
                      <button type="button" disabled={idx === 0} onClick={() => moveTarget(idx, -1)} className="p-0.5 text-muted-foreground hover:text-foreground disabled:opacity-30"><ArrowUp className="h-4 w-4" /></button>
                      <button type="button" disabled={idx === targets.length - 1} onClick={() => moveTarget(idx, 1)} className="p-0.5 text-muted-foreground hover:text-foreground disabled:opacity-30"><ArrowDown className="h-4 w-4" /></button>
                    </div>
                    <Pill tone={idx === 0 ? "success" : "muted"} className="shrink-0 text-[10px]">{idx === 0 ? "primary" : `fb ${idx}`}</Pill>
                    <select className="min-w-0 flex-1 h-10 sm:h-8 border border-border/40 bg-transparent px-2 font-mono text-sm sm:text-xs focus:outline-none focus:border-accent/70 rounded"
                      value={t} onChange={(e) => editTarget(idx, e.target.value)}>
                      {modelOptions.map((m) => <option key={m} value={m}>{m}</option>)}
                    </select>
                    <Button type="button" variant="ghost" size="icon" onClick={() => removeTarget(idx)} className="h-10 w-10 sm:h-8 sm:w-8 shrink-0"><Trash2 className="h-4 w-4 text-destructive" /></Button>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {error ? <p className="text-sm text-warning bg-warning/10 border border-warning/30 rounded-lg p-2">{error}</p> : null}

        <DialogFooter className="flex-col-reverse sm:flex-row gap-2 sm:justify-end pt-2">
          <Button variant="secondary" onClick={onClose} disabled={saving} className="w-full sm:w-auto flex-1 sm:flex-none h-12 sm:h-10">Cancel</Button>
          <Button onClick={onSave} disabled={saving || !source || targets.length === 0} className="w-full sm:w-auto flex-1 sm:flex-none h-12 sm:h-10">
            {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />} Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

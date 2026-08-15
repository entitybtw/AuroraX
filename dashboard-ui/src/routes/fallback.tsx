import * as React from "react";
import {
  ArrowDown,
  ArrowUp,
  Check,
  Edit3,
  Loader2,
  Plus,
  Trash2,
} from "lucide-react";
import { PageHeader } from "@/components/ui/page-header";
import { Button } from "@/components/ui/button";
import { EmptyState, Pill, Surface } from "@/components/ui/surface";
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
  const [creating, setCreating] = React.useState(false);
  const [saving, setSaving] = React.useState(false);

  const models = useModels();
  const modelOptions = React.useMemo(() => (models.data ?? []).map(modelDisplayName).filter(Boolean), [models.data]);

  const loadRules = React.useCallback(async () => {
    try {
      setError("");
      setRules(await fetchFallback());
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

  const saveAll = async (next: FallbackRule[]) => {
    setSaving(true);
    try {
      setError("");
      const updated = await updateFallback(next);
      setRules(updated);
      setNotice("Saved.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to save.");
    } finally {
      setSaving(false);
    }
  };

  const addRule = async (source: string, targets: string[]) => {
    await saveAll([...rules, { source, targets }]);
    setCreating(false);
  };

  const updateRule = async (index: number, rule: FallbackRule) => {
    const next = [...rules];
    next[index] = rule;
    await saveAll(next);
  };

  const deleteRule = async (index: number) => {
    const rule = rules[index];
    if (!rule) return;
    if (!window.confirm(`Delete fallback rule for "${rule.source}"?`)) return;
    await saveAll(rules.filter((_, i) => i !== index));
  };

  return (
    <div className="flex flex-col gap-4 md:gap-6">
      <PageHeader
        title="Fallback"
        subtitle="Configure fallback routing. Order is preserved from configs/fallback.json."
        actions={<Button onClick={() => setCreating(true)}><Plus className="h-4 w-4" /> Add Rule</Button>}
      />

      {error ? <Banner tone="warning">{error}</Banner> : null}
      {notice ? <Banner tone="success">{notice}</Banner> : null}

      {loading ? (
        <Surface className="flex items-center gap-2 p-6 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" /> Loading rules…
        </Surface>
      ) : rules.length === 0 && !creating ? (
        <EmptyState title="No fallback rules" description="Add a rule to configure fallback routing." action={<Button onClick={() => setCreating(true)}><Plus className="h-4 w-4" /> Add Rule</Button>} />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {creating && (
            <RuleCard editing source="" targets={[]} modelOptions={modelOptions} saving={saving}
              onSave={(s, t) => void addRule(s, t)} onCancel={() => setCreating(false)} />
          )}
          {rules.map((rule, idx) => (
            <RuleCard key={`${rule.source}-${idx}`} source={rule.source} targets={rule.targets} modelOptions={modelOptions} saving={saving}
              onSave={(s, t) => void updateRule(idx, { source: s, targets: t })} onDelete={() => void deleteRule(idx)} />
          ))}
        </div>
      )}
    </div>
  );
}

/* ── Rule Card ───────────────────────────────────────────────────── */

function RuleCard({
  source: initSource,
  targets: initTargets,
  modelOptions,
  saving,
  onSave,
  onDelete,
  onCancel,
  editing: startEditing,
}: {
  source: string;
  targets: string[];
  modelOptions: string[];
  saving: boolean;
  onSave: (source: string, targets: string[]) => void;
  onDelete?: () => void;
  onCancel?: () => void;
  editing?: boolean;
}): JSX.Element {
  const [editing, setEditing] = React.useState(startEditing ?? false);
  const [source, setSource] = React.useState(initSource);
  const [targets, setTargets] = React.useState([...initTargets]);
  const [targetInput, setTargetInput] = React.useState("");

  const addTarget = (v: string) => {
    const val = v.trim();
    if (!val || targets.includes(val)) return;
    setTargets((p) => [...p, val]);
    setTargetInput("");
  };

  const removeTarget = (i: number) => setTargets((p) => p.filter((_, idx) => idx !== i));

  const moveTarget = (i: number, dir: -1 | 1) => {
    const ni = i + dir;
    if (ni < 0 || ni >= targets.length) return;
    const next = [...targets];
    next[i] = next[ni]!;
    next[ni] = next[i]!;
    setTargets(next);
  };

  const editTarget = (i: number, value: string) => {
    const next = [...targets];
    next[i] = value;
    setTargets(next);
  };

  const availableModels = modelOptions.filter((m) => !targets.includes(m));

  if (editing) {
    return (
      <Surface className="p-4 flex flex-col gap-3">
        <div className="flex items-center gap-2">
          <span className="text-xs font-medium text-muted-foreground shrink-0 w-14">Source</span>
          <input className="flex-1 h-9 border border-border/50 bg-background/40 px-3 font-mono text-sm focus:outline-none focus:border-accent/70 focus:ring-2 focus:ring-accent/15"
            value={source} onChange={(e) => setSource(e.target.value)} placeholder="e.g. openai/gpt-4o" autoFocus />
        </div>

        <div className="flex items-center gap-2">
          <span className="text-xs font-medium text-muted-foreground shrink-0 w-14">Add</span>
          <select className="flex-1 h-9 border border-border/50 bg-background/40 px-3 font-mono text-sm focus:outline-none focus:border-accent/70"
            value={targetInput} onChange={(e) => setTargetInput(e.target.value)}>
            <option value="">Select model…</option>
            {availableModels.map((m) => <option key={m} value={m}>{m}</option>)}
          </select>
          <Button type="button" variant="secondary" size="sm" onClick={() => addTarget(targetInput)} disabled={!targetInput.trim()}>Add</Button>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs font-medium text-muted-foreground shrink-0 w-14">Or type</span>
          <input className="flex-1 h-9 border border-border/50 bg-background/40 px-3 font-mono text-sm focus:outline-none focus:border-accent/70"
            value={targetInput} onChange={(e) => setTargetInput(e.target.value)} placeholder="custom model name…"
            onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addTarget(targetInput); } }} />
        </div>

        {targets.length > 0 && (
          <div className="border border-border/60 bg-background/40 divide-y divide-border/40">
            {targets.map((t, idx) => (
              <div key={`${t}-${idx}`} className="flex items-center gap-2 px-3 py-2">
                <div className="flex flex-col gap-0.5 shrink-0">
                  <button type="button" disabled={idx === 0} onClick={() => moveTarget(idx, -1)} className="text-muted-foreground hover:text-foreground disabled:opacity-30"><ArrowUp className="h-3 w-3" /></button>
                  <button type="button" disabled={idx === targets.length - 1} onClick={() => moveTarget(idx, 1)} className="text-muted-foreground hover:text-foreground disabled:opacity-30"><ArrowDown className="h-3 w-3" /></button>
                </div>
                <Pill tone={idx === 0 ? "success" : "muted"} className="shrink-0 text-[10px]">{idx === 0 ? "primary" : `fb ${idx}`}</Pill>
                <input className="flex-1 h-7 border border-border/40 bg-transparent px-2 font-mono text-sm focus:outline-none focus:border-accent/70"
                  value={t} onChange={(e) => editTarget(idx, e.target.value)} />
                <Button type="button" variant="ghost" size="icon" onClick={() => removeTarget(idx)}><Trash2 className="h-3.5 w-3.5 text-destructive" /></Button>
              </div>
            ))}
          </div>
        )}

        <div className="flex gap-2 justify-end">
          {onCancel && <Button variant="secondary" size="sm" onClick={onCancel} disabled={saving}>Cancel</Button>}
          <Button size="sm" onClick={() => onSave(source, targets)} disabled={saving || !source.trim() || targets.length === 0}>
            {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Check className="h-3.5 w-3.5" />} Save
          </Button>
        </div>
      </Surface>
    );
  }

  return (
    <Surface className="p-4 flex flex-col gap-3">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <h3 className="font-mono text-sm font-semibold text-foreground truncate">{initSource}</h3>
            <Pill tone="accent">source</Pill>
          </div>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <Button variant="ghost" size="icon" onClick={() => setEditing(true)} title="Edit rule"><Edit3 className="h-4 w-4" /></Button>
          {onDelete && <Button variant="ghost" size="icon" onClick={onDelete} title="Delete rule"><Trash2 className="h-4 w-4 text-destructive" /></Button>}
        </div>
      </div>

      <div className="border border-border/60 bg-background/40 p-3">
        <div className="text-[10px] font-bold tracking-widest uppercase text-muted-foreground mb-2">Target Chain</div>
        <div className="flex flex-col">
          {initTargets.map((target, idx) => (
            <React.Fragment key={`${target}-${idx}`}>
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground w-5 text-right shrink-0">{idx + 1}.</span>
                <span className="font-mono text-sm text-foreground truncate">{target}</span>
                <Pill tone={idx === 0 ? "success" : "muted"} className="ml-auto shrink-0">{idx === 0 ? "primary" : `fallback ${idx}`}</Pill>
              </div>
              {idx < initTargets.length - 1 && <div className="flex items-center justify-center py-0.5"><ArrowDown className="h-3 w-3 text-muted-foreground" /></div>}
            </React.Fragment>
          ))}
        </div>
      </div>
    </Surface>
  );
}

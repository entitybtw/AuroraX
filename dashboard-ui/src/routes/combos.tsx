import * as React from "react";
import { Edit3, Loader2, Plus, Save, Trash2, X } from "lucide-react";
import { PageHeader } from "@/components/ui/page-header";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DataTable, TableWrap, Td, Th } from "@/components/ui/data-table";
import { EmptyState, Pill, Surface } from "@/components/ui/surface";
import { createCombo, deleteCombo, fetchCombos, updateCombo, type ComboPayload, type ComboView } from "@/lib/api/combos";
import { useModels } from "@/lib/api/useModels";
import { modelDisplayName } from "@/lib/api/models-types";

interface ComboForm extends ComboPayload { mode: "create" | "edit"; originalName: string; modelsText: string; }

export function CombosPage(): JSX.Element {
  const [combos, setCombos] = React.useState<ComboView[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [form, setForm] = React.useState<ComboForm | null>(null);
  const [error, setError] = React.useState("");
  const [notice, setNotice] = React.useState("");
  const models = useModels();
  const modelOptions = React.useMemo(() => (models.data ?? []).map(modelDisplayName).filter(Boolean), [models.data]);

  const load = React.useCallback(async () => {
    try { setError(""); setCombos(await fetchCombos()); } catch (err) { setError(err instanceof Error ? err.message : "Unable to load combos."); } finally { setLoading(false); }
  }, []);
  React.useEffect(() => { void load(); }, [load]);

  const openCreate = () => setForm({ mode: "create", originalName: "", name: "", description: "", models: [], modelsText: "", enabled: true });
  const openEdit = (view: ComboView) => setForm({ mode: "edit", originalName: view.combo.name, name: view.combo.name, description: view.combo.description ?? "", models: view.combo.models, modelsText: view.combo.models.join("\n"), enabled: view.combo.enabled });

  async function submit(): Promise<void> {
    if (!form) return;
    const payload: ComboPayload = { name: form.name.trim(), enabled: form.enabled, models: form.modelsText.split(/\r?\n|,/).map((v) => v.trim()).filter(Boolean) };
    if (form.description?.trim()) payload.description = form.description.trim();
    if (!payload.name || payload.models.length < 2) { setError("Combo name and at least two models are required."); return; }
    try {
      setError("");
      if (form.mode === "edit") await updateCombo(form.originalName, payload); else await createCombo(payload);
      setForm(null); setNotice(form.mode === "edit" ? "Combo saved." : "Combo created."); await load();
    } catch (err) { setError(err instanceof Error ? err.message : "Unable to save combo."); }
  }

  async function remove(view: ComboView): Promise<void> {
    if (!window.confirm(`Delete combo ${view.combo.name}?`)) return;
    try { setError(""); await deleteCombo(view.combo.name); setNotice("Combo deleted."); await load(); } catch (err) { setError(err instanceof Error ? err.message : "Unable to delete combo."); }
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Combos" subtitle="Create named model fallback chains that appear as selectable models." actions={<Button onClick={openCreate}><Plus className="h-4 w-4" />Create Combo</Button>} />
      {error ? <Alert tone="warning">{error}</Alert> : null}{notice ? <Alert tone="success">{notice}</Alert> : null}
      <Surface className="p-4 text-sm text-muted-foreground">Requests using a combo name execute the first model as primary and use the remaining models as ordered fallback targets.</Surface>
      {loading ? <Surface className="flex items-center gap-2 p-6 text-sm text-muted-foreground"><Loader2 className="h-4 w-4 animate-spin" />Loading combos...</Surface> : combos.length === 0 ? <EmptyState title="No combos configured">Create a combo to expose an ordered fallback chain as a model.</EmptyState> : (
        <TableWrap><DataTable><thead><tr><Th>Name</Th><Th>Source</Th><Th>Status</Th><Th>Chain</Th><Th>Validation</Th><Th className="text-right">Actions</Th></tr></thead><tbody>{combos.map((view) => (
          <tr key={view.combo.id || view.combo.name}>
            <Td><div className="font-mono text-sm font-medium">{view.combo.name}</div><div className="text-xs text-muted-foreground">{view.combo.description}</div></Td>
            <Td><Pill tone={view.readonly ? "muted" : "accent"}>{view.combo.source}</Pill></Td>
            <Td><Pill tone={view.combo.enabled ? "success" : "warning"}>{view.combo.enabled ? "Enabled" : "Disabled"}</Pill></Td>
            <Td><div className="space-y-1 text-xs font-mono">{view.combo.models.map((model, idx) => <div key={`${model}-${idx}`}><span className="text-muted-foreground">{idx === 0 ? "primary" : `fallback ${idx}`}:</span> {model}</div>)}</div></Td>
            <Td>{view.valid ? <Pill tone="success">Valid</Pill> : <div className="space-y-1 text-xs text-warning">{view.errors?.map((e) => <div key={e}>{e}</div>)}</div>}{view.warnings?.map((w) => <div key={w} className="text-xs text-muted-foreground">{w}</div>)}</Td>
            <Td><div className="flex justify-end gap-2"><Button variant="ghost" size="icon" disabled={view.readonly} onClick={() => openEdit(view)}><Edit3 className="h-4 w-4" /></Button><Button variant="ghost" size="icon" disabled={view.readonly} onClick={() => void remove(view)}><Trash2 className="h-4 w-4 text-destructive" /></Button></div></Td>
          </tr>
        ))}</tbody></DataTable></TableWrap>
      )}
      <ComboDialog form={form} error={error} modelOptions={modelOptions} onChange={setForm} onClose={() => setForm(null)} onSubmit={() => void submit()} />
    </div>
  );
}

function ComboDialog({ form, error, modelOptions, onChange, onClose, onSubmit }: { form: ComboForm | null; error: string; modelOptions: string[]; onChange: (form: ComboForm | null) => void; onClose: () => void; onSubmit: () => void }): JSX.Element {
  const [selectedModel, setSelectedModel] = React.useState("");
  React.useEffect(() => { setSelectedModel(modelOptions[0] ?? ""); }, [modelOptions]);
  if (!form) return <Dialog open={false} onOpenChange={() => undefined} />;
  const selectedModels = form.modelsText.split(/\r?\n|,/).map((value) => value.trim()).filter(Boolean);
  const addSelected = (): void => {
    const model = selectedModel.trim();
    if (!model || selectedModels.includes(model)) return;
    onChange({ ...form, modelsText: [...selectedModels, model].join("\n") });
  };
  const removeModel = (model: string): void => onChange({ ...form, modelsText: selectedModels.filter((item) => item !== model).join("\n") });
  return <Dialog open={Boolean(form)} onOpenChange={(open) => !open && onClose()}><DialogContent className="max-w-2xl"><DialogHeader><DialogTitle>{form.mode === "edit" ? "Edit combo" : "Create combo"}</DialogTitle><DialogDescription>Select available models from the live registry. The first model is primary; later models are fallbacks.</DialogDescription></DialogHeader><Field label="Name"><input className="field-input font-mono" value={form.name} onChange={(e) => onChange({ ...form, name: e.target.value })} /></Field><Field label="Description"><input className="field-input" value={form.description ?? ""} onChange={(e) => onChange({ ...form, description: e.target.value })} /></Field><Field label="Add model"><div className="flex gap-2"><select className="field-input font-mono" value={selectedModel} onChange={(e) => setSelectedModel(e.target.value)}>{modelOptions.map((model) => <option key={model} value={model}>{model}</option>)}</select><Button type="button" variant="secondary" onClick={addSelected}>Add</Button></div></Field><div className="space-y-2"><span className="text-xs font-medium text-muted-foreground">Fallback chain</span>{selectedModels.length === 0 ? <div className="border border-border bg-background/35 p-3 text-sm text-muted-foreground">No models selected.</div> : selectedModels.map((model, index) => <div key={model} className="flex items-center justify-between border border-border bg-background/35 px-3 py-2"><span className="font-mono text-sm"><span className="text-muted-foreground">{index === 0 ? "primary" : `fallback ${index}`}:</span> {model}</span><Button type="button" variant="ghost" size="sm" onClick={() => removeModel(model)}>Remove</Button></div>)}</div><label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={form.enabled} onChange={(e) => onChange({ ...form, enabled: e.target.checked })} />Enabled</label>{error ? <p className="text-sm text-warning">{error}</p> : null}<DialogFooter><Button variant="secondary" onClick={onClose}><X className="h-4 w-4" />Cancel</Button><Button onClick={onSubmit}><Save className="h-4 w-4" />Save Combo</Button></DialogFooter></DialogContent></Dialog>;
}

function Field({ label, children }: { label: string; children: React.ReactNode }): JSX.Element { return <label className="block space-y-2"><span className="text-xs font-medium text-muted-foreground">{label}</span>{children}</label>; }
function Alert({ children, tone }: { children: string; tone: "warning" | "success" }): JSX.Element { return <div className={`border px-4 py-3 text-sm ${tone === "warning" ? "border-warning/30 bg-warning/15 text-warning" : "border-success/30 bg-success/15 text-success"}`}>{children}</div>; }

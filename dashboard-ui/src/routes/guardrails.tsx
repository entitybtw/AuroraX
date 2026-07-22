import { useState, useEffect } from "react";
import { PlusIcon, SearchIcon, PencilIcon, Trash2Icon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { Surface, EmptyState } from "@/components/ui/surface";
import { DataTable, TableWrap, Td, Th } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { useGuardrails } from "@/lib/api/useGuardrails";
import { useDashboardConfig } from "@/lib/api/useDashboardConfig";
import { flagOn } from "@/lib/api/dashboard-config";
import type { UpsertGuardrailInput, Guardrail, GuardrailTypeField } from "@/lib/api/guardrails-types";

export function GuardrailsPage(): JSX.Element {
  const { data: config } = useDashboardConfig();
  const { data: guardrails = [], isLoading, types, typesLoading, upsertMutation, deleteMutation } = useGuardrails();

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<"create" | "edit">("create");
  const [filter, setFilter] = useState("");

  // Form State
  const [formData, setFormData] = useState<UpsertGuardrailInput>({
    name: "",
    type: "",
    direction: "input",
    description: "",
    user_path: "",
    config: {},
  });

  // Default type selection on load if none selected
  useEffect(() => {
    if (formOpen && formMode === "create" && !formData.type && types.length > 0) {
      setFormData(prev => ({ ...prev, type: types[0]!.type }));
    }
  }, [formOpen, formMode, types, formData.type]);

  const filteredGuardrails = guardrails.filter((g) => {
    if (!filter) return true;
    const q = filter.toLowerCase();
    return (
      g.name.toLowerCase().includes(q) ||
      g.type.toLowerCase().includes(q) ||
      (g.user_path && g.user_path.toLowerCase().includes(q)) ||
      (g.summary && g.summary.toLowerCase().includes(q))
    );
  });

  const handleOpenForm = (g?: Guardrail) => {
    if (g) {
      setFormMode("edit");
      setFormData({
        name: g.name,
        type: g.type,
        direction: g.direction || "input",
        description: g.description,
        user_path: g.user_path,
        config: { ...g.config },
      });
    } else {
      setFormMode("create");
      setFormData({
        name: "",
        type: types.length > 0 ? types[0]!.type : "",
        direction: "input",
        description: "",
        user_path: "",
        config: {},
      });
    }
    setFormOpen(true);
  };

  const handleTypeChange = (newType: string) => {
    setFormData(prev => ({ ...prev, type: newType, config: {} }));
  };

  const setConfigValue = (key: string, value: unknown) => {
    setFormData(prev => ({
      ...prev,
      config: { ...prev.config, [key]: value },
    }));
  };

  const toggleArrayConfigValue = (key: string, value: string, checked: boolean) => {
    setFormData(prev => {
      const arr = (prev.config?.[key] as string[]) || [];
      const newArr = checked ? [...arr, value] : arr.filter(v => v !== value);
      return {
        ...prev,
        config: { ...prev.config, [key]: newArr },
      };
    });
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    upsertMutation.mutate(formData, {
      onSuccess: () => {
        setFormOpen(false);
      },
    });
  };

  const activeTypeDef = types.find(t => t.type === formData.type);

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col sm:flex-row sm:items-end justify-between gap-4 pb-6 pt-4 border-b border-border/60">
        <div className="min-w-0 flex-1">
          <h1 className="font-serif text-[34px] font-normal leading-tight tracking-tight text-foreground">Guardrails</h1>
          <p className="mt-1.5 text-[15px] text-muted-foreground">Reusable policy objects stored in the database and kept hot in memory for workflow execution.</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <Button onClick={() => handleOpenForm()} disabled={typesLoading}>
            <PlusIcon className="mr-2 h-4 w-4" />
            Create Guardrail
          </Button>
        </div>
      </header>

      <Surface className="p-6 border-border/40 relative overflow-hidden group">
        <div className="flex flex-col md:flex-row gap-6 justify-between items-start relative z-10">
          <div className="flex flex-col gap-2 max-w-2xl">
            <p className="text-[12px] font-bold uppercase tracking-wider text-muted-foreground">Reusable Policy Objects</p>
            <h3 className="font-serif text-xl font-normal tracking-tight text-foreground">Guardrail Library</h3>
            <p className="text-[15px] text-muted-foreground leading-relaxed">
              Store input, output, or bidirectional guardrails in the database, keep them hot in memory, and attach them to workflows by reference.
            </p>
          </div>
          <div className="flex gap-8">
            <div className="flex flex-col gap-1.5">
              <span className="text-[13px] font-medium text-muted-foreground uppercase tracking-wider">Instances</span>
              <strong className="text-[40px] leading-none font-bold tracking-tight text-foreground" style={{ fontFeatureSettings: '"tnum"' }}>{guardrails.length}</strong>
            </div>
            <div className="flex flex-col gap-1.5">
              <span className="text-[13px] font-medium text-muted-foreground uppercase tracking-wider">Types</span>
              <strong className="text-[40px] leading-none font-bold tracking-tight text-foreground" style={{ fontFeatureSettings: '"tnum"' }}>{types.length}</strong>
            </div>
          </div>
        </div>
      </Surface>

      <Surface className="p-0 overflow-hidden border-muted">
        <details className="group">
          <summary className="cursor-pointer p-4 font-semibold text-sm bg-muted/20 hover:bg-muted/30 transition-colors list-none flex justify-between items-center outline-none">
            <span>How to use guardrails</span>
            <span className="text-muted-foreground transition-transform duration-200 group-open:rotate-180">▼</span>
          </summary>
          <div className="p-5 flex flex-col gap-6 text-sm bg-background border-t">
            <p className="text-muted-foreground leading-relaxed">
              Guardrails are reusable policy objects. You define them once and attach them to workflows by name. The runtime executes them in the order configured by the workflow before the request is dispatched to the upstream provider.
            </p>

            <div className="flex flex-col gap-4">
              <div className="flex flex-col gap-1.5">
                <h4 className="font-semibold text-foreground">1. Lifecycle</h4>
                <ol className="list-decimal pl-5 space-y-1.5 text-muted-foreground">
                  <li><strong className="text-foreground font-medium">Define</strong> a guardrail here (or in <code className="text-xs bg-muted px-1.5 py-0.5 rounded">config.yaml</code> under <code className="text-xs bg-muted px-1.5 py-0.5 rounded">guardrails.rules</code>).</li>
                  <li><strong className="text-foreground font-medium">Attach</strong> it to a workflow on the Workflows page (or it auto-attaches to <code className="text-xs bg-muted px-1.5 py-0.5 rounded">default-global</code> when seeded from <code className="text-xs bg-muted px-1.5 py-0.5 rounded">config.yaml</code>).</li>
                  <li><strong className="text-foreground font-medium">Runtime</strong> applies input guardrails before provider dispatch and output guardrails to non-streaming provider responses before clients receive them.</li>
                </ol>
              </div>

              <div className="flex flex-col gap-1.5">
                <h4 className="font-semibold text-foreground">2. Step ordering</h4>
                <p className="text-muted-foreground leading-relaxed">Guardrails attached at the same numeric <code className="text-xs bg-muted px-1.5 py-0.5 rounded">step</code> run <strong className="text-foreground font-medium">in parallel</strong>. Different steps run <strong className="text-foreground font-medium">sequentially</strong>, lowest first. Use steps to express dependencies (e.g. PII redact at step 0, then a system-prompt decorator at step 10).</p>
              </div>

              <div className="flex flex-col gap-1.5">
                <h4 className="font-semibold text-foreground">3. Available guardrail types</h4>
                <ul className="list-disc pl-5 space-y-1.5 text-muted-foreground">
                  <li><strong className="text-foreground font-medium">system_prompt</strong> — inject, override, or decorate the system message. Use for compliance disclaimers, safety prompts, persona injection.</li>
                  <li><strong className="text-foreground font-medium">regex_block</strong> — match patterns in user/system messages and either <em>block</em> the request with a 400 or <em>sanitize</em> by replacing matches with a placeholder. Use for keyword bans, secret detection.</li>
                  <li><strong className="text-foreground font-medium">pii_redact</strong> — deterministic regex redaction for emails, phone numbers, US SSN, and credit-card-like numbers. Replaces matches with role-aware placeholders.</li>
                  <li><strong className="text-foreground font-medium">length_limit</strong> — hard cap on combined message size in characters or estimated tokens. Rejects oversized requests early to protect upstream rate limits.</li>
                </ul>
              </div>

              <div className="flex flex-col gap-1.5">
                <h4 className="font-semibold text-foreground">4. Fail policy</h4>
                <p className="text-muted-foreground leading-relaxed">Each guardrail has an <code className="text-xs bg-muted px-1.5 py-0.5 rounded">on_error</code> field: <code className="text-xs bg-muted px-1.5 py-0.5 rounded">block</code> (default for blocking guardrails — request is rejected if the guardrail itself errors out) or <code className="text-xs bg-muted px-1.5 py-0.5 rounded">allow</code> (fail-open — request continues if the guardrail crashes). Sanitizing guardrails default to <code className="text-xs bg-muted px-1.5 py-0.5 rounded">allow</code> so a guardrail bug never breaks your traffic.</p>
              </div>

              <div className="flex flex-col gap-1.5">
                <h4 className="font-semibold text-foreground">5. Quick recipes</h4>
                <ul className="list-disc pl-5 space-y-1.5 text-muted-foreground">
                  <li><strong className="text-foreground font-medium">Enforce safety prompt globally</strong>: create a <code className="text-xs bg-muted px-1.5 py-0.5 rounded">system_prompt</code> guardrail with mode <code className="text-xs bg-muted px-1.5 py-0.5 rounded">decorator</code>, attach to the global workflow.</li>
                  <li><strong className="text-foreground font-medium">Block secrets</strong>: create a <code className="text-xs bg-muted px-1.5 py-0.5 rounded">regex_block</code> with <code className="text-xs bg-muted px-1.5 py-0.5 rounded">action: block</code> and patterns like <code className="text-xs bg-muted px-1.5 py-0.5 rounded">(?i)api[_-]?key|password|bearer\s+[A-Za-z0-9]+</code>.</li>
                  <li><strong className="text-foreground font-medium">Anonymize PII before sending to OpenAI</strong>: attach <code className="text-xs bg-muted px-1.5 py-0.5 rounded">pii_redact</code> at step 0; let user paths that opt-in route through this workflow.</li>
                  <li><strong className="text-foreground font-medium">Cap input size</strong>: <code className="text-xs bg-muted px-1.5 py-0.5 rounded">length_limit</code> with <code className="text-xs bg-muted px-1.5 py-0.5 rounded">max_chars: 50000</code> stops 1MB blob uploads from reaching the upstream.</li>
                </ul>
              </div>
            </div>
          </div>
        </details>
      </Surface>

      {!flagOn(config?.GUARDRAILS_ENABLED) && (
        <div className="border border-info/30 bg-info/10 p-4">
          <h3 className="text-sm font-medium text-info">Runtime guardrail execution is off</h3>
          <p className="mt-1 text-sm text-info/80">
            GUARDRAILS_ENABLED is disabled. You can manage definitions here, but they won't execute.
          </p>
        </div>
      )}

      {(guardrails.length > 0 || filter) && (
        <div className="relative">
          <SearchIcon className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Filter by name, type, summary..."
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="pl-9 max-w-md"
          />
        </div>
      )}

      {isLoading ? (
        <Surface variant="subtle" className="py-12">
          <EmptyState
            title="Loading guardrails..."
            description="Fetching your policy definitions."
          />
        </Surface>
      ) : filteredGuardrails.length === 0 ? (
        <Surface variant="subtle" className="py-12">
          <EmptyState
            title={filter ? "No guardrails match your filter" : "No guardrails defined yet"}
            description={filter ? "Clear the search field to see all guardrails." : "Create a guardrail to inspect or block requests before they reach a provider."}
            action={
              !filter && (
                <Button onClick={() => handleOpenForm()}>
                  <PlusIcon className="mr-2 h-4 w-4" />
                  Create Guardrail
                </Button>
              )
            }
          />
        </Surface>
      ) : (
        <Surface>
          <TableWrap>
            <DataTable>
              <thead>
                <tr>
                  <Th>Name</Th>
                  <Th>Type</Th>
                  <Th>Direction</Th>
                  <Th>User Path</Th>
                  <Th>Summary</Th>
                  <Th className="text-right">Actions</Th>
                </tr>
              </thead>
              <tbody>
                {filteredGuardrails.map((g) => {
                  const typeDef = types.find(t => t.type === g.type);
                  return (
                    <tr key={g.name}>
                      <Td className="font-mono font-medium">{g.name}</Td>
                      <Td>
                        <span className="inline-flex items-center  bg-surface px-2.5 py-0.5 text-xs font-medium text-muted-foreground ring-1 ring-inset ring-border">
                          {typeDef?.label || g.type}
                        </span>
                      </Td>
                      <Td><code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">{g.direction || "input"}</code></Td>
                      <Td><code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">{g.user_path || "—"}</code></Td>
                      <Td>
                        <div className="flex flex-col">
                          <span className="text-sm font-medium">{g.summary || g.description || "No summary yet."}</span>
                          {g.description && g.summary && <span className="text-xs text-muted-foreground">{g.description}</span>}
                        </div>
                      </Td>
                      <Td className="text-right">
                        <Button variant="ghost" size="icon" onClick={() => handleOpenForm(g)} title="Edit Guardrail">
                          <PencilIcon className="h-4 w-4 text-muted-foreground" />
                        </Button>
                        <Button variant="ghost" size="icon" disabled={deleteMutation.isPending} onClick={() => deleteMutation.mutate(g.name)} title="Delete Guardrail">
                          <Trash2Icon className="h-4 w-4 text-destructive" />
                        </Button>
                      </Td>
                    </tr>
                  );
                })}
              </tbody>
            </DataTable>
          </TableWrap>
        </Surface>
      )}

      <Dialog open={formOpen} onOpenChange={setFormOpen}>
        <DialogContent className="w-[calc(100vw-2rem)] max-w-4xl sm:max-w-4xl max-h-[90vh] overflow-y-auto">
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>{formMode === "edit" ? "Edit Guardrail" : "Create Guardrail"}</DialogTitle>
              <DialogDescription>
                Workflows reference these names directly, so renames are intentionally avoided after creation.
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4 md:grid-cols-2">
              <div className="flex flex-col gap-2">
                <label htmlFor="name" className="text-sm font-medium">Name</label>
                <Input
                  id="name"
                  required
                  placeholder="e.g. safety-system-prompt"
                  value={formData.name}
                  disabled={formMode === "edit"}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                />
              </div>

              <div className="flex flex-col gap-2">
                <label htmlFor="type" className="text-sm font-medium">Type</label>
                <select
                  id="type"
                  className="field-input"
                  required
                  value={formData.type}
                  disabled={formMode === "edit"}
                  onChange={(e) => handleTypeChange(e.target.value)}
                >
                  {types.map((t) => (
                    <option key={t.type} value={t.type}>{t.label}</option>
                  ))}
                </select>
              </div>

              <div className="flex flex-col gap-2 md:col-span-2">
                <label htmlFor="description" className="text-sm font-medium">Description</label>
                <Input
                  id="description"
                  placeholder="What this guardrail is meant to do"
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                />
              </div>

              <div className="flex flex-col gap-2">
                <label htmlFor="direction" className="text-sm font-medium">Direction</label>
                <select
                  id="direction"
                  className="field-input"
                  required
                  value={formData.direction || "input"}
                  onChange={(e) => setFormData({ ...formData, direction: e.target.value as "input" | "output" | "both" })}
                >
                  <option value="input">Input only</option>
                  <option value="output">Output only</option>
                  <option value="both">Input and output</option>
                </select>
                <p className="text-xs text-muted-foreground">
                  Output guardrails run for non-streaming chat and responses results. Streaming output guardrails require a buffered stream mode and are not applied to partial SSE chunks yet.
                </p>
              </div>

              <div className="flex flex-col gap-2">
                <label htmlFor="user_path" className="text-sm font-medium">User Path <span className="text-muted-foreground font-normal text-xs">(optional, for aux rewrites)</span></label>
                <Input
                  id="user_path"
                  placeholder="/team/alpha"
                  value={formData.user_path}
                  onChange={(e) => setFormData({ ...formData, user_path: e.target.value })}
                />
              </div>

              {activeTypeDef?.fields.map((field: GuardrailTypeField) => {
                const val = formData.config?.[field.key] ?? "";

                if (field.input === "checkboxes") {
                  const arrVal = (formData.config?.[field.key] as string[]) || [];
                  return (
                    <div key={field.key} className="flex flex-col gap-2 mt-2 md:col-span-2">
                      <label className="text-sm font-medium">{field.label}</label>
                      <div className="flex flex-col gap-2 rounded-md border p-3">
                        {field.options?.map(opt => (
                          <label key={opt.value} className="flex items-center gap-2 text-sm">
                            <input
                              type="checkbox"
                              className="rounded border-zinc-300"
                              checked={arrVal.includes(opt.value)}
                              onChange={(e) => toggleArrayConfigValue(field.key, opt.value, e.target.checked)}
                            />
                            {opt.label}
                          </label>
                        ))}
                      </div>
                      {field.help && <p className="text-xs text-muted-foreground">{field.help}</p>}
                    </div>
                  );
                }

                if (field.input === "select") {
                  return (
                    <div key={field.key} className="flex flex-col gap-2 mt-2">
                      <label className="text-sm font-medium">{field.label}</label>
                      <select
                        className="field-input"
                        value={val as string}
                        onChange={(e) => setConfigValue(field.key, e.target.value)}
                      >
                        {field.options?.map(opt => (
                          <option key={opt.value} value={opt.value}>{opt.label}</option>
                        ))}
                      </select>
                      {field.help && <p className="text-xs text-muted-foreground">{field.help}</p>}
                    </div>
                  );
                }

                if (field.input === "textarea" || field.input === "textarea_lines") {
                  return (
                    <div key={field.key} className="flex flex-col gap-2 mt-2 md:col-span-2">
                      <label className="text-sm font-medium">{field.label}</label>
                      <textarea
                        className="field-input min-h-[100px] resize-y"
                        placeholder={field.placeholder}
                        value={val as string}
                        onChange={(e) => setConfigValue(field.key, e.target.value)}
                      />
                      {field.help && <p className="text-xs text-muted-foreground">{field.help}</p>}
                    </div>
                  );
                }

                return (
                  <div key={field.key} className="flex flex-col gap-2 mt-2">
                    <label className="text-sm font-medium">{field.label}</label>
                    <Input
                      type={field.input || "text"}
                      placeholder={field.placeholder}
                      value={val as string}
                      onChange={(e) => setConfigValue(field.key, field.input === "number" ? Number(e.target.value) : e.target.value)}
                    />
                    {field.help && <p className="text-xs text-muted-foreground">{field.help}</p>}
                  </div>
                );
              })}

              {upsertMutation.isError && (
                <p className="text-sm text-destructive md:col-span-2">{upsertMutation.error.message}</p>
              )}
            </div>
            <div className="flex justify-end gap-2 mt-4 pt-4 border-t">
              <Button type="button" variant="outline" onClick={() => setFormOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={upsertMutation.isPending}>
                {upsertMutation.isPending ? "Saving..." : "Save Guardrail"}
              </Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}

import { useState, useMemo } from "react";
import { PlusIcon, PencilIcon, ShieldIcon, XCircleIcon, AlertCircleIcon, EyeIcon, ChevronDown, Info, ListOrdered, SlidersHorizontal, Crosshair } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ToggleField } from "@/components/ui/toggle-field";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { Surface, EmptyState } from "@/components/ui/surface";
import { Input } from "@/components/ui/input";
import { SearchInput } from "@/components/ui/search-input";
import { useWorkflows } from "@/lib/api/useWorkflows";
import { useGuardrails } from "@/lib/api/useGuardrails";
import { useDashboardConfig } from "@/lib/api/useDashboardConfig";
import { flagOn } from "@/lib/api/dashboard-config";
import { useProviderStatus } from "@/lib/api/useProviders";
import type { UpsertWorkflowInput, Workflow, WorkflowGuardrailStep } from "@/lib/api/workflows-types";
import { WorkflowChart } from "@/components/workflows/WorkflowChart";
import { useBulkSelection } from "@/lib/hooks/useBulkSelection";
import { SelectCheckbox, BulkActionsBar } from "@/components/ui/bulk-selection";
import { t } from "@/lib/i18n";


export function WorkflowsPage(): JSX.Element {
  const { data: config } = useDashboardConfig();
  const { data: workflows = [], isLoading, upsertMutation, deactivateMutation } = useWorkflows();
  const { data: guardrails = [] } = useGuardrails();
  const { data: providersInfo } = useProviderStatus();
  const bulk = useBulkSelection<string>();

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<"create" | "edit">("create");
  const [filter, setFilter] = useState("");
  const [detailWorkflow, setDetailWorkflow] = useState<Workflow | null>(null);

  // Form State
  const [formData, setFormData] = useState<UpsertWorkflowInput>({
    name: "",
    scope_provider: "",
    scope_model: "",
    scope_user_path: "",
    description: "",
    features: {
      caching: false,
      budget: false,
      guardrails: false,
      failover: false,
      rate_limit: false,
      audit: false,
      usage_tracking: false,
    },
    guardrails: [],
  });

  const filteredWorkflows = useMemo(() => {
    return workflows.filter((w) => {
      if (!filter) return true;
      const q = filter.toLowerCase();
      return (
        w.name.toLowerCase().includes(q) ||
        (w.scope_provider && w.scope_provider.toLowerCase().includes(q)) ||
        (w.scope_model && w.scope_model.toLowerCase().includes(q)) ||
        (w.scope_user_path && w.scope_user_path.toLowerCase().includes(q)) ||
        (w.workflow_hash && w.workflow_hash.toLowerCase().includes(q))
      );
    });
  }, [workflows, filter]);

  const bulkDeactivatableIds = useMemo(() => {
    const globalCount = workflows.filter(x => !x.scope_provider && !x.scope_model && !x.scope_user_path).length;
    return filteredWorkflows
      .filter((w) => {
        if (w.name === 'default-global') return false;
        const isGlobal = !w.scope_provider && !w.scope_model && !w.scope_user_path;
        if (isGlobal && globalCount <= 1) return false;
        return true;
      })
      .map((w) => w.id);
  }, [filteredWorkflows, workflows]);

  const bulkDeactivate = async () => {
    const ids = bulk.selected;
    for (const id of ids) {
      const w = workflows.find((wf) => wf.id === id);
      if (!w) continue;
      if (w.name === 'default-global') continue;
      const globalCount = workflows.filter(x => !x.scope_provider && !x.scope_model && !x.scope_user_path).length;
      const isGlobal = !w.scope_provider && !w.scope_model && !w.scope_user_path;
      if (isGlobal && globalCount <= 1) continue;
      await deactivateMutation.mutateAsync(id);
    }
    bulk.clear();
  };

  // Derived providers/models lists
  const providerNames = useMemo(() => {
    if (!providersInfo?.providers) return [];
    return providersInfo.providers.map((p) => p.name);
  }, [providersInfo]);

  const modelOptions = useMemo(() => {
    if (!formData.scope_provider || !providersInfo?.providers) return [];
    const p = providersInfo.providers.find((prov) => prov.name === formData.scope_provider);
    if (!p) return [];
    return Object.keys(p.config?.models || {});
  }, [formData.scope_provider, providersInfo]);

  const handleOpenForm = (w?: Workflow) => {
    if (w) {
      setFormMode("edit");
      setFormData({
        name: w.name,
        scope_provider: w.scope_provider,
        scope_model: w.scope_model,
        scope_user_path: w.scope_user_path,
        description: w.description,
        features: { ...w.features },
        guardrails: [...w.guardrails],
      });
    } else {
      setFormMode("create");
      setFormData({
        name: "",
        scope_provider: "",
        scope_model: "",
        scope_user_path: "",
        description: "",
        features: {
          caching: flagOn(config?.CACHE_ENABLED),
          budget: false,
          guardrails: flagOn(config?.GUARDRAILS_ENABLED),
          failover: config?.FEATURE_FALLBACK_MODE !== "off",
          rate_limit: true,
          audit: true,
          usage_tracking: flagOn(config?.USAGE_ENABLED),
        },
        guardrails: [] as WorkflowGuardrailStep[],
      });
    }
    setFormOpen(true);
  };

  const handleToggleFeature = (key: keyof typeof formData.features) => {
    setFormData(prev => ({
      ...prev,
      features: { ...prev.features, [key]: !prev.features?.[key] },
    }));
  };

  const handleAddGuardrailStep = () => {
    setFormData(prev => ({
      ...prev,
      guardrails: [...(prev.guardrails || []), { ref: "", step: 0 } as WorkflowGuardrailStep],
    }));
  };

  const handleRemoveGuardrailStep = (index: number) => {
    setFormData(prev => {
      const arr = [...(prev.guardrails || [])];
      arr.splice(index, 1);
      return { ...prev, guardrails: arr };
    });
  };

  const handleUpdateGuardrailStep = (index: number, field: keyof WorkflowGuardrailStep, value: string | number) => {
    setFormData(prev => {
      const arr = [...(prev.guardrails || [])];
      arr[index] = { ...arr[index], [field]: value } as WorkflowGuardrailStep;
      return { ...prev, guardrails: arr };
    });
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    upsertMutation.mutate(formData, {
      onSuccess: () => {
        setFormOpen(false);
      },
      onError: (err) => {
        if (err.message && err.message.includes("managed default workflow name/description is reserved")) {
          // We ignore or just pass this up since the form handles displaying mutation errors
        }
      }
    });
  };

  const getScopeLabel = (w: Workflow | UpsertWorkflowInput) => {
    const parts = [];
    if (w.scope_user_path) parts.push(`path: ${w.scope_user_path}`);
    if (w.scope_provider) parts.push(`provider: ${w.scope_provider}`);
    if (w.scope_model) parts.push(`model: ${w.scope_model}`);
    return parts.length > 0 ? parts.join(" â€¢ ") : "Global (All requests)";
  };

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col sm:flex-row sm:items-end justify-between gap-4 pb-6 pt-4 border-b border-border/60">
        <div className="min-w-0 flex-1">
          <h1 className="font-display text-[28px] md:text-[34px] font-normal leading-tight tracking-tight text-foreground">{t("Workflows")}</h1>
          <p className="mt-1.5 text-[15px] text-muted-foreground">{t("Active workflows are matched path-first, then provider and model.")}</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <Button onClick={() => handleOpenForm()}>
            <PlusIcon className="mr-2 h-4 w-4" />{t("New Workflow")}</Button>
        </div>
      </header>

      <Surface className="p-0 overflow-hidden border-border/40 mb-6 transition-all">
        <details className="group">
          <summary className="cursor-pointer p-4 font-semibold text-[15px] bg-surface-hover/30 hover:bg-surface-hover/60 transition-colors list-none flex justify-between items-center outline-none">
            <div className="flex items-center gap-2">
              <ShieldIcon className="h-4 w-4 text-accent" />
              <span>{t("How to use workflows")}</span>
            </div>
            <ChevronDown className="h-4 w-4 text-muted-foreground transition-transform duration-200 group-open:rotate-180" />
          </summary>
          <div className="p-6 flex flex-col gap-6 text-[14px] bg-background/50 border-t border-border/40 backdrop-blur-sm">
            <div className="flex items-start gap-3 text-muted-foreground leading-relaxed bg-accent/5 border border-accent/10 rounded-lg p-4">
              <Info className="h-5 w-5 mt-0.5 shrink-0 text-accent" />
              <span>{t("Workflows define routing, failover, guardrails, and usage policies. They are matched dynamically based on request parameters.")}</span>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="flex flex-col gap-3 rounded-lg border border-border/40 p-4 bg-background/30">
                <div className="flex items-center gap-2.5">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-accent/10 text-accent">
                    <ListOrdered className="h-4 w-4" />
                  </div>
                  <h4 className="font-semibold text-[15px] tracking-tight text-foreground">{t("Matching order")}</h4>
                </div>
                <p className="text-muted-foreground text-[13px] leading-relaxed">{t("Workflows are evaluated in priority order: user path takes precedence, then provider+model combination, then global scope as the default fallback.")}</p>
              </div>

              <div className="flex flex-col gap-3 rounded-lg border border-border/40 p-4 bg-background/30">
                <div className="flex items-center gap-2.5">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-accent/10 text-accent">
                    <SlidersHorizontal className="h-4 w-4" />
                  </div>
                  <h4 className="font-semibold text-[15px] tracking-tight text-foreground">{t("Feature toggles")}</h4>
                </div>
                <p className="text-muted-foreground text-[13px] leading-relaxed">{t("Each workflow enables a set of pipeline features — caching, guardrails, failover, rate limits, audit logging, and usage tracking — that apply to matched requests.")}</p>
              </div>

              <div className="flex flex-col gap-3 rounded-lg border border-border/40 p-4 bg-background/30">
                <div className="flex items-center gap-2.5">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-accent/10 text-accent">
                    <Crosshair className="h-4 w-4" />
                  </div>
                  <h4 className="font-semibold text-[15px] tracking-tight text-foreground">{t("Scoping examples")}</h4>
                </div>
                <ul className="flex flex-col gap-2 text-[13px] text-muted-foreground">
                  <li className="flex items-start gap-2">
                    <span className="flex h-5 w-5 items-center justify-center rounded-full bg-success/10 text-success text-[10px] font-bold shrink-0 mt-0.5">{t("G")}</span>
                    <span><strong className="text-foreground font-medium">{t("Provider & Model Empty")}</strong>{t(": Targets all requests (Global scope). Cannot be deactivated.")}</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="flex h-5 w-5 items-center justify-center rounded-full bg-info/10 text-info text-[10px] font-bold shrink-0 mt-0.5">{t("P")}</span>
                    <span><strong className="text-foreground font-medium">{t("Provider only")}</strong>{t(": Targets all models under that specific provider name.")}</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="flex h-5 w-5 items-center justify-center rounded-full bg-warning/10 text-warning text-[10px] font-bold shrink-0 mt-0.5">{t("U")}</span>
                    <span><strong className="text-foreground font-medium">{t("User Path")}</strong>{t(": Applies features exclusively to requests passing the ")}<code className="text-[11px] bg-background px-1.5 py-0.5 border border-border/60 rounded">User-Path: /team/alpha</code>{t(" header or derived JWT path.")}</span>
                  </li>
                </ul>
              </div>
            </div>
          </div>
        </details>
      </Surface>

      {(workflows.length > 0 || filter) && (
        <div className="flex items-center justify-between mt-2">
          <SearchInput
            placeholder={t("Filter by scope, name, hash...")}
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            containerClassName="flex-1 max-w-md"
          />
          <div className="text-sm text-muted-foreground">
            <span className="font-medium text-foreground">{filteredWorkflows.length}</span>{t(" active scopes")}</div>
        </div>
      )}

      {isLoading ? (
        <Surface variant="subtle" className="py-12">
          <EmptyState
            title={t("Loading workflows...")}
            description={t("Fetching your routing and policy configurations.")}
          />
        </Surface>
      ) : filteredWorkflows.length === 0 ? (
        <Surface variant="subtle" className="py-12">
          <EmptyState
            title={filter ? t("No workflows match your filter") : t("No active workflows found")}
            description={filter ? t("Clear the search field to see all workflows.") : t("Create a workflow to control routing, failover, and guardrail behavior.")}
            action={
              !filter && (
                <Button onClick={() => handleOpenForm()}>
                  <PlusIcon className="mr-2 h-4 w-4" />{t("New Workflow")}</Button>
              )
            }
          />
        </Surface>
      ) : (
        <div className="flex flex-col gap-3">
          <BulkActionsBar count={bulk.selectedCount} onClear={bulk.clear}>
            <Button
              size="sm"
              variant="outline"
              className="text-destructive hover:bg-destructive/10 hover:text-destructive border-destructive/20"
              disabled={deactivateMutation.isPending}
              onClick={bulkDeactivate}
            >{t("Deactivate")}</Button>
          </BulkActionsBar>

          <div className="flex items-center gap-2 px-1">
            <SelectCheckbox
              checked={bulkDeactivatableIds.length > 0 && bulk.allSelected(bulkDeactivatableIds)}
              indeterminate={bulk.someSelected(bulkDeactivatableIds)}
              onClick={(e: React.MouseEvent) => e.shiftKey ? null : bulk.toggleAll(bulkDeactivatableIds)}
              title={t("Select all")}
            />
            <span className="text-xs text-muted-foreground">{t("Select all")}</span>
          </div>

          <div className="grid grid-cols-1 gap-4">
            {filteredWorkflows.map((w) => {
              const isGlobal = !w.scope_provider && !w.scope_model && !w.scope_user_path;
              const globalCount = workflows.filter(x => !x.scope_provider && !x.scope_model && !x.scope_user_path).length;
              const isOnlyGlobal = isGlobal && globalCount <= 1;
              const canBulkDeactivate = w.name !== 'default-global' && !(isGlobal && globalCount <= 1);

              return (
                <Surface key={w.id} className="p-0 flex flex-col h-full overflow-hidden">
                  <div className="p-5 flex flex-col gap-4 flex-1">
                    <div className="flex justify-between items-start">
                      <div className="flex items-center gap-2">
                        {canBulkDeactivate && (
                          <SelectCheckbox
                            checked={bulk.isSelected(w.id)}
                            onClick={(e: React.MouseEvent) => e.shiftKey ? bulk.rangeSelect(w.id, bulkDeactivatableIds) : bulk.toggle(w.id)}
                            title={`Select ${w.name || getScopeLabel(w)}`}
                          />
                        )}
                        <div className="flex flex-col gap-1.5">
                          <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
                            <span className={`w-1.5 h-1.5  ${isGlobal ? 'bg-accent' : w.scope_user_path ? 'bg-info' : 'bg-success'}`}></span>
                            {isGlobal ? t("Global scope") : w.scope_user_path ? t("User path scope") : t("Provider/model scope")}
                          </span>
                          <h3 className="font-display text-xl font-normal tracking-tight text-foreground">{w.name || getScopeLabel(w)}</h3>
                        </div>
                      </div>
                    </div>

                    {w.description && (
                      <p className="text-sm text-muted-foreground leading-relaxed">{w.description}</p>
                    )}

                    <div className="my-4 py-4  border bg-muted/10 overflow-hidden shadow-inner">
                      <WorkflowChart workflow={w} />
                    </div>

                    <div className="flex flex-col gap-2 mt-auto pt-2">
                      <div className="flex flex-wrap gap-1.5">
                        {w.features.caching && <span className="inline-flex items-center rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset bg-success/10 text-success ring-success/30">{t("Caching")}</span>}
                        {w.features.guardrails && <span className="inline-flex items-center rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset bg-accent/10 text-accent ring-accent/20">{t("Guardrails")}</span>}
                        {w.features.failover && <span className="inline-flex items-center rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset bg-warning/10 text-warning ring-warning/30">{t("Failover")}</span>}
                        {w.features.rate_limit && <span className="inline-flex items-center rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset bg-surface-hover/50 text-muted-foreground ring-border/30">{t("Rate Limits")}</span>}
                        {w.features.audit && <span className="inline-flex items-center rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset bg-surface-hover/50 text-muted-foreground ring-border/30">{t("Audit")}</span>}
                        {w.features.usage_tracking && <span className="inline-flex items-center rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset bg-surface-hover/50 text-muted-foreground ring-border/30">{t("Usage")}</span>}
                      </div>
                    </div>

                    {w.features.guardrails && w.guardrails.length > 0 && (
                      <div className="flex flex-col gap-2 rounded-md bg-muted p-3">
                        <div className="flex items-center gap-2">
                          <ShieldIcon className="h-4 w-4 text-muted-foreground" />
                          <span className="text-sm font-medium">{t("Guardrails (")}{w.guardrails.length}{t(" steps)")}</span>
                        </div>
                        <div className="flex flex-col gap-1 pl-6">
                          {w.guardrails.map((step, idx) => (
                            <div key={idx} className="flex items-center gap-2 text-xs">
                              <span className="font-mono">{step.ref}</span>
                              <span className="text-muted-foreground ml-auto">{t("step ")}{step.step}</span>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>

                  <div className="bg-surface-hover/30 p-4 border-t border-border/40 flex items-center justify-between mt-auto">
                    <div className="flex items-center gap-2">
                      <Button
                        variant="outline"
                        className="text-destructive hover:bg-destructive/10 hover:text-destructive border-destructive/20"
                        size="sm"
                        disabled={deactivateMutation.isPending || isOnlyGlobal || w.name === 'default-global'}
                        onClick={() => deactivateMutation.mutate(w.id)}
                        title={isOnlyGlobal ? t("The last active global workflow cannot be deactivated to prevent routing failures.") : w.name === 'default-global' ? t("Managed default workflow cannot be deactivated.") : t("Deactivate workflow")}
                      >{t("Deactivate")}</Button>
                      <Button variant="outline" size="sm" onClick={() => setDetailWorkflow(w)} className="bg-surface/50">
                        <EyeIcon className="mr-2 h-4 w-4" />{t("View")}</Button>
                      <Button variant="outline" size="sm" onClick={() => handleOpenForm(w)} className="bg-surface/50">
                        <PencilIcon className="mr-2 h-4 w-4" />{t("Edit")}</Button>
                    </div>
                    <div className="flex flex-col items-end gap-0.5 text-[11px] text-muted-foreground font-mono">
                      <span className="font-semibold uppercase tracking-wider">v{w.version}</span>
                      <span className="opacity-70">{t("hash: ")}{w.workflow_hash ? w.workflow_hash.substring(0, 8) : "â€”"}</span>
                    </div>
                  </div>
                </Surface>
              );
            })}
          </div>
        </div>
      )}

      <Dialog open={formOpen} onOpenChange={setFormOpen}>
        <DialogContent className="sm:max-w-4xl max-h-[85vh] overflow-y-auto">
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>{formMode === "edit" ? t("Edit Workflow") : t("Create Workflow")}</DialogTitle>
              <DialogDescription>{t("Create an immutable workflow version. Submitting activates it for the selected scope.")}</DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-6 py-4">
              <div className="grid grid-cols-2 gap-4">
                <div className="flex flex-col gap-2">
                  <label htmlFor="scope_provider" className="text-sm font-medium">{t("Provider Name")}</label>
                  <select
                    id="scope_provider"
                    className="field-input"
                    value={formData.scope_provider}
                    onChange={(e) => setFormData({ ...formData, scope_provider: e.target.value, scope_model: "" })}
                  >
                    <option value="">{t("All providers and models")}</option>
                    {providerNames.map((p) => (
                      <option key={p} value={p}>{p}</option>
                    ))}
                  </select>
                </div>
                {formData.scope_provider && (
                  <div className="flex flex-col gap-2">
                    <label htmlFor="scope_model" className="text-sm font-medium">{t("Model")}</label>
                    <select
                      id="scope_model"
                      className="field-input"
                      value={formData.scope_model}
                      onChange={(e) => setFormData({ ...formData, scope_model: e.target.value })}
                    >
                      <option value="">{t("All models for provider")}</option>
                      {modelOptions.map((m) => (
                        <option key={m} value={m}>{m}</option>
                      ))}
                    </select>
                  </div>
                )}
                <div className="flex flex-col gap-2">
                  <label htmlFor="name" className="text-sm font-medium">{t("Name")}</label>
                  <Input
                    id="name"
                    placeholder={t("Optional. Defaults to scope label.")}
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  />
                  {formData.name === "default-global" && (
                    <p className="text-xs text-warning">{t("The name \"default-global\" is reserved for the managed default workflow.")}</p>
                  )}
                </div>
                <div className="flex flex-col gap-2">
                  <label htmlFor="user_path" className="text-sm font-medium">{t("User Path")}</label>
                  <Input
                    id="user_path"
                    placeholder={t("team/alpha or /team/alpha")}
                    value={formData.scope_user_path}
                    onChange={(e) => setFormData({ ...formData, scope_user_path: e.target.value })}
                  />
                </div>
              </div>

              <div className="flex flex-col gap-2">
                <label htmlFor="description" className="text-sm font-medium">{t("Description")}</label>
                <textarea
                  id="description"
                  className="field-input min-h-[60px]"
                  placeholder={t("Optional operator note for why this version exists.")}
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                />
              </div>

              <div className="flex flex-col gap-3">
                <h4 className="text-sm font-medium text-foreground border-b pb-2">{t("Features")}</h4>
                <div className="mb-4 my-4 py-4  border bg-muted/10 overflow-hidden shadow-inner">
                  <WorkflowChart workflow={{ ...formData, id: 'preview', version: 0, created_at: '', workflow_hash: '' } as Workflow} />
                </div>
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
                  {[
                    { key: "caching", label: t("Caching"), detail: "Response caching" },
                    { key: "guardrails", label: t("Guardrails"), detail: "Policy evaluation" },
                    { key: "failover", label: t("Failover"), detail: "Automatic retry" },
                    { key: "rate_limit", label: t("Rate Limits"), detail: "Prevent abuse" },
                    { key: "audit", label: t("Audit"), detail: "Store requests" },
                    { key: "usage_tracking", label: t("Usage"), detail: "Track tokens" },
                  ].map((feat) => {
                    const isEnabled = formData.features?.[feat.key as keyof typeof formData.features];
                    return (
                      <ToggleField
                        key={feat.key}
                        label={feat.label}
                        description={feat.detail}
                        size="sm"
                        checked={!!isEnabled}
                        onCheckedChange={() => handleToggleFeature(feat.key as keyof typeof formData.features)}
                        aria-label={`Toggle ${feat.label}`}
                        className={`rounded-lg border p-3 ${isEnabled ? "border-accent bg-accent/5" : "bg-background hover:bg-muted/50"}`}
                      />
                    );
                  })}
                </div>
              </div>

              {formData.features?.guardrails && (
                <div className="flex flex-col gap-3">
                  <div className="flex items-center justify-between border-b pb-2">
                    <h4 className="text-sm font-medium text-foreground">{t("Guardrail Steps")}</h4>
                    <Button type="button" variant="outline" size="sm" onClick={handleAddGuardrailStep}>{t("Add Step")}</Button>
                  </div>

                  {guardrails.length === 0 && (
                    <div className="flex items-start gap-3 bg-info/10 p-4">
                      <AlertCircleIcon className="h-5 w-5 text-info shrink-0 mt-0.5" />
                      <div className="flex flex-col gap-1">
                        <span className="text-sm font-medium text-info">{t("No guardrails found")}</span>
                        <span className="text-sm text-info/80">{t("You can draft this workflow, but you need to create guardrails before they can execute.")}</span>
                      </div>
                    </div>
                  )}

                  <div className="flex flex-col gap-3">
                    {formData.guardrails?.map((step, idx) => (
                      <div key={idx} className="flex items-center gap-3">
                        <div className="flex-1 flex flex-col gap-1.5">
                          <label className="text-xs text-muted-foreground">{t("Guardrail reference")}</label>
                          <select
                            className="field-input h-9"
                            value={step.ref}
                            onChange={(e) => handleUpdateGuardrailStep(idx, "ref", e.target.value)}
                          >
                            <option value="">{t("Select guardrail...")}</option>
                            {guardrails.map(g => (
                              <option key={g.name} value={g.name}>{g.name}</option>
                            ))}
                          </select>
                        </div>
                        <div className="w-24 flex flex-col gap-1.5">
                          <label className="text-xs text-muted-foreground">{t("Step")}</label>
                          <Input
                            type="number"
                            min="0"
                            step="10"
                            className="h-9"
                            value={step.step}
                            onChange={(e) => handleUpdateGuardrailStep(idx, "step", parseInt(e.target.value, 10) || 0)}
                          />
                        </div>
                        <div className="flex flex-col gap-1.5 pt-5">
                          <Button type="button" variant="ghost" size="icon" onClick={() => handleRemoveGuardrailStep(idx)}>
                            <XCircleIcon className="h-5 w-5 text-destructive" />
                          </Button>
                        </div>
                      </div>
                    ))}
                    {(!formData.guardrails || formData.guardrails.length === 0) && (
                      <div className="text-sm text-muted-foreground text-center py-4 border border-dashed rounded-md">{t("No guardrail steps configured yet.")}</div>
                    )}
                  </div>
                </div>
              )}

              {upsertMutation.isError && (
                <p className="text-sm text-destructive">{upsertMutation.error.message}</p>
              )}
            </div>
            <div className="flex justify-end gap-2 mt-4 pt-4 border-t">
              <Button type="button" variant="outline" onClick={() => setFormOpen(false)}>{t("Cancel")}</Button>
              <Button type="submit" disabled={upsertMutation.isPending}>
                {upsertMutation.isPending ? t("Saving...") : t("Save Workflow")}
              </Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>

      <WorkflowDetailDialog
        workflow={detailWorkflow}
        onClose={() => setDetailWorkflow(null)}
      />
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Workflow detail dialog                                             */
/* ------------------------------------------------------------------ */

function WorkflowDetailDialog({
  workflow,
  onClose,
}: {
  workflow: Workflow | null;
  onClose: () => void;
}): JSX.Element {
  return (
    <Dialog open={workflow !== null} onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="sm:max-w-[600px] max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {workflow?.name || t("Workflow")}
          </DialogTitle>
          <DialogDescription>{t("Full configuration detail for this workflow scope.")}</DialogDescription>
        </DialogHeader>

        {workflow && (
          <div className="flex flex-col gap-5 py-2">
            <DetailSection title={t("Workflow")}>
              <DetailRow label={t("ID")} value={workflow.id} mono />
              <DetailRow label={t("Name")} value={workflow.name || "â€”"} />
              <DetailRow label={t("Version")} value={`v${workflow.version}`} />
              <DetailRow label={t("Hash")} value={workflow.workflow_hash ? workflow.workflow_hash.substring(0, 16) : "â€”"} mono />
              <DetailRow label={t("Created")} value={workflow.created_at ? new Date(workflow.created_at).toLocaleString() : "â€”"} />
            </DetailSection>

            {/* Scope */}
            <DetailSection title={t("Scope")}>
              <DetailRow label={t("Provider")} value={workflow.scope_provider || t("All")} />
              <DetailRow label={t("Model")} value={workflow.scope_model || t("All")} />
              <DetailRow label={t("User Path")} value={workflow.scope_user_path || t("All requests")} mono />
              <DetailRow
                label={t("Scope label")}
                value={
                  (!workflow.scope_provider && !workflow.scope_model && !workflow.scope_user_path)
                    ? t("Global (All requests)")
                    : [workflow.scope_user_path && `path: ${workflow.scope_user_path}`, workflow.scope_provider && `provider: ${workflow.scope_provider}`, workflow.scope_model && `model: ${workflow.scope_model}`].filter(Boolean).join(" â€¢ ")
                }
              />
            </DetailSection>

            {/* Description */}
            {workflow.description && (
              <DetailSection title={t("Description")}>
                <p className="text-sm text-muted-foreground leading-relaxed whitespace-pre-wrap">{workflow.description}</p>
              </DetailSection>
            )}

            {/* Features */}
            <DetailSection title={t("Features")}>
              <div className="flex flex-wrap gap-1.5">
                {workflow.features.caching && <FeatureBadge label={t("Caching")} />}
                {workflow.features.guardrails && <FeatureBadge label={t("Guardrails")} color="accent" />}
                {workflow.features.failover && <FeatureBadge label={t("Failover")} color="warning" />}
                {workflow.features.rate_limit && <FeatureBadge label={t("Rate Limits")} />}
                {workflow.features.audit && <FeatureBadge label={t("Audit")} />}
                {workflow.features.usage_tracking && <FeatureBadge label={t("Usage")} />}
              </div>
            </DetailSection>

            {/* Guardrail Steps */}
            {workflow.features.guardrails && workflow.guardrails.length > 0 && (
              <DetailSection title={t("Guardrail Steps")}>
                <div className="flex flex-col gap-1.5">
                  {workflow.guardrails.map((step, idx) => (
                    <div key={idx} className="flex items-center gap-2 rounded-md bg-muted/50 px-3 py-2 text-sm">
                      <span className="flex h-5 w-5 items-center justify-center  bg-accent/10 text-[10px] font-bold text-accent">
                        {idx + 1}
                      </span>
                      <code className="font-mono text-xs">{step.ref}</code>
                      <span className="ml-auto text-[11px] text-muted-foreground">{t("step ")}{step.step}</span>
                    </div>
                  ))}
                </div>
              </DetailSection>
            )}
          </div>
        )}

        <div className="flex justify-end gap-2 pt-2 border-t border-border/40">
          <Button type="button" variant="outline" onClick={onClose}>{t("Close")}</Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function DetailSection({ title, children }: { title: string; children: React.ReactNode }): JSX.Element {
  return (
    <div className="flex flex-col gap-2">
      <h4 className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">{title}</h4>
      <div className="rounded-lg border border-border/40 bg-background/30 p-3">
        {children}
      </div>
    </div>
  );
}

function DetailRow({ label, value, mono }: { label: string; value: string; mono?: boolean }): JSX.Element {
  return (
    <div className="flex items-center justify-between gap-4 py-1 text-sm">
      <span className="text-muted-foreground shrink-0">{label}</span>
      <span className={`text-foreground font-medium text-right max-w-[60%] truncate ${mono ? 'font-mono text-xs' : ''}`}>
        {value}
      </span>
    </div>
  );
}

function FeatureBadge({ label, color }: { label: string; color?: string }): JSX.Element {
  const cls = color === "warning"
    ? "bg-warning/10 text-warning ring-warning/30"
    : color === "accent"
      ? "bg-accent/10 text-accent ring-accent/20"
      : "bg-surface-hover/50 text-muted-foreground ring-border/30";
  return (
    <span className={`inline-flex items-center rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset ${cls}`}>
      {label}
    </span>
  );
}

import { useMemo, useState } from "react";
import { Search, Check } from "lucide-react";
import type { CLIModelField } from "@/lib/api/cli-tools";
import { modelDisplayName, type ModelInventoryItem } from "@/lib/api/models-types";

interface CLIModelFieldGridProps {
  modelFields: CLIModelField[];
  modelOverrides: Record<string, string>;
  fallbackModel: string;
  models: ModelInventoryItem[];
  onChange: (key: string, value: string) => void;
  selectedModels: string[] | undefined;
  onSelectedModelsChange: ((models: string[]) => void) | undefined;
}

function MultiModelSelect({
  allModels,
  selectedModels,
  fallbackModel,
  onModelsChange,
}: {
  allModels: string[];
  selectedModels: string[];
  fallbackModel: string;
  onModelsChange: (models: string[]) => void;
}): JSX.Element {
  const [query, setQuery] = useState("");
  const normalizedQuery = query.trim().toLowerCase();
  const available = useMemo(() => {
    if (!normalizedQuery) return allModels;
    return allModels.filter((m) => m.toLowerCase().includes(normalizedQuery));
  }, [allModels, normalizedQuery]);

  function toggleModel(model: string): void {
    if (selectedModels.includes(model)) {
      onModelsChange(selectedModels.filter((m) => m !== model));
    } else {
      onModelsChange([...selectedModels, model]);
    }
  }

  const empty = allModels.length === 0;

  return (
    <div className="min-w-0 space-y-2">
      <div className="flex items-center gap-2 rounded-lg border border-border bg-background/40 px-3 py-1.5">
        <Search className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <input
          className="min-w-0 flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search models..."
        />
        {selectedModels.length > 0 ? (
          <span className="shrink-0 rounded-full bg-accent/20 px-2 py-0.5 text-[10px] font-medium text-accent">{selectedModels.length}</span>
        ) : null}
      </div>
      {empty ? (
        <p className="rounded-lg border border-dashed border-border bg-background/20 p-3 text-xs text-muted-foreground">
          No models found. Select a fallback model above.
        </p>
      ) : (
        <div className="max-h-48 min-w-0 overflow-y-auto rounded-lg border border-border bg-background/20">
          {available.map((model) => {
            const checked = selectedModels.includes(model);
            return (
              <label
                key={model}
                className={`flex min-w-0 cursor-pointer items-center gap-2 border-b border-border/30 px-3 py-2 text-xs transition last:border-b-0 hover:bg-background/40 ${checked ? "bg-accent/5" : ""}`}
              >
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={() => toggleModel(model)}
                  className="shrink-0 accent-accent"
                />
                <span className="min-w-0 flex-1 truncate font-mono text-foreground">{model}</span>
                {checked ? <Check className="h-3 w-3 shrink-0 text-accent" /> : null}
              </label>
            );
          })}
          {available.length === 0 ? (
            <p className="p-3 text-xs text-muted-foreground">No models match &quot;{query}&quot;. Try a different search term.</p>
          ) : null}
        </div>
      )}
      <p className="text-[10px] leading-4 text-muted-foreground">
        {selectedModels.length === 0
          ? `No models selected. The fallback model "${fallbackModel}" will be used.`
          : `Selected: ${selectedModels.join(", ")}`}
      </p>
    </div>
  );
}

export function CLIModelFieldGrid({ modelFields, modelOverrides, fallbackModel, models, onChange, selectedModels, onSelectedModelsChange }: CLIModelFieldGridProps): JSX.Element | null {
  const modelOptions = useMemo(() => Array.from(new Set(models.map(modelDisplayName).filter(Boolean))).sort(), [models]);

  if (modelFields.length === 0) {
    return null;
  }

  return (
    <div className="min-w-0 space-y-3 md:col-span-3">
      <div className="min-w-0 break-words rounded-lg border border-border bg-background/20 p-3 text-xs leading-5 text-muted-foreground">
        Configure tool-specific model defaults. Empty values use the fallback model <code className="break-all font-mono text-foreground">{fallbackModel || "auto"}</code>.
      </div>
      <div className="grid min-w-0 gap-3 lg:grid-cols-3">
        {modelFields.map((field) => {
          if (field.multi && onSelectedModelsChange) {
            return (
              <div key={field.key} className="min-w-0 space-y-2 rounded-lg border border-border bg-background/25 p-3 md:col-span-3">
                <span className="block truncate text-xs font-medium text-muted-foreground">{field.label}</span>
                <MultiModelSelect
                  allModels={modelOptions}
                  selectedModels={selectedModels ?? []}
                  fallbackModel={fallbackModel}
                  onModelsChange={onSelectedModelsChange}
                />
                {field.description ? <span className="block break-words text-xs leading-5 text-muted-foreground">{field.description}</span> : null}
                <code className="block truncate font-mono text-[10px] text-muted-foreground">{field.key}</code>
              </div>
            );
          }

          const value = modelOverrides[field.key] ?? "";
          return (
            <label key={field.key} className="min-w-0 space-y-2 rounded-lg border border-border bg-background/25 p-3">
              <span className="block truncate text-xs font-medium text-muted-foreground">{field.label}</span>
              <select
                aria-label={field.label}
                className="field-input min-w-0 max-w-full truncate font-mono"
                value={value}
                onChange={(event) => onChange(field.key, event.target.value)}
              >
                <option value="">{field.default_model || fallbackModel || "Use fallback model"}</option>
                {value && !modelOptions.includes(value) ? <option value={value}>{value}</option> : null}
                {modelOptions.map((model) => <option key={`${field.key}:${model}`} value={model}>{model}</option>)}
              </select>
              {field.description ? <span className="block break-words text-xs leading-5 text-muted-foreground">{field.description}</span> : null}
              <code className="block truncate font-mono text-[10px] text-muted-foreground">{field.key}</code>
            </label>
          );
        })}
      </div>
    </div>
  );
}
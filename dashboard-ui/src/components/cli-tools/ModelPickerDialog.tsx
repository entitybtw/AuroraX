import { useMemo, useState } from "react";
import { Check, Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { modelDisplayName, type ModelInventoryItem } from "@/lib/api/models-types";

interface ModelPickerDialogProps {
  open: boolean;
  title: string;
  selectedModel: string;
  models: ModelInventoryItem[];
  onSelect: (model: string) => void;
  onClose: () => void;
}

interface ModelGroup {
  provider: string;
  items: ModelInventoryItem[];
}

function compactModelName(item: ModelInventoryItem): string {
  const modelId = String(item.model?.id ?? (item as { id?: unknown }).id ?? "").trim();
  return modelId || modelDisplayName(item);
}

function groupModels(models: ModelInventoryItem[], query: string): ModelGroup[] {
  const normalizedQuery = query.trim().toLowerCase();
  const groups = models.reduce<Record<string, ModelInventoryItem[]>>((accumulator, item) => {
    const displayName = modelDisplayName(item);
    const searchable = `${displayName} ${item.provider_type ?? ""} ${compactModelName(item)}`.toLowerCase();
    if (normalizedQuery && !searchable.includes(normalizedQuery)) {
      return accumulator;
    }
    const provider = String(item.provider_name ?? item.provider_type ?? "Models").trim() || "Models";
    return { ...accumulator, [provider]: [...(accumulator[provider] ?? []), item] };
  }, {});

  return Object.entries(groups)
    .map(([provider, items]) => ({ provider, items: [...items].sort((a, b) => modelDisplayName(a).localeCompare(modelDisplayName(b))) }))
    .sort((a, b) => a.provider.localeCompare(b.provider));
}

export function ModelPickerDialog({ open, title, selectedModel, models, onSelect, onClose }: ModelPickerDialogProps): JSX.Element | null {
  const [query, setQuery] = useState("");
  const groups = useMemo(() => groupModels(models, query), [models, query]);

  if (!open) {
    return null;
  }

  return (
    <div className="flex min-w-0 max-w-full flex-col overflow-hidden rounded-xl border border-border bg-surface/95 shadow-sm" role="region" aria-label={title}>
      <div className="flex min-w-0 items-center justify-between gap-3 border-b border-border/50 px-3 py-2">
          <div className="min-w-0">
            <h3 className="truncate text-sm font-semibold text-foreground">{title}</h3>
            <p className="break-words text-xs text-muted-foreground">Search and select a model from this gateway's inventory.</p>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="Close model picker">
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="border-b border-border/50 p-3">
          <label className="flex items-center gap-2 rounded-lg border border-border bg-background/40 px-3 py-1.5">
            <Search className="h-4 w-4 text-muted-foreground" />
            <input
              className="min-w-0 flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search provider or model..."
            />
          </label>
        </div>
        <div className="max-h-72 min-w-0 overflow-auto p-3">
          {groups.length === 0 ? (
            <div className="min-w-0 break-words rounded-lg border border-dashed border-border bg-background/20 p-4 text-sm text-muted-foreground">No models match your search. You can still type a custom model manually.</div>
          ) : (
            <div className="min-w-0 space-y-3">
              {groups.map((group) => (
                <section key={group.provider} className="min-w-0 space-y-2">
                  <div className="flex min-w-0 items-center justify-between gap-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    <span className="min-w-0 truncate">{group.provider}</span>
                    <span className="shrink-0">{group.items.length}</span>
                  </div>
                  <div className="grid min-w-0 gap-1.5">
                    {group.items.map((item) => {
                      const value = modelDisplayName(item);
                      const label = compactModelName(item);
                      const selected = value === selectedModel;
                      return (
                        <button
                          key={`${group.provider}:${value}`}
                          type="button"
                          onClick={() => {
                            onSelect(value);
                            onClose();
                          }}
                          title={value}
                          className={`min-w-0 rounded-md border px-2.5 py-1.5 text-left transition ${selected ? "border-accent bg-accent/10" : "border-border bg-background/30 hover:bg-surface-hover"}`}
                        >
                          <span className="flex min-w-0 items-center justify-between gap-2">
                            <span className="min-w-0 truncate font-mono text-xs text-foreground">{label}</span>
                            {selected ? <Check className="h-3.5 w-3.5 shrink-0 text-accent" /> : null}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                </section>
              ))}
            </div>
          )}
      </div>
    </div>
  );
}

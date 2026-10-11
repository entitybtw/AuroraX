import * as React from "react";
import { FilterXIcon, SlidersHorizontalIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SearchInput } from "@/components/ui/search-input";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";


export interface LogFilterValues {
  search: string;
  method: string;
  statusCode: string;
  stream: string;
  requestedModel: string;
  provider: string;
  path: string;
  userPath: string;
  errorType: string;
}

interface LogSidebarFiltersProps {
  filters: LogFilterValues;
  onChange: (filters: LogFilterValues) => void;
  onClear: () => void;
  entryCount?: number;
}

export function LogSidebarFilters({ filters, onChange, onClear, entryCount }: LogSidebarFiltersProps) {
  const set = (key: keyof LogFilterValues, value: string) => onChange({ ...filters, [key]: value });

  const hasFilters = Object.values(filters).some((v) => v !== "");
  // Collapsed by default on mobile; desktop always shows the full panel.
  const [mobileOpen, setMobileOpen] = React.useState(false);

  return (
    <div className="w-full shrink-0 border-b border-border/50 bg-surface/50 p-4 flex flex-col gap-4 md:w-64 md:border-b-0 md:border-r md:overflow-y-auto">
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{t("Filters")}</h3>
        <div className="flex items-center gap-2">
          {entryCount !== undefined && (
            <span className="text-[10px] font-mono text-muted-foreground">{entryCount}{t(" results")}</span>
          )}
          <button
            type="button"
            onClick={() => setMobileOpen((v) => !v)}
            aria-expanded={mobileOpen}
            aria-label={mobileOpen ? t("Hide filters") : t("Show filters")}
            className={cn(
              "flex h-11 items-center gap-1.5 border border-border/50 bg-background px-3 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground md:hidden",
              hasFilters && !mobileOpen && "border-accent/40 text-accent",
            )}
          >
            <SlidersHorizontalIcon className="h-3.5 w-3.5" />
            {mobileOpen ? t("Hide") : t("Filters")}
          </button>
        </div>
      </div>

      <div className={cn("flex flex-col gap-4", !mobileOpen && "hidden md:flex")}>
        <SearchInput
          placeholder={t("Search logs...")}
          value={filters.search}
          onChange={(e) => { set("search", e.target.value); }}
          className="h-11 md:h-9 text-xs bg-background"
        />

        <div className="space-y-3">
          <FilterSelect label={t("Method")} value={filters.method} onChange={(v) => set("method", v)} options={["", "GET", "POST", "PUT", "PATCH", "DELETE"]} displayLabels={["All methods", "GET", "POST", "PUT", "PATCH", "DELETE"]} />
          <FilterSelect label={t("Status")} value={filters.statusCode} onChange={(v) => set("statusCode", v)} options={["", "200", "201", "400", "401", "403", "404", "429", "500", "502", "503", "504"]} displayLabels={["All statuses", "200", "201", "400", "401", "403", "404", "429", "500", "502", "503", "504"]} />
          <FilterSelect label={t("Mode")} value={filters.stream} onChange={(v) => set("stream", v)} options={["", "true", "false"]} displayLabels={["All modes", "Streaming", "Non-streaming"]} />
          <FilterInput label={t("Model")} value={filters.requestedModel} onChange={(v) => set("requestedModel", v)} placeholder={t("e.g. gpt-4")} />
          <FilterInput label={t("Provider")} value={filters.provider} onChange={(v) => set("provider", v)} placeholder={t("e.g. openai")} />
          <FilterInput label={t("Path")} value={filters.path} onChange={(v) => set("path", v)} placeholder={t("e.g. /v1/chat/completions")} />
          <FilterInput label={t("User path")} value={filters.userPath} onChange={(v) => set("userPath", v)} placeholder={t("e.g. /team/alpha")} />
          <FilterInput label={t("Error type")} value={filters.errorType} onChange={(v) => set("errorType", v)} placeholder={t("e.g. rate_limit")} />
        </div>

        {hasFilters && (
          <Button variant="outline" size="sm" onClick={onClear} className="w-full gap-2">
            <FilterXIcon className="h-3.5 w-3.5" />{t("Clear all")}</Button>
        )}
      </div>
    </div>
  );
}

function FilterSelect({ label, value, onChange, options, displayLabels }: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: string[];
  displayLabels: string[];
}) {
  return (
    <label className="block space-y-1">
      <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</span>
      <select
        className="field-input h-11 md:h-9 w-full bg-background text-xs"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      >
        {options.map((opt, i) => (
          <option key={opt} value={opt}>{displayLabels[i]}</option>
        ))}
      </select>
    </label>
  );
}

function FilterInput({ label, value, onChange, placeholder }: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
}) {
  return (
    <label className="block space-y-1">
      <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</span>
      <Input
        className="h-11 md:h-9 text-xs bg-background"
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    </label>
  );
}

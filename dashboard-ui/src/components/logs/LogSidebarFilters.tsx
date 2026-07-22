import { SearchIcon, FilterXIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

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

  return (
    <div className="w-64 shrink-0 border-r border-border/50 bg-surface/50 p-4 flex flex-col gap-4 overflow-y-auto">
      <div className="flex items-center justify-between">
        <h3 className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">Filters</h3>
        {entryCount !== undefined && (
          <span className="text-[10px] font-mono text-muted-foreground">{entryCount} results</span>
        )}
      </div>

      <div className="relative">
        <SearchIcon className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Search logs..."
          value={filters.search}
          onChange={(e) => { set("search", e.target.value); }}
          className="h-8 pl-8 text-xs bg-background"
        />
      </div>

      <div className="space-y-3">
        <FilterSelect label="Method" value={filters.method} onChange={(v) => set("method", v)} options={["", "GET", "POST", "PUT", "PATCH", "DELETE"]} displayLabels={["All methods", "GET", "POST", "PUT", "PATCH", "DELETE"]} />
        <FilterSelect label="Status" value={filters.statusCode} onChange={(v) => set("statusCode", v)} options={["", "200", "201", "400", "401", "403", "404", "429", "500", "502", "503", "504"]} displayLabels={["All statuses", "200", "201", "400", "401", "403", "404", "429", "500", "502", "503", "504"]} />
        <FilterSelect label="Mode" value={filters.stream} onChange={(v) => set("stream", v)} options={["", "true", "false"]} displayLabels={["All modes", "Streaming", "Non-streaming"]} />
        <FilterInput label="Model" value={filters.requestedModel} onChange={(v) => set("requestedModel", v)} placeholder="e.g. gpt-4" />
        <FilterInput label="Provider" value={filters.provider} onChange={(v) => set("provider", v)} placeholder="e.g. openai" />
        <FilterInput label="Path" value={filters.path} onChange={(v) => set("path", v)} placeholder="e.g. /v1/chat/completions" />
        <FilterInput label="User path" value={filters.userPath} onChange={(v) => set("userPath", v)} placeholder="e.g. /team/alpha" />
        <FilterInput label="Error type" value={filters.errorType} onChange={(v) => set("errorType", v)} placeholder="e.g. rate_limit" />
      </div>

      {hasFilters && (
        <Button variant="outline" size="sm" onClick={onClear} className="w-full gap-2">
          <FilterXIcon className="h-3.5 w-3.5" />
          Clear all
        </Button>
      )}
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
        className="field-input h-8 w-full bg-background text-xs"
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
        className="h-8 text-xs bg-background"
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    </label>
  );
}

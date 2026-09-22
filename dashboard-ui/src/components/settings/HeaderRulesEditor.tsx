import { Input } from "@/components/ui/input";
import { PlusIcon, Trash2Icon } from "lucide-react";

export interface HeaderRule {
  name: string;
  mode: string;
  prefix: string;
  length: number;
  value: string;
  values: string[];
  charset?: string;
}

const CHARSETS = [
  { value: "hex", label: "hex (0-9 a-f)" },
  { value: "alphanumeric", label: "alphanumeric (A-Z a-z 0-9)" },
  { value: "digits", label: "digits (0-9)" },
];

function charsetLabel(charset?: string): string {
  switch ((charset || "").toLowerCase()) {
    case "hex":
      return "hex";
    case "digits":
      return "digits";
    default:
      return "alphanumeric";
  }
}

export function emptyHeaderRule(): HeaderRule {
  return { name: "", mode: "passthrough", prefix: "", length: 0, value: "", values: [], charset: "" };
}

/** The canonical, free-tier-safe upstream session rule. */
export function openCodeSessionRule(): HeaderRule {
  return {
    name: "x-opencode-session",
    mode: "map_or_generate",
    prefix: "ses_",
    length: 26,
    value: "",
    values: [],
    charset: "hex",
  };
}

const GENERATE_MODES = new Set(["map", "generate", "map_or_generate"]);

interface HeaderRulesEditorProps {
  headers: HeaderRule[];
  onChange: (headers: HeaderRule[]) => void;
  /** Compact spacing for use inside expanded cards. */
  compact?: boolean;
}

/**
 * Reusable editor for a set of header transformation rules.
 * Mobile-first: all inputs are full-width on small screens and the
 * mode-specific fields wrap instead of overflowing.
 */
export function HeaderRulesEditor({ headers, onChange, compact = false }: HeaderRulesEditorProps): JSX.Element {
  const update = (index: number, patch: Partial<HeaderRule>) => {
    onChange(headers.map((h, i) => (i === index ? { ...h, ...patch } : h)));
  };

  const remove = (index: number) => onChange(headers.filter((_, i) => i !== index));

  const add = () => onChange([...headers, emptyHeaderRule()]);

  return (
    <div className="flex flex-col gap-3">
      {headers.map((hr, idx) => (
        <div
          key={idx}
          className={`rounded-lg border border-border/40 bg-background/50 ${compact ? "p-3" : "p-4"} flex flex-col gap-3`}
        >
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
            <Input
              placeholder="Header name (e.g. x-opencode-session)"
              value={hr.name}
              onChange={(e) => update(idx, { name: e.target.value })}
              className="flex-1 font-mono"
              aria-label="Header name"
            />
            <div className="flex items-center gap-2">
              <select
                value={hr.mode}
                onChange={(e) => update(idx, { mode: e.target.value })}
                className="h-11 md:h-9 flex-1 rounded-lg border border-border/60 bg-surface px-3 text-base md:text-[13px] text-foreground sm:flex-none"
                aria-label="Header mode"
              >
                <option value="map_or_generate">Map or Generate (recommended)</option>
                <option value="map">Map (stable per provider)</option>
                <option value="generate">Generate (fresh each time)</option>
                <option value="passthrough">Passthrough</option>
                <option value="static">Static value</option>
                <option value="random_from_list">Random from list</option>
                <option value="remove">Remove</option>
              </select>
              <button
                type="button"
                onClick={() => remove(idx)}
                className="shrink-0 rounded-lg border border-border/40 p-2.5 text-muted-foreground transition-colors hover:border-destructive/40 hover:bg-destructive/10 hover:text-destructive"
                aria-label={`Remove rule for ${hr.name || "header"}`}
              >
                <Trash2Icon className="h-4 w-4" />
              </button>
            </div>
          </div>

          {GENERATE_MODES.has(hr.mode) && (
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-[6rem_6rem_1fr]">
              <label className="flex flex-col gap-1">
                <span className="text-[11px] font-medium text-muted-foreground">Prefix</span>
                <Input
                  value={hr.prefix}
                  placeholder="ses_"
                  onChange={(e) => update(idx, { prefix: e.target.value })}
                  className="font-mono"
                />
              </label>
              <label className="flex flex-col gap-1">
                <span className="text-[11px] font-medium text-muted-foreground">Length</span>
                <Input
                  type="number"
                  min={1}
                  max={128}
                  value={hr.length || 0}
                  placeholder="26"
                  onChange={(e) => update(idx, { length: parseInt(e.target.value) || 0 })}
                />
              </label>
              <label className="col-span-2 flex flex-col gap-1 sm:col-span-1">
                <span className="text-[11px] font-medium text-muted-foreground">Charset</span>
                <select
                  value={charsetLabel(hr.charset)}
                  onChange={(e) => update(idx, { charset: e.target.value })}
                  className="h-11 md:h-9 rounded-lg border border-border/60 bg-surface px-3 text-base md:text-[13px] text-foreground"
                >
                  {CHARSETS.map((c) => (
                    <option key={c.value} value={c.value}>{c.label}</option>
                  ))}
                </select>
              </label>
              <p className="col-span-2 text-[11px] text-muted-foreground/70 sm:col-span-3">
                Generates e.g.{" "}
                <span className="font-mono text-foreground">
                  {hr.prefix || "ses_"}
                </span>{" "}
                followed by {hr.length || 26} {charsetLabel(hr.charset)} characters.
              </p>
            </div>
          )}

          {hr.mode === "static" && (
            <label className="flex flex-col gap-1">
              <span className="text-[11px] font-medium text-muted-foreground">Value</span>
              <Input
                placeholder="Fixed value to send"
                value={hr.value || ""}
                onChange={(e) => update(idx, { value: e.target.value })}
                className="font-mono"
              />
            </label>
          )}

          {hr.mode === "random_from_list" && (
            <label className="flex flex-col gap-1">
              <span className="text-[11px] font-medium text-muted-foreground">Values (comma-separated)</span>
              <Input
                placeholder="value-one, value-two"
                value={(hr.values || []).join(", ")}
                onChange={(e) =>
                  update(idx, { values: e.target.value.split(",").map((s) => s.trim()).filter(Boolean) })
                }
              />
            </label>
          )}

          {hr.mode === "passthrough" && (
            <p className="text-[12px] text-muted-foreground/70">Original client value is forwarded unchanged.</p>
          )}

          {hr.mode === "remove" && (
            <p className="text-[12px] text-muted-foreground/70">Header is stripped before sending upstream.</p>
          )}
        </div>
      ))}

      <button
        type="button"
        onClick={add}
        className="flex min-h-[44px] items-center justify-center gap-1.5 rounded-lg border border-dashed border-border/60 px-3 py-2 text-[13px] font-medium text-muted-foreground transition-colors hover:border-accent/50 hover:bg-accent/5 hover:text-foreground"
      >
        <PlusIcon className="h-3.5 w-3.5" />
        Add header rule
      </button>
    </div>
  );
}

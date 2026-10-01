import { useEffect, useRef, useState } from "react";
import { CheckSquareIcon, ChevronDownIcon, CopyIcon, PauseIcon, PlayIcon, PlusIcon, SquareIcon, XIcon } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Pill } from "@/components/ui/surface";
import { cn } from "@/lib/utils";
import type { ExitStatus } from "@/lib/api/egress-types";
import { exitForIp, exitState, exitSuccessRate, relativeTime, tierLabel } from "@/lib/egress-view";

/** Literal IPv4 or a compact IPv6 form — providers only ever bind addresses. */
export function isLiteralIp(value: string): boolean {
  const v = value.trim();
  if (v === "") return false;
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(v)) {
    return v.split(".").every((octet) => Number(octet) <= 255);
  }
  return /^[0-9a-fA-F:]+$/.test(v) && v.includes(":") && v.split(":").length <= 9;
}

interface Row {
  ip: string;
  included: boolean;
}

export interface IpListEditorProps {
  /** The addresses that are actually saved (only checked rows). */
  value: string[];
  onChange: (next: string[]) => void;
  label?: string;
  description?: string;
  /** Live egress rows for this provider. With them each address shows its
   *  state, its traffic, and a per-address rotation control. */
  exits?: ExitStatus[];
  /** Turns one address in the rotation on or off immediately, without saving. */
  onExitToggle?: (ip: string, disabled: boolean) => void;
  /** True while a rotation toggle is in flight. */
  togglePending?: boolean;
}

/**
 * The provider's source addresses as a detailed checklist: every known IP is a
 * row, the checkbox says whether it is used, and new ones are added inline.
 * Rows that are unchecked stay visible until the form is saved so a toggle can
 * be undone in the same edit.
 *
 * When live exits are supplied, each row also carries its current state (in
 * use / cooling down / rate limited), its success ratio, and controls to pause
 * it in the rotation or inspect the full detail.
 */
export function IpListEditor({
  value,
  onChange,
  label = "Source IPs",
  description,
  exits,
  onExitToggle,
  togglePending,
}: IpListEditorProps): JSX.Element {
  const [rows, setRows] = useState<Row[]>(() => value.map((ip) => ({ ip, included: true })));
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [expandedIp, setExpandedIp] = useState<string | null>(null);
  const [copiedIp, setCopiedIp] = useState<string | null>(null);
  const emitted = useRef(value.join("\n"));

  const syncFrom = (next: Row[]) => {
    setRows(next);
    const included = next.filter((row) => row.included).map((row) => row.ip);
    emitted.current = included.join("\n");
    onChange(included);
  };

  // The parent may hand back a different list (another provider was opened,
  // the config was saved elsewhere); follow it unless we were the source.
  const incoming = value.join("\n");
  useEffect(() => {
    if (incoming === emitted.current) return;
    emitted.current = incoming;
    setRows(value.map((ip) => ({ ip, included: true })));
  }, [incoming]); // eslint-disable-line react-hooks/exhaustive-deps

  const toggle = (ip: string) =>
    syncFrom(rows.map((row) => (row.ip === ip ? { ...row, included: !row.included } : row)));

  const remove = (ip: string) => {
    setExpandedIp((current) => (current === ip ? null : current));
    syncFrom(rows.filter((row) => row.ip !== ip));
  };

  const add = () => {
    const ip = draft.trim();
    if (ip === "") {
      setError("Enter an address first.");
      return;
    }
    if (!isLiteralIp(ip)) {
      setError(`"${ip}" is not a valid IP address.`);
      return;
    }
    if (rows.some((row) => row.ip.toLowerCase() === ip.toLowerCase())) {
      setError("That address is already in the list.");
      return;
    }
    setError(null);
    setDraft("");
    syncFrom([...rows, { ip, included: true }]);
  };

  const copy = async (ip: string) => {
    try {
      await navigator.clipboard?.writeText(ip);
      setCopiedIp(ip);
      window.setTimeout(() => setCopiedIp((current) => (current === ip ? null : current)), 1500);
    } catch {
      // Clipboard unavailable (insecure context); the address stays selectable.
    }
  };

  const includedCount = rows.filter((row) => row.included).length;
  const liveCount = rows.filter((row) => row.included && exitForIp(exits, row.ip)).length;

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">{label}</span>
        <Pill tone={includedCount > 0 ? "accent" : "muted"}>
          {includedCount} of {rows.length} used
        </Pill>
        {exits && rows.length > 0 ? (
          <Pill tone={liveCount > 0 ? "success" : "muted"}>{liveCount} registered</Pill>
        ) : null}
      </div>

      {rows.length > 0 ? (
        <ul className="flex flex-col gap-1.5">
          {rows.map((row) => {
            const exit = exitForIp(exits, row.ip);
            const state = exit ? exitState(exit) : null;
            const rate = exit ? exitSuccessRate(exit) : null;
            const expanded = expandedIp === row.ip;
            // The registry may normalise an address (IPv6); show the exit's
            // own name only when it is not simply the same address again.
            const exitName = exit && exit.name.toLowerCase() !== row.ip.toLowerCase() ? exit.name : null;
            return (
              <li
                key={row.ip}
                className={cn(
                  "flex flex-col gap-1.5 border border-border/30 bg-background/30 px-2.5 py-2",
                  !row.included && "opacity-60",
                  state && "border-l-2 border-l-accent/40",
                )}
              >
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    role="checkbox"
                    aria-checked={row.included}
                    aria-label={`${row.included ? "Use" : "Not using"} ${row.ip}`}
                    onClick={() => toggle(row.ip)}
                    className="shrink-0 p-0.5 hover:bg-border/20 transition-colors"
                    title={row.included ? "Click to stop saving this address" : "Click to save this address"}
                  >
                    {row.included ? (
                      <CheckSquareIcon className="h-4 w-4 text-accent" />
                    ) : (
                      <SquareIcon className="h-4 w-4 text-muted-foreground" />
                    )}
                  </button>

                  <div className="flex min-w-0 flex-1 flex-col gap-1">
                    <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                      <span
                        className={cn(
                          "min-w-0 break-all font-mono text-[12px] text-foreground",
                          !row.included && "line-through text-muted-foreground",
                        )}
                      >
                        {row.ip}
                      </span>
                      {!row.included ? <Pill tone="muted">dropped on save</Pill> : null}
                      {state ? <Pill tone={state.tone}>{state.label}</Pill> : null}
                      {exit ? <Pill tone="muted">{tierLabel(exit.tier)}</Pill> : null}
                    </div>
                    {exit ? (
                      <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-0.5 text-[10px] text-muted-foreground">
                        <span className="font-mono">
                          {exit.ok}/{exit.requests} ok
                        </span>
                        {rate !== null ? <span>{rate}%</span> : null}
                        {exit.failures > 0 ? <span className="text-warning">{exit.failures} failing</span> : null}
                        {exit.last_at ? <span>last {relativeTime(exit.last_at)}</span> : null}
                        {exit.last_error ? (
                          <span className="max-w-[18rem] truncate text-destructive" title={exit.last_error}>
                            {exit.last_error}
                          </span>
                        ) : null}
                      </div>
                    ) : null}
                  </div>

                  <div className="flex shrink-0 items-center gap-0.5">
                    <button
                      type="button"
                      onClick={() => void copy(row.ip)}
                      className="p-1 hover:bg-border/20 transition-colors"
                      title={`Copy ${row.ip}`}
                      aria-label={copiedIp === row.ip ? `Copied ${row.ip}` : `Copy ${row.ip}`}
                    >
                      <CopyIcon className="h-3.5 w-3.5 text-muted-foreground" />
                    </button>
                    {exit && onExitToggle ? (
                      <button
                        type="button"
                        onClick={() => onExitToggle(row.ip, !exit.disabled)}
                        disabled={togglePending}
                        className="flex items-center gap-1 border border-border/40 px-1.5 py-1 text-[10px] font-bold uppercase tracking-wider text-muted-foreground transition-colors hover:border-accent/40 hover:text-accent disabled:opacity-50"
                        title={exit.disabled ? "Put this address back into the rotation" : "Take this address out of the rotation without saving"}
                        aria-label={exit.disabled ? `Resume ${row.ip} in the rotation` : `Pause ${row.ip} in the rotation`}
                      >
                        {exit.disabled ? <PlayIcon className="h-3 w-3" /> : <PauseIcon className="h-3 w-3" />}
                        {exit.disabled ? "Resume" : "Pause"}
                      </button>
                    ) : null}
                    <button
                      type="button"
                      onClick={() => setExpandedIp((current) => (current === row.ip ? null : row.ip))}
                      className="p-1 hover:bg-border/20 transition-colors"
                      title={expanded ? `Hide details for ${row.ip}` : `Show details for ${row.ip}`}
                      aria-label={expanded ? `Hide details for ${row.ip}` : `Show details for ${row.ip}`}
                      aria-expanded={expanded}
                    >
                      <ChevronDownIcon className={cn("h-3.5 w-3.5 text-muted-foreground transition-transform", expanded && "rotate-180")} />
                    </button>
                    <button
                      type="button"
                      onClick={() => remove(row.ip)}
                      className="p-1 hover:bg-destructive/10 transition-colors"
                      title={`Remove ${row.ip} from the list`}
                      aria-label={`Remove ${row.ip}`}
                    >
                      <XIcon className="h-3.5 w-3.5 text-muted-foreground hover:text-destructive" />
                    </button>
                  </div>
                </div>

                {expanded ? (
                  <dl className="grid grid-cols-1 gap-x-4 gap-y-1 border-t border-border/30 pt-1.5 text-[11px] sm:grid-cols-2">
                    {exitName ? (
                      <div className="flex gap-2">
                        <dt className="w-24 shrink-0 text-muted-foreground">Exit name</dt>
                        <dd className="min-w-0 break-all font-mono text-foreground">{exitName}</dd>
                      </div>
                    ) : null}
                    {exit ? (
                      <>
                        <div className="flex gap-2">
                          <dt className="w-24 shrink-0 text-muted-foreground">Source</dt>
                          <dd className="text-foreground">{exit.source === "extension" ? "extension" : "provider config"}</dd>
                        </div>
                        <div className="flex gap-2">
                          <dt className="w-24 shrink-0 text-muted-foreground">Tier</dt>
                          <dd className="text-foreground">{tierLabel(exit.tier)}</dd>
                        </div>
                        {exit.weight ? (
                          <div className="flex gap-2">
                            <dt className="w-24 shrink-0 text-muted-foreground">Weight</dt>
                            <dd className="text-foreground">{exit.weight}</dd>
                          </div>
                        ) : null}
                        <div className="flex gap-2">
                          <dt className="w-24 shrink-0 text-muted-foreground">Traffic</dt>
                          <dd className="text-foreground">
                            {exit.ok} ok / {exit.requests} requests, {exit.failures} failing
                          </dd>
                        </div>
                        <div className="flex gap-2">
                          <dt className="w-24 shrink-0 text-muted-foreground">Last seen</dt>
                          <dd className="text-foreground">{exit.last_at ? relativeTime(exit.last_at) : "never"}</dd>
                        </div>
                        <div className="flex gap-2">
                          <dt className="w-24 shrink-0 text-muted-foreground">Last outcome</dt>
                          <dd className="min-w-0 break-all text-foreground">{exit.last_outcome || "—"}</dd>
                        </div>
                        {exit.cooldown_until ? (
                          <div className="flex gap-2">
                            <dt className="w-24 shrink-0 text-muted-foreground">Cooldown</dt>
                            <dd className="text-warning">until {relativeTime(exit.cooldown_until)}</dd>
                          </div>
                        ) : null}
                        {exit.last_error ? (
                          <div className="flex gap-2 sm:col-span-2">
                            <dt className="w-24 shrink-0 text-muted-foreground">Last error</dt>
                            <dd className="min-w-0 break-all text-destructive">{exit.last_error}</dd>
                          </div>
                        ) : null}
                      </>
                    ) : (
                      <p className="text-muted-foreground sm:col-span-2">
                        Not registered with the egress rotation yet — it appears after the next runtime refresh.
                      </p>
                    )}
                  </dl>
                ) : null}
              </li>
            );
          })}
        </ul>
      ) : (
        <div className="border border-dashed border-border/40 bg-background/20 px-3 py-3 text-[11px] text-muted-foreground">
          No source IPs yet. Add one below — the gateway rotates between every checked address per attempt.
        </div>
      )}

      <div className="flex items-center gap-2">
        <Input
          type="text"
          placeholder="203.0.113.10"
          value={draft}
          onChange={(e) => {
            setDraft(e.target.value);
            if (error) setError(null);
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              add();
            }
          }}
          className="h-9 flex-1 font-mono text-[12px]"
          aria-label="New source IP"
        />
        <Button type="button" variant="outline" size="sm" onClick={add} className="h-9 shrink-0">
          <PlusIcon className="mr-1 h-3.5 w-3.5" /> Add
        </Button>
      </div>
      {error ? <p className="text-[11px] font-medium text-destructive">{error}</p> : null}
      <p className="text-[11px] leading-relaxed text-muted-foreground">
        {description ??
          "Checked addresses are the ones this provider sends from; the first one is its bind_ip. Unchecked rows are dropped when you save."}
      </p>
    </div>
  );
}

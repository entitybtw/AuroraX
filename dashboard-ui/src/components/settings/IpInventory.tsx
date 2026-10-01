import { useMemo, useState } from "react";
import { ChevronDownIcon, Loader2Icon, PauseIcon, PlayIcon } from "lucide-react";
import { Pill } from "@/components/ui/surface";
import { cn } from "@/lib/utils";
import { useEgressStatus, useToggleEgressExit } from "@/lib/api/useEgress";
import { buildIpInventory, inventoryState, toneDotClass } from "@/lib/egress-view";
import type { IpInventoryRow } from "@/lib/egress-view";

/**
 * Every source address the gateway can send from, across all providers, in one
 * list: one row per address, the providers that use it, its state, its traffic,
 * and a switch that takes it out of (or back into) every rotation at once.
 *
 * An address is only as healthy as its sickest user, so the state shown is the
 * worst one reported by the providers sharing it.
 */
export function IpInventory(): JSX.Element | null {
  const [open, setOpen] = useState(true);
  const { data, isLoading } = useEgressStatus();
  const toggle = useToggleEgressExit();
  const rows = useMemo(() => buildIpInventory(data?.providers), [data]);

  if (!isLoading && rows.length === 0) return null;

  const attention = rows.filter((row) => ["warning", "danger"].includes(inventoryState(row).tone)).length;
  const off = rows.filter((row) => inventoryState(row).tone === "muted").length;

  const toggleRow = (row: IpInventoryRow): void => {
    // Any provider still using it means "pause"; otherwise this is a resume.
    const disabled = row.used.some((use) => !use.exit.disabled);
    for (const use of row.used) {
      toggle.mutate({ provider: use.provider, exit: use.exit.name, disabled });
    }
  };

  return (
    <div className="border border-border/50 bg-background/30">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        className="flex w-full flex-wrap items-center gap-2 px-3 py-2.5 text-left transition-colors hover:bg-surface-hover/30"
      >
        <span className="text-[11px] font-bold uppercase tracking-wider text-accent">Source IP inventory</span>
        <Pill tone="muted">{isLoading && rows.length === 0 ? "…" : `${rows.length} ${rows.length === 1 ? "address" : "addresses"}`}</Pill>
        {attention > 0 ? <Pill tone="warning">{attention} need attention</Pill> : null}
        {off > 0 ? <Pill tone="warning">{off} off</Pill> : null}
        {isLoading ? <Loader2Icon className="h-3 w-3 animate-spin text-muted-foreground" /> : null}
        <ChevronDownIcon className={cn("ml-auto h-3.5 w-3.5 text-muted-foreground transition-transform", open && "rotate-180")} />
      </button>

      {open ? (
        <div className="flex flex-col gap-2 border-t border-border/40 px-3 py-2.5">
          <ul className="flex flex-col gap-1">
            {rows.map((row) => {
              const state = inventoryState(row);
              const requests = row.used.reduce((sum, use) => sum + use.exit.requests, 0);
              const ok = row.used.reduce((sum, use) => sum + use.exit.ok, 0);
              const failures = row.used.reduce((sum, use) => sum + use.exit.failures, 0);
              const paused = !row.used.some((use) => !use.exit.disabled);
              return (
                <li
                  key={row.ip}
                  className="flex flex-wrap items-center gap-2 border border-border/30 bg-surface/40 px-2.5 py-1.5"
                >
                  <span className={cn("h-1.5 w-1.5 shrink-0 rounded-full", toneDotClass(state.tone))} />
                  <span className="break-all font-mono text-[11px] font-semibold text-foreground">{row.ip}</span>
                  <Pill tone={state.tone}>{state.label}</Pill>
                  {row.used.map((use) => (
                    <Pill key={use.provider} tone="muted">
                      {use.provider}
                    </Pill>
                  ))}
                  <div className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-[10px] text-muted-foreground">
                    <span className="font-mono">
                      {ok}/{requests} ok
                    </span>
                    {failures > 0 ? <span className="text-warning">{failures} failing</span> : null}
                  </div>
                  <button
                    type="button"
                    onClick={() => toggleRow(row)}
                    disabled={toggle.isPending}
                    className="ml-auto flex items-center gap-1 border border-border/40 px-1.5 py-1 text-[10px] font-bold uppercase tracking-wider text-muted-foreground transition-colors hover:border-accent/40 hover:text-accent disabled:opacity-50"
                    title={`${paused ? "Put" : "Take"} this address ${paused ? "back into" : "out of"} the rotation of every provider using it`}
                    aria-label={`${paused ? "Resume" : "Pause"} ${row.ip} everywhere`}
                  >
                    {paused ? <PlayIcon className="h-3 w-3" /> : <PauseIcon className="h-3 w-3" />}
                    {paused ? "Resume" : "Pause"}
                  </button>
                </li>
              );
            })}
          </ul>
          <p className="text-[10px] leading-relaxed text-muted-foreground">
            Addresses are grouped across providers: pausing one takes it out of every provider that sends through it.
            The checkbox in a provider's settings decides whether the address is kept at all — this switch only pauses it.
          </p>
        </div>
      ) : null}
    </div>
  );
}

import { useState } from "react";
import { ChevronDownIcon, Loader2Icon } from "lucide-react";
import { Pill } from "@/components/ui/surface";
import { cn } from "@/lib/utils";
import { useEgressStatus, useToggleEgressExit } from "@/lib/api/useEgress";
import type { ExitStatus } from "@/lib/api/egress-types";
import { exitDestination, exitState, sourceLabel, tierLabel } from "@/lib/egress-view";

function ExitRow({
  exit,
  provider,
  pending,
  onToggle,
}: {
  exit: ExitStatus;
  provider: string;
  pending: boolean;
  onToggle: (exit: string, disabled: boolean) => void;
}): JSX.Element {
  const state = exitState(exit);
  const destination = exitDestination(exit);
  return (
    <li
      className={cn(
        "flex flex-wrap items-start gap-2 border border-border/30 bg-background/30 px-3 py-2",
        exit.disabled && "opacity-60",
      )}
    >
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex min-w-0 flex-wrap items-center gap-1.5">
          <span className="truncate font-mono text-[11px] font-semibold text-foreground" title={exit.name}>
            {destination || exit.name}
          </span>
          <Pill tone="muted">{tierLabel(exit.tier)}</Pill>
          <Pill tone={sourceLabel(exit.source) === "extension" ? "accent" : "muted"}>{sourceLabel(exit.source)}</Pill>
          <Pill tone={state.tone}>{state.label}</Pill>
        </div>
        <div className="flex flex-wrap items-center gap-3 text-[10px] text-muted-foreground">
          {destination !== exit.name ? <span className="font-mono">{exit.name}</span> : null}
          <span>
            {exit.ok}/{exit.requests} ok
          </span>
          {exit.failures > 0 ? <span className="text-warning">{exit.failures} failing</span> : null}
          {exit.last_error ? (
            <span className="truncate text-destructive" title={exit.last_error}>
              {exit.last_error}
            </span>
          ) : null}
        </div>
      </div>
      <button
        type="button"
        onClick={() => onToggle(exit.name, !exit.disabled)}
        disabled={pending}
        className="shrink-0 border border-border/40 px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-muted-foreground transition-colors hover:border-accent/40 hover:text-accent disabled:opacity-50"
        title={exit.disabled ? `Enable this exit for ${provider}` : `Turn this exit off for ${provider}`}
      >
        {exit.disabled ? "Enable" : "Disable"}
      </button>
    </li>
  );
}

export interface ProviderEgressPanelProps {
  provider: string;
}

/**
 * The per-provider exit drill-down: every source address and extension exit
 * behind a provider, whether the operator turned it off, whether it is
 * cooling down, and the last error it reported.
 */
export function ProviderEgressPanel({ provider }: ProviderEgressPanelProps): JSX.Element {
  const [open, setOpen] = useState(false);
  const { data, isLoading, error } = useEgressStatus();
  const toggle = useToggleEgressExit();

  const status = data?.providers.find((entry) => entry.provider === provider);
  const exits = status?.exits ?? [];
  const disabledCount = exits.filter((exit) => exit.disabled).length;

  const onToggle = (exit: string, disabled: boolean): void => {
    toggle.mutate({ provider, exit, disabled });
  };

  return (
    <div className="mt-2 border-t border-border/30 pt-2">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        className="flex w-full items-center gap-2 text-[11px] font-bold uppercase tracking-wider text-muted-foreground transition-colors hover:text-foreground"
        aria-expanded={open}
      >
        <span>Egress</span>
        <span className="font-mono normal-case tracking-normal">
          {isLoading && !status ? "…" : `${exits.length} exit${exits.length === 1 ? "" : "s"}`}
        </span>
        {disabledCount > 0 ? <Pill tone="warning">{disabledCount} off</Pill> : null}
        <ChevronDownIcon className={cn("h-3.5 w-3.5 ml-auto transition-transform", open && "rotate-180")} />
      </button>

      {open ? (
        <div className="mt-2 space-y-2">
          {error ? (
            <p className="border border-destructive/25 bg-destructive/10 px-3 py-2 font-mono text-[11px] text-destructive">
              {error.message}
            </p>
          ) : null}
          {!isLoading && !error && exits.length === 0 ? (
            <p className="px-1 text-[11px] leading-relaxed text-muted-foreground">
              No exits configured. Add source IPs under Edit → Egress, or let an extension contribute them.
            </p>
          ) : null}
          {exits.length > 0 ? (
            <ul className="space-y-1.5">
              {exits.map((exit) => (
                <ExitRow
                  key={exit.name}
                  exit={exit}
                  provider={provider}
                  pending={toggle.isPending}
                  onToggle={onToggle}
                />
              ))}
            </ul>
          ) : null}
          {status ? (
            <div className="flex items-center gap-2 text-[10px] uppercase tracking-wider text-muted-foreground">
              <span>Strategy</span>
              <span className="font-mono normal-case tracking-normal text-foreground">{status.strategy}</span>
              {isLoading && !status ? <Loader2Icon className="h-3 w-3 animate-spin" /> : null}
            </div>
          ) : null}
          <p className="text-[10px] leading-relaxed text-muted-foreground">
            Each attempt picks one exit from the lowest tier still available. A failed exit is cooled down
            on its own; a rate limit holds back its whole tier until the window ends.
          </p>
        </div>
      ) : null}
    </div>
  );
}

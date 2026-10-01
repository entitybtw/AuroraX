import { useMemo, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CheckSquareIcon, Loader2Icon, SquareIcon } from "lucide-react";
import { Pill } from "@/components/ui/surface";
import { cn } from "@/lib/utils";
import { updateExtensionConfig } from "@/lib/api/extensions";
import {
  bindsTo,
  toggleBindTarget,
  useEgressServerLists,
  type EgressServer,
  type EgressServerList,
} from "@/lib/api/egress-servers";
import { useEgressStatus } from "@/lib/api/useEgress";
import { usePools } from "@/lib/api/usePools";

const EGRESS_MODES = [
  { value: "rotate", label: "Rotate with the source IPs (tier 0)" },
  { value: "fallback", label: "Fallback after the source IPs (tier 1)" },
  { value: "vpn_only", label: "Preferred, source IPs last (tier -1)" },
];

/** "1 endpoint" / "3 endpoints" — the pill reads better in full. */
function plural(count: number, noun: string): string {
  return `${count} ${noun}${count === 1 ? "" : "s"}`;
}

/** One bind target — a pool or a provider — as a checkbox row. */
function BindTarget({
  name,
  kind,
  checked,
  disabled,
  onClick,
}: {
  name: string;
  kind: "pool" | "provider";
  checked: boolean;
  disabled: boolean;
  onClick: (name: string) => void;
}): JSX.Element {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onClick(name)}
      className={cn(
        "flex items-center gap-1.5 border px-2 py-1 text-[11px] transition-colors disabled:opacity-60",
        checked
          ? "border-accent/50 bg-accent/10 text-foreground"
          : "border-border/40 bg-background/30 text-muted-foreground hover:border-border/70",
      )}
      title={`${checked ? "Unbind the endpoints from" : "Bind the endpoints to"} ${name}`}
    >
      {checked ? (
        <CheckSquareIcon className="h-3.5 w-3.5 shrink-0 text-accent" />
      ) : (
        <SquareIcon className="h-3.5 w-3.5 shrink-0" />
      )}
      <span className="truncate font-mono">{name}</span>
      <span className="text-[9px] uppercase tracking-wider text-muted-foreground">{kind}</span>
    </button>
  );
}

/** The endpoint rows one extension contributes. */
function EndpointRows({ servers }: { servers: EgressServer[] }): JSX.Element {
  const [showAll, setShowAll] = useState(false);
  // Kept endpoints first, dropped ones behind them — a dead endpoint stays
  // visible, it is the proof the extension is probing and rotating.
  const listed = [...servers.filter((server) => server.selected), ...servers.filter((server) => !server.selected)];
  const visible = showAll ? listed : listed.slice(0, 12);

  return (
    <div className="flex flex-col gap-1.5">
      <ul className="flex flex-col gap-1">
        {visible.map((server, index) => (
          <li
            key={`${server.host}:${server.port}:${index}`}
            className="flex flex-wrap items-center gap-2 border border-border/30 bg-surface/40 px-2.5 py-1.5"
          >
            <span
              className={cn(
                "h-1.5 w-1.5 shrink-0 rounded-full",
                server.alive ? "bg-success" : "bg-destructive",
              )}
            />
            <span className="min-w-0 max-w-[16rem] truncate text-[11px] text-foreground" title={server.name ?? ""}>
              {server.name ?? `${server.host}:${server.port}`}
            </span>
            <span className="truncate font-mono text-[10px] text-muted-foreground">
              {server.host}:{server.port}
            </span>
            {server.protocol ? <Pill tone="muted">{server.protocol}</Pill> : null}
            <Pill tone={server.alive ? "success" : "danger"}>{server.alive ? "alive" : "dead"}</Pill>
            {server.alive && server.latency_ms ? (
              <span className="text-[10px] text-muted-foreground">{server.latency_ms}ms</span>
            ) : null}
            {server.selected ? <Pill tone="accent">kept</Pill> : null}
          </li>
        ))}
      </ul>
      {listed.length > 12 ? (
        <button
          type="button"
          onClick={() => setShowAll((value) => !value)}
          className="self-start text-[11px] text-accent transition-colors hover:text-accent-hover"
        >
          {showAll ? "Show fewer" : `Show all ${listed.length} endpoints`}
        </button>
      ) : null}
    </div>
  );
}

export interface ExternalIpsPanelProps {
  /** Provider names offered as bind targets next to the pools. */
  providerNames: string[];
}

/**
 * External IP addresses contributed by an egress (VPN) extension: the endpoint
 * list itself, plus where it is bound — pools and providers, checked here and
 * saved straight into the extension's `apply_to`.
 *
 * Only extensions that declare the `egress` feature appear, so this block is
 * the VPN extension's by construction.
 */
export function ExternalIpsPanel({ providerNames }: ExternalIpsPanelProps): JSX.Element | null {
  const lists = useEgressServerLists();
  const pools = usePools();
  const { data: egress } = useEgressStatus();
  const queryClient = useQueryClient();

  const save = useMutation({
    mutationFn: ({ id, config }: { id: string; config: Record<string, string> }) =>
      updateExtensionConfig(id, config),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ["extensions"] });
      queryClient.invalidateQueries({ queryKey: ["extension-servers"] });
      queryClient.invalidateQueries({ queryKey: ["egress", "status"] });
    },
  });

  const poolNames = useMemo(
    () => (pools.data?.pools ?? []).map((pool) => pool.name),
    [pools.data],
  );

  // Live extension exits per provider, so the block can say where the
  // endpoints are actually in the rotation right now.
  const liveByProvider = useMemo(
    () =>
      (egress?.providers ?? [])
        .map((entry) => ({
          provider: entry.provider,
          count: entry.exits.filter((exit) => exit.source === "extension").length,
        }))
        .filter((entry) => entry.count > 0),
    [egress],
  );

  if (lists.length === 0) return null;

  const allTargets = [...poolNames, ...providerNames];

  return (
    <div className="flex flex-col gap-3 border border-border/50 bg-background/30 px-3 py-3 sm:px-4 sm:py-4">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-[11px] font-bold uppercase tracking-wider text-accent">
          External IP addresses
        </span>
        <span className="text-[11px] text-muted-foreground">
          endpoints contributed by the VPN extension
        </span>
        {save.isPending ? <Loader2Icon className="h-3 w-3 animate-spin text-muted-foreground" /> : null}
      </div>

      {save.isError ? (
        <p className="border border-destructive/30 bg-destructive/10 px-2.5 py-1.5 text-[11px] text-destructive">
          {save.error instanceof Error ? save.error.message : "Failed to save the binding."}
        </p>
      ) : null}

      {lists.map((list) => (
        <div key={list.id} className="flex flex-col gap-3 border border-border/40 bg-background/20 px-3 py-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
              {list.name}
            </span>
            <Pill tone="accent">
              {plural(
                list.payload.servers.filter((server) => server.selected).length ||
                  list.payload.servers.length,
                "endpoint",
              )}
            </Pill>
            <Pill tone="success">
              {list.payload.servers.filter((server) => server.alive).length} alive
            </Pill>
            {list.payload.egress_mode ? <Pill tone="muted">{list.payload.egress_mode}</Pill> : null}
            {list.payload.core_kind ? (
              <Pill tone={list.payload.core_kind === "none" ? "warning" : "muted"}>
                {list.payload.core_kind === "none" ? "report-only" : `core: ${list.payload.core_kind}`}
              </Pill>
            ) : null}
          </div>

          <BindSection
            list={list}
            poolNames={poolNames}
            providerNames={providerNames}
            allTargets={allTargets}
            busy={save.isPending}
            onSave={(config) => save.mutate({ id: list.id, config })}
          />

          <div className="flex flex-col gap-1.5">
            <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
              Endpoints
            </span>
            <EndpointRows servers={list.payload.servers} />
          </div>
        </div>
      ))}

      {liveByProvider.length > 0 ? (
        <p className="text-[11px] leading-relaxed text-muted-foreground">
          In the rotation right now:{" "}
          {liveByProvider.map((entry) => `${entry.count} on ${entry.provider}`).join(", ")}. Pause or
          resume a single address from the provider's Egress drill-down below.
        </p>
      ) : (
        <p className="text-[11px] leading-relaxed text-muted-foreground">
          No extension endpoint is in a rotation yet — with <span className="font-mono">core: none</span>{" "}
          the endpoints are listed for review only; a tunnel core makes them usable exits.
        </p>
      )}
    </div>
  );
}

/** The bind checkboxes and the rotation mode for one extension list. */
function BindSection({
  list,
  poolNames,
  providerNames,
  allTargets,
  busy,
  onSave,
}: {
  list: EgressServerList;
  poolNames: string[];
  providerNames: string[];
  allTargets: string[];
  busy: boolean;
  onSave: (config: Record<string, string>) => void;
}): JSX.Element {
  const applyTo = list.payload.apply_to;
  const bound = (name: string): boolean => bindsTo(applyTo, name);

  const toggle = (name: string): void => {
    onSave({ apply_to: toggleBindTarget(applyTo, name, allTargets) });
  };

  return (
    <div className="flex flex-col gap-2 border-t border-border/30 pt-2.5">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
          Bound to
        </span>
        <Pill tone={(applyTo ?? "").trim() === "" ? "warning" : "muted"}>
          {(applyTo ?? "").trim() === "" ? "all pools and providers" : applyTo}
        </Pill>
      </div>

      <div className="flex flex-col gap-1.5">
        <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">Pools</span>
        <div className="flex flex-wrap gap-1.5">
          {poolNames.length > 0 ? (
            poolNames.map((name) => (
              <BindTarget
                key={`pool:${name}`}
                name={name}
                kind="pool"
                checked={bound(name)}
                disabled={busy}
                onClick={toggle}
              />
            ))
          ) : (
            <span className="text-[11px] text-muted-foreground">No pools configured.</span>
          )}
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        <span className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
          Providers
        </span>
        <div className="flex flex-wrap gap-1.5">
          {providerNames.map((name) => (
            <BindTarget
              key={`provider:${name}`}
              name={name}
              kind="provider"
              checked={bound(name)}
              disabled={busy}
              onClick={toggle}
            />
          ))}
        </div>
        <p className="text-[10px] leading-relaxed text-muted-foreground">
          A bound pool reaches every provider that is a member of it. Unchecking writes the remaining
          targets out explicitly, so nothing re-binds by accident.
        </p>
      </div>

      {list.payload.egress_mode !== undefined ? (
        <div className="flex flex-col gap-1.5">
          <label className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
            Where they sit in the rotation
          </label>
          <select
            className="field-input w-full sm:max-w-md"
            value={list.payload.egress_mode}
            disabled={busy}
            onChange={(event) => onSave({ egress_mode: event.target.value })}
          >
            {EGRESS_MODES.map((mode) => (
              <option key={mode.value} value={mode.value}>
                {mode.label}
              </option>
            ))}
          </select>
        </div>
      ) : null}
    </div>
  );
}

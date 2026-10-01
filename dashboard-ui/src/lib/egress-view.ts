import type { ExitStatus, ProviderEgressStatus } from "@/lib/api/egress-types";

/** The palette an exit state maps onto: muted / success / warning / danger. */
export type ExitTone = "muted" | "success" | "warning" | "danger";

/** Short human label for where an exit came from. */
export function sourceLabel(source: string): string {
  if (source === "extension") return "extension";
  return "bind";
}

/** Short human label for the tier an exit sits in. */
export function tierLabel(tier: number): string {
  if (tier < 0) return "preferred";
  if (tier === 0) return "rotation";
  return "fallback";
}

/** The address an exit actually sends traffic through. */
export function exitDestination(exit: ExitStatus): string {
  return (exit.kind === "ip" ? exit.address : exit.proxy) || exit.name;
}

/** The one-line state of an exit, worst condition first. */
export function exitState(exit: ExitStatus): { label: string; tone: ExitTone } {
  if (exit.disabled) return { label: "off", tone: "muted" };
  if (exit.cooldown_until) return { label: "cooling down", tone: "warning" };
  if (exit.tier_limited) return { label: "rate limited", tone: "warning" };
  if (!exit.available) return { label: "unavailable", tone: "danger" };
  return { label: "in use", tone: "success" };
}

/** Success rate as a whole percentage, or null while nothing has been tried. */
export function exitSuccessRate(exit: ExitStatus): number | null {
  if (exit.requests <= 0) return null;
  return Math.round((exit.ok / exit.requests) * 100);
}

/** Background class for the state dot on a compact exit chip. */
export function toneDotClass(tone: ExitTone): string {
  if (tone === "success") return "bg-success";
  if (tone === "warning") return "bg-warning";
  if (tone === "danger") return "bg-destructive";
  return "bg-muted-foreground/50";
}

/** Finds the exit behind a configured source address; the registry normalises
 *  addresses (IPv6 case, `ip.String()`), so compare both name and address. */
export function exitForIp(exits: ExitStatus[] | undefined, ip: string): ExitStatus | undefined {
  if (!exits || exits.length === 0) return undefined;
  const needle = ip.trim().toLowerCase();
  if (needle === "") return undefined;
  return exits.find(
    (exit) =>
      exit.name.toLowerCase() === needle ||
      (exit.address ?? "").toLowerCase() === needle ||
      exitDestination(exit).toLowerCase() === needle,
  );
}

/** One source address and every provider that sends through it. */
export interface IpInventoryRow {
  ip: string;
  used: { provider: string; exit: ExitStatus }[];
}

const severityOf = (tone: ExitTone): number =>
  tone === "danger" ? 3 : tone === "warning" ? 2 : tone === "muted" ? 1 : 0;

/**
 * Every source address across providers, grouped so an address bound to
 * several providers is one row. Extension exits are endpoints, not addresses,
 * so they are left out.
 */
export function buildIpInventory(providers: ProviderEgressStatus[] | undefined): IpInventoryRow[] {
  const byIp = new Map<string, IpInventoryRow>();
  for (const entry of providers ?? []) {
    for (const exit of entry.exits) {
      if (exit.source === "extension") continue;
      const ip = exitDestination(exit);
      if (!ip) continue;
      const row = byIp.get(ip) ?? { ip, used: [] };
      row.used.push({ provider: entry.provider, exit });
      byIp.set(ip, row);
    }
  }
  return [...byIp.values()].sort((a, b) => rowSeverity(b) - rowSeverity(a) || a.ip.localeCompare(b.ip));
}

function rowSeverity(row: IpInventoryRow): number {
  let worst = 0;
  for (const use of row.used) {
    worst = Math.max(worst, severityOf(exitState(use.exit).tone));
  }
  return worst;
}

/** The worst state among the providers using an address: one address shared
 *  by several providers is only as healthy as its sickest user. */
export function inventoryState(row: IpInventoryRow): { label: string; tone: ExitTone } {
  let worst: { label: string; tone: ExitTone } = { label: "in use", tone: "success" };
  for (const use of row.used) {
    const state = exitState(use.exit);
    if (severityOf(state.tone) > severityOf(worst.tone)) worst = state;
  }
  // "off" only when every user paused it; otherwise the address is still
  // sending traffic for somebody and "off" would be a lie.
  if (worst.tone === "muted" && row.used.some((use) => !use.exit.disabled)) {
    return { label: "partly off", tone: "muted" };
  }
  return worst;
}

/** "2m ago" for a timestamp the gateway reported; empty when unknown. */
export function relativeTime(value: string | undefined): string {
  if (!value) return "";
  const at = Date.parse(value);
  if (Number.isNaN(at)) return "";
  const deltaSeconds = Math.round((at - Date.now()) / 1000);
  const past = deltaSeconds <= 0;
  const abs = Math.abs(deltaSeconds);
  let amount: string;
  if (abs < 60) amount = `${abs}s`;
  else if (abs < 3600) amount = `${Math.round(abs / 60)}m`;
  else if (abs < 86400) amount = `${Math.round(abs / 3600)}h`;
  else amount = `${Math.round(abs / 86400)}d`;
  return past ? `${amount} ago` : `in ${amount}`;
}

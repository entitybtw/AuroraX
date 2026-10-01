import { describe, expect, it } from "vitest";
import type { ExitStatus, ProviderEgressStatus } from "@/lib/api/egress-types";
import { buildIpInventory, inventoryState, exitState, exitSuccessRate, relativeTime, exitForIp, type IpInventoryRow } from "./egress-view";

function exit(name: string, overrides: Partial<ExitStatus> = {}): ExitStatus {
  return {
    name,
    source: "config",
    kind: "ip",
    tier: 0,
    address: name,
    available: true,
    disabled: false,
    tier_limited: false,
    failures: 0,
    requests: 0,
    ok: 0,
    ...overrides,
  };
}

function provider(provider: string, exits: ExitStatus[]): ProviderEgressStatus {
  return { provider, strategy: "round_robin", exits };
}

/** The single row a case expects, so assertions stay type-safe. */
function only(rows: IpInventoryRow[]): IpInventoryRow {
  const row = rows[0];
  if (!row) throw new Error(`expected one inventory row, got ${rows.length}`);
  return row;
}

describe("exitState", () => {
  it("reports the worst condition first", () => {
    expect(exitState(exit("1.1.1.1")).label).toBe("in use");
    expect(exitState(exit("1.1.1.1", { disabled: true })).label).toBe("off");
    expect(exitState(exit("1.1.1.1", { cooldown_until: "2030-01-01T00:00:00Z" })).label).toBe("cooling down");
    expect(exitState(exit("1.1.1.1", { tier_limited: true })).label).toBe("rate limited");
    expect(exitState(exit("1.1.1.1", { available: false })).label).toBe("unavailable");
  });
});

describe("exitSuccessRate", () => {
  it("is null until something has been tried", () => {
    expect(exitSuccessRate(exit("1.1.1.1"))).toBeNull();
    expect(exitSuccessRate(exit("1.1.1.1", { requests: 4, ok: 3 }))).toBe(75);
  });
});

describe("relativeTime", () => {
  it("renders a past and an unknown timestamp", () => {
    const past = new Date(Date.now() - 120_000).toISOString();
    expect(relativeTime(past)).toBe("2m ago");
    expect(relativeTime(undefined)).toBe("");
    expect(relativeTime("not a date")).toBe("");
  });
});

describe("exitForIp", () => {
  it("matches a normalised address", () => {
    const exits = [exit("2001:db8::1", { kind: "ip", address: "2001:db8::1" })];
    expect(exitForIp(exits, "2001:DB8::1")?.name).toBe("2001:db8::1");
    expect(exitForIp(exits, "10.0.0.1")).toBeUndefined();
    expect(exitForIp(undefined, "10.0.0.1")).toBeUndefined();
  });
});

describe("buildIpInventory", () => {
  it("groups an address shared by several providers into one row", () => {
    const rows = buildIpInventory([
      provider("zen", [exit("203.0.113.10"), exit("203.0.113.11")]),
      provider("or-main", [exit("203.0.113.10", { requests: 10, ok: 9 })]),
    ]);
    expect(rows).toHaveLength(2);
    expect(only(rows).used.map((use) => use.provider).sort()).toEqual(["or-main", "zen"]);
  });

  it("leaves extension endpoints out and keeps sick addresses on top", () => {
    const rows = buildIpInventory([
      provider("zen", [
        exit("203.0.113.10"),
        exit("203.0.113.11", { available: false }),
        exit("socks5://host:1080", { source: "extension", kind: "proxy", proxy: "socks5://host:1080" }),
      ]),
    ]);
    expect(rows.map((row) => row.ip)).toEqual(["203.0.113.11", "203.0.113.10"]);
  });

  it("reports the worst state among the providers sharing an address", () => {
    // Paused for one provider but still sending for the other.
    const rows = buildIpInventory([
      provider("zen", [exit("203.0.113.10", { disabled: true })]),
      provider("or-main", [exit("203.0.113.10")]),
    ]);
    expect(inventoryState(only(rows)).label).toBe("partly off");
    const allPaused = buildIpInventory([
      provider("zen", [exit("203.0.113.10", { disabled: true })]),
      provider("or-main", [exit("203.0.113.10", { disabled: true })]),
    ]);
    expect(inventoryState(only(allPaused)).label).toBe("off");
    const cooling = buildIpInventory([provider("zen", [exit("203.0.113.10", { cooldown_until: "2030-01-01T00:00:00Z" })])]);
    expect(inventoryState(only(cooling)).label).toBe("cooling down");
  });

  it("is empty without any provider data", () => {
    expect(buildIpInventory(undefined)).toEqual([]);
  });
});

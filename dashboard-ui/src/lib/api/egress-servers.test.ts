import { describe, expect, it } from "vitest";
import { bindsTo, EgressServersPayloadSchema } from "./egress-servers";

describe("bindsTo", () => {
  it("matches the extension-side apply_to rules", () => {
    expect(bindsTo("*", "opencode-zen")).toBe(true);
    expect(bindsTo("opencode-zen", "opencode-zen")).toBe(true);
    expect(bindsTo("OpenCode-Zen", "opencode-zen")).toBe(true);
    expect(bindsTo("opencode-zen, backup", "backup")).toBe(true);
    expect(bindsTo("opencode-zen\nbackup", "backup")).toBe(true);
    expect(bindsTo("zen-*", "zen-backup")).toBe(true);
    expect(bindsTo("zen-*", "backup")).toBe(false);
    expect(bindsTo("opencode-zen", "backup")).toBe(false);
  });

  it("falls back to binding everything when nothing is listed", () => {
    expect(bindsTo(undefined, "opencode-zen")).toBe(true);
    expect(bindsTo("", "opencode-zen")).toBe(true);
    expect(bindsTo("  ,  ", "opencode-zen")).toBe(true);
    expect(bindsTo("*", "")).toBe(true);
  });
});

describe("EgressServersPayloadSchema", () => {
  it("defaults to an empty endpoint list", () => {
    const parsed = EgressServersPayloadSchema.parse({});
    expect(parsed.servers).toEqual([]);
    expect(parsed.apply_to).toBeUndefined();
  });

  it("keeps the flags the pool view renders", () => {
    const parsed = EgressServersPayloadSchema.parse({
      apply_to: "opencode-zen",
      egress_mode: "fallback",
      core_kind: "none",
      servers: [
        { host: "203.0.113.7", port: 443, name: "Prague 01", alive: true, selected: true },
        { host: "203.0.113.8", port: 443, name: "Berlin 02", alive: false, selected: false },
      ],
    });
    expect(parsed.apply_to).toBe("opencode-zen");
    expect(parsed.core_kind).toBe("none");
    expect(parsed.servers.filter((server) => server.selected)).toHaveLength(1);
    const [first, second] = parsed.servers;
    expect(first?.alive).toBe(true);
    expect(second?.alive).toBe(false);
  });
});

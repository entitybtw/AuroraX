import { describe, expect, it } from "vitest";
import { parseProviderNames } from "./provider-names";

describe("parseProviderNames", () => {
  it("splits on commas, spaces, semicolons and newlines", () => {
    expect(parseProviderNames("a, b c;d\ne").names).toEqual(["a", "b", "c", "d", "e"]);
  });

  it("accepts the names the gateway itself uses", () => {
    const { names, invalid } = parseProviderNames("zen, openrouter-backup-2, vllm-cheapvibecode, api.v1");
    expect(names).toEqual(["zen", "openrouter-backup-2", "vllm-cheapvibecode", "api.v1"]);
    expect(invalid).toEqual([]);
  });

  it("drops empty runs and surrounding whitespace", () => {
    expect(parseProviderNames("  a ,,  b  ").names).toEqual(["a", "b"]);
    expect(parseProviderNames("   ").names).toEqual([]);
  });

  it("keeps only the first spelling of a repeated name", () => {
    expect(parseProviderNames("Zen, zen, ZEN").names).toEqual(["Zen"]);
  });

  it("reports tokens that are not identifiers instead of creating them", () => {
    const { names, invalid } = parseProviderNames("good, bad/name, -lead");
    expect(names).toEqual(["good"]);
    expect(invalid).toEqual(["bad/name", "-lead"]);
  });
});

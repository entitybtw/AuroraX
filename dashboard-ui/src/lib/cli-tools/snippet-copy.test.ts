import { describe, expect, it } from "vitest";
import { isRedactedKey, snippetForCopy } from "./snippet-copy";

describe("isRedactedKey", () => {
  it("accepts real keys and the shared placeholder", () => {
    expect(isRedactedKey("sk-real-key-123")).toBe(false);
    expect(isRedactedKey("<AURORA_API_KEY>")).toBe(false);
    expect(isRedactedKey("")).toBe(false);
  });

  it("rejects masked and starred values", () => {
    expect(isRedactedKey("sk-t\u20263456")).toBe(true);
    expect(isRedactedKey("********")).toBe(true);
    expect(isRedactedKey("abc*def")).toBe(true);
  });
});

describe("snippetForCopy", () => {
  const snippet = 'TOKEN=sk-t\u20263456\nALT=<AURORA_API_KEY>';

  it("keeps the mask in placeholder mode", () => {
    expect(
      snippetForCopy(snippet, { mode: "placeholder", apiKey: "sk-full-key" }),
    ).toBe(snippet);
  });

  it("swaps mask and placeholder for the pasted key", () => {
    const out = snippetForCopy(snippet, {
      mode: "paste",
      apiKey: "sk-full-key",
      maskedKey: "sk-t\u20263456",
    });
    expect(out).toBe("TOKEN=sk-full-key\nALT=sk-full-key");
  });

  it("returns the snippet unchanged without a key", () => {
    expect(snippetForCopy(snippet, { mode: "paste", apiKey: "  " })).toBe(snippet);
  });
});

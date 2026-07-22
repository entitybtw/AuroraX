import { describe, expect, it } from "vitest";
import { buildCLIPreviewRequest, compactModelOverrides } from "./model-fields";

describe("compactModelOverrides", () => {
  it("returns undefined for empty overrides", () => {
    expect(compactModelOverrides({})).toBeUndefined();
  });

  it("trims non-empty override keys and values", () => {
    expect(compactModelOverrides({ " KEY ": " provider/model " })).toEqual({
      KEY: "provider/model",
    });
  });

  it("removes empty and whitespace-only override values", () => {
    expect(
      compactModelOverrides({
        KEEP: "provider/model",
        EMPTY: "",
        SPACES: "   ",
      }),
    ).toEqual({ KEEP: "provider/model" });
  });

  it("does not mutate the input object", () => {
    const input = { KEY: " provider/model " };
    const result = compactModelOverrides(input);

    expect(input).toEqual({ KEY: " provider/model " });
    expect(result).toEqual({ KEY: "provider/model" });
  });
});

describe("buildCLIPreviewRequest", () => {
  it("omits model_overrides when no non-empty values remain", () => {
    expect(
      buildCLIPreviewRequest({
        base_url: "http://localhost:8080",
        api_key: "sk-test",
        model: "fallback/model",
        model_overrides: { EMPTY: " " },
      }),
    ).toEqual({
      base_url: "http://localhost:8080",
      api_key: "sk-test",
      model: "fallback/model",
    });
  });

  it("includes compacted model_overrides when present", () => {
    expect(
      buildCLIPreviewRequest({
        base_url: "http://localhost:8080",
        api_key: "sk-test",
        model: "fallback/model",
        model_overrides: { HAIKU: " provider/haiku " },
      }),
    ).toEqual({
      base_url: "http://localhost:8080",
      api_key: "sk-test",
      model: "fallback/model",
      model_overrides: { HAIKU: "provider/haiku" },
    });
  });
});

import { describe, expect, it } from "vitest";
import type { AutoFetchFilter } from "@/lib/api/providers";
import { filterToText, hasAdvancedConditions, resolveAutofetchFilter, textToFilter } from "./autofetch-filter";

// The regression this file guards: an advanced filter renders as "free, ..."
// and parsing that text back turned the "..." marker into a literal substring
// that matched no model, so the provider discovered zero models.
const advanced: AutoFetchFilter = {
  mode: "all",
  conditions: [{ contains: "free" }, { not_contains: "deepseek" }],
};

const containsOnly: AutoFetchFilter = {
  mode: "all",
  conditions: [{ contains: "free" }, { contains: "flash" }],
};

describe("filterToText", () => {
  it("renders contains-only filters as plain substrings", () => {
    expect(filterToText(containsOnly)).toBe("free, flash");
    expect(filterToText({ mode: "all", conditions: [{ contains: "free" }] })).toBe("free");
  });

  it("marks filters the text form cannot express", () => {
    expect(filterToText(advanced)).toBe("free, ...");
    expect(filterToText({ mode: "all", conditions: [{ regex: "^mimo-" }] })).toBe("advanced filter");
    expect(filterToText({ mode: "all", conditions: [] })).toBe("");
    expect(filterToText(null)).toBe("");
  });
});

describe("hasAdvancedConditions", () => {
  it("detects rules outside the text form", () => {
    expect(hasAdvancedConditions(advanced)).toBe(true);
    expect(hasAdvancedConditions(containsOnly)).toBe(false);
    expect(hasAdvancedConditions(null)).toBe(false);
  });
});

describe("textToFilter", () => {
  it("parses substrings and clears on empty input", () => {
    expect(textToFilter("free, flash")).toEqual({ mode: "all", conditions: [{ contains: "free" }, { contains: "flash" }] });
    expect(textToFilter("  ")).toEqual({ mode: "all", conditions: [] });
  });

  it("drops the '...' marker instead of matching it literally", () => {
    expect(textToFilter("free, ...")).toEqual({ mode: "all", conditions: [{ contains: "free" }] });
  });
});

describe("resolveAutofetchFilter", () => {
  it("keeps an untouched advanced filter verbatim", () => {
    expect(resolveAutofetchFilter(advanced, "free, ...")).toEqual(advanced);
    expect(resolveAutofetchFilter(advanced, " free, ... ")).toEqual(advanced);
  });

  it("keeps an untouched contains-only filter verbatim", () => {
    expect(resolveAutofetchFilter(containsOnly, "free, flash")).toEqual(containsOnly);
  });

  it("parses the text only after the operator edits it", () => {
    expect(resolveAutofetchFilter(advanced, "free, flash")).toEqual({
      mode: "all",
      conditions: [{ contains: "free" }, { contains: "flash" }],
    });
  });

  it("clears the filter when the text is emptied", () => {
    expect(resolveAutofetchFilter(advanced, "")).toEqual({ mode: "all", conditions: [] });
  });

  it("parses plain text when nothing is stored yet", () => {
    expect(resolveAutofetchFilter(undefined, "free")).toEqual({ mode: "all", conditions: [{ contains: "free" }] });
    expect(resolveAutofetchFilter(null, "free")).toEqual({ mode: "all", conditions: [{ contains: "free" }] });
  });
});

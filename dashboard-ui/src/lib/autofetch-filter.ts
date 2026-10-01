import type { AutoFetchFilter } from "@/lib/api/providers";

// The provider form edits filters as comma-separated substrings, which can
// only express `contains` rules. Everything else (not_contains, regex, price)
// renders as a trailing ", ..." marker, so the text is a lossy view of the
// stored filter and must never be parsed back unless the operator edited it.

export function filterToText(filter?: AutoFetchFilter | null): string {
  if (!filter?.conditions?.length) return "";
  const contains = filter.conditions
    .map((condition) => condition.contains ?? "")
    .filter((value) => value !== "");
  const advanced = hasAdvancedConditions(filter);
  if (contains.length === 0 && advanced) return "advanced filter";
  const base = contains.join(", ");
  return advanced ? `${base}, ...` : base;
}

export function hasAdvancedConditions(filter?: AutoFetchFilter | null): boolean {
  return Boolean(filter?.conditions?.some((condition) => !condition.contains || condition.contains === ""));
}

/** Parses the comma-separated text back into a filter. An empty string returns
 *  an empty filter object (not null) so the backend clears the stored one. */
export function textToFilter(text: string): AutoFetchFilter {
  const values = text
    .split(",")
    .map((value) => value.trim())
    // "..." is the marker filterToText appends for rules the text cannot
    // show; parsing it back as a substring matches no model ID.
    .filter((value) => value !== "" && value !== "...");
  if (values.length === 0) return { mode: "all", conditions: [] };
  return { mode: "all", conditions: values.map((value) => ({ contains: value })) };
}

/** The filter to submit for the text currently in the form. An untouched field
 *  keeps the stored filter verbatim so advanced rules survive an unrelated
 *  edit of the provider; only an edited text is parsed. */
export function resolveAutofetchFilter(stored: AutoFetchFilter | null | undefined, text: string): AutoFetchFilter {
  const trimmed = text.trim();
  if (stored && trimmed === filterToText(stored).trim()) return stored;
  return textToFilter(trimmed);
}

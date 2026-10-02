/**
 * Splits what the operator typed (or pasted) into provider names.
 *
 * Names arrive as a comma-, semicolon- or whitespace-separated list so a whole
 * batch can be created in one go. Every entry still has to look like an
 * identifier: names end up in URLs, pool members and model selectors, so a
 * stray character is rejected rather than silently turned into two providers.
 */
export interface ParsedProviderNames {
  /** Valid, deduplicated (case-insensitively) names in input order. */
  names: string[];
  /** Tokens that are not valid provider names. */
  invalid: string[];
}

const NAME_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;

export function parseProviderNames(input: string): ParsedProviderNames {
  const seen = new Set<string>();
  const names: string[] = [];
  const invalid: string[] = [];
  for (const token of input.split(/[\s,;]+/)) {
    const name = token.trim();
    if (name === "") continue;
    const key = name.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    if (NAME_PATTERN.test(name)) {
      names.push(name);
    } else {
      invalid.push(name);
    }
  }
  return { names, invalid };
}

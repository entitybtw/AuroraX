/**
 * CLI tool snippet helpers shared by the configurator UI.
 */

/** True for placeholder or masked key values that must not be written out. */
export function isRedactedKey(value: string): boolean {
  if (!value) return false;
  if (value === "<AURORA_API_KEY>") return false;
  return value.includes("\u2026") || value.includes("*");
}

/**
 * Previews mask the API key so screenshots stay safe; when the operator
 * pasted a real key, the copied snippet swaps the mask (or the placeholder)
 * back for that key so it is ready to use.
 */
export function snippetForCopy(
  value: string,
  options: { mode: "placeholder" | "paste"; apiKey?: string | undefined; maskedKey?: string | undefined },
): string {
  const key = options.apiKey?.trim();
  if (options.mode !== "paste" || !key) return value;
  let out = value;
  if (options.maskedKey) out = out.split(options.maskedKey).join(key);
  return out.split("<AURORA_API_KEY>").join(key);
}

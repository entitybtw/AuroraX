/**
 * Number / cost / token formatters extracted from
 * internal/admin/dashboard/static/js/modules/usage.js. These are pure and
 * shared across Overview, Usage, and Audit pages.
 */

const COMPACT_FORMATTER = new Intl.NumberFormat("en-US", {
  notation: "compact",
  maximumFractionDigits: 1,
});

const PLAIN_FORMATTER = new Intl.NumberFormat("en-US", {
  maximumFractionDigits: 0,
});

const COST_FORMATTER = new Intl.NumberFormat("en-US", {
  style: "currency",
  currency: "USD",
  minimumFractionDigits: 2,
  maximumFractionDigits: 4,
});

const COST_FORMATTER_TINY = new Intl.NumberFormat("en-US", {
  style: "currency",
  currency: "USD",
  minimumFractionDigits: 2,
  maximumFractionDigits: 6,
});

export function formatTokens(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) {
    return "—";
  }
  if (Math.abs(value) >= 10_000) {
    return COMPACT_FORMATTER.format(value);
  }
  return PLAIN_FORMATTER.format(value);
}

export function formatTokensExact(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) {
    return "—";
  }
  return PLAIN_FORMATTER.format(value);
}

export function formatCost(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) {
    return "—";
  }
  if (Math.abs(value) > 0 && Math.abs(value) < 0.01) {
    return COST_FORMATTER_TINY.format(value);
  }
  return COST_FORMATTER.format(value);
}

export function formatPercent(value: number | null | undefined, fractionDigits = 1): string {
  if (value === null || value === undefined || !Number.isFinite(value)) {
    return "—";
  }
  return `${value.toFixed(fractionDigits)}%`;
}

export function formatRequests(value: number | null | undefined): string {
  return formatTokens(value);
}

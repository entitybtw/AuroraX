import * as React from "react";
import { formatCost, formatRequests, formatTokens } from "@/lib/format/numbers";
import {
  addDaysToDateKey,
  dateKeyInTimeZone,
  dateKeyToDate,
  dateToDateKey,
  detectBrowserTimeZone,
  loadTimezoneOverride,
} from "@/lib/timezone/timezone";
import type { DailyUsage } from "@/lib/api/usage-types";

export interface ContributionCalendarProps {
  daily: DailyUsage[] | undefined;
  weeks: number;
  isLoading: boolean;
}

interface Cell {
  dateKey: string;
  value: number;
  level: 0 | 1 | 2 | 3 | 4;
}

type CalendarMode = "requests" | "tokens" | "cost";

interface MonthLabel {
  key: string;
  label: string;
  col: number;
  span: number;
}

const DAY_LABELS = ["", "Mon", "", "Wed", "", "Fri", ""];
const MONTH_LABELS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

const MODE_ACCESSOR: { [K in CalendarMode]: (row: DailyUsage) => number } = {
  requests: (row) => row.requests,
  tokens: (row) => row.total_tokens,
  cost: (row) => row.total_cost ?? 0,
};

const MODE_FORMATTER: { [K in CalendarMode]: (value: number) => string } = {
  requests: (v) => `${formatRequests(v)} requests`,
  tokens: (v) => `${formatTokens(v)} tokens`,
  cost: (v) => formatCost(v),
};

const MODE_LABEL: { [K in CalendarMode]: string } = {
  requests: "requests",
  tokens: "tokens",
  cost: "cost",
};

function bucketLevel(value: number, sortedNonZero: number[]): Cell["level"] {
  if (value <= 0 || sortedNonZero.length === 0) return 0;
  const q1 = sortedNonZero[Math.floor(sortedNonZero.length * 0.25)] ?? 0;
  const q2 = sortedNonZero[Math.floor(sortedNonZero.length * 0.5)] ?? 0;
  const q3 = sortedNonZero[Math.floor(sortedNonZero.length * 0.75)] ?? 0;
  if (value <= q1) return 1;
  if (value <= q2) return 2;
  if (value <= q3) return 3;
  return 4;
}

function alignStartToMonday(dateKey: string): string {
  const date = dateKeyToDate(dateKey);
  if (!date) return dateKey;
  const day = date.getUTCDay();
  const diff = day === 0 ? -6 : 1 - day;
  date.setUTCDate(date.getUTCDate() + diff);
  return dateToDateKey(date);
}

function effectiveTz(): string {
  return loadTimezoneOverride() ?? detectBrowserTimeZone();
}

const CALENDAR_MODES: CalendarMode[] = ["requests", "tokens", "cost"];

export function ContributionCalendar({
  daily,
  weeks,
  isLoading,
}: ContributionCalendarProps): JSX.Element {
  const [mode, setMode] = React.useState<CalendarMode>("requests");

  const cells = React.useMemo<Cell[]>(() => {
    const valueByDate = new Map<string, number>();
    const accessor = MODE_ACCESSOR[mode];
    for (const d of daily ?? []) {
      valueByDate.set(d.date, accessor(d));
    }

    const today = dateKeyInTimeZone(new Date(), effectiveTz());
    const totalDays = weeks * 7;
    const startKey = alignStartToMonday(addDaysToDateKey(today, -(totalDays - 1)));

    const sortedNonZero = Array.from(valueByDate.values())
      .filter((value) => value > 0)
      .toSorted((a, b) => a - b);

    const out: Cell[] = [];
    for (let i = 0; i < totalDays; i += 1) {
      const dateKey = addDaysToDateKey(startKey, i);
      const value = valueByDate.get(dateKey) ?? 0;
      out.push({ dateKey, value, level: bucketLevel(value, sortedNonZero) });
    }
    return out;
  }, [daily, mode, weeks]);

  const grid = React.useMemo<Cell[][]>(() => {
    // Group into columns of 7 (a full week per column).
    const cols: Cell[][] = [];
    for (let i = 0; i < cells.length; i += 7) {
      cols.push(cells.slice(i, i + 7));
    }
    return cols;
  }, [cells]);

  const monthLabels = React.useMemo<MonthLabel[]>(() => {
    const labels: Omit<MonthLabel, "span">[] = [];
    const seen = new Set<string>();
    grid.forEach((week, col) => {
      const firstOfMonth = week.find((cell) => {
        const date = dateKeyToDate(cell.dateKey);
        return date ? date.getUTCDate() === 1 : false;
      });
      const labelCell = col === 0 ? week[0] : firstOfMonth;
      const date = labelCell ? dateKeyToDate(labelCell.dateKey) : null;
      if (!date) return;
      const key = `${date.getUTCFullYear()}-${date.getUTCMonth()}`;
      if (seen.has(key)) return;
      seen.add(key);
      labels.push({ key, label: MONTH_LABELS[date.getUTCMonth()] ?? "", col });
    });
    return labels.map((label, index) => ({
      ...label,
      span: Math.max(1, (labels[index + 1]?.col ?? grid.length) - label.col),
    }));
  }, [grid]);

  const total = React.useMemo(() => cells.reduce((sum, cell) => sum + cell.value, 0), [cells]);

  return (
    <section className="border border-border/40 bg-surface">
      <header className="p-6 flex flex-wrap items-start justify-between gap-3 border-b border-border/40">
        <div>
          <div className="text-[10px] font-bold tracking-widest uppercase text-accent mb-2">[ Activity ]</div>
          <h2 className="font-serif text-lg font-normal text-foreground">Contribution Calendar</h2>
          <p className="mt-1 text-xs text-muted-foreground">Daily {MODE_LABEL[mode]} over the last {weeks} weeks.</p>
        </div>
        {isLoading ? (
          <span className="text-xs text-muted-foreground">Loading…</span>
        ) : (
          <div className="inline-flex border border-border bg-background/40 p-0.5" aria-label="Activity metric">
            {CALENDAR_MODES.map((entry) => (
              <button
                key={entry}
                type="button"
                onClick={() => setMode(entry)}
                className={
                  entry === mode
                    ? "bg-accent px-3 py-1 text-[11px] font-bold text-accent-foreground"
                    : "px-3 py-1 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
                }
              >
                {entry === "cost" ? "Cost" : entry[0]!.toUpperCase() + entry.slice(1)}
              </button>
            ))}
          </div>
        )}
      </header>
      <div className="p-6">
        <div className="grid grid-cols-[2rem_minmax(0,1fr)] gap-2">
          <div className="mt-5 grid grid-rows-7 gap-[3px] text-[10px] leading-3 text-muted-foreground">
            {DAY_LABELS.map((label, index) => (
              <span key={`${label}-${index}`}>{label}</span>
            ))}
          </div>
          <div className="overflow-x-auto pb-1" style={{ width: "100%" }}>
            <div
              className="mb-1 flex gap-[3px] text-[10px] text-muted-foreground w-full justify-between"
            >
              {monthLabels.map((label) => (
                <span
                  key={label.key}
                  className="truncate block"
                  style={{ width: `calc((100% / ${grid.length}) * ${label.span})` }}
                >
                  {label.label}
                </span>
              ))}
            </div>
            <div
              role="grid"
              aria-label={`Daily ${MODE_LABEL[mode]} activity`}
              className="flex gap-[3px] justify-between w-full"
            >
              {grid.map((col) => (
                <div key={col[0]?.dateKey} role="row" className="flex flex-col gap-[3px] flex-1">
                  {col.map((cell) => (
                    <div
                      key={cell.dateKey}
                      role="gridcell"
                      title={`${cell.dateKey}: ${MODE_FORMATTER[mode](cell.value)}`}
                      aria-label={`${cell.dateKey}: ${MODE_FORMATTER[mode](cell.value)}`}
                      className="aspect-square w-full ring-1 ring-[var(--border-subtle)] transition-transform hover:scale-125"
                      style={{ background: `var(--cal-level-${cell.level})` }}
                    />
                  ))}
                </div>
              ))}
            </div>
          </div>
        </div>
        <div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-[10px] text-muted-foreground border-t border-border/40 pt-4">
          <span>{MODE_FORMATTER[mode](total)} in the visible window</span>
          <div className="flex items-center gap-1">
            <span>Less</span>
            {[0, 1, 2, 3, 4].map((lvl) => (
              <span
                key={lvl}
                className="h-3 w-3 ring-1 ring-[var(--border-subtle)]"
                style={{ background: `var(--cal-level-${lvl})` }}
              />
            ))}
            <span>More</span>
          </div>
        </div>
      </div>
    </section>
  );
}

import { useMemo, useState } from "react";
import {
  addDaysToDateKey,
  dateKeyInTimeZone,
  detectBrowserTimeZone,
  loadTimezoneOverride,
} from "@/lib/timezone/timezone";

/**
 * Minimal port of internal/admin/dashboard/static/js/modules/date-picker.js.
 * The legacy module supports preset windows ("7d", "30d", "90d") plus a
 * custom range picker. Phase 3 ships the preset selector; the calendar
 * grid lands in Phase 7 with the Usage page.
 */

export type DateRangePreset = "7d" | "30d" | "90d" | "custom";

export interface DateRangeState {
  preset: DateRangePreset;
  startDate: string;
  endDate: string;
  days: number;
}

const PRESET_DAYS: Record<Exclude<DateRangePreset, "custom">, number> = {
  "7d": 7,
  "30d": 30,
  "90d": 90,
};

function effectiveTimeZone(): string {
  const override = loadTimezoneOverride();
  if (override) {
    return override;
  }
  return detectBrowserTimeZone();
}

function presetRange(preset: Exclude<DateRangePreset, "custom">): DateRangeState {
  const days = PRESET_DAYS[preset];
  const today = dateKeyInTimeZone(new Date(), effectiveTimeZone());
  const start = addDaysToDateKey(today, -(days - 1));
  return { preset, startDate: start, endDate: today, days };
}

export interface UseDateRangeResult extends DateRangeState {
  setPreset(preset: Exclude<DateRangePreset, "custom">): void;
  setCustom(start: string, end: string): void;
}

export function useDateRange(initial: Exclude<DateRangePreset, "custom"> = "30d"): UseDateRangeResult {
  const [state, setState] = useState<DateRangeState>(() => presetRange(initial));

  return useMemo<UseDateRangeResult>(
    () => ({
      ...state,
      setPreset(preset) {
        setState(presetRange(preset));
      },
      setCustom(start, end) {
        if (!start || !end) {
          return;
        }
        // Approximate day count for use as the `days` query param fallback.
        const startMs = Date.parse(`${start}T00:00:00Z`);
        const endMs = Date.parse(`${end}T00:00:00Z`);
        const dayMs = 24 * 60 * 60 * 1000;
        const days =
          Number.isFinite(startMs) && Number.isFinite(endMs)
            ? Math.max(1, Math.round((endMs - startMs) / dayMs) + 1)
            : 30;
        setState({ preset: "custom", startDate: start, endDate: end, days });
      },
    }),
    [state],
  );
}

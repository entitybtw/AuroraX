/**
 * Port of internal/admin/dashboard/static/js/modules/timezone.js.
 *
 * Pure helpers extracted from the legacy Alpine mixin: date-key math,
 * offset labels, formatter caching. Override persistence stays on the
 * legacy localStorage key aurora_timezone_override so flipping
 * preserves the operator's preference.
 */

export const DEFAULT_TIMEZONE = "UTC";
export const TIMEZONE_STORAGE_KEY = "aurora_timezone_override";

const formatterCache = new Map<string, Intl.DateTimeFormat>();
const supportedTimeZoneCache = new Map<string, boolean>();

function pad(value: number | string): string {
  return String(value).padStart(2, "0");
}

function formatterCacheKey(locale: string, options: Intl.DateTimeFormatOptions): string {
  return locale + "|" + JSON.stringify(options);
}

export function getCachedFormatter(
  locale: string,
  options: Intl.DateTimeFormatOptions,
): Intl.DateTimeFormat {
  const key = formatterCacheKey(locale, options);
  const cached = formatterCache.get(key);
  if (cached) {
    return cached;
  }
  const formatter = new Intl.DateTimeFormat(locale, options);
  formatterCache.set(key, formatter);
  return formatter;
}

export function isSupportedTimeZone(zone: string | null | undefined): boolean {
  if (!zone) {
    return false;
  }
  const cached = supportedTimeZoneCache.get(zone);
  if (cached !== undefined) {
    return cached;
  }
  try {
    getCachedFormatter("en-US", { timeZone: zone }).format(new Date());
    supportedTimeZoneCache.set(zone, true);
    return true;
  } catch {
    supportedTimeZoneCache.set(zone, false);
    return false;
  }
}

export function detectBrowserTimeZone(): string {
  try {
    const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (isSupportedTimeZone(zone)) {
      return zone;
    }
  } catch {
    // fall through to UTC
  }
  return DEFAULT_TIMEZONE;
}

interface PartsByType {
  year?: string;
  month?: string;
  day?: string;
  hour?: string;
  minute?: string;
  second?: string;
  timeZoneName?: string;
}

function partsToMap(parts: Intl.DateTimeFormatPart[]): PartsByType {
  const map: PartsByType = {};
  parts.forEach((part) => {
    (map as Record<string, string>)[part.type] = part.value;
  });
  return map;
}

export function dateKeyInTimeZone(date: Date, timeZone: string): string {
  const zone = isSupportedTimeZone(timeZone) ? timeZone : DEFAULT_TIMEZONE;
  const map = partsToMap(
    getCachedFormatter("en-CA", {
      timeZone: zone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).formatToParts(date),
  );
  return `${map.year}-${map.month}-${map.day}`;
}

export function formatTimestampInTimeZone(
  ts: number | string | Date | null | undefined,
  timeZone: string,
): string {
  if (ts === null || ts === undefined) {
    return "-";
  }
  const date = ts instanceof Date ? ts : new Date(ts);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }

  const zone = isSupportedTimeZone(timeZone) ? timeZone : DEFAULT_TIMEZONE;
  const map = partsToMap(
    getCachedFormatter("en-CA", {
      timeZone: zone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hourCycle: "h23",
    }).formatToParts(date),
  );

  return `${map.year}-${map.month}-${map.day} ${map.hour}:${map.minute}:${map.second}`;
}

export function dateKeyToDate(key: string | null | undefined): Date | null {
  if (!key) {
    return null;
  }
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(key);
  if (!match || !match[1] || !match[2] || !match[3]) {
    return null;
  }
  return new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])));
}

export function dateToDateKey(date: Date | null | undefined): string {
  if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
    return "";
  }
  return (
    `${date.getUTCFullYear()}-` +
    `${pad(date.getUTCMonth() + 1)}-` +
    `${pad(date.getUTCDate())}`
  );
}

export function addDaysToDateKey(key: string, days: number): string {
  const date = dateKeyToDate(key);
  if (!date) {
    return "";
  }
  date.setUTCDate(date.getUTCDate() + days);
  return dateToDateKey(date);
}

export function startOfMonthDate(date: Date): Date {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 1));
}

export function timeZoneOffsetLabel(zone: string, now: Date = new Date()): string {
  const timeZone = isSupportedTimeZone(zone) ? zone : DEFAULT_TIMEZONE;
  try {
    const map = partsToMap(
      getCachedFormatter("en-US", {
        timeZone,
        hour: "2-digit",
        minute: "2-digit",
        hourCycle: "h23",
        timeZoneName: "longOffset",
      }).formatToParts(now),
    );
    const raw = map.timeZoneName;
    if (!raw) {
      return "UTC+00:00";
    }
    const value = raw.replace("GMT", "UTC");
    return value === "UTC" ? "UTC+00:00" : value;
  } catch {
    return "UTC+00:00";
  }
}

export function timeZoneOffsetMinutes(zone: string, now: Date = new Date()): number {
  const match = /^UTC([+-])(\d{2}):(\d{2})$/.exec(timeZoneOffsetLabel(zone, now));
  if (!match || !match[1] || !match[2] || !match[3]) {
    return 0;
  }
  const minutes = Number(match[2]) * 60 + Number(match[3]);
  return match[1] === "-" ? -minutes : minutes;
}

export function timeZoneOptionLabel(zone: string, now: Date = new Date()): string {
  return `${zone} (${timeZoneOffsetLabel(zone, now)})`;
}

export interface TimeZoneOption {
  value: string;
  label: string;
}

export function listSupportedTimeZones(now: Date = new Date()): string[] {
  let zones: string[] = [];
  try {
    if (typeof Intl.supportedValuesOf === "function") {
      zones = Intl.supportedValuesOf("timeZone");
    }
  } catch {
    zones = [];
  }
  zones = zones.filter((zone) => isSupportedTimeZone(zone));
  zones.sort((left, right) => {
    const offsetDiff =
      timeZoneOffsetMinutes(left, now) - timeZoneOffsetMinutes(right, now);
    if (offsetDiff !== 0) {
      return offsetDiff;
    }
    return left.localeCompare(right);
  });
  return zones;
}

export function buildTimeZoneOptions(
  extras: ReadonlyArray<string | undefined | null> = [],
  now: Date = new Date(),
): TimeZoneOption[] {
  const zones = new Set(listSupportedTimeZones(now));
  for (const extra of extras) {
    if (extra && isSupportedTimeZone(extra)) {
      zones.add(extra);
    }
  }
  const ordered = Array.from(zones).toSorted((left, right) => {
    const offsetDiff =
      timeZoneOffsetMinutes(left, now) - timeZoneOffsetMinutes(right, now);
    if (offsetDiff !== 0) {
      return offsetDiff;
    }
    return left.localeCompare(right);
  });
  return ordered.map((value) => ({ value, label: timeZoneOptionLabel(value, now) }));
}

function safeStorage(): Storage | null {
  try {
    return typeof window !== "undefined" ? window.localStorage : null;
  } catch {
    return null;
  }
}

export function loadTimezoneOverride(): string {
  const storage = safeStorage();
  if (!storage) {
    return "";
  }
  try {
    const saved = storage.getItem(TIMEZONE_STORAGE_KEY) || "";
    return isSupportedTimeZone(saved) ? saved : "";
  } catch {
    return "";
  }
}

export function saveTimezoneOverride(zone: string): void {
  const storage = safeStorage();
  if (!storage) {
    return;
  }
  try {
    if (zone && isSupportedTimeZone(zone)) {
      storage.setItem(TIMEZONE_STORAGE_KEY, zone);
    } else {
      storage.removeItem(TIMEZONE_STORAGE_KEY);
    }
  } catch {
    // Swallow storage errors — in-memory state remains the source of truth.
  }
}

import { describe, expect, it } from "vitest";
import {
  addDaysToDateKey,
  dateKeyInTimeZone,
  dateKeyToDate,
  dateToDateKey,
  detectBrowserTimeZone,
  formatTimestampInTimeZone,
  isSupportedTimeZone,
  startOfMonthDate,
  timeZoneOffsetLabel,
  timeZoneOffsetMinutes,
  timeZoneOptionLabel,
} from "./timezone";

describe("timezone helpers", () => {
  it("isSupportedTimeZone accepts canonical zones", () => {
    expect(isSupportedTimeZone("UTC")).toBe(true);
    expect(isSupportedTimeZone("America/New_York")).toBe(true);
    expect(isSupportedTimeZone("Not/AZone")).toBe(false);
    expect(isSupportedTimeZone("")).toBe(false);
    expect(isSupportedTimeZone(null)).toBe(false);
  });

  it("detectBrowserTimeZone returns a supported zone or UTC", () => {
    const zone = detectBrowserTimeZone();
    expect(isSupportedTimeZone(zone)).toBe(true);
  });

  it("dateKeyInTimeZone returns ISO yyyy-mm-dd", () => {
    const date = new Date(Date.UTC(2026, 0, 15, 23, 30));
    expect(dateKeyInTimeZone(date, "UTC")).toBe("2026-01-15");
    expect(dateKeyInTimeZone(date, "America/New_York")).toBe("2026-01-15");
  });

  it("formatTimestampInTimeZone matches yyyy-mm-dd hh:mm:ss in zone", () => {
    const ts = Date.UTC(2026, 4, 10, 12, 0, 0);
    expect(formatTimestampInTimeZone(ts, "UTC")).toBe("2026-05-10 12:00:00");
    expect(formatTimestampInTimeZone(null, "UTC")).toBe("-");
    expect(formatTimestampInTimeZone(undefined, "UTC")).toBe("-");
    expect(formatTimestampInTimeZone("not-a-date", "UTC")).toBe("-");
  });

  it("date-key arithmetic round-trips and adds days", () => {
    const key = "2026-05-10";
    const date = dateKeyToDate(key);
    expect(date).not.toBeNull();
    expect(dateToDateKey(date)).toBe(key);
    expect(addDaysToDateKey(key, 1)).toBe("2026-05-11");
    expect(addDaysToDateKey(key, -10)).toBe("2026-04-30");
    expect(addDaysToDateKey("garbage", 1)).toBe("");
    expect(dateKeyToDate(null)).toBeNull();
    expect(dateKeyToDate("not-a-key")).toBeNull();
  });

  it("startOfMonthDate normalizes to first of month UTC", () => {
    const date = new Date(Date.UTC(2026, 4, 17));
    const start = startOfMonthDate(date);
    expect(start.getUTCFullYear()).toBe(2026);
    expect(start.getUTCMonth()).toBe(4);
    expect(start.getUTCDate()).toBe(1);
  });

  it("timeZoneOffsetLabel and minutes resolve UTC", () => {
    const now = new Date(Date.UTC(2026, 0, 1, 0, 0, 0));
    expect(timeZoneOffsetLabel("UTC", now)).toBe("UTC+00:00");
    expect(timeZoneOffsetMinutes("UTC", now)).toBe(0);
    expect(timeZoneOffsetLabel("invalid/zone", now)).toBe("UTC+00:00");
  });

  it("timeZoneOptionLabel composes 'zone (offset)'", () => {
    const now = new Date(Date.UTC(2026, 0, 1, 0, 0, 0));
    expect(timeZoneOptionLabel("UTC", now)).toBe("UTC (UTC+00:00)");
  });
});

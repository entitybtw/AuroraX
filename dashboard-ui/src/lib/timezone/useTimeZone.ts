import { useEffect, useMemo, useState } from "react";
import {
  buildTimeZoneOptions,
  DEFAULT_TIMEZONE,
  detectBrowserTimeZone,
  isSupportedTimeZone,
  loadTimezoneOverride,
  saveTimezoneOverride,
  timeZoneOptionLabel,
  type TimeZoneOption,
} from "./timezone";

/**
 * Hook mirroring the legacy timezone Alpine mixin. Tracks the detected
 * browser zone and the operator override (persisted to
 * aurora_timezone_override). The list of selectable zones is resolved
 * lazily so the initial render does not pay the Intl.supportedValuesOf
 * cost — match the legacy ensureTimezoneOptions() behavior.
 */
export interface UseTimeZoneState {
  detected: string;
  override: string;
  effective: string;
  effectiveLabel: string;
  detectedLabel: string;
  isOverridden: boolean;
  options: TimeZoneOption[];
  setOverride(zone: string): void;
  clearOverride(): void;
}

export function useTimeZone(): UseTimeZoneState {
  const [detected] = useState<string>(() => detectBrowserTimeZone());
  const [override, setOverrideState] = useState<string>(() => loadTimezoneOverride());
  const [optionsLoaded, setOptionsLoaded] = useState<boolean>(false);

  useEffect(() => {
    if (optionsLoaded) {
      return;
    }
    const handle = setTimeout(() => setOptionsLoaded(true), 0);
    return () => clearTimeout(handle);
  }, [optionsLoaded]);

  const effective = override || detected || DEFAULT_TIMEZONE;

  const options = useMemo<TimeZoneOption[]>(() => {
    if (!optionsLoaded) {
      return [];
    }
    return buildTimeZoneOptions([DEFAULT_TIMEZONE, detected, override]);
  }, [optionsLoaded, detected, override]);

  return {
    detected,
    override,
    effective,
    effectiveLabel: timeZoneOptionLabel(effective),
    detectedLabel: timeZoneOptionLabel(detected),
    isOverridden: Boolean(override) && override !== detected,
    options,
    setOverride(zone: string) {
      const next = isSupportedTimeZone(zone) ? zone : "";
      setOverrideState(next);
      saveTimezoneOverride(next);
    },
    clearOverride() {
      setOverrideState("");
      saveTimezoneOverride("");
    },
  };
}

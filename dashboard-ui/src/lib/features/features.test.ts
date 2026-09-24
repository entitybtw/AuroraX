import { describe, expect, it } from "vitest";
import {
  extractHiddenFeatures,
  featureIdForPath,
  hiddenFeatureSet,
  HIDEABLE_FEATURES,
  isHideableFeature,
  normalizeHiddenFeatures,
} from "./features";

describe("normalizeHiddenFeatures", () => {
  it("trims, lowercases, and de-duplicates", () => {
    expect(normalizeHiddenFeatures([" Pools ", "AUDIT_LOGS", "pools", ""])).toEqual([
      "pools",
      "audit_logs",
    ]);
  });

  it("returns empty for nullish input", () => {
    expect(normalizeHiddenFeatures(undefined)).toEqual([]);
    expect(normalizeHiddenFeatures(null)).toEqual([]);
  });
});

describe("extractHiddenFeatures", () => {
  it("reads settings.ui.hidden_features from config snapshot", () => {
    expect(
      extractHiddenFeatures({ settings: { ui: { hidden_features: ["Pools"] } } }),
    ).toEqual(["pools"]);
  });

  it("returns empty when missing", () => {
    expect(extractHiddenFeatures(undefined)).toEqual([]);
    expect(extractHiddenFeatures({})).toEqual([]);
  });
});

describe("feature catalog", () => {
  it("never exposes overview/settings/models as hideable", () => {
    expect(isHideableFeature("overview")).toBe(false);
    expect(isHideableFeature("settings")).toBe(false);
    expect(isHideableFeature("models")).toBe(false);
    expect(isHideableFeature("pools")).toBe(true);
    expect(isHideableFeature("audit_logs")).toBe(true);
  });

  it("hides only listed features", () => {
    const set = hiddenFeatureSet(["pools", "console", "unknown"]);
    expect(set.has("pools")).toBe(true);
    expect(set.has("console")).toBe(true);
    expect(set.has("unknown")).toBe(true);
    expect(HIDEABLE_FEATURES.every((f) => typeof f.label === "string")).toBe(true);
  });
});

describe("featureIdForPath", () => {
  it("maps route segments", () => {
    expect(featureIdForPath("/admin/dashboard/pools")).toBe("pools");
    expect(featureIdForPath("/admin/dashboard/audit-logs")).toBe("audit_logs");
    expect(featureIdForPath("/admin/dashboard/guide")).toBe("guide");
    expect(featureIdForPath("/admin/dashboard/cli-tools")).toBe("guide");
    expect(featureIdForPath("/admin/dashboard/overview")).toBeUndefined();
    expect(featureIdForPath("/admin/dashboard/settings")).toBeUndefined();
  });
});

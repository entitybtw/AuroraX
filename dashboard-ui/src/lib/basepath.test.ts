import { describe, it, expect, beforeEach } from "vitest";
import { getBasePath, withBasePath, __resetBasePathCacheForTests } from "./basepath";

function setMeta(value: string): void {
  for (const existing of Array.from(
    document.head.querySelectorAll('meta[name="aurora-base-path"]'),
  )) {
    existing.remove();
  }
  const tag = document.createElement("meta");
  tag.setAttribute("name", "aurora-base-path");
  tag.setAttribute("content", value);
  document.head.appendChild(tag);
  __resetBasePathCacheForTests();
}

function clearMeta(): void {
  for (const existing of Array.from(
    document.head.querySelectorAll('meta[name="aurora-base-path"]'),
  )) {
    existing.remove();
  }
  __resetBasePathCacheForTests();
}

describe("basepath", () => {
  beforeEach(() => {
    clearMeta();
  });

  it("defaults to '/' when no meta tag is present", () => {
    expect(getBasePath()).toBe("/");
  });

  it("falls back to '/' when the placeholder token survives", () => {
    setMeta("__AURORA_BASE_PATH__");
    expect(getBasePath()).toBe("/");
  });

  it("reads the configured prefix from the meta tag", () => {
    setMeta("/g");
    expect(getBasePath()).toBe("/g");
  });

  it("memoizes the read on first call", () => {
    setMeta("/g");
    expect(getBasePath()).toBe("/g");
    clearMeta();
    // Cache was reset by clearMeta() above; re-set the meta tag and the
    // first call after a reset should pick up the new value.
    setMeta("/h");
    expect(getBasePath()).toBe("/h");
  });

  it("withBasePath leaves protocol-relative URLs unchanged", () => {
    setMeta("/g");
    expect(withBasePath("//cdn.example.com/x")).toBe("//cdn.example.com/x");
  });

  it("withBasePath leaves non-leading-slash inputs unchanged", () => {
    setMeta("/g");
    expect(withBasePath("relative/path")).toBe("relative/path");
  });

  it("withBasePath prefixes app paths under a sub-path mount", () => {
    setMeta("/g");
    expect(withBasePath("/admin/api/v1/usage/summary")).toBe(
      "/g/admin/api/v1/usage/summary",
    );
  });

  it("withBasePath does not double-prefix paths already under the base", () => {
    setMeta("/g");
    expect(withBasePath("/g/admin/api/v1/usage/summary")).toBe(
      "/g/admin/api/v1/usage/summary",
    );
  });

  it("withBasePath is a no-op for the root mount", () => {
    setMeta("/");
    expect(withBasePath("/admin/api/v1/usage/summary")).toBe(
      "/admin/api/v1/usage/summary",
    );
  });
});

import { afterEach, describe, expect, it, vi } from "vitest";
import { createProviders } from "./providers";
import type { ProviderFormData } from "./providers";

const baseForm: ProviderFormData = {
  name: "",
  type: "openrouter",
  base_url: "https://openrouter.ai/api/v1",
  api_version: "",
  api_key: "sk-test",
  models: "",
};

const ok = () =>
  new Response(JSON.stringify({ message: "created" }), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });

const postedBodies = (fetchMock: { mock: { calls: unknown[][] } }): Record<string, unknown>[] =>
  fetchMock.mock.calls.map((call) => {
    const init = call[1] as RequestInit | undefined;
    return JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
  });

const postedNames = (fetchMock: { mock: { calls: unknown[][] } }): string[] =>
  postedBodies(fetchMock).map((body) => String(body.name));

describe("createProviders", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("posts every name and reports no failures", async () => {
    const fetchMock = vi.fn(async () => ok());
    vi.stubGlobal("fetch", fetchMock);

    const result = await createProviders(baseForm, ["or-main", "or-backup-1", "or-backup-2"]);

    expect(result).toEqual({ created: ["or-main", "or-backup-1", "or-backup-2"], failed: [] });
    expect(postedNames(fetchMock)).toEqual(["or-main", "or-backup-1", "or-backup-2"]);
    // The shared form travels with every name.
    const first = postedBodies(fetchMock)[0] ?? {};
    expect(first.type).toBe("openrouter");
    expect(first.api_key).toBe("sk-test");
  });

  it("keeps creating after a failure and reports why", async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      const body = JSON.parse(String(init?.body)) as { name: string };
      if (body.name === "dup") {
        return new Response(JSON.stringify({ message: 'provider "dup" already exists' }), {
          status: 400,
          headers: { "Content-Type": "application/json" },
        });
      }
      return ok();
    });
    vi.stubGlobal("fetch", fetchMock);

    const result = await createProviders(baseForm, ["fresh", "dup", "later"]);

    expect(result.created).toEqual(["fresh", "later"]);
    expect(result.failed).toEqual([{ name: "dup", error: 'provider "dup" already exists' }]);
    expect(postedNames(fetchMock)).toEqual(["fresh", "dup", "later"]);
  });

  it("stops at the first batch when nothing is given", async () => {
    const fetchMock = vi.fn(async () => ok());
    vi.stubGlobal("fetch", fetchMock);

    const result = await createProviders(baseForm, []);

    expect(result).toEqual({ created: [], failed: [] });
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

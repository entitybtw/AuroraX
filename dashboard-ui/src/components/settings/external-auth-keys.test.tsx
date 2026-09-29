import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SettingsProvider } from "./SettingsContext";
import { ProvidersTab } from "./ProvidersTab";

/**
 * Colored auth keys: every applied extension that provides the
 * "external_auth" feature renders its own key button tinted with that
 * extension's accent (Device Auth -> #E87040, Claude Auth -> #D97757). With
 * both activated the row shows two keys sorted by color; with none the
 * plain accent key stays as the fallback.
 */

const deviceAuthExtension = {
  id: "device-auth",
  name: "Device Auth",
  type: "sidecar",
  builtin: false,
  applied: true,
  version: "1",
  config: {},
  provides: { features: ["external_auth"] },
  ui: { accent: "#E87040" },
};

const claudeAuthExtension = {
  id: "claude-auth",
  name: "Claude Auth",
  type: "sidecar",
  builtin: false,
  applied: true,
  version: "2",
  config: {},
  provides: { features: ["external_auth"] },
  ui: { accent: "#D97757" },
};

const externalAuthProvidersFixture = [
  { name: "claude", flow: "authorization_code", has_token: false, expired: false },
  { name: "zen-pool", flow: "device", has_token: false, expired: false },
];

const providerFixture = [
  {
    name: "claude",
    type: "cli-emulation",
    status: "healthy",
    status_label: "Healthy",
    status_reason: "",
    config: { name: "claude", type: "cli-emulation", enabled: true, auth_method: "external" },
    runtime: {
      name: "claude",
      type: "cli-emulation",
      registered: true,
      registry_initialized: true,
      discovered_model_count: 1,
      using_cached_models: false,
    },
    external_auth_status: { has_token: false, expired: false },
  },
];

const baseConfig = {
  EDITION: "oss",
  fallback: {},
  runtime_features: [],
  settings: {
    ui: { hidden_features: [] as string[] },
  },
};

function makeFetch(extensions: unknown[]) {
  return vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input.toString();
    const json = (body: unknown): Response =>
      new Response(JSON.stringify(body), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    if (url.includes("/sidecar/extensions")) return json({ extensions, presets: [] });
    if (url.includes("/external-auth/providers")) return json(externalAuthProvidersFixture);
    if (url.includes("/providers/status")) {
      return json({
        summary: { total: 1, healthy: 1, degraded: 0, unhealthy: 0, overall_status: "healthy" },
        providers: providerFixture,
      });
    }
    if (url.includes("/dashboard/config")) return json(baseConfig);
    return json({});
  });
}

function renderTab(extensions: unknown[]) {
  vi.stubGlobal("fetch", makeFetch(extensions));
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 5 * 60 * 1000, refetchOnMount: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <SettingsProvider>
        <ProvidersTab />
      </SettingsProvider>
    </QueryClientProvider>,
  );
}

describe("Auth keys colored per enabled extension", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows two tinted keys when both auth extensions are applied", async () => {
    renderTab([deviceAuthExtension, claudeAuthExtension]);
    const deviceKey = await screen.findByTitle("Link Device account");
    const claudeKey = await screen.findByTitle("Link Claude account");
    expect((deviceKey as HTMLElement).style.color.toLowerCase()).toBe("#e87040");
    expect((claudeKey as HTMLElement).style.color.toLowerCase()).toBe("#d97757");
  });

  it("shows a single tinted key when only one auth extension is applied", async () => {
    renderTab([claudeAuthExtension]);
    const claudeKey = await screen.findByTitle("Link Claude account");
    expect((claudeKey as HTMLElement).style.color.toLowerCase()).toBe("#d97757");
    await waitFor(() => {
      expect(screen.queryByTitle("Link Device account")).toBeNull();
    });
  });

  it("falls back to the plain accent key when no auth extension is applied", async () => {
    renderTab([]);
    const key = await screen.findByLabelText("Link account for claude");
    expect(key.className).toContain("text-accent");
  });
});

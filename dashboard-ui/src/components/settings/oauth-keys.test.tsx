import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SettingsProvider } from "./SettingsContext";
import { ProvidersTab } from "./ProvidersTab";

/**
 * Colored OAuth keys: every applied extension that provides the "oauth"
 * feature renders its own key button tinted with that extension's accent
 * (OpenCode OAuth -> #E87040, Claude OAuth -> #D97757). With both activated
 * the row shows two keys sorted by color; with none the legacy accent key
 * stays as the fallback.
 */

const openCodeOAuth = {
  id: "opencode-oauth",
  name: "OpenCode OAuth",
  type: "sidecar",
  builtin: false,
  applied: true,
  version: "1",
  config: {},
  provides: { features: ["oauth"] },
  ui: { accent: "#E87040" },
};

const claudeOAuth = {
  id: "claude-oauth",
  name: "Claude OAuth",
  type: "sidecar",
  builtin: false,
  applied: true,
  version: "2",
  config: {},
  provides: { features: ["oauth"] },
  ui: { accent: "#D97757" },
};

const oauthProvidersFixture = [
  { name: "claude", flow: "authorization_code", has_token: false, expired: false },
  { name: "opencode-zen", flow: "device", has_token: false, expired: false },
];

const providerFixture = [
  {
    name: "claude",
    type: "cli-emulation",
    status: "healthy",
    status_label: "Healthy",
    status_reason: "",
    config: { name: "claude", type: "cli-emulation", enabled: true, auth_method: "oauth" },
    runtime: {
      name: "claude",
      type: "cli-emulation",
      registered: true,
      registry_initialized: true,
      discovered_model_count: 1,
      using_cached_models: false,
    },
    oauth_status: { has_token: false, expired: false },
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
    if (url.includes("/oauth/providers")) return json(oauthProvidersFixture);
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

describe("OAuth keys colored per enabled extension", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows two tinted keys when both OAuth extensions are applied", async () => {
    renderTab([openCodeOAuth, claudeOAuth]);
    const openCodeKey = await screen.findByTitle("Link OAuth account via OpenCode OAuth");
    const claudeKey = await screen.findByTitle("Link OAuth account via Claude OAuth");
    expect((openCodeKey as HTMLElement).style.color.toLowerCase()).toBe("#e87040");
    expect((claudeKey as HTMLElement).style.color.toLowerCase()).toBe("#d97757");
  });

  it("shows a single tinted key when only one OAuth extension is applied", async () => {
    renderTab([claudeOAuth]);
    const claudeKey = await screen.findByTitle("Link OAuth account via Claude OAuth");
    expect((claudeKey as HTMLElement).style.color.toLowerCase()).toBe("#d97757");
    await waitFor(() => {
      expect(screen.queryByTitle("Link OAuth account via OpenCode OAuth")).toBeNull();
    });
  });

  it("falls back to the plain accent key when no OAuth extension is applied", async () => {
    renderTab([]);
    const key = await screen.findByLabelText("Link OAuth account for claude");
    expect(key.className).toContain("text-accent");
  });
});

import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SettingsProvider } from "./SettingsContext";
import { GeneralTab } from "./GeneralTab";
import { useSettings } from "./SettingsContext";

/**
 * Feature visibility: toggling a hideable feature and pressing
 * "Save Feature Visibility" must PUT ui.hidden_features and the config
 * snapshot served afterwards must reflect the change so nav/tabs re-render.
 */

const baseConfig = {
  EDITION: "oss",
  LOGGING_ENABLED: "false",
  USAGE_ENABLED: "false",
  BUDGETS_ENABLED: "false",
  GUARDRAILS_ENABLED: "false",
  CACHE_ENABLED: "false",
  SEMANTIC_CACHE_ENABLED: "false",
  USAGE_PRICING_RECALCULATION_ENABLED: "false",
  IDENTITY_ENABLED: "false",
  IDENTITY_OIDC_ENABLED: "false",
  IDENTITY_OIDC_PROVIDERS: [],
  fallback: {},
  runtime_features: [],
  settings: {
    client: {
      body_size_limit: "10M",
      configured_provider_models_mode: "fallback",
      admin_endpoints_enabled: true,
      swagger_enabled: false,
      pprof_enabled: false,
    },
    caching: { model_refresh_interval_seconds: 3600 },
    logging: { enabled: false },
    observability: { metrics_enabled: false, metrics_endpoint: "/metrics" },
    performance: {
      http_timeout_seconds: 600,
      http_response_header_timeout_seconds: 600,
      workflow_refresh_interval_seconds: 60,
      retry_max_retries: 3,
      retry_initial_backoff_milliseconds: 1000,
      retry_max_backoff_milliseconds: 30000,
      circuit_breaker_failure_threshold: 5,
      circuit_breaker_success_threshold: 2,
      circuit_breaker_timeout_milliseconds: 30000,
    },
    security: {},
    pricing: {},
    proxy: {},
    response_headers: {},
    ui: { hidden_features: [] as string[] },
  },
};

describe("Feature visibility toggle", () => {
  let putBody: unknown = null;
  let hidden: string[] = [];

  beforeEach(() => {
    window.localStorage.clear();
    putBody = null;
    hidden = [];
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("saves hidden features and the config snapshot reflects them", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      const method = (init?.method ?? "GET").toUpperCase();
      const json = (body: unknown): Response =>
        new Response(JSON.stringify(body), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });

      if (method === "PUT" && url.includes("/dashboard/settings")) {
        const parsed = JSON.parse(String(init?.body ?? "{}")) as {
          ui?: { hidden_features?: string[] };
        };
        putBody = parsed;
        hidden = parsed.ui?.hidden_features ?? [];
        return json({ message: "dashboard settings saved", refresh_suggested: false, requires_restart: false });
      }
      if (url.includes("/dashboard/config")) {
        const cfg = structuredClone(baseConfig);
        (cfg.settings.ui as { hidden_features: string[] }).hidden_features = [...hidden];
        return json(cfg);
      }
      if (url.includes("/models/categories")) return json([]);
      if (url.includes("/models")) return json([]);
      if (url.includes("/providers/status")) {
        return json({ summary: { total: 0, healthy: 0, unhealthy: 0 }, providers: [] });
      }
      return json({});
    });
    vi.stubGlobal("fetch", fetchMock);

    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: 5 * 60 * 1000, refetchOnMount: false } },
    });

    function Probe() {
      const ctx = useSettings();
      return <div data-testid="hf">{JSON.stringify(ctx.dashboardSettings.ui?.hidden_features)}</div>;
    }
    render(
      <QueryClientProvider client={client}>
        <SettingsProvider>
          <GeneralTab />
          <Probe />
        </SettingsProvider>
      </QueryClientProvider>,
    );

    // Wait for the dashboard config snapshot to land (it seeds the form).
    await waitFor(() => {
      expect(fetchMock.mock.calls.some(c => String(c[0]).includes("/dashboard/config"))).toBe(true);
    });
    await new Promise(r => setTimeout(r, 150));

    // Toggle "Settings · Caching" to hidden (checked === visible).
    const hideSwitch = await screen.findByRole("switch", { name: "Hide Settings · Caching" });
    expect(hideSwitch).toHaveAttribute("aria-checked", "true");
    fireEvent.click(hideSwitch);
    await waitFor(() => {
      expect(hideSwitch).toHaveAttribute("aria-checked", "false");
    });

    fireEvent.click(screen.getByRole("button", { name: /save feature visibility/i }));

    await waitFor(() => {
      expect(putBody).not.toBeNull();
    });
    expect((putBody as { ui: { hidden_features: string[] } }).ui.hidden_features).toContain(
      "caching",
    );

    // Config snapshot refetch reflects the saved value (what Sidebar reads).
    await waitFor(() => {
      expect(hidden).toContain("caching");
    });
    const cfgAfter = await fetch("http://localhost/admin/api/v1/dashboard/config");
    const parsedCfg = (await cfgAfter.json()) as typeof baseConfig;
    expect(parsedCfg.settings.ui.hidden_features).toContain("caching");
  });

  it("keeps an unsaved toggle across a background config refetch", async () => {
    let configFetches = 0;
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : input.toString();
      const json = (body: unknown): Response =>
        new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
      if (url.includes("/dashboard/config")) {
        configFetches += 1;
        return json(structuredClone(baseConfig));
      }
      if (url.includes("/models")) return json([]);
      if (url.includes("/providers/status")) return json({ summary: { total: 0, healthy: 0, unhealthy: 0 }, providers: [] });
      return json({});
    });
    vi.stubGlobal("fetch", fetchMock);

    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: 0, refetchOnMount: false } },
    });

    function Probe() {
      const ctx = useSettings();
      return <div data-testid="hf">{JSON.stringify(ctx.dashboardSettings.ui?.hidden_features)}</div>;
    }
    render(
      <QueryClientProvider client={client}>
        <SettingsProvider>
          <GeneralTab />
          <Probe />
        </SettingsProvider>
      </QueryClientProvider>,
    );

    const hideSwitch = await screen.findByRole("switch", { name: "Hide Settings · Caching" });
    await waitFor(() => expect(configFetches).toBeGreaterThan(0));
    await new Promise(r => setTimeout(r, 150));

    fireEvent.click(hideSwitch);
    await waitFor(() => expect(hideSwitch).toHaveAttribute("aria-checked", "false"));

    // Background refresh (auto-refresh / any invalidation) refetches config
    // with the old (still empty) hidden_features; the unsaved toggle must
    // NOT be clobbered.
    await client.refetchQueries({ queryKey: ["dashboard", "config"] });
    await new Promise(r => setTimeout(r, 150));
    const fresh = await screen.findByRole("switch", { name: /Settings · Caching/ });
    expect(fresh).toHaveAttribute("aria-checked", "false");
    expect(screen.getByTestId("hf").textContent).toContain("caching");
  });
});

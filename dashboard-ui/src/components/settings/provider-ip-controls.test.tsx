import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SettingsProvider } from "./SettingsContext";
import { ProvidersTab } from "./ProvidersTab";

/**
 * Per-address control: the same source IP is bound to two providers. The
 * inventory lists it once and pauses it everywhere with one click, while the
 * per-provider chip pauses it for that provider only.
 */

const ip = "203.0.113.10";

const egressExit = (disabled: boolean) => ({
  name: ip,
  source: "config",
  kind: "ip",
  tier: 0,
  address: ip,
  available: true,
  disabled,
  tier_limited: false,
  failures: 0,
  requests: 4,
  ok: 4,
});

const egressFixture = {
  providers: [
    { provider: "zen", strategy: "round_robin", exits: [egressExit(false)] },
    { provider: "or-main", strategy: "round_robin", exits: [egressExit(false)] },
  ],
};

const providerFixture = [
  {
    name: "zen",
    type: "vllm",
    status: "healthy",
    status_label: "Healthy",
    status_reason: "",
    config_source: "ui",
    config: { name: "zen", type: "vllm", enabled: true, bind_ip: ip, bind_ips: [ip] },
    runtime: {
      name: "zen",
      type: "vllm",
      registered: true,
      registry_initialized: true,
      discovered_model_count: 10,
      using_cached_models: false,
    },
    external_auth_status: { has_token: false, expired: false },
  },
  {
    name: "or-main",
    type: "openrouter",
    status: "healthy",
    status_label: "Healthy",
    status_reason: "",
    config_source: "ui",
    config: { name: "or-main", type: "openrouter", enabled: true, bind_ip: ip, bind_ips: [ip] },
    runtime: {
      name: "or-main",
      type: "openrouter",
      registered: true,
      registry_initialized: true,
      discovered_model_count: 17,
      using_cached_models: false,
    },
    external_auth_status: { has_token: false, expired: false },
  },
];

function makeFetch() {
  return vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input.toString();
    const json = (body: unknown): Response =>
      new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
    if (url.includes("/sidecar/extensions")) return json({ extensions: [], presets: [] });
    if (url.includes("/external-auth/providers")) return json([]);
    if (url.includes("/providers/status")) {
      return json({
        summary: { total: 2, healthy: 2, degraded: 0, unhealthy: 0, overall_status: "healthy" },
        providers: providerFixture,
      });
    }
    if (url.includes("/admin/api/v1/egress")) return json(egressFixture);
    if (url.includes("/dashboard/config")) return json({ EDITION: "oss", fallback: {}, runtime_features: [], settings: { ui: { hidden_features: [] } } });
    return json({});
  });
}

function renderTab(): ReturnType<typeof makeFetch> {
  const fetchMock = makeFetch();
  vi.stubGlobal("fetch", fetchMock);
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
  return fetchMock;
}

describe("Provider source IP controls", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("lists a shared address once in the inventory with every provider using it", async () => {
    renderTab();
    await screen.findByText("Source IP inventory");
    await waitFor(() => expect(screen.getByText("1 address")).toBeTruthy());

    const inventoryButton = await screen.findByRole("button", { name: `Pause ${ip} everywhere` });
    // The shared address is one inventory row, plus a chip on each card.
    expect(screen.getAllByText(ip).length).toBeGreaterThanOrEqual(3);
    expect(inventoryButton).toBeTruthy();

    fireEvent.click(inventoryButton);
    await waitFor(() => {
      const toggles = (vi.mocked(fetch).mock.calls as unknown[][]).filter((call) =>
        String(call[0]).includes("/admin/api/v1/egress/"),
      );
      expect(toggles).toHaveLength(2);
      expect(toggles.map((call) => String(call[0]).split("?")[0]).sort()).toEqual([
        "/admin/api/v1/egress/or-main/disable",
        "/admin/api/v1/egress/zen/disable",
      ]);
    });
  });

  it("pauses an address for a single provider from its card chip", async () => {
    renderTab();
    const chip = await screen.findByRole("button", { name: `Pause ${ip} for zen` });
    fireEvent.click(chip);
    await waitFor(() => {
      const toggles = (vi.mocked(fetch).mock.calls as unknown[][]).filter((call) =>
        String(call[0]).includes("/admin/api/v1/egress/"),
      );
      expect(toggles).toHaveLength(1);
      expect(String(toggles[0]?.[0]).split("?")[0]).toBe("/admin/api/v1/egress/zen/disable");
    });
  });
});

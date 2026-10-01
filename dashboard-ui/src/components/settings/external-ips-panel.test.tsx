import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SettingsProvider } from "./SettingsContext";
import { ProvidersTab } from "./ProvidersTab";

/**
 * The external IP block in the Providers tab: an egress (VPN) extension shows
 * its endpoints and lets the operator bind them to pools and providers, which
 * is saved straight into the extension's apply_to.
 */

const vpnExtension = {
  id: "vpn-support",
  name: "VPN Support",
  tagline: "Subscription VPN endpoints for the providers you choose",
  type: "sidecar",
  builtin: false,
  applied: true,
  version: "4",
  config: {},
  provides: { features: ["egress"] },
  ui: { accent: "#7dcfff" },
};

const serversPayload = {
  apply_to: "opencode-zen",
  egress_mode: "fallback",
  core_kind: "static",
  servers: [
    { host: "198.51.100.7", port: 443, name: "node-a", protocol: "vless", alive: true, latency_ms: 84, selected: true },
    { host: "203.0.113.9", port: 8443, name: "node-b", protocol: "vmess", alive: false, selected: false },
  ],
};

const poolsPayload = {
  summary: { total: 2, healthy_members: 2, total_members: 2 },
  pools: [
    { name: "opencode-zen", strategy: "round_robin", members: [{ provider_name: "zen", healthy: true, total_requests: 0, total_errors: 0 }] },
    { name: "cheapvibecode", strategy: "round_robin", members: [{ provider_name: "vllm-cheapvibecode", healthy: true, total_requests: 0, total_errors: 0 }] },
  ],
};

const providerFixture = [
  {
    name: "zen",
    type: "vllm",
    status: "healthy",
    status_label: "Healthy",
    status_reason: "",
    config: { name: "zen", type: "vllm", enabled: true },
    runtime: { name: "zen", type: "vllm", registered: true, registry_initialized: true, discovered_model_count: 10, using_cached_models: false },
  },
  {
    name: "or-main",
    type: "openrouter",
    status: "healthy",
    status_label: "Healthy",
    status_reason: "",
    config: { name: "or-main", type: "openrouter", enabled: true },
    runtime: { name: "or-main", type: "openrouter", registered: true, registry_initialized: true, discovered_model_count: 18, using_cached_models: false },
  },
];

function makeFetch() {
  return vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input.toString();
    const json = (body: unknown): Response =>
      new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
    // Order matters: the data route is under /sidecar/extensions/:id.
    if (url.includes("/data/servers")) return json(serversPayload);
    if (url.includes("/sidecar/extensions")) return json({ extensions: [vpnExtension], presets: [] });
    if (url.includes("/external-auth/providers")) return json([]);
    if (url.includes("/providers/status")) {
      return json({
        summary: { total: 2, healthy: 2, degraded: 0, unhealthy: 0, overall_status: "healthy" },
        providers: providerFixture,
      });
    }
    if (url.includes("/admin/api/v1/pools")) return json(poolsPayload);
    if (url.includes("/admin/api/v1/egress")) {
      return json({
        providers: [
          {
            provider: "zen",
            strategy: "round_robin",
            exits: [
              { name: "ip:203.0.113.10", source: "config", kind: "ip", tier: 0, address: "203.0.113.10", available: true, disabled: false, tier_limited: false, failures: 0, requests: 4, ok: 4 },
              { name: "vpn:node-a", source: "extension", kind: "proxy", tier: 1, proxy: "socks5://127.0.0.1:1080", available: true, disabled: false, tier_limited: false, failures: 0, requests: 0, ok: 0 },
            ],
          },
        ],
      });
    }
    if (url.includes("/dashboard/config")) {
      return json({ EDITION: "oss", fallback: {}, runtime_features: [], settings: { ui: { hidden_features: [] } } });
    }
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

const configWrites = (fetchMock: ReturnType<typeof makeFetch>): string[] =>
  (fetchMock.mock.calls as unknown[][])
    .map((call) => ({ url: String(call[0]), init: call[1] as RequestInit | undefined }))
    .filter((call) => call.init?.method === "PUT" && call.url.includes("/sidecar/extensions/vpn-support/config"))
    .map((call) => (typeof call.init?.body === "string" ? call.init.body : ""));

describe("External IP addresses block", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("lists the endpoints and their live state", async () => {
    renderTab();
    await screen.findByText("External IP addresses");
    expect(await screen.findByText("node-a")).toBeTruthy();
    expect(screen.getByText("198.51.100.7:443")).toBeTruthy();
    expect(screen.getByText("alive")).toBeTruthy();
    expect(screen.getByText("dead")).toBeTruthy();
    expect(screen.getByText("core: static")).toBeTruthy();
    // One endpoint survived probing, one did not.
    expect(screen.getByText("1 endpoint")).toBeTruthy();
  });

  it("shows the current binding: the pool it applies to is checked", async () => {
    renderTab();
    const pool = await screen.findByRole("checkbox", { name: /opencode-zen/ });
    await waitFor(() => expect(pool.getAttribute("aria-checked")).toBe("true"));
    // A pool reaches its member provider, but a provider outside the list
    // stays unbound.
    const provider = screen.getByRole("checkbox", { name: /or-main/ });
    expect(provider.getAttribute("aria-checked")).toBe("false");
  });

  it("binds a provider by saving apply_to on the extension", async () => {
    const fetchMock = renderTab();
    const provider = await screen.findByRole("checkbox", { name: /or-main/ });
    fireEvent.click(provider);

    await waitFor(() => {
      const writes = configWrites(fetchMock);
      expect(writes.length).toBeGreaterThan(0);
      expect(writes[writes.length - 1]).toContain('"apply_to"');
      expect(writes[writes.length - 1]).toContain("or-main");
    });
    // The existing binding is preserved alongside the new one.
    expect(configWrites(fetchMock)[0]).toContain("opencode-zen");
  });

  it("saves a rotation mode change", async () => {
    const fetchMock = renderTab();
    const select = (await screen.findByText("Where they sit in the rotation"))
      .closest("div")
      ?.querySelector("select");
    expect(select).toBeTruthy();
    fireEvent.change(select as HTMLSelectElement, { target: { value: "rotate" } });

    await waitFor(() => {
      const writes = configWrites(fetchMock);
      expect(writes.some((body) => body.includes('"egress_mode"') && body.includes("rotate"))).toBe(true);
    });
  });
});

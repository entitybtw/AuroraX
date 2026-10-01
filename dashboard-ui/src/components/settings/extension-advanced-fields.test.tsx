import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ExtensionsTab } from "@/components/settings/ExtensionsTab";

/**
 * An extension marks the settings an operator rarely touches as `advanced`,
 * so its everyday form stays down to a handful of inputs until they are
 * opened explicitly.
 */

const extension = {
  id: "vpn-support",
  name: "VPN Support",
  type: "sidecar",
  builtin: false,
  applied: true,
  config: {},
  ui: {
    fields: [
      { key: "subscriptions", label: "Subscriptions", type: "textarea" },
      { key: "node_count", label: "IPs to keep", type: "number" },
      { key: "egress_mode", label: "Exit mode", type: "select", options: ["rotate", "fallback"] },
      { key: "core_kind", label: "Tunnel core", type: "select", options: ["none", "static"], advanced: true },
      { key: "probe_count", label: "Probe samples", type: "number", advanced: true },
    ],
  },
};

describe("extension advanced settings", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("hides advanced fields until they are opened", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : input.toString();
      const json = (b: unknown) =>
        new Response(JSON.stringify(b), { status: 200, headers: { "Content-Type": "application/json" } });
      if (url.includes("/extensions/ui")) return json({ contributions: [] });
      if (url.includes("/extensions/stores")) return json({ stores: [] });
      if (url.includes("/extensions")) return json({ extensions: [extension], presets: [] });
      return json({});
    });
    vi.stubGlobal("fetch", fetchMock);

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <ExtensionsTab />
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("Subscriptions")).toBeTruthy();
    });

    // Everyday settings are visible, advanced ones are not.
    expect(screen.getByText("IPs to keep")).toBeTruthy();
    expect(screen.getByText("Exit mode")).toBeTruthy();
    expect(screen.queryByText("Tunnel core")).toBeNull();
    expect(screen.queryByText("Probe samples")).toBeNull();

    const toggle = screen.getByRole("button", { name: /Show advanced settings \(2\)/ });
    fireEvent.click(toggle);

    expect(screen.getByText("Tunnel core")).toBeTruthy();
    expect(screen.getByText("Probe samples")).toBeTruthy();
    expect(screen.getByRole("button", { name: /Hide advanced settings \(2\)/ })).toBeTruthy();

    // Closing them again keeps the everyday form short.
    fireEvent.click(screen.getByRole("button", { name: /Hide advanced settings \(2\)/ }));
    expect(screen.queryByText("Tunnel core")).toBeNull();
    expect(screen.getByText("Subscriptions")).toBeTruthy();
  });
});

import { render, screen, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SettingsProvider, useSettings } from "./SettingsContext";

function Probe() {
  const ctx = useSettings();
  return (
    <div>
      <span data-testid="hf">{JSON.stringify(ctx.dashboardSettings.ui?.hidden_features)}</span>
      <button onClick={() => ctx.handleHiddenFeatureToggle("caching", true)}>hide</button>
      <button onClick={() => ctx.handleDashboardSettingsSave()}>save</button>
    </div>
  );
}

describe("SettingsContext hidden feature toggle", () => {
  beforeEach(() => window.localStorage.clear());
  afterEach(() => vi.restoreAllMocks());

  it("keeps toggled value in form state", async () => {
    let saved: string[] = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      const json = (b: unknown) => new Response(JSON.stringify(b), { status: 200, headers: { "Content-Type": "application/json" } });
      console.log("FETCH", (init?.method ?? "GET"), url);
      if ((init?.method ?? "GET").toUpperCase() === "PUT" && url.includes("/dashboard/settings")) {
        // The toggle persists immediately; a real server stores the value.
        const parsed = JSON.parse(String(init?.body ?? "{}")) as { ui?: { hidden_features?: string[] } };
        saved = parsed.ui?.hidden_features ?? [];
        return json({ message: "dashboard settings saved", refresh_suggested: false, requires_restart: false });
      }
      if (url.includes("/dashboard/config")) {
        return json({ EDITION: "oss", LOGGING_ENABLED: "false", settings: { client: {}, ui: { hidden_features: [...saved] } } });
      }
      if (url.includes("/models") || url.includes("/providers")) return json([]);
      return json({});
    });
    vi.stubGlobal("fetch", fetchMock);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <SettingsProvider><Probe /></SettingsProvider>
      </QueryClientProvider>,
    );
    await screen.findByTestId("hf");
    await new Promise(r => setTimeout(r, 100));
    console.log("before:", screen.getByTestId("hf").textContent);
    await act(async () => { screen.getByText("hide").click(); });
    console.log("after toggle:", screen.getByTestId("hf").textContent);
    await new Promise(r => setTimeout(r, 200));
    console.log("after wait:", screen.getByTestId("hf").textContent);
    expect(screen.getByTestId("hf").textContent).toContain("caching");
  });
});

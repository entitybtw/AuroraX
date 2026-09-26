import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ExtensionsTab } from "@/components/settings/ExtensionsTab";

const themeExt = {
  id: "theme-catppuccin-mocha",
  name: "Catppuccin Mocha",
  type: "theme",
  builtin: false,
  applied: true,
  config: {},
  ui: {
    accent: "#cba6f7",
    theme: { "--accent": "#cba6f7" },
    fields: [{ key: "accent", label: "Accent", type: "color", default: "#cba6f7" }],
  },
};
const sidecarExt = {
  id: "opencode",
  name: "Opencode CLI Emulation",
  type: "sidecar",
  builtin: false,
  applied: false,
  config: {},
  ui: { fields: [{ key: "base_url", label: "Base URL", type: "text", default: "" }] },
};

describe("theme selection hides target input", () => {
  beforeEach(() => { window.localStorage.clear(); });
  afterEach(() => { vi.restoreAllMocks(); });

  it("target input hidden for theme, shown for sidecar", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : input.toString();
      const json = (b: unknown) => new Response(JSON.stringify(b), { status: 200, headers: { "Content-Type": "application/json" } });
      if (url.includes("/extensions/ui")) return json({ contributions: [] });
      if (url.includes("/extensions/stores")) return json({ stores: [] });
      if (url.includes("/extensions")) return json({ extensions: [themeExt, sidecarExt], presets: [] });
      return json({});
    });
    vi.stubGlobal("fetch", fetchMock);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <ExtensionsTab />
      </QueryClientProvider>,
    );

    // Default selected = extensions[0] (theme in this fixture) -> no target input.
    await waitFor(() => {
      expect(screen.queryByText("Apply")).not.toBeNull();
    });
    expect(screen.queryByPlaceholderText("my-pool")).toBeNull();
    expect(screen.queryByText("Target pool or provider")).toBeNull();
    expect(screen.queryByText("Theme")).not.toBeNull();

    // Switch to the sidecar extension -> target input appears.
    const cards = screen.getAllByRole("button");
    const sidecarCard = cards.find((b) => b.textContent?.includes("Opencode CLI Emulation"));
    expect(sidecarCard).toBeTruthy();
    sidecarCard!.click();

    await waitFor(() => {
      expect(screen.queryByPlaceholderText("my-pool")).not.toBeNull();
      expect(screen.queryByText("Target pool or provider")).not.toBeNull();
    });

    // Back to the theme -> target input disappears again.
    const cards2 = screen.getAllByRole("button");
    const themeCard = cards2.find((b) => b.textContent?.includes("Catppuccin Mocha"));
    themeCard!.click();
    await waitFor(() => {
      expect(screen.queryByPlaceholderText("my-pool")).toBeNull();
      expect(screen.queryByText("Target pool or provider")).toBeNull();
    });
  });
});

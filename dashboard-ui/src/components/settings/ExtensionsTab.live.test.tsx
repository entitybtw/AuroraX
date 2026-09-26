import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ExtensionsTab } from "./ExtensionsTab";
import { ExtensionUIProvider } from "@/lib/extensions/ui-context";

/**
 * Live settings update: saving an extension config must invalidate the
 * shared [extensions] / [extensions, ui] queries so the injected theme
 * <style> tag picks up the new accent without a page reload.
 */

interface FakeState {
  config: Record<string, string>;
}

function makeExtension(state: FakeState) {
  return {
    id: "theme-catppuccin-mocha",
    name: "Catppuccin Mocha",
    type: "theme",
    builtin: false,
    applied: true,
    config: { ...state.config },
    settings: {},
    ui: {
      accent: state.config.accent ?? "#cba6f7",
      theme: { "--accent": state.config.accent ?? "#cba6f7", "--bg": "#1e1e2e" },
      theme_light: { "--accent": state.config.accent ?? "#cba6f7", "--bg": "#eff1f5" },
      theme_dark: { "--accent": state.config.accent ?? "#cba6f7", "--bg": "#1e1e2e" },
      fields: [
        { key: "accent", label: "Accent", type: "color", default: "#cba6f7" },
        { key: "--bg", label: "Background", type: "color", default: "#1e1e2e" },
      ],
    },
  };
}

function makeContribution(state: FakeState) {
  const ext = makeExtension(state);
  return { id: ext.id, name: ext.name, type: "theme", ui: ext.ui };
}

describe("ExtensionsTab live config update", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("updates the injected theme CSS after saving accent config", async () => {
    const state: FakeState = { config: { accent: "#cba6f7" } };

    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      const method = (init?.method ?? "GET").toUpperCase();
      const json = (body: unknown): Response =>
        new Response(JSON.stringify(body), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });

      if (method === "PUT" && url.includes("/config")) {
        const parsed = JSON.parse(String(init?.body ?? "{}")) as {
          config?: Record<string, string>;
        };
        state.config = { ...state.config, ...parsed.config };
        return json({ extension: makeExtension(state) });
      }
      if (url.includes("/extensions/ui")) {
        return json({ contributions: [makeContribution(state)] });
      }
      if (url.includes("/extensions/stores")) {
        return json({ stores: [] });
      }
      if (url.includes("/extensions")) {
        return json({ extensions: [makeExtension(state)], presets: [] });
      }
      return json({});
    });
    vi.stubGlobal("fetch", fetchMock);

    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    render(
      <QueryClientProvider client={client}>
        <ExtensionUIProvider>
          <ExtensionsTab />
        </ExtensionUIProvider>
      </QueryClientProvider>,
    );

    // Initial theme CSS is injected with the default accent.
    await waitFor(() => {
      const style = document.getElementById("aurora-ext-theme");
      expect(style?.textContent).toContain("#cba6f7");
    });

    const colorInput = await screen.findByLabelText("Accent color");
    fireEvent.change(colorInput, { target: { value: "#ff0000" } });

    fireEvent.click(screen.getByRole("button", { name: /save settings/i }));

    await waitFor(
      () => {
        const style = document.getElementById("aurora-ext-theme");
        expect(style?.textContent).toContain("#ff0000");
      },
      { timeout: 5000 },
    );
  });
});

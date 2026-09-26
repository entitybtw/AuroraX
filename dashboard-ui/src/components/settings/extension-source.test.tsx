import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ExtensionsTab } from "./ExtensionsTab";
import { ExtensionUIProvider } from "@/lib/extensions/ui-context";

/**
 * Sync source: the operator can point an extension at any repo URL (or clear
 * it so the source resolves from configured stores). The PUT must carry
 * {source} and the refreshed list must show the new value.
 */

function makeExtension(source: string) {
  return {
    id: "opencode",
    name: "OpenCode CLI Emulation",
    type: "sidecar",
    builtin: false,
    applied: false,
    version: "1",
    source,
    settings: {},
    ui: {},
  };
}

describe("ExtensionsTab sync source editing", () => {
  let source: string;
  let putBody: unknown;

  beforeEach(() => {
    window.localStorage.clear();
    source = "";
    putBody = null;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("saves a new source URL and clears it back to store resolution", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      const method = (init?.method ?? "GET").toUpperCase();
      const json = (body: unknown): Response =>
        new Response(JSON.stringify(body), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });

      if (method === "PUT" && url.endsWith("/extensions/opencode")) {
        const parsed = JSON.parse(String(init?.body ?? "{}")) as { source?: string };
        putBody = parsed;
        if (typeof parsed.source === "string") source = parsed.source;
        return json({ extension: makeExtension(source) });
      }
      if (url.includes("/check-update")) {
        return json({
          id: "opencode",
          source: source || "https://store.example.com/extensions/opencode.extension.json",
          current_version: "1",
          remote_version: "2",
          update_available: true,
          resolved: source === "",
        });
      }
      if (url.includes("/extensions/stores")) return json({ stores: [] });
      if (url.includes("/extensions")) return json({ extensions: [makeExtension(source)], presets: [] });
      return json({});
    });
    vi.stubGlobal("fetch", fetchMock);

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <ExtensionUIProvider>
          <ExtensionsTab />
        </ExtensionUIProvider>
      </QueryClientProvider>,
    );

    // No source set: the panel shows the store-resolution hint.
    await screen.findByText(/not set — resolves from configured stores/);

    // Edit → save an explicit source.
    fireEvent.click(screen.getByRole("button", { name: "Edit sync source" }));
    const input = await screen.findByLabelText("Sync source URL");
    fireEvent.change(input, { target: { value: "https://fork.example/ext.json" } });
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(putBody).toEqual({ source: "https://fork.example/ext.json" });
    });
    await screen.findByText("https://fork.example/ext.json");

    // Clear it → falls back to configured stores again.
    fireEvent.click(screen.getByRole("button", { name: "Edit sync source" }));
    const input2 = await screen.findByLabelText("Sync source URL");
    fireEvent.change(input2, { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(putBody).toEqual({ source: "" });
    });
    await screen.findByText(/not set — resolves from configured stores/);
  });
});

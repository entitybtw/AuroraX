import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ExtensionsTab } from "./ExtensionsTab";
import { ExtensionUIProvider } from "@/lib/extensions/ui-context";

/**
 * Import challenge: a manifest whose files entries are refs the gateway could
 * not fetch comes back 400 + missing_files. The dialog must switch to an
 * upload step listing the missing companions, block the import until every
 * file is provided, then retry with the uploads attached.
 */

const manifest = {
  id: "auth-ext",
  name: "Auth Ext",
  type: "sidecar",
  builtin: false,
  applied: false,
  version: "1",
  files: { "auth-x.go": { ref: "auth-ext/auth-x.go" } },
};

const resolvedExtension = {
  ...manifest,
  files: { "auth-x.go": { ref: "auth-ext/auth-x.go", content: ["package main"] } },
};

function renderTab() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ExtensionUIProvider>
        <ExtensionsTab />
      </ExtensionUIProvider>
    </QueryClientProvider>,
  );
}

describe("ExtensionsTab import missing-files challenge", () => {
  let importAttempts: number[];
  let lastImportBody: Record<string, unknown> | null;

  beforeEach(() => {
    window.localStorage.clear();
    importAttempts = [];
    lastImportBody = null;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("asks for companion files and retries with them uploaded", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      const method = (init?.method ?? "GET").toUpperCase();
      const json = (body: unknown, status = 200): Response =>
        new Response(JSON.stringify(body), {
          status,
          headers: { "Content-Type": "application/json" },
        });

      if (method === "POST" && url.endsWith("/extensions/import")) {
        importAttempts.push(importAttempts.length + 1);
        const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
        lastImportBody = body;
        const files = body.files as Record<string, { content?: unknown }> | undefined;
        if (files?.["auth-x.go"]?.content) {
          return json({ status: "ok", extension: resolvedExtension });
        }
        return json(
          {
            error: "extension references files that were not provided: auth-x.go",
            missing_files: ["auth-x.go"],
          },
          400,
        );
      }
      if (url.includes("/extensions/ui")) return json({ contributions: [] });
      if (url.includes("/extensions/stores")) return json({ stores: [] });
      if (url.includes("/extensions")) return json({ extensions: [] });
      return json({});
    });
    vi.stubGlobal("fetch", fetchMock);

    renderTab();

    fireEvent.click(await screen.findByRole("button", { name: /^import$/i }));
    const jsonTab = await screen.findByRole("button", { name: /paste json/i });
    fireEvent.click(jsonTab);
    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: JSON.stringify(manifest, null, 2) } });
    fireEvent.click(screen.getByRole("button", { name: /^import$/i }));

    // The gateway challenges: upload UI appears, Import is gated on uploads.
    await screen.findByText(/companion file/i);
    expect(screen.getByText("auth-x.go")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole("button", { name: /^import$/i })).toBeDisabled());
    expect(importAttempts).toHaveLength(1);

    // Upload the missing file.
    const input = screen.getByLabelText("Upload auth-x.go") as HTMLInputElement;
    const file = new File(["package main\n"], "auth-x.go", { type: "text/plain" });
    fireEvent.change(input, { target: { files: [file] } });
    await screen.findByText("uploaded");

    // Retry now carries the content.
    fireEvent.click(screen.getByRole("button", { name: /^import$/i }));
    await waitFor(() => expect(importAttempts).toHaveLength(2));
    const files = lastImportBody?.files as Record<string, { content?: string | string[] }> | undefined;
    const entry = files?.["auth-x.go"];
    const content = Array.isArray(entry?.content) ? entry?.content.join("\n") : entry?.content;
    expect(content).toContain("package main");

    // Dialog closes after the successful retry.
    await waitFor(() => expect(screen.queryByText(/companion file/i)).toBeNull());
  });
});

describe("ExtensionsTab companion file buttons", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // Editing a JSON companion is pointless: it is the same JSON editor with
  // the content escaped onto a single line. Only real source companions
  // (.go, scripts) get their own button; JSON stays behind "Edit JSON".
  it("offers an edit button only for non-JSON companion files", async () => {
    const extension = {
      id: "auth-ext",
      name: "Auth Ext",
      type: "sidecar",
      builtin: false,
      applied: true,
      version: "1",
      files: {
        "auth-x.go": { ref: "auth-ext/auth-x.go", content: ["package main"] },
        "tools/schema.json": ["{\"a\":1}"],
      },
    };
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : input.toString();
      const json = (body: unknown): Response =>
        new Response(JSON.stringify(body), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      if (url.includes("/extensions/ui")) return json({ contributions: [] });
      if (url.includes("/extensions/stores")) return json({ stores: [] });
      if (url.includes("/check-update")) {
        return json({ id: "auth-ext", current_version: "1", remote_version: "1", update_available: false });
      }
      if (url.includes("/extensions/auth-ext")) return json(extension);
      if (url.includes("/extensions")) return json({ extensions: [extension] });
      return json({});
    });
    vi.stubGlobal("fetch", fetchMock);

    renderTab();

    // The .go companion gets its own button…
    const goButton = await screen.findByRole("button", { name: /edit \.go file/i });
    expect(goButton).toBeInTheDocument();

    // …the JSON companion does not — "Edit JSON" covers it.
    expect(screen.queryByRole("button", { name: /schema\.json/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /^edit json$/i })).toBeInTheDocument();
  });
});

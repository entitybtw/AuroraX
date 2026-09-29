import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  ExtensionJsonEditor,
  findMatches,
  lineColAt,
  validateJson,
} from "./ExtensionJsonEditor";

/**
 * Extension JSON editor: find/replace (literal + regex + case), live JSON
 * validation, and Save re-importing the document through the import endpoint.
 */

const baseExtension = {
  id: "cli-profile",
  name: "CLI Profile",
  type: "sidecar",
  builtin: false,
  applied: false,
  version: "1",
  base_url: "https://one.example/v1",
  settings: { note: "alpha alpha alpha" },
};

function renderEditor(props: Partial<Parameters<typeof ExtensionJsonEditor>[0]> = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ExtensionJsonEditor
        open
        onOpenChange={() => {}}
        extensionId="cli-profile"
        {...props}
      />
    </QueryClientProvider>,
  );
}

describe("findMatches", () => {
  it("finds literal occurrences, honouring case sensitivity", () => {
    const text = "Alpha alpha ALPHA";
    expect(findMatches(text, "alpha", { caseSensitive: false, regex: false }).ranges).toEqual([
      [0, 5],
      [6, 11],
      [12, 17],
    ]);
    expect(findMatches(text, "alpha", { caseSensitive: true, regex: false }).ranges).toEqual([
      [6, 11],
    ]);
  });

  it("supports regex patterns and reports invalid ones", () => {
    const ok = findMatches("a1 b2 c3", "[a-z]\\d", { caseSensitive: false, regex: true });
    expect(ok.ranges).toEqual([
      [0, 2],
      [3, 5],
      [6, 8],
    ]);
    expect(ok.error).toBeUndefined();

    const bad = findMatches("abc", "(", { caseSensitive: false, regex: true });
    expect(bad.ranges).toEqual([]);
    expect(bad.error).toBeTruthy();
  });
});

describe("validateJson / lineColAt", () => {
  it("accepts valid documents", () => {
    expect(validateJson('{"a":1}').ok).toBe(true);
  });

  it("flags invalid documents", () => {
    const result = validateJson('{\n  "a": 1,\n  "b": \n}');
    expect(result.ok).toBe(false);
    expect(result.message).toBeTruthy();
    // Position info depends on the JS engine — assert only when present.
    if (result.line !== undefined) expect(result.line).toBeGreaterThanOrEqual(1);
    if (result.column !== undefined) expect(result.column).toBeGreaterThanOrEqual(1);
  });

  it("maps offsets to 1-based line/column", () => {
    expect(lineColAt("ab\ncd", 0)).toEqual({ line: 1, column: 1 });
    expect(lineColAt("ab\ncd", 3)).toEqual({ line: 2, column: 1 });
    expect(lineColAt("ab\ncd", 4)).toEqual({ line: 2, column: 2 });
  });
});

describe("ExtensionJsonEditor", () => {
  let savedBody: unknown;
  let savedCount: number;

  beforeEach(() => {
    window.localStorage.clear();
    savedBody = null;
    savedCount = 0;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  function stubFetch() {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      const method = (init?.method ?? "GET").toUpperCase();
      const json = (body: unknown): Response =>
        new Response(JSON.stringify(body), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      if (method === "POST" && url.endsWith("/extensions/import")) {
        savedBody = JSON.parse(String(init?.body ?? "{}"));
        savedCount += 1;
        return json({ status: "ok", extension: savedBody });
      }
      if (method === "GET" && url.includes("/extensions/stores")) return json({ stores: [] });
      if (method === "GET" && url.includes("/extensions/ui")) return json({ contributions: [] });
      if (method === "GET" && url.includes("/extensions/cli-profile"))
        return json({ ...baseExtension, settings: { note: "alpha alpha alpha" } });
      if (method === "GET" && url.includes("/extensions")) return json({ extensions: [baseExtension] });
      return json({});
    });
    vi.stubGlobal("fetch", fetchMock);
    return fetchMock;
  }

  it("loads pretty-printed JSON and reports validity", async () => {
    stubFetch();
    renderEditor();
    const ta = (await screen.findByLabelText("Extension JSON")) as HTMLTextAreaElement;
    await waitFor(() => expect(ta.value).toContain('"id": "cli-profile"'));
    expect(ta.value).toContain("\n"); // pretty-printed, not a one-liner
    expect(screen.getByText("Valid JSON")).toBeInTheDocument();
  });

  it("finds and replaces every match", async () => {
    stubFetch();
    renderEditor();
    const ta = (await screen.findByLabelText("Extension JSON")) as HTMLTextAreaElement;
    await waitFor(() => expect(ta.value).toContain("alpha"));

    fireEvent.click(screen.getByRole("button", { name: /find & replace/i }));
    const find = screen.getByLabelText("Find");
    fireEvent.change(find, { target: { value: "alpha" } });
    // Label shows the current match (1-based) over the total: 3 occurrences.
    await waitFor(() => expect(screen.getByText("1/3")).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText("Replace with"), { target: { value: "omega" } });
    fireEvent.click(screen.getByRole("button", { name: /^replace all$/i }));
    await waitFor(() => {
      expect(ta.value).not.toContain("alpha");
      expect(ta.value).toContain("omega omega omega");
    });
  });

  it("flags invalid JSON and blocks Save", async () => {
    stubFetch();
    renderEditor();
    const ta = (await screen.findByLabelText("Extension JSON")) as HTMLTextAreaElement;
    await waitFor(() => expect(ta.value).toContain('"id"'));

    fireEvent.change(ta, { target: { value: "{ oops" } });
    await waitFor(() => expect(screen.getByText(/^invalid/i)).toBeInTheDocument());
    expect(screen.getByRole("button", { name: /save/i })).toBeDisabled();
  });

  it("saves the edited document through the import endpoint", async () => {
    stubFetch();
    const onSaved = vi.fn();
    renderEditor({ onSaved });
    const ta = (await screen.findByLabelText("Extension JSON")) as HTMLTextAreaElement;
    await waitFor(() => expect(ta.value).toContain('"id"'));

    fireEvent.change(ta, {
      target: { value: JSON.stringify({ ...baseExtension, version: "2" }, null, 2) },
    });
    await waitFor(() => expect(screen.getByRole("button", { name: /save/i })).toBeEnabled());
    fireEvent.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => expect(savedCount).toBe(1));
    expect(savedBody).toMatchObject({ id: "cli-profile", version: "2" });
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
  });

  it("refuses to save a document whose id changed", async () => {
    stubFetch();
    renderEditor();
    const ta = (await screen.findByLabelText("Extension JSON")) as HTMLTextAreaElement;
    await waitFor(() => expect(ta.value).toContain('"id"'));

    fireEvent.change(ta, {
      target: { value: JSON.stringify({ ...baseExtension, id: "other-ext" }, null, 2) },
    });
    await waitFor(() => expect(screen.getByRole("button", { name: /save/i })).toBeEnabled());
    fireEvent.click(screen.getByRole("button", { name: /save/i }));

    await screen.findByText(/"id" must remain/i);
    expect(savedCount).toBe(0);
  });
});

describe("ExtensionJsonEditor file mode", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  function stubFileFetch(opts: { ref?: boolean; content?: string }) {
    const importBodies: string[] = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      if (url.includes("/extensions/import") && init?.body) {
        importBodies.push(String(init.body));
      }
      const json = (body: unknown): Response =>
        new Response(JSON.stringify(body), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      if (url.includes("/extensions/import")) {
        return json({ status: "ok", extension: {} });
      }
      if (url.includes("/extensions/ui")) return json({ contributions: [] });
      if (url.includes("/extensions/stores")) return json({ stores: [] });
      if (url.includes("/extensions/cli-profile")) {
        const entry = opts.ref
          ? { ref: "cli-profile/auth-x.go", content: opts.content ?? "" }
          : opts.content ?? "package main\n";
        return json({
          ...baseExtension,
          files: { "auth-x.go": entry },
        });
      }
      if (url.includes("/extensions")) return json({ extensions: [baseExtension] });
      return json({});
    });
    vi.stubGlobal("fetch", fetchMock);
    return { fetchMock, importBodies };
  }

  it("opens a referenced .go file with its resolved content", async () => {
    stubFileFetch({ ref: true, content: "// addon-kind: auth\npackage main\n" });
    renderEditor({ fileKey: "auth-x.go" });
    const ta = (await screen.findByLabelText("File auth-x.go")) as HTMLTextAreaElement;
    await waitFor(() => expect(ta.value).toContain("package main"));
    expect(ta.value).toContain("// addon-kind: auth");
    expect(screen.getByText(/companion source file/i)).toBeInTheDocument();
    // Non-JSON content must not be validated as JSON.
    expect(screen.getByText("Valid JSON")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /format/i })).toBeNull();
  });

  it("refuses to open a file whose content could not be loaded", async () => {
    stubFileFetch({ ref: true, content: "" });
    renderEditor({ fileKey: "auth-x.go" });
    await screen.findByText(/content of "auth-x\.go" is not loaded/i);
  });

  it("saves file edits back into the manifest", async () => {
    const { importBodies } = stubFileFetch({ ref: true, content: "package main\n" });
    renderEditor({ fileKey: "auth-x.go" });
    const ta = (await screen.findByLabelText("File auth-x.go")) as HTMLTextAreaElement;
    await waitFor(() => expect(ta.value).toContain("package main"));

    fireEvent.change(ta, { target: { value: "package main\n// edited\n" } });
    await waitFor(() => expect(screen.getByRole("button", { name: /save/i })).toBeEnabled());
    fireEvent.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => expect(importBodies.length).toBeGreaterThan(0));
    const body = JSON.parse(importBodies[importBodies.length - 1] ?? "{}") as {
      files?: Record<string, { ref?: string; content?: string } | string>;
    };
    const entry = body.files?.["auth-x.go"];
    expect(entry && typeof entry === "object" ? entry.ref : undefined).toBe("cli-profile/auth-x.go");
    expect(entry && typeof entry === "object" ? entry.content : entry).toContain("// edited");
  });
});

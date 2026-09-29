import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ExternalAuthDialog } from "./ExternalAuthDialog";

/**
 * After unlinking, the dialog must show a "Successfully unlinked" confirmation
 * with action buttons (Close / Link Account) instead of the empty window the
 * old idle-step rendering produced. A provider the addon does not own must
 * explain the scope error rather than blaming an expired device code.
 */

function renderDialog(props: Partial<Parameters<typeof ExternalAuthDialog>[0]> = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ExternalAuthDialog providerName="vllm-zen-backup-2" open onOpenChange={() => {}} {...props} />
    </QueryClientProvider>,
  );
}

type Handler = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function stubFetch(handler: Handler) {
  vi.stubGlobal("fetch", vi.fn(handler));
}

const linkedStatus = { has_token: true, expired: false, email: "a@b.c", account_id: "acc-1" };
const flowInfo = { provider: "p", grant: "device", has_token: true, authorization_code: false };

describe("ExternalAuthDialog unlink confirmation", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows a success window with actions after unlinking", async () => {
    let deleted = false;
    stubFetch(async (input, init) => {
      const url = String(input);
      const method = (init?.method ?? "GET").toUpperCase();
      if (method === "DELETE" && url.includes("/token")) {
        deleted = true;
        return json({ status: "ok" });
      }
      if (url.includes("/status")) return json(deleted ? { has_token: false, expired: false } : linkedStatus);
      if (url.includes("/flow")) return json(flowInfo);
      return json({});
    });

    const onComplete = vi.fn();
    renderDialog({ onComplete });

    // Linked state first: the dialog opens on the success surface.
    const unlinkBtn = await screen.findByRole("button", { name: /unlink/i });
    fireEvent.click(unlinkBtn);

    await waitFor(() => expect(deleted).toBe(true));
    await waitFor(() => expect(screen.getByText("Account Unlinked")).toBeInTheDocument());

    // Not an empty window: title, explanation and both actions are present.
    expect(screen.getAllByText(/successfully unlinked/i).length).toBeGreaterThan(0);
    expect(screen.getByText(/stored token for/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^close$/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /link account/i })).toBeInTheDocument();
    expect(onComplete).toHaveBeenCalled();
  });

  it("explains a provider-scope error instead of blaming an expired code", async () => {
    stubFetch(async (input, init) => {
      const url = String(input);
      const method = (init?.method ?? "GET").toUpperCase();
      if (url.includes("/status")) return json({ has_token: false, expired: false });
      if (url.includes("/flow")) return json(flowInfo);
      if (method === "POST" && url.includes("/start")) {
        return json(
          {
            error:
              "provider not found or external auth not configured: vllm-zen-backup-2 — pick a provider this linked-account extension owns",
          },
          400,
        );
      }
      return json({});
    });

    renderDialog();

    await screen.findByText(/not owned by the linked-account extension/i);
    // The misleading expired-device-code hint must be gone.
    expect(screen.queryByText(/device code may have expired/i)).toBeNull();
    // Retrying cannot help — no Try Again button for this class of error.
    expect(screen.queryByRole("button", { name: /try again/i })).toBeNull();
  });
});

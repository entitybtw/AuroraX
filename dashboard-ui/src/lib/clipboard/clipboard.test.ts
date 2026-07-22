import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { writeTextToClipboard, createClipboardButton } from "./clipboard";

describe("writeTextToClipboard", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("uses navigator.clipboard.writeText when available", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    await writeTextToClipboard("hello");
    expect(writeText).toHaveBeenCalledWith("hello");
  });

  it("coerces null/undefined to empty string", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    await writeTextToClipboard(null);
    await writeTextToClipboard(undefined);
    expect(writeText).toHaveBeenNthCalledWith(1, "");
    expect(writeText).toHaveBeenNthCalledWith(2, "");
  });
});

describe("createClipboardButton", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.restoreAllMocks();
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("toggles copied=true on success and resets after the delay", async () => {
    const states: Array<{ copied: boolean; error: boolean }> = [];
    const button = createClipboardButton({
      resetDelayMs: 100,
      onChange: (s) => states.push({ ...s }),
    });

    await button.copy("payload");
    expect(button.copied).toBe(true);
    expect(button.error).toBe(false);

    vi.advanceTimersByTime(100);
    expect(button.copied).toBe(false);
    expect(button.error).toBe(false);
    expect(states.at(-1)).toEqual({ copied: false, error: false });
  });

  it("toggles error=true when writeText rejects", async () => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: vi.fn().mockRejectedValue(new Error("denied")) },
    });
    vi.spyOn(console, "error").mockImplementation(() => {});
    const button = createClipboardButton({ resetDelayMs: 100 });

    await button.copy("payload");
    expect(button.copied).toBe(false);
    expect(button.error).toBe(true);
  });

  it("ignores empty values", async () => {
    const writeText = vi.fn();
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    const button = createClipboardButton();
    await button.copy("");
    await button.copy(null);
    await button.copy(undefined);
    expect(writeText).not.toHaveBeenCalled();
    expect(button.copied).toBe(false);
  });

  it("formatValue is applied to the payload", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    const button = createClipboardButton();
    await button.copy({ id: 7 }, (v) => `id=${(v as { id: number }).id}`);
    expect(writeText).toHaveBeenCalledWith("id=7");
  });
});

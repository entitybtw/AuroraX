/**
 * Port of internal/admin/dashboard/static/js/modules/clipboard.js.
 *
 * Two responsibilities, kept isolated and pure:
 *
 *   1. writeTextToClipboard(value)        — async write that prefers the
 *      navigator.clipboard API and falls back to a hidden <textarea> +
 *      document.execCommand("copy") so the copy still works in
 *      non-secure-context iframes (matches the legacy behavior).
 *   2. createClipboardButton(options)     — small finite-state controller
 *      for "copy" buttons. The controller exposes copied/error flags and
 *      auto-resets after `resetDelayMs`.
 *
 * The legacy module returned an Alpine reactive object; this version
 * returns a vanilla controller you wire into a React component via
 * useReducer/useState. See lib/clipboard/useClipboardButton for the hook.
 */

export interface ClipboardController {
  copied: boolean;
  error: boolean;
  copy(value: unknown, formatValue?: (value: unknown) => string): Promise<void>;
  reset(): void;
  dispose(): void;
}

export interface CreateClipboardButtonOptions {
  resetDelayMs?: number;
  logPrefix?: string;
  onChange?: (state: { copied: boolean; error: boolean }) => void;
}

function isPromiseLike(value: unknown): value is Promise<unknown> {
  return (
    typeof value === "object" &&
    value !== null &&
    typeof (value as { then?: unknown }).then === "function"
  );
}

export async function writeTextToClipboard(value: unknown): Promise<void> {
  const payload = value === null || value === undefined ? "" : String(value);

  const nav = typeof navigator !== "undefined" ? navigator : undefined;
  const clipboard = nav?.clipboard;
  if (clipboard && typeof clipboard.writeText === "function") {
    const result = clipboard.writeText(payload);
    if (isPromiseLike(result)) {
      await result;
      return;
    }
    return;
  }

  const doc = typeof document !== "undefined" ? document : undefined;
  if (
    !doc ||
    !doc.body ||
    typeof doc.createElement !== "function" ||
    typeof doc.execCommand !== "function"
  ) {
    throw new Error("Clipboard API unavailable");
  }

  const textarea = doc.createElement("textarea");
  textarea.value = payload;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.top = "0";
  textarea.style.left = "0";
  textarea.style.opacity = "0";

  try {
    doc.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    textarea.setSelectionRange(0, textarea.value.length);
    if (!doc.execCommand("copy")) {
      throw new Error("execCommand copy returned false");
    }
  } finally {
    if (textarea.parentNode) {
      textarea.parentNode.removeChild(textarea);
    }
  }
}

export function createClipboardButton(
  options: CreateClipboardButtonOptions = {},
): ClipboardController {
  const resetDelayMs = Number.isFinite(options.resetDelayMs)
    ? (options.resetDelayMs as number)
    : 2000;
  const logPrefix = options.logPrefix ?? "Failed to copy text:";

  const state = {
    copied: false,
    error: false,
  };
  let resetTimer: ReturnType<typeof setTimeout> | null = null;

  function emit(): void {
    if (typeof options.onChange === "function") {
      options.onChange({ copied: state.copied, error: state.error });
    }
  }

  function clearTimer(): void {
    if (resetTimer !== null) {
      clearTimeout(resetTimer);
      resetTimer = null;
    }
  }

  function scheduleReset(): void {
    clearTimer();
    resetTimer = setTimeout(() => {
      state.copied = false;
      state.error = false;
      resetTimer = null;
      emit();
    }, resetDelayMs);
  }

  function setFeedback(copied: boolean, error: boolean): void {
    state.copied = copied;
    state.error = error;
    emit();
    scheduleReset();
  }

  return {
    get copied() {
      return state.copied;
    },
    get error() {
      return state.error;
    },
    reset() {
      clearTimer();
      state.copied = false;
      state.error = false;
      emit();
    },
    dispose() {
      clearTimer();
    },
    async copy(value, formatValue) {
      if (value === null || value === undefined || value === "") {
        return;
      }

      clearTimer();
      state.copied = false;
      state.error = false;
      emit();

      try {
        const payload =
          typeof formatValue === "function" ? formatValue(value) : String(value);
        await writeTextToClipboard(payload);
        setFeedback(true, false);
      } catch (error: unknown) {
        // eslint-disable-next-line no-console
        console.error(logPrefix, error);
        setFeedback(false, true);
      }
    },
  };
}

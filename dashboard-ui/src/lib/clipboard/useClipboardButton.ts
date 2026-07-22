import { useEffect, useRef, useState } from "react";
import {
  createClipboardButton,
  type ClipboardController,
  type CreateClipboardButtonOptions,
} from "./clipboard";

/**
 * React hook wrapper around createClipboardButton. Returns the live
 * {copied, error} feedback flags plus a stable copy() callback. The
 * underlying timer is disposed when the component unmounts so the hook
 * is safe to use inside list rows that mount/unmount frequently.
 */
export interface UseClipboardButtonResult {
  copied: boolean;
  error: boolean;
  copy: (value: unknown, formatValue?: (value: unknown) => string) => Promise<void>;
  reset: () => void;
}

export function useClipboardButton(
  options: CreateClipboardButtonOptions = {},
): UseClipboardButtonResult {
  const [feedback, setFeedback] = useState<{ copied: boolean; error: boolean }>({
    copied: false,
    error: false,
  });

  const controllerRef = useRef<ClipboardController | null>(null);
  if (controllerRef.current === null) {
    controllerRef.current = createClipboardButton({
      ...options,
      onChange: setFeedback,
    });
  }

  useEffect(() => {
    const controller = controllerRef.current;
    return () => {
      controller?.dispose();
    };
  }, []);

  const controller = controllerRef.current;
  return {
    copied: feedback.copied,
    error: feedback.error,
    copy: (value, formatValue) =>
      controller ? controller.copy(value, formatValue) : Promise.resolve(),
    reset: () => controller?.reset(),
  };
}

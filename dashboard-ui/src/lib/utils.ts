import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

/**
 * Combines class names with Tailwind-aware deduplication. Standard helper
 * used by every shadcn/ui component and by our own components — keeps
 * conditional class composition readable.
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}

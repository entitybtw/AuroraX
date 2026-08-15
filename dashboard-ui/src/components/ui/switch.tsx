import { cn } from "@/lib/utils";

interface SwitchProps {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  size?: "sm" | "md";
  "aria-label"?: string;
  title?: string;
}

/**
 * Minimal toggle switch themed against aurora's CSS variables.
 * Accessible via a real button + aria-checked.
 */
export function Switch({
  checked,
  onCheckedChange,
  disabled,
  size = "md",
  "aria-label": ariaLabel,
  title,
}: SwitchProps): JSX.Element {
  const dims = size === "sm" ? "h-5 w-9" : "h-6 w-11";
  const knob = size === "sm" ? "h-3.5 w-3.5" : "h-4.5 w-4.5";
  const translate = size === "sm" ? "translate-x-4" : "translate-x-5";

  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      title={title}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        "relative inline-flex shrink-0 items-center rounded-full border transition-colors duration-200 ease-[var(--ease-ios)]",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
        "disabled:cursor-not-allowed disabled:opacity-40",
        dims,
        checked
          ? "border-accent bg-accent"
          : "border-border bg-background/60 hover:border-border/80"
      )}
    >
      <span
        className={cn(
          "inline-block rounded-full bg-foreground shadow-sm transition-transform duration-200 ease-[var(--ease-ios)]",
          knob,
          checked ? translate : "translate-x-0.5"
        )}
      />
    </button>
  );
}
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
 * Toggle switch themed against aurora's CSS variables. Slightly larger on
 * touch screens (mobile) for an easier tap target, compact on desktop.
 *
 * Geometry (md, track = w x h, border 1px):
 *  - mobile: track 48x28, knob 20, translate-x-6 (checked) / 2px (off)
 *  - desktop: track 44x24, knob 20, translate-x-5 (checked) / 2px (off)
 */
export function Switch({
  checked,
  onCheckedChange,
  disabled,
  size = "md",
  "aria-label": ariaLabel,
  title,
}: SwitchProps): JSX.Element {
  const track =
    size === "sm"
      ? "h-5 w-9"
      : "h-7 w-12 md:h-6 md:w-11";
  const knob = size === "sm" ? "h-3 w-3" : "h-5 w-5";
  const translate =
    size === "sm"
      ? "translate-x-5"
      : "translate-x-6 md:translate-x-5";

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
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background",
        "disabled:cursor-not-allowed disabled:opacity-40",
        "active:scale-[0.96]",
        track,
        checked
          ? "border-accent bg-accent"
          : "border-border bg-background/60 hover:border-border/80"
      )}
    >
      <span
        className={cn(
          "pointer-events-none inline-block rounded-full shadow-sm transition-transform duration-200 ease-[var(--ease-spring)]",
          "bg-foreground",
          knob,
          checked ? translate : "translate-x-0.5"
        )}
      />
    </button>
  );
}
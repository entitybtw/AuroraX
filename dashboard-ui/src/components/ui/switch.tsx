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
 * Toggle switch — pill-shaped track with a circular knob, iOS-style.
 * No border, just a colored track and a white knob with shadow.
 *
 * Geometry (md, borderless, symmetric):
 *  - mobile: track 48×28, knob 20, translate-x-[26px] / 2px
 *  - desktop: track 44×24, knob 20, translate-x-[22px] / 2px
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
      ? "h-[22px] w-[38px]"
      : "h-7 w-12 md:h-6 md:w-11";
  const knob = size === "sm" ? "h-4 w-4" : "h-5 w-5";
  const translate =
    size === "sm"
      ? "translate-x-[20px]"
      : "translate-x-[26px] md:translate-x-[22px]";

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
        "relative inline-flex shrink-0 items-center rounded-full transition-colors duration-200 ease-[var(--ease-ios)]",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background",
        "disabled:cursor-not-allowed disabled:opacity-40",
        "active:scale-[0.96]",
        track,
        checked
          ? "bg-accent"
          : "bg-border/60"
      )}
    >
      <span
        className={cn(
          "pointer-events-none inline-block rounded-full bg-background shadow transition-transform duration-200 ease-[var(--ease-spring)]",
          knob,
          checked ? translate : "translate-x-0.5"
        )}
      />
    </button>
  );
}
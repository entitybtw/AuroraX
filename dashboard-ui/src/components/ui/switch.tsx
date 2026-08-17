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
 * iOS-style toggle switch — pill track, white knob with shadow.
 * Uses Catppuccin semantic colors: --success for on, --border/--bg-surface-hover for off.
 * Fixed compact size on every screen (same as desktop), so toggles stay small
 * and pleasant on mobile too. min-h-0 keeps the global mobile
 * button{min-height:44px} rule from stretching the pill.
 *
 * Geometry (borderless, symmetric 2px margins):
 *  sm:  track 46×22, knob 18
 *  md:  track 50×24, knob 20
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
      ? "h-[22px] w-[46px]"
      : "h-6 w-[50px]";
  const knob =
    size === "sm"
      ? "h-[18px] w-[18px]"
      : "h-[20px] w-[20px]";
  const translate =
    size === "sm"
      ? "translate-x-[26px]"
      : "translate-x-[28px]";

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
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
        "disabled:cursor-not-allowed disabled:opacity-40",
        "active:scale-[0.97]",
        "min-h-0 touch-manipulation",
        track,
        checked
          ? "bg-success"
          : "bg-border/60 dark:bg-surface-hover/60"
      )}
    >
      <span
        className={cn(
          "pointer-events-none inline-block rounded-full bg-background shadow-[0_2px_4px_rgba(0,0,0,0.2)] dark:shadow-[0_2px_4px_rgba(0,0,0,0.4)] transition-transform duration-200 ease-[var(--ease-spring)]",
          knob,
          checked ? translate : "translate-x-[2px]"
        )}
      />
    </button>
  );
}
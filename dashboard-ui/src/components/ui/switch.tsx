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
 * Responsive: larger touch targets on mobile.
 *
 * Geometry (borderless, symmetric 2px margins):
 *  sm:  track 52×26 (mobile) / 46×22 (desktop), knob 22 / 18
 *  md:  track 56×28 (mobile) / 50×24 (desktop), knob 24 / 20
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
      ? "h-[26px] w-[52px] md:h-[22px] md:w-[46px]"
      : "h-7 w-[56px] md:h-6 md:w-[50px]";
  const knob =
    size === "sm"
      ? "h-[22px] w-[22px] md:h-[18px] md:w-[18px]"
      : "h-[24px] w-[24px] md:h-[20px] md:w-[20px]";
  const translate =
    size === "sm"
      ? "translate-x-[28px] md:translate-x-[26px]"
      : "translate-x-[30px] md:translate-x-[28px]";

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
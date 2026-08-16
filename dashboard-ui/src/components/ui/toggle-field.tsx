import { type ReactNode } from "react";
import { cn } from "@/lib/utils";
import { Switch } from "@/components/ui/switch";

interface ToggleFieldProps {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  label: ReactNode;
  description?: ReactNode;
  disabled?: boolean;
  size?: "sm" | "md";
  "aria-label"?: string;
  className?: string;
}

/**
 * iOS-style toggle row: label + optional description on the left, switch pinned
 * to the right. The whole row is tappable and keeps a comfortable 44px touch
 * target on mobile, while the label area truncates instead of pushing the
 * switch off-screen.
 */
export function ToggleField({
  checked,
  onCheckedChange,
  label,
  description,
  disabled,
  size = "md",
  "aria-label": ariaLabel,
  className,
}: ToggleFieldProps): JSX.Element {
  return (
    <label
      className={cn(
        "group flex min-h-[44px] items-center gap-4 rounded-md px-1 py-1.5 select-none",
        "transition-colors",
        disabled ? "cursor-not-allowed opacity-60" : "cursor-pointer",
        className
      )}
    >
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-[14px] font-medium leading-tight text-foreground">{label}</span>
        {description ? (
          <span className="text-[12px] leading-snug text-muted-foreground">{description}</span>
        ) : null}
      </span>
      <span className="shrink-0">
        <Switch
          checked={checked}
          onCheckedChange={onCheckedChange}
          size={size}
          {...(ariaLabel !== undefined ? { "aria-label": ariaLabel } : {})}
          {...(disabled ? { disabled: true } : {})}
        />
      </span>
    </label>
  );
}

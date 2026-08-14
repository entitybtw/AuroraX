import * as React from "react";
import { cn } from "@/lib/utils";

export type InputProps = React.InputHTMLAttributes<HTMLInputElement>;

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  function Input({ className, type, ...props }, ref) {
    return (
      <input
        type={type}
        ref={ref}
        className={cn(
          "flex h-11 md:h-9 w-full rounded-lg border border-border/50 bg-background/40 px-4 md:px-3 py-2 text-base md:text-[13px] transition-all duration-200 ease-[var(--ease-ios)]",
          "placeholder:text-muted-foreground/70",
          "focus-visible:outline-none focus-visible:border-accent/70 focus-visible:ring-2 focus-visible:ring-accent/15",
          "disabled:cursor-not-allowed disabled:opacity-50 hover:bg-background/60",
          "file:border-0 file:bg-transparent file:text-sm file:font-medium",
          "touch-manipulation",
          className,
        )}
        {...props}
      />
    );
  },
);

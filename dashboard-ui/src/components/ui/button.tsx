import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

/**
 * Button primitive shaped after shadcn/ui defaults but re-themed against
 * aurora's existing CSS variables. Variants chosen to cover what the
 * legacy dashboard already uses (primary CTA, ghost icon button, danger).
 */
const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap text-[12px] font-bold uppercase tracking-widest transition-all duration-200 ease-[var(--ease-ios)] rounded-lg " +
  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background " +
  "disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-4 [&_svg]:shrink-0 active:scale-[0.97] touch-manipulation min-h-[44px] md:min-h-0",
  {
    variants: {
      variant: {
        default: "bg-accent text-accent-foreground hover:bg-accent-hover shadow-sm",
        secondary:
          "border border-border/50 bg-surface text-foreground hover:bg-surface-hover/60",
        ghost: "text-foreground hover:bg-surface-hover/50",
        outline:
          "border border-border/50 bg-transparent text-foreground hover:bg-surface-hover/60 hover:border-border",
        destructive:
          "bg-destructive text-destructive-foreground hover:opacity-90",
        link: "text-accent underline-offset-4 hover:underline min-h-0",
      },
      size: {
        default: "h-11 md:h-9 px-4 py-2",
        sm: "h-10 md:h-8 px-3 text-[10px]",
        lg: "h-12 md:h-10 px-5",
        icon: "h-11 w-11 md:h-9 md:w-9 min-h-[44px] md:min-h-0",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  },
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
  VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  function Button({ className, variant, size, asChild = false, ...props }, ref) {
    const Comp = asChild ? Slot : "button";
    return (
      <Comp
        ref={ref}
        className={cn(buttonVariants({ variant, size }), className)}
        {...props}
      />
    );
  },
);

export { buttonVariants };

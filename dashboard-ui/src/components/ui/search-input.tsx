import { forwardRef } from "react";
import { SearchIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface SearchInputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  containerClassName?: string;
}

/**
 * Iconed search field. Uses a plain <input> with left padding large enough to
 * clear the absolute-positioned search icon, so the placeholder text never
 * sits on top of the icon (unlike stacking padding overrides on the shared
 * Input component, where md:px-3 can clobber pl-* on desktop).
 */
export const SearchInput = forwardRef<HTMLInputElement, SearchInputProps>(
  function SearchInput({ className, containerClassName, ...props }, ref) {
    return (
      <div className={cn("relative w-full", containerClassName)}>
        <SearchIcon className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <input
          {...props}
          ref={ref}
          className={cn(
            "h-11 md:h-9 w-full rounded-lg border border-border bg-background pl-9 pr-3 py-2 text-sm text-foreground outline-none transition-all duration-200",
            "placeholder:text-muted-foreground/70",
            "focus-visible:border-accent/70 focus-visible:ring-2 focus-visible:ring-accent/15",
            "touch-manipulation",
            className,
          )}
        />
      </div>
    );
  },
);
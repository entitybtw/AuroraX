import * as React from "react";
import { cn } from "@/lib/utils";

interface PageHeaderProps {
  title: string;
  subtitle?: string;
  kicker?: string;
  actions?: React.ReactNode;
  className?: string;
}

export function PageHeader({
  title,
  subtitle,
  kicker,
  actions,
  className,
}: PageHeaderProps): JSX.Element {
  return (
    <header
      className={cn(
        "flex flex-col sm:flex-row sm:items-end justify-between gap-4 border-b border-border/40 pb-6 pt-4",
        className,
      )}
    >
      <div className="min-w-0 flex-1">
        {kicker && (
          <div className="text-[10px] font-bold tracking-widest uppercase text-accent mb-3">
            [ {kicker} ]
          </div>
        )}
        <h2 className="font-serif text-[34px] font-normal leading-tight tracking-tight text-foreground">
          {title}
        </h2>
        {subtitle ? (
          <p className="mt-1.5 text-[15px] text-muted-foreground">{subtitle}</p>
        ) : null}
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-3">{actions}</div> : null}
    </header>
  );
}

interface KickerProps {
  children: React.ReactNode;
  className?: string;
}

export function Kicker({ children, className }: KickerProps): JSX.Element {
  return (
    <span
      className={cn(
        "text-[10px] font-bold tracking-widest uppercase text-accent",
        className,
      )}
    >
      {children}
    </span>
  );
}

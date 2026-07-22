import * as React from "react";
import { cn } from "@/lib/utils";

export interface SurfaceProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "elevated" | "subtle" | "glass" | "flush";
}

export function Surface({
  className,
  variant = "default",
  ...props
}: SurfaceProps): JSX.Element {
  return (
    <div
      className={cn(
        "text-foreground transition-all duration-200 ease-[var(--ease-ios)]",
        variant === "default" && " border border-border/50 bg-surface",
        variant === "elevated" && " border border-border/60 bg-surface ",
        variant === "subtle" && " bg-surface/40 border border-border/30",
        variant === "glass" && "glass",
        variant === "flush" && "border border-border/40 bg-surface hover:bg-surface-hover/50 transition-colors duration-200",
        className,
      )}
      {...props}
    />
  );
}

export interface SectionHeaderProps {
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
  className?: string;
}

export function SectionHeader({
  title,
  subtitle,
  action,
  className,
}: SectionHeaderProps): JSX.Element {
  return (
    <div className={cn("flex items-start justify-between gap-3 mb-2", className)}>
      <div className="min-w-0">
        <h3 className="font-serif text-xl font-normal tracking-tight text-foreground">
          {title}
        </h3>
        {subtitle ? (
          <p className="mt-1 text-[13px] text-muted-foreground">{subtitle}</p>
        ) : null}
      </div>
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  );
}

export function EmptyState({
  title,
  description,
  action,
  children,
  className,
}: {
  title: string;
  description?: string;
  action?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}): JSX.Element {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center  border border-dashed border-border/40 bg-surface/20 px-6 py-12 text-center",
        className,
      )}
    >
      <div className="text-[16px] font-semibold text-foreground">{title}</div>
      {description ? (
        <div className="mx-auto mt-2 max-w-lg text-[14px] leading-relaxed text-muted-foreground">
          {description}
        </div>
      ) : null}
      {children ? (
        <div className="mx-auto mt-4 max-w-lg text-sm leading-6 text-muted-foreground">
          {children}
        </div>
      ) : null}
      {action ? <div className="mt-6">{action}</div> : null}
    </div>
  );
}

export function Pill({
  children,
  tone = "muted",
  className,
}: {
  children: React.ReactNode;
  tone?: "muted" | "success" | "warning" | "danger" | "accent";
  className?: string;
}): JSX.Element {
  return (
    <span
      className={cn(
        "inline-flex items-center  border px-2.5 py-0.5 text-[11px] font-semibold tracking-wide uppercase",
        tone === "muted" && "border-border/60 bg-surface/60 text-muted-foreground",
        tone === "success" && "border-success/30 bg-success/15 text-success",
        tone === "warning" && "border-warning/30 bg-warning/15 text-warning",
        tone === "danger" && "border-destructive/30 bg-destructive/15 text-destructive",
        tone === "accent" && "border-accent/30 bg-accent/15 text-accent",
        className,
      )}
    >
      {children}
    </span>
  );
}

export function CodeBlock({
  children,
  className,
}: {
  children: string;
  className?: string;
}): JSX.Element {
  return (
    <pre
      className={cn(
        "overflow-x-auto border border-border/50 bg-background/50 p-4 font-mono text-[13px] leading-relaxed text-foreground",
        className,
      )}
    >
      {children}
    </pre>
  );
}

export interface FlushSectionProps extends React.HTMLAttributes<HTMLDivElement> {
  title?: string;
  subtitle?: string;
  action?: React.ReactNode;
}

export function FlushSection({
  title,
  subtitle,
  action,
  className,
  children,
  ...props
}: FlushSectionProps): JSX.Element {
  return (
    <div
      className={cn(
        "border border-border/40 bg-surface divide-y divide-border/40",
        className,
      )}
      {...props}
    >
      {(title || subtitle || action) && (
        <div className="flex items-start justify-between gap-3 p-6">
          <div className="min-w-0">
            {title && (
              <h3 className="font-serif text-xl font-normal tracking-tight text-foreground">
                {title}
              </h3>
            )}
            {subtitle && (
              <p className="mt-1 text-[13px] text-muted-foreground">{subtitle}</p>
            )}
          </div>
          {action && <div className="shrink-0">{action}</div>}
        </div>
      )}
      {children}
    </div>
  );
}

export function FlushCell({
  className,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement>): JSX.Element {
  return (
    <div
      className={cn(
        "p-6 transition-colors duration-200 hover:bg-surface-hover/30",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}

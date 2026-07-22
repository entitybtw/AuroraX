import * as React from "react";
import { cn } from "@/lib/utils";

export function TableWrap({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}): JSX.Element {
  return (
    <div
      className={cn(
        "overflow-hidden border border-border/40 bg-surface",
        className,
      )}
    >
      <div className="overflow-x-auto">{children}</div>
    </div>
  );
}

export function DataTable({
  children,
  className,
}: React.TableHTMLAttributes<HTMLTableElement>): JSX.Element {
  return (
    <table
      className={cn("min-w-full border-separate border-spacing-0 text-[14px]", className)}
    >
      {children}
    </table>
  );
}

export function Th({
  children,
  className,
}: React.ThHTMLAttributes<HTMLTableCellElement>): JSX.Element {
  return (
    <th
      className={cn(
        "border-b border-border/40 bg-surface-hover/20 px-5 py-3.5 text-left text-[10px] font-bold uppercase tracking-widest text-muted-foreground",
        className,
      )}
    >
      {children}
    </th>
  );
}

export function Td({
  children,
  className,
  ...props
}: React.TdHTMLAttributes<HTMLTableCellElement>): JSX.Element {
  return (
    <td
      className={cn("border-b border-border/20 px-5 py-3.5 align-top text-foreground transition-colors hover:bg-surface-hover/30", className)}
      {...props}
    >
      {children}
    </td>
  );
}

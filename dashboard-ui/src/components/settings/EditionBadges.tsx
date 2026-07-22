import type { ReactNode } from "react";

interface EditionChipProps {
  children: ReactNode;
  tone?: "default" | "success" | "warning" | "danger";
}

export function EditionChip({ children, tone = "default" }: EditionChipProps): JSX.Element {
  const toneClass = {
    default: "border-border/60 bg-background/70 text-muted-foreground",
    success: "border-success/30 bg-success/10 text-success",
    warning: "border-warning/30 bg-warning/10 text-warning",
    danger: "border-destructive/30 bg-destructive/10 text-destructive",
  }[tone];

  return <span className={`inline-flex items-center border px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-widest ${toneClass}`}>{children}</span>;
}

export function EditionStatusChip(): JSX.Element {
  return <EditionChip>OSS</EditionChip>;
}

export function AdvancedFeatureNotice(): JSX.Element | null {
  return null;
}

export function isAdvancedFeatureEnabled(): boolean {
  return false;
}

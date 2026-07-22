import { Construction } from "lucide-react";

/**
 * Reusable "Page in migration" placeholder. Each route file uses this until
 * its bucketed phase lands.
 */
export interface PageStubProps {
  title: string;
  phase: string;
  source: string;
}

export function PageStub({ title, phase, source }: PageStubProps): JSX.Element {
  return (
    <section className="mx-auto flex w-full max-w-2xl flex-col gap-3 px-6 py-12">
      <div className="flex items-center gap-3">
        <span className="grid h-9 w-9 place-items-center rounded-md bg-surface-hover text-accent">
          <Construction className="h-4 w-4" aria-hidden />
        </span>
        <div>
          <h2 className="font-serif text-[34px] font-normal leading-tight tracking-tight text-foreground">{title}</h2>
          <p className="text-xs text-muted-foreground">React port pending</p>
        </div>
      </div>
      <p className="text-sm text-muted-foreground">
        This page lands in <span className="font-mono text-foreground">{phase}</span>{" "}
        of the React migration. Until then, switch{" "}
        <code className="rounded bg-surface-hover px-1.5 py-0.5 font-mono text-xs">
          previous dashboard
        </code>{" "}
        for the original.
      </p>
      <p className="text-xs text-muted-foreground">
        Source for this page in the legacy dashboard:{" "}
        <span className="font-mono text-foreground">{source}</span>
      </p>
    </section>
  );
}

import { Link } from "@tanstack/react-router";
import { EyeOff } from "lucide-react";

interface FeatureDisabledPageProps {
  featureId?: string;
}

export function FeatureDisabledPage({ featureId }: FeatureDisabledPageProps): JSX.Element {
  return (
    <div className="flex min-h-[60vh] items-center justify-center">
      <div className="max-w-md border border-border/60 bg-surface px-6 py-5 text-center">
        <span className="mx-auto grid h-10 w-10 place-items-center rounded-md bg-surface-hover text-muted-foreground">
          <EyeOff className="h-5 w-5" aria-hidden />
        </span>
        <h1 className="mt-3 font-display text-2xl text-foreground">Feature disabled</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          This dashboard surface is hidden by an operator feature toggle. Config and historical data were not deleted — re-enable it in Settings → General → Feature visibility.
        </p>
        {featureId ? (
          <p className="mt-2 font-mono text-[11px] uppercase tracking-wider text-muted-foreground/80">
            {featureId}
          </p>
        ) : null}
        <Link
          to="/admin/dashboard/settings"
          className="mt-4 inline-flex items-center justify-center rounded-md border border-border/60 bg-background/60 px-4 py-2 text-[13px] font-medium text-foreground hover:bg-surface-hover"
        >
          Open Settings
        </Link>
      </div>
    </div>
  );
}

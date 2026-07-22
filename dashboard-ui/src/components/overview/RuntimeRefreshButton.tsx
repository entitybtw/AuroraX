import { RefreshCw } from "lucide-react";
import { cn } from "@/lib/utils";
import { useRuntimeRefresh } from "@/lib/api/useProviders";

export interface RuntimeRefreshButtonProps {
  className?: string;
}

export function RuntimeRefreshButton({ className }: RuntimeRefreshButtonProps): JSX.Element {
  const refresh = useRuntimeRefresh();
  const isPending = refresh.isPending;
  const lastReport = refresh.data;
  const error = refresh.error;

  let hint: string | null = null;
  if (error) {
    hint = error.message;
  } else if (lastReport) {
    hint = `Refreshed: ${lastReport.provider_count} providers · ${lastReport.model_count} models (${lastReport.duration_ms}ms)`;
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <button
        type="button"
        disabled={isPending}
        onClick={() => refresh.mutate()}
        className={cn(
          "inline-flex items-center gap-1.5 rounded-md border border-border bg-surface px-3 py-1 text-xs font-medium text-foreground transition-colors",
          "hover:bg-muted disabled:cursor-not-allowed disabled:opacity-60",
          className,
        )}
      >
        <RefreshCw className={cn("h-3.5 w-3.5", isPending && "animate-spin")} />
        {isPending ? "Refreshing…" : "Refresh runtime"}
      </button>
      {hint ? (
        <span
          className={cn(
            "text-[10px]",
            error ? "text-destructive" : "text-muted-foreground",
          )}
        >
          {hint}
        </span>
      ) : null}
    </div>
  );
}

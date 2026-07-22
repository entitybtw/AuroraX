import { cn } from "@/lib/utils";

interface ApiEndpoint {
  method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  path: string;
  description: string;
}

interface DocsCollapsibleProps {
  title: string;
  manual: string;
  endpoints: ApiEndpoint[];
  authNote?: string;
}

export function DocsCollapsible({ title, manual, endpoints, authNote }: DocsCollapsibleProps) {
  return (
    <details className="group  border border-border/40 bg-surface/20 overflow-hidden">
      <summary className="flex cursor-pointer list-none items-center justify-between gap-4 px-5 py-3 text-sm font-medium text-foreground hover:bg-surface/30 transition-colors">
        <span>{title}</span>
        <Chevron className="h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200 group-open:rotate-180" />
      </summary>
      <div className="border-t border-border/20 px-5 py-4 space-y-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-1.5">Via UI</p>
          <p className="text-sm text-muted-foreground leading-relaxed">{manual}</p>
        </div>
        <div>
          <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-1.5">Via API</p>
          {authNote && (
            <div className="mb-3 rounded-md border border-amber-500/20 bg-amber-500/5 px-3 py-2 text-xs text-muted-foreground">
              {authNote}
            </div>
          )}
          <div className="space-y-2">
            {endpoints.map((ep, i) => (
              <div key={i} className="flex items-start gap-3 text-sm">
                <span className={cn(
                  "inline-flex shrink-0 items-center justify-center rounded px-1.5 py-0.5 text-[10px] font-bold uppercase leading-none tracking-wider min-w-[4rem] text-center",
                  ep.method === "GET" && "bg-blue-500/10 text-blue-500",
                  ep.method === "POST" && "bg-green-500/10 text-green-500",
                  ep.method === "PUT" && "bg-orange-500/10 text-orange-500",
                  ep.method === "PATCH" && "bg-amber-500/10 text-amber-500",
                  ep.method === "DELETE" && "bg-red-500/10 text-red-500",
                )}>
                  {ep.method}
                </span>
                <code className="font-mono text-[12px] text-foreground/80 whitespace-nowrap">{ep.path}</code>
                <span className="text-muted-foreground">{ep.description}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </details>
  );
}

function Chevron({ className }: { className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
    >
      <path d="m6 9 6 6 6-6" />
    </svg>
  );
}

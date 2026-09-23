import * as React from "react";
import { cn } from "@/lib/utils";
import type { ExtensionUIBlock, ExtensionNavLink, ExtensionWidget } from "@/lib/api/extensions";
import { useExtensionWidgets } from "@/lib/extensions/ui-context";

function safeHref(href: string): boolean {
  // Allow in-app paths and http(s).
  if (href.startsWith("/") && !href.startsWith("//")) return true;
  if (href.startsWith("#")) return true;
  try {
    const u = new URL(href, window.location.origin);
    return u.protocol === "http:" || u.protocol === "https:";
  } catch {
    return false;
  }
}

function NavLink({ link }: { link: ExtensionNavLink }): JSX.Element | null {
  if (!safeHref(link.href)) return null;
  const external = /^https?:/i.test(link.href);
  return (
    <a
      href={link.href}
      {...(external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
      className="text-accent hover:underline underline-offset-2"
    >
      {link.label}
    </a>
  );
}

function kvPairs(kv: Array<Record<string, string>> | undefined): Array<[string, string]> {
  if (!kv) return [];
  return kv.map((row) => {
    const k = row.k ?? row.label ?? row.key ?? "";
    const v = row.v ?? row.value ?? "";
    return [k, v];
  });
}

export function ExtensionBlocks({ blocks }: { blocks: ExtensionUIBlock[] }): JSX.Element {
  return (
    <div className="flex flex-col gap-4">
      {blocks.map((block, i) => {
        switch (block.kind) {
          case "heading":
            return (
              <h3 key={i} className="text-lg font-semibold text-foreground">
                {block.text}
              </h3>
            );
          case "text":
            return (
              <p key={i} className="text-sm text-muted-foreground whitespace-pre-wrap">
                {block.text}
              </p>
            );
          case "code":
            return (
              <pre
                key={i}
                className="overflow-x-auto border border-border/40 bg-background/60 p-4 text-xs font-mono text-foreground"
              >
                <code>{block.text}</code>
              </pre>
            );
          case "list":
            return (
              <ul key={i} className="list-disc pl-5 text-sm text-muted-foreground space-y-1">
                {(block.items ?? []).map((item, j) => (
                  <li key={j}>{item}</li>
                ))}
              </ul>
            );
          case "links":
            return (
              <div key={i} className="flex flex-wrap gap-3 text-sm">
                {(block.links ?? []).map((link, j) => (
                  <NavLink key={j} link={link} />
                ))}
              </div>
            );
          case "divider":
            return <hr key={i} className="border-border/40" />;
          case "kv": {
            const pairs = kvPairs(block.kv);
            if (pairs.length === 0) return null;
            return (
              <dl key={i} className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2 text-sm">
                {pairs.map(([k, v], j) => (
                  <React.Fragment key={j}>
                    <dt className="text-muted-foreground">{k}</dt>
                    <dd className="text-foreground font-medium">{v}</dd>
                  </React.Fragment>
                ))}
              </dl>
            );
          }
          default:
            return null;
        }
      })}
    </div>
  );
}

function WidgetCard({ widget }: { widget: ExtensionWidget }): JSX.Element {
  const kind = widget.kind || "text";
  return (
    <div className="border border-border/40 bg-surface/35 p-5">
      {widget.title && (
        <div className="text-[10px] font-bold uppercase tracking-widest text-accent mb-2">
          {widget.title}
        </div>
      )}
      {kind === "text" && widget.body && (
        <p className="text-sm text-muted-foreground whitespace-pre-wrap">{widget.body}</p>
      )}
      {kind === "stats" && (
        <div className="grid grid-cols-2 gap-3">
          {(widget.stats ?? []).map((row, i) => {
            const k = row.k ?? row.label ?? row.key ?? "";
            const v = row.v ?? row.value ?? "";
            return (
              <div key={i}>
                <div className="text-[10px] uppercase tracking-widest text-muted-foreground">{k}</div>
                <div className="text-lg font-semibold text-foreground">{v}</div>
              </div>
            );
          })}
        </div>
      )}
      {kind === "links" && (
        <div className="flex flex-col gap-2 text-sm">
          {(widget.links ?? []).map((link, i) => (
            <NavLink key={i} link={link} />
          ))}
        </div>
      )}
    </div>
  );
}

/** Renders extension widgets for a layout slot (overview | settings). */
export function ExtensionWidgets({ slot, className }: { slot: string; className?: string }): JSX.Element | null {
  const widgets = useExtensionWidgets(slot);
  if (widgets.length === 0) return null;
  return (
    <div className={cn("grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4", className)}>
      {widgets.map((w) => (
        <WidgetCard key={`${w.extensionId}:${w.id}`} widget={w} />
      ))}
    </div>
  );
}

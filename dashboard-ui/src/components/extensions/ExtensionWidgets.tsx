import * as React from "react";
import { cn } from "@/lib/utils";
import type { ExtensionUIBlock, ExtensionNavLink, ExtensionWidget } from "@/lib/api/extensions";
import { useExtensionWidgets } from "@/lib/extensions/ui-context";
import { useExtensionData } from "@/lib/extensions/useExtensionData";

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

function StatsGrid({ stats }: { stats: Array<Record<string, string>> }): JSX.Element {
  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 gap-4">
      {stats.map((row, i) => {
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

function WidgetCard({ widget }: { widget: ExtensionWidget & { extensionId?: string } }): JSX.Element {
  const live = useExtensionData(widget.extensionId, widget.source, widget.refresh);
  const kind = widget.kind || "text";

  // A widget with a `source` renders whatever its addon returns; the static
  // fields below are only the fallback while nothing has loaded yet.
  if (widget.source) {
    const payload = live.data as Record<string, unknown> | undefined;
    const liveStats = payload && Array.isArray(payload.stats) ? (payload.stats as Array<Record<string, string>>) : undefined;
    const liveBody = payload && typeof payload.body === "string" ? (payload.body as string) : undefined;
    const liveLinks =
      payload && Array.isArray(payload.links) ? (payload.links as ExtensionNavLink[]) : undefined;
    return (
      <div className="border border-border/40 bg-surface/35 p-5">
        {widget.title && (
          <div className="text-[10px] font-bold uppercase tracking-widest text-accent mb-2">
            {widget.title}
          </div>
        )}
        {liveStats && <StatsGrid stats={liveStats} />}
        {!liveStats && liveBody && <p className="text-sm text-muted-foreground whitespace-pre-wrap">{liveBody}</p>}
        {liveLinks && (
          <div className="flex flex-col gap-2 text-sm">
            {liveLinks.map((link, i) => (
              <NavLink key={i} link={link} />
            ))}
          </div>
        )}
        {!liveStats && !liveBody && !liveLinks && widget.kind !== "stats" && (
          <p className="text-sm text-muted-foreground">
            {live.isError ? "Live data unavailable." : "Loading…"}
          </p>
        )}
        {!liveStats && !liveBody && !liveLinks && widget.kind === "stats" && (
          <StatsGrid stats={widget.stats ?? []} />
        )}
      </div>
    );
  }

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
      {kind === "stats" && <StatsGrid stats={widget.stats ?? []} />}
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

/**
 * Renders one live payload returned by an extension's addon.
 *
 * Returned content is rendered statically: a `source` inside a live reply is
 * ignored so an addon cannot put the dashboard into a fetch loop.
 */
function LiveContent({ data }: { data: unknown }): JSX.Element | null {
  if (data === null || data === undefined) return null;
  if (typeof data === "string") {
    return <p className="text-sm text-muted-foreground whitespace-pre-wrap">{data}</p>;
  }
  if (Array.isArray(data)) {
    return <ExtensionBlocks blocks={data as ExtensionUIBlock[]} />;
  }
  if (typeof data !== "object") return null;
  const obj = data as Record<string, unknown>;
  if (Array.isArray(obj.blocks)) {
    return <ExtensionBlocks blocks={obj.blocks as ExtensionUIBlock[]} />;
  }
  if (Array.isArray(obj.kv)) {
    const pairs = kvPairs(obj.kv as Array<Record<string, string>>);
    if (pairs.length === 0) return null;
    return (
      <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2 text-sm">
        {pairs.map(([k, v], j) => (
          <React.Fragment key={j}>
            <dt className="text-muted-foreground">{k}</dt>
            <dd className="text-foreground font-medium break-words">{v}</dd>
          </React.Fragment>
        ))}
      </dl>
    );
  }
  if (Array.isArray(obj.stats)) {
    return <StatsGrid stats={obj.stats as Array<Record<string, string>>} />;
  }
  if (Array.isArray(obj.items)) {
    return (
      <ul className="list-disc pl-5 text-sm text-muted-foreground space-y-1">
        {(obj.items as string[]).map((item, j) => (
          <li key={j}>{item}</li>
        ))}
      </ul>
    );
  }
  if (typeof obj.text === "string") {
    return <p className="text-sm text-muted-foreground whitespace-pre-wrap">{obj.text}</p>;
  }
  return null;
}

/** Fetches and renders a block that names a live `source` key. */
function LiveBlock({
  extensionId,
  source,
  refresh,
  block,
}: {
  extensionId: string;
  source: string;
  refresh: number | undefined;
  block: ExtensionUIBlock;
}): JSX.Element {
  const live = useExtensionData(extensionId, source, refresh);
  if (live.isError) {
    return (
      <div className="border border-border/40 bg-surface/35 p-4 text-sm text-muted-foreground">
        Live data unavailable{block.text ? ` — ${block.text}` : ""}.
      </div>
    );
  }
  if (live.data === undefined) {
    return (
      <div className="border border-border/40 bg-surface/35 p-4 text-sm text-muted-foreground">
        Loading{block.text ? ` — ${block.text}` : ""}…
      </div>
    );
  }
  return (
    <div className="border border-border/40 bg-surface/35 p-4">
      {block.text && (
        <div className="text-[10px] font-bold uppercase tracking-widest text-accent mb-2">
          {block.text}
        </div>
      )}
      <LiveContent data={live.data} />
    </div>
  );
}

/**
 * Renders structured extension blocks. No HTML/JS from the extension is
 * executed — only these shapes render.
 *
 * `extensionId` is optional: without it a block that names a live `source`
 * falls back to its static content instead of fetching.
 */
export function ExtensionBlocks({
  blocks,
  extensionId,
}: {
  blocks: ExtensionUIBlock[];
  extensionId?: string;
}): JSX.Element {
  return (
    <div className="flex flex-col gap-4">
      {blocks.map((block, i) => {
        if (block.source && extensionId) {
          return (
            <LiveBlock
              key={i}
              extensionId={extensionId}
              source={block.source}
              refresh={block.refresh}
              block={block}
            />
          );
        }
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
                    <dd className="text-foreground font-medium break-words">{v}</dd>
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

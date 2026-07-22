import * as React from "react";
import { ChevronLeft, ChevronRight, Loader2, Pause, Play, RefreshCw, Trash2 } from "lucide-react";
import { PageHeader } from "@/components/ui/page-header";
import { Button } from "@/components/ui/button";
import { DataTable, TableWrap, Td, Th } from "@/components/ui/data-table";
import { EmptyState, Pill, Surface } from "@/components/ui/surface";
import { fetchConsoleRecent, type ConsoleEvent } from "@/lib/api/console";

const PAGE_SIZE = 50;

export function ConsolePage(): JSX.Element {
  const [events, setEvents] = React.useState<ConsoleEvent[]>([]);
  const [total, setTotal] = React.useState(0);
  const [offset, setOffset] = React.useState(0);
  const [loading, setLoading] = React.useState(true);
  const [paused, setPaused] = React.useState(false);
  const [error, setError] = React.useState("");

  const load = React.useCallback(async (pageOffset: number, append: boolean) => {
    try {
      setError("");
      setLoading(true);
      const page = await fetchConsoleRecent(PAGE_SIZE, pageOffset);
      if (append) {
        setEvents((prev) => [...prev, ...page.events]);
      } else {
        setEvents([...page.events].reverse());
      }
      setTotal(page.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load console events.");
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => { void load(0, false); }, [load]);

  React.useEffect(() => {
    if (paused) return;
    const id = window.setInterval(() => {
      void load(0, false);
      setOffset(0);
    }, 3000);
    return () => window.clearInterval(id);
  }, [load, paused]);

  const hasOlder = offset + PAGE_SIZE < total;
  const hasNewer = offset > 0;

  const handleOlder = () => {
    const nextOffset = offset + PAGE_SIZE;
    setOffset(nextOffset);
    void load(nextOffset, false);
  };

  const handleNewer = () => {
    const prevOffset = Math.max(0, offset - PAGE_SIZE);
    setOffset(prevOffset);
    void load(prevOffset, false);
  };

  const handleRefresh = () => {
    setOffset(0);
    void load(0, false);
  };

  const handleClear = () => {
    setEvents([]);
    setTotal(0);
    setOffset(0);
  };

  const from = total > 0 ? offset + 1 : 0;
  const to = Math.min(offset + events.length, total);

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Live Console"
        subtitle="Sanitized request and fallback events from the gateway. Bodies, headers, and secrets are intentionally omitted."
        actions={
          <div className="flex gap-2">
            <Button variant="secondary" onClick={() => setPaused((v) => !v)}>{paused ? <Play className="h-4 w-4" /> : <Pause className="h-4 w-4" />}{paused ? "Resume" : "Pause"}</Button>
            <Button variant="secondary" onClick={handleRefresh}><RefreshCw className="h-4 w-4" />Refresh</Button>
            <Button variant="ghost" onClick={handleClear}><Trash2 className="h-4 w-4" />Clear</Button>
          </div>
        }
      />
      {error ? <Alert>{error}</Alert> : null}
      <Surface className="flex items-center justify-between p-4 text-sm text-muted-foreground">
        <span>Ring buffer: {total} events in memory.</span>
        <div className="flex items-center gap-4">
          <span className="text-xs">{from}–{to} of {total}</span>
          <div className="flex gap-1">
            <Button variant="secondary" size="sm" disabled={!hasNewer} onClick={handleNewer}><ChevronLeft className="h-4 w-4" />Newer</Button>
            <Button variant="secondary" size="sm" disabled={!hasOlder} onClick={handleOlder}>Older<ChevronRight className="h-4 w-4" /></Button>
          </div>
        </div>
      </Surface>
      {loading ? (
        <Surface className="flex items-center gap-2 p-6 text-sm text-muted-foreground"><Loader2 className="h-4 w-4 animate-spin" />Loading console...</Surface>
      ) : events.length === 0 ? (
        <EmptyState title="No console events yet">Send traffic through the gateway to populate this view.</EmptyState>
      ) : (
        <TableWrap>
          <DataTable>
            <thead><tr><Th>Time</Th><Th>Level</Th><Th>Route</Th><Th>Status</Th><Th>Model</Th><Th>Provider</Th><Th>Message</Th><Th>Latency</Th></tr></thead>
            <tbody>
              {events.map((event) => (
                <tr key={`${event.id}-${event.time}`}>
                  <Td className="font-mono text-xs">{formatTime(event.time)}</Td>
                  <Td><Pill tone={event.level === "error" ? "danger" : event.level === "warn" ? "warning" : "success"}>{event.level}</Pill></Td>
                  <Td><span className="font-mono text-xs">{event.method ?? "-"} {event.path ?? "-"}</span></Td>
                  <Td>{event.status ?? "-"}</Td>
                  <Td className="font-mono text-xs">{event.model ?? "-"}</Td>
                  <Td>{event.provider ?? "-"}</Td>
                  <Td className="max-w-xl text-sm text-muted-foreground">{event.message}{event.fallback?.target_model ? <span className="ml-2 text-accent">→ {event.fallback.target_model}</span> : null}</Td>
                  <Td className="font-mono text-xs">{event.duration_ms ?? 0}ms</Td>
                </tr>
              ))}
            </tbody>
          </DataTable>
        </TableWrap>
      )}
    </div>
  );
}

function formatTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleTimeString();
}

function Alert({ children }: { children: string }): JSX.Element {
  return <div className="border border-warning/30 bg-warning/15 px-4 py-3 text-sm text-warning">{children}</div>;
}
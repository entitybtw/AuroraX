import * as React from "react";
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from "recharts";
import { format } from "date-fns";

interface VolumeBucket {
  time: number;
  count: number;
}

interface LogVolumeChartProps {
  entries: readonly any[];
  onTimeRangeChange?: (start: number, end: number) => void;
  isConnected?: boolean | undefined;
}

function binEntries(entries: readonly any[], now: number): VolumeBucket[] {
  if (entries.length === 0) return [];

  const sorted = [...entries]
    .filter((e) => e.timestamp)
    .map((e) => new Date(e.timestamp!).getTime())
    .toSorted((a, b) => a - b);

  if (sorted.length === 0) return [];

  const minT = sorted[0]!;
  const maxT = Math.max(sorted[sorted.length - 1]!, now);
  const range = maxT - minT;

  let binCount: number;
  let binMs: number;
  if (range < 60000) {
    binMs = 1000;
    binCount = Math.ceil(range / binMs);
  } else if (range < 600000) {
    binMs = 10000;
    binCount = Math.ceil(range / binMs);
  } else if (range < 3600000) {
    binMs = 60000;
    binCount = Math.ceil(range / binMs);
  } else {
    binMs = 300000;
    binCount = Math.ceil(range / binMs);
  }

  binCount = Math.min(binCount, 120);
  binMs = range / binCount;

  const buckets: VolumeBucket[] = [];
  for (let i = 0; i < binCount; i++) {
    const start = minT! + i * binMs;
    const end = start + binMs;
    const count = sorted.filter((t) => t >= start && t < end).length;
    buckets.push({ time: start, count });
  }

  return buckets;
}

function formatTime(ms: number): string {
  const d = new Date(ms);
  const diff = Date.now() - ms;
  if (diff < 3600000) return format(d, "HH:mm:ss");
  if (diff < 86400000) return format(d, "HH:mm");
  return format(d, "MMM d HH:mm");
}

export function LogVolumeChart({ entries, isConnected }: LogVolumeChartProps) {
  const now = Date.now();
  const data = React.useMemo(() => binEntries(entries, now), [entries, now]);
  const total = entries.length;

  return (
    <div className="border border-border/60 bg-surface/70 p-4">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h4 className="font-serif text-xl font-normal tracking-tight text-foreground">
            Request Volume
          </h4>
          <span className="text-xs font-mono text-muted-foreground">
            {total} request{total !== 1 ? "s" : ""}
          </span>
        </div>
        {isConnected !== undefined && (
          <span className={`inline-flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider ${isConnected ? "text-success" : "text-muted-foreground"}`}>
            <span className={`inline-block h-1.5 w-1.5  ${isConnected ? "bg-success animate-pulse" : "bg-muted-foreground"}`} />
            {isConnected ? "Live" : "Buffered"}
          </span>
        )}
      </div>
      <div className="h-24">
        {data.length > 0 ? (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data} margin={{ top: 4, right: 4, bottom: 4, left: 4 }}>
              <defs>
                <linearGradient id="volumeGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="var(--color-accent)" stopOpacity={0.3} />
                  <stop offset="100%" stopColor="var(--color-accent)" stopOpacity={0.02} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" strokeOpacity={0.3} />
              <XAxis
                dataKey="time"
                tickFormatter={formatTime}
                tick={{ fontSize: 9, fill: "var(--color-muted-foreground)" }}
                axisLine={false}
                tickLine={false}
                minTickGap={40}
              />
              <YAxis
                allowDecimals={false}
                tick={{ fontSize: 9, fill: "var(--color-muted-foreground)" }}
                axisLine={false}
                tickLine={false}
                width={24}
              />
              <Tooltip
                content={({ active, payload }) => {
                  if (!active || !payload?.[0]) return null;
                  const d = payload[0].payload as VolumeBucket;
                  return (
                    <div className="rounded-lg border border-border/60 bg-surface/90 p-2 text-xs shadow-lg backdrop-blur-sm">
                      <div className="font-mono text-muted-foreground">{formatTime(d.time)}</div>
                      <div className="font-semibold text-foreground">{d.count} requests</div>
                    </div>
                  );
                }}
              />
              <Area
                type="monotone"
                dataKey="count"
                stroke="var(--color-accent)"
                strokeWidth={1.5}
                fill="url(#volumeGrad)"
              />
            </AreaChart>
          </ResponsiveContainer>
        ) : (
          <div className="flex h-full items-center justify-center text-[11px] text-muted-foreground">
            No request volume data yet
          </div>
        )}
      </div>
    </div>
  );
}

import * as React from "react";
import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  Legend
} from "recharts";
import { formatRequests } from "@/lib/format/numbers";
import type { CacheOverviewDaily, DailyUsage } from "@/lib/api/usage-types";

export interface ThroughputChartProps {
  daily: DailyUsage[] | undefined;
  cacheDaily: CacheOverviewDaily[] | undefined;
  isLoading: boolean;
}

interface Point {
  date: string;
  requests: number;
  cacheHits: number;
}

export function ThroughputChart({
  daily,
  cacheDaily,
  isLoading,
}: ThroughputChartProps): JSX.Element {
  const data = React.useMemo<Point[]>(() => {
    const byDate = new Map<string, Point>();
    for (const d of daily ?? []) {
      byDate.set(d.date, { date: d.date, requests: d.requests, cacheHits: 0 });
    }
    for (const c of cacheDaily ?? []) {
      const existing = byDate.get(c.date);
      if (existing) {
        existing.cacheHits = c.hits;
      } else {
        byDate.set(c.date, { date: c.date, requests: 0, cacheHits: c.hits });
      }
    }
    
    // Convert to sorted array
    let sorted = Array.from(byDate.values()).toSorted((a, b) => a.date.localeCompare(b.date));
    
    // Fill missing days
    if (sorted.length >= 2) {
      const filled: Point[] = [];
      const firstDate = new Date(sorted[0]!.date);
      const lastDate = new Date(sorted[sorted.length - 1]!.date);
      
      const map = new Map(sorted.map(p => [p.date, p]));
      
      for (let d = new Date(firstDate); d <= lastDate; d.setUTCDate(d.getUTCDate() + 1)) {
        const dateStr = d.toISOString().split("T")[0]!;
        if (map.has(dateStr)) {
          filled.push(map.get(dateStr)!);
        } else {
          filled.push({ date: dateStr, requests: 0, cacheHits: 0 });
        }
      }
      sorted = filled;
    }
    
    return sorted;
  }, [daily, cacheDaily]);

  const hasCacheHits = data.some((point) => point.cacheHits > 0);

  return (
    <div className="border border-border bg-surface p-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <div>
          <h2 className="font-serif text-xl font-normal tracking-tight text-foreground">Throughput</h2>
          <p className="mt-1 text-xs text-muted-foreground">Requests per day across the selected range.</p>
        </div>
        {isLoading ? (
          <span className="text-xs text-muted-foreground">Loading…</span>
        ) : null}
      </div>
      <div className="h-64 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={data} margin={{ top: 8, right: 16, bottom: 8, left: 0 }}>
            <defs>
              <linearGradient id="throughputFill" x1="0" x2="0" y1="0" y2="1">
                <stop offset="5%" stopColor="var(--chart-requests)" stopOpacity={0.28} />
                <stop offset="95%" stopColor="var(--chart-requests)" stopOpacity={0.02} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
            <XAxis
              dataKey="date"
              tick={{ fontSize: 11, fill: "var(--text-muted)" }}
              stroke="var(--border)"
              tickLine={false}
              axisLine={false}
              tickFormatter={(v: string) => {
                const parts = v.split("-");
                return parts.length === 3 ? `${parts[1]}/${parts[2]}` : v;
              }}
              dy={8}
            />
            <YAxis
              tick={{ fontSize: 11, fill: "var(--text-muted)" }}
              stroke="var(--border)"
              tickLine={false}
              axisLine={false}
              tickFormatter={(v: number) => formatRequests(v)}
              width={48}
              dx={-8}
            />
            <Tooltip cursor={{ stroke: 'var(--border)', strokeWidth: 1, strokeDasharray: '3 3' }}
              formatter={(value: number, name: string) => [formatRequests(value), name]}
              contentStyle={{
                background: "var(--bg-surface)",
                border: "1px solid var(--border)",
                borderRadius: 8,
                fontSize: 12,
                boxShadow: "0 4px 6px -1px rgb(0 0 0 / 0.1)",
              }}
              itemStyle={{ color: "var(--text)" }}
            />
            <Legend 
              verticalAlign="top" 
              height={36} 
              iconType="circle" 
              iconSize={8}
              wrapperStyle={{ fontSize: '11px', color: 'var(--text-muted)' }} 
            />
            <Area
              type="monotone"
              dataKey="requests"
              name="Requests"
              stroke="var(--chart-requests)"
              strokeWidth={2}
              fill="url(#throughputFill)"
              dot={{ r: 0 }}
              activeDot={{ r: 4, strokeWidth: 0, fill: "var(--chart-requests)" }}
            />
            {hasCacheHits ? (
              <Line
                type="monotone"
                dataKey="cacheHits"
                name="Cache hits"
                stroke="var(--chart-cache-hits)"
                strokeDasharray="5 4"
                strokeWidth={2}
                dot={{ r: 0 }}
                activeDot={{ r: 4, strokeWidth: 0, fill: "var(--chart-cache-hits)" }}
              />
            ) : null}
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

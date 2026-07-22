import * as React from "react";
import {
  CartesianGrid,
  ComposedChart,
  Area,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { formatTokens } from "@/lib/format/numbers";
import type { CacheOverviewDaily, DailyUsage } from "@/lib/api/usage-types";

export interface TokenUsageChartProps {
  daily: DailyUsage[] | undefined;
  cacheDaily: CacheOverviewDaily[] | undefined;
  isLoading: boolean;
}

interface Point {
  date: string;
  input: number;
  output: number;
  cacheInput: number;
  cacheOutput: number;
}

export function TokenUsageChart({
  daily,
  cacheDaily,
  isLoading,
}: TokenUsageChartProps): JSX.Element {
  const data = React.useMemo<Point[]>(() => {
    const byDate = new Map<string, Point>();
    for (const d of daily ?? []) {
      byDate.set(d.date, {
        date: d.date,
        input: d.input_tokens || 0,
        output: d.output_tokens || 0,
        cacheInput: 0,
        cacheOutput: 0,
      });
    }
    for (const c of cacheDaily ?? []) {
      const existing = byDate.get(c.date);
      if (existing) {
        existing.cacheInput = c.input_tokens || 0;
        existing.cacheOutput = c.output_tokens || 0;
      } else {
        byDate.set(c.date, {
          date: c.date,
          input: 0,
          output: 0,
          cacheInput: c.input_tokens || 0,
          cacheOutput: c.output_tokens || 0,
        });
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
          filled.push({ date: dateStr, input: 0, output: 0, cacheInput: 0, cacheOutput: 0 });
        }
      }
      sorted = filled;
    }
    
    return sorted;
  }, [daily, cacheDaily]);

  const hasCacheSeries = data.some((point) => point.cacheInput > 0 || point.cacheOutput > 0);

  return (
    <div className="border border-border bg-surface p-4">
      <div className="mb-3 flex items-center justify-between">
        <div>
          <h2 className="font-serif text-xl font-normal tracking-tight text-foreground">Token usage</h2>
          <p className="mt-1 text-xs text-muted-foreground">Input, output, and locally served cache tokens.</p>
        </div>
        {isLoading ? (
          <span className="text-xs text-muted-foreground">Loading…</span>
        ) : null}
      </div>
      <div className="h-64 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={data} margin={{ top: 8, right: 16, bottom: 8, left: 0 }}>
            <defs>
              <linearGradient id="inputFill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--chart-input)" stopOpacity={0.2} />
                <stop offset="95%" stopColor="var(--chart-input)" stopOpacity={0} />
              </linearGradient>
              <linearGradient id="outputFill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--chart-output)" stopOpacity={0.2} />
                <stop offset="95%" stopColor="var(--chart-output)" stopOpacity={0} />
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
              tickFormatter={(v: number) =>
                v >= 1000000
                  ? `${(v / 1000000).toFixed(1)}M`
                  : v >= 1000
                    ? `${(v / 1000).toFixed(0)}k`
                    : String(v)
              }
              width={48}
              dx={-8}
            />
            <Tooltip cursor={{ stroke: 'var(--border)', strokeWidth: 1, strokeDasharray: '3 3' }}
              formatter={(value: number, name: string) => [formatTokens(value), name]}
              contentStyle={{
                background: "var(--bg-surface)",
                border: "1px solid var(--border)",
                borderRadius: 6,
                fontSize: 12,
                boxShadow: "0 4px 6px -1px rgb(0 0 0 / 0.1)",
              }}
              itemStyle={{ color: "var(--text)" }}
            />
            <Area
              type="monotone"
              dataKey="input"
              name="Input tokens"
              stroke="var(--chart-input)"
              strokeWidth={2}
              fill="url(#inputFill)"
              dot={{ r: 0 }}
              activeDot={{ r: 4, strokeWidth: 0, fill: "var(--chart-input)" }}
            />
            <Area
              type="monotone"
              dataKey="output"
              name="Output tokens"
              stroke="var(--chart-output)"
              strokeWidth={2}
              fill="url(#outputFill)"
              dot={{ r: 0 }}
              activeDot={{ r: 4, strokeWidth: 0, fill: "var(--chart-output)" }}
            />
            {hasCacheSeries ? (
              <>
                <Line
                  type="monotone"
                  dataKey="cacheInput"
                  name="Cache input tokens"
                  stroke="var(--chart-cache-input)"
                  strokeDasharray="6 4"
                  strokeWidth={2}
                  dot={{ r: 0 }}
                  activeDot={{ r: 4, strokeWidth: 0, fill: "var(--chart-cache-input)" }}
                />
                <Line
                  type="monotone"
                  dataKey="cacheOutput"
                  name="Cache output tokens"
                  stroke="var(--chart-cache-output)"
                  strokeDasharray="6 4"
                  strokeWidth={2}
                  dot={{ r: 0 }}
                  activeDot={{ r: 4, strokeWidth: 0, fill: "var(--chart-cache-output)" }}
                />
              </>
            ) : null}
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

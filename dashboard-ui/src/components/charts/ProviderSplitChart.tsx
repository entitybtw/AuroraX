import { useMemo } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  Legend
} from "recharts";
import { formatTokens, formatCost } from "@/lib/format/numbers";
import type { ModelUsage } from "@/lib/api/usage-types";

export interface ProviderSplitChartProps {
  models: ModelUsage[] | undefined;
  isLoading: boolean;
  mode: "tokens" | "costs";
}

interface Bucket {
  provider: string;
  input: number;
  output: number;
  total: number;
}

export function ProviderSplitChart({ models, isLoading, mode }: ProviderSplitChartProps): JSX.Element {
  const data = useMemo<Bucket[]>(() => {
    // Group by provider
    const groups = new Map<string, Bucket>();
    
    for (const m of (models ?? [])) {
      const p = m.provider_name || m.provider || "unknown";
      
      const input = mode === "costs" ? (m.input_cost ?? 0) : m.input_tokens;
      const output = mode === "costs" ? (m.output_cost ?? 0) : m.output_tokens;
      const total = mode === "costs" ? (m.total_cost ?? 0) : (input + output);
      
      if (!groups.has(p)) {
        groups.set(p, { provider: p, input, output, total });
      } else {
        const ext = groups.get(p)!;
        ext.input += input;
        ext.output += output;
        ext.total += total;
      }
    }

    return Array.from(groups.values())
      .filter((b) => b.total > 0)
      .toSorted((a, b) => b.total - a.total)
      .slice(0, 10);
  }, [models, mode]);

  return (
    <div className="w-full p-4" style={{ height: 350 }}>
      {isLoading ? (
        <div className="flex h-full items-center justify-center text-[13px] text-muted-foreground">Loading...</div>
      ) : data.length === 0 ? (
        <div className="flex h-full items-center justify-center text-[13px] text-muted-foreground">No provider data available.</div>
      ) : (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ top: 8, right: 16, bottom: 8, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
            <XAxis
              dataKey="provider"
              tick={{ fontSize: 11, fill: "var(--text-muted)" }}
              stroke="var(--border)"
              tickLine={false}
              axisLine={false}
              interval={0}
              angle={-25}
              textAnchor="end"
              height={60}
            />
            <YAxis
              tick={{ fontSize: 11, fill: "var(--text-muted)" }}
              stroke="var(--border)"
              tickLine={false}
              axisLine={false}
              tickFormatter={(v: number) => mode === "costs" ? formatCost(v) : formatTokens(v)}
              width={48}
              dx={-8}
            />
            <Tooltip cursor={{ fill: 'var(--bg-surface-hover)', opacity: 0.5 }}
              formatter={(value: number, name: string) => [mode === "costs" ? formatCost(value) : formatTokens(value), name === "input" ? "Input" : "Output"]}
              contentStyle={{
                background: "var(--bg-surface)",
                border: "1px solid var(--border)",
                borderRadius: 8,
                fontSize: 12,
                boxShadow: "0 4px 6px -1px rgb(0 0 0 / 0.1)",
              }}
              itemStyle={{ color: "var(--text)", fontWeight: 600 }}
              labelStyle={{ color: "var(--text-muted)", marginBottom: 4 }}
            />
            <Legend 
              verticalAlign="top" 
              height={36} 
              iconType="circle" 
              iconSize={8}
              wrapperStyle={{ fontSize: '11px', color: 'var(--text-muted)' }} 
            />
            <Bar dataKey="input" name="input" stackId="a" fill="var(--chart-input)" radius={[0, 0, 4, 4]} barSize={32} />
            <Bar dataKey="output" name="output" stackId="a" fill="var(--chart-output)" radius={[4, 4, 0, 0]} barSize={32} />
          </BarChart>
        </ResponsiveContainer>
      )}
    </div>
  );
}

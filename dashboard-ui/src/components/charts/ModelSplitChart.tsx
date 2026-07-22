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
import { formatTokens } from "@/lib/format/numbers";
import type { ModelUsage } from "@/lib/api/usage-types";

export interface ModelSplitChartProps {
  models: ModelUsage[] | undefined;
  isLoading: boolean;
}

interface Bucket {
  model: string;
  input: number;
  output: number;
  total: number;
}

export function ModelSplitChart({ models, isLoading }: ModelSplitChartProps): JSX.Element {
  const data = useMemo<Bucket[]>(() => {
    return (models ?? [])
      .map((m) => ({ 
        model: m.model, 
        input: m.input_tokens, 
        output: m.output_tokens,
        total: m.input_tokens + m.output_tokens
      }))
      .filter((b) => b.total > 0)
      .toSorted((a, b) => b.total - a.total)
      .slice(0, 10);
  }, [models]);

  return (
    <div className="w-full p-4" style={{ height: 350 }}>
      {isLoading ? (
        <div className="flex h-full items-center justify-center text-sm text-muted-foreground">Loading...</div>
      ) : data.length === 0 ? (
        <div className="flex h-full items-center justify-center text-sm text-muted-foreground">No model data available.</div>
      ) : (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ top: 8, right: 16, bottom: 8, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
            <XAxis
              dataKey="model"
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
            <Tooltip cursor={{ fill: 'var(--bg-surface-hover)', opacity: 0.5 }}
              formatter={(value: number, name: string) => [formatTokens(value), name === "input" ? "Input Tokens" : "Output Tokens"]}
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

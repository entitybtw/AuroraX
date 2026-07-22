import { useMemo } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { formatCost } from "@/lib/format/numbers";
import type { ModelUsage } from "@/lib/api/usage-types";

export interface CostByModelChartProps {
  models: ModelUsage[] | undefined;
  isLoading: boolean;
}

interface Bucket {
  model: string;
  cost: number;
}

export function CostByModelChart({ models, isLoading }: CostByModelChartProps): JSX.Element {
  const data = useMemo<Bucket[]>(() => {
    return (models ?? [])
      .map((m) => ({ model: m.model, cost: m.total_cost ?? 0 }))
      .filter((b) => b.cost > 0)
      .toSorted((a, b) => b.cost - a.cost)
      .slice(0, 10);
  }, [models]);

  return (
    <div className="border border-border bg-surface p-4">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="font-serif text-xl font-normal tracking-tight text-foreground">Cost by model</h2>
        {isLoading ? (
          <span className="text-xs text-muted-foreground">Loading…</span>
        ) : null}
      </div>
      <div className="h-64 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ top: 8, right: 16, bottom: 8, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
            <XAxis
              dataKey="model"
              tick={{ fontSize: 10, fill: "var(--text-muted)" }}
              stroke="var(--border)"
              interval={0}
              angle={-25}
              textAnchor="end"
              height={60}
            />
            <YAxis
              tick={{ fontSize: 11, fill: "var(--text-muted)" }}
              stroke="var(--border)"
              tickFormatter={(v: number) => formatCost(v)}
              width={64}
            />
            <Tooltip cursor={{ fill: 'var(--bg-surface-hover)', opacity: 0.5 }}
              formatter={(v: number) => formatCost(v)}
              contentStyle={{
                background: "var(--bg-surface)",
                border: "1px solid var(--border)",
                borderRadius: 6,
                fontSize: 12,
              }}
              itemStyle={{ color: "var(--text)" }}
            />
            <Bar dataKey="cost" fill="var(--accent)" radius={[3, 3, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

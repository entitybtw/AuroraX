import { useMemo } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  Cell
} from "recharts";
import { formatCost, formatTokens } from "@/lib/format/numbers";
import type { UserPathUsage } from "@/lib/api/usage-types";

export interface UserPathChartProps {
  userPaths: UserPathUsage[] | undefined;
  isLoading: boolean;
  mode: "tokens" | "costs";
}

interface Bucket {
  path: string;
  value: number;
}

const CHART_COLORS = [
  "var(--chart-cat-1)",
  "var(--chart-cat-2)",
  "var(--chart-cat-3)",
  "var(--chart-cat-4)",
  "var(--chart-cat-5)",
  "var(--chart-cat-6)",
  "var(--chart-cat-7)",
  "var(--chart-cat-8)",
  "var(--chart-cat-9)",
  "var(--chart-cat-10)",
];

export function UserPathChart({ userPaths, isLoading, mode }: UserPathChartProps): JSX.Element {
  const data = useMemo<Bucket[]>(() => {
    return (userPaths ?? [])
      .map((u) => {
        const val = mode === "costs" ? (u.total_cost ?? 0) : (u.input_tokens + u.output_tokens);
        return { path: u.user_path || "/", value: val };
      })
      .filter((b) => b.value > 0)
      .toSorted((a, b) => b.value - a.value)
      .slice(0, 10);
  }, [userPaths, mode]);

  return (
    <div className="w-full p-4" style={{ height: 350 }}>
      {isLoading ? (
        <div className="flex h-full items-center justify-center text-sm text-muted-foreground">Loading...</div>
      ) : data.length === 0 ? (
        <div className="flex h-full items-center justify-center text-sm text-muted-foreground">No user path data available.</div>
      ) : (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} layout="vertical" margin={{ top: 0, right: 16, bottom: 0, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" horizontal={true} vertical={false} />
            <XAxis
              type="number"
              tick={{ fontSize: 11, fill: "var(--text-muted)" }}
              stroke="var(--border)"
              tickLine={false}
              axisLine={false}
              tickFormatter={(v: number) => mode === "costs" ? formatCost(v) : formatTokens(v)}
            />
            <YAxis
              type="category"
              dataKey="path"
              tick={{ fontSize: 11, fill: "var(--text)", fontWeight: 500 }}
              stroke="var(--border)"
              tickLine={false}
              axisLine={false}
              width={100}
              tickFormatter={(v: string) => v.length > 15 ? v.substring(0, 15) + "..." : v}
            />
            <Tooltip cursor={{ fill: 'var(--bg-surface-hover)', opacity: 0.5 }}
              formatter={(v: number) => [mode === "costs" ? formatCost(v) : formatTokens(v), mode === "costs" ? "Cost" : "Tokens"]}
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
            <Bar dataKey="value" radius={[0, 4, 4, 0]} barSize={24}>
              {data.map((_, index) => (
                <Cell key={`cell-${index}`} fill={CHART_COLORS[index % CHART_COLORS.length]} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      )}
    </div>
  );
}

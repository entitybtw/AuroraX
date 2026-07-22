import * as React from "react";
import type { PoolSnapshot } from "@/lib/api/pools-types";

export interface PoolSceneSVGProps {
  pool: PoolSnapshot;
  onMemberHover?: (name: string | null) => void;
}

export function PoolSceneSVG({ pool, onMemberHover }: PoolSceneSVGProps) {
  const containerRef = React.useRef<HTMLDivElement>(null);
  const dims = useContainerSize(containerRef);

  const cx = dims.width / 2;
  const cy = dims.height / 2;
  const orbitR = Math.min(dims.width, dims.height) * 0.33;

  const totalActive = pool.members.reduce((s, m) => s + m.active_requests, 0);
  const totalTraffic = pool.members.reduce((s, m) => s + m.total_requests, 0);

  const colors = useThemeColors();

  return (
    <div
      ref={containerRef}
      className="relative h-[520px] w-full overflow-hidden border border-border/40 bg-surface"
    >
      <svg
        width={dims.width}
        height={dims.height}
        className="h-full w-full"
        style={{ overflow: "visible" }}
      >
        <defs>
          <radialGradient id="hub-glow" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stopColor={colors.accent} stopOpacity="0.25" />
            <stop offset="100%" stopColor={colors.accent} stopOpacity="0" />
          </radialGradient>

          <filter id="node-shadow">
            <feDropShadow dx="0" dy="1" stdDeviation="2" floodColor="#000" floodOpacity="0.3" />
          </filter>
        </defs>

        {/* Subtle dot-grid background */}
        <pattern id="dot-grid" width="24" height="24" patternUnits="userSpaceOnUse">
          <circle cx="12" cy="12" r="0.5" fill={colors.textMuted} opacity="0.06" />
        </pattern>
        <rect width={dims.width} height={dims.height} fill="url(#dot-grid)" />

        {/* Orbit ring */}
        <circle
          cx={cx} cy={cy} r={orbitR}
          fill="none"
          stroke={colors.textMuted}
          strokeOpacity="0.1"
          strokeWidth="1"
        />

        {/* Spoke lines + health arcs */}
        {pool.members.map((member, i) => {
          const angle = (i / pool.members.length) * Math.PI * 2 - Math.PI / 2;
          const x = cx + Math.cos(angle) * orbitR;
          const y = cy + Math.sin(angle) * orbitR;
          const color = member.healthy ? colors.success : colors.danger;

          const arcStartAngle = angle - 0.08;
          const arcEndAngle = angle + 0.08;

          return (
            <g key={`spoke-${member.provider_name}`}>
              <line
                x1={cx} y1={cy} x2={x} y2={y}
                stroke={color}
                strokeOpacity={0.12}
                strokeWidth="1"
              />
              <path
                d={describeArc(cx, cy, orbitR + 6, arcStartAngle, arcEndAngle)}
                fill="none"
                stroke={color}
                strokeWidth="3"
                strokeLinecap="round"
              />
            </g>
          );
        })}

        {/* Hub glow backdrop */}
        <circle cx={cx} cy={cy} r={orbitR * 0.3} fill="url(#hub-glow)" />

        {/* Hub */}
        <g filter="url(#node-shadow)">
          <circle
            cx={cx} cy={cy}
            r={36}
            fill={colors.surface}
            stroke={colors.accent}
            strokeWidth="1.5"
            strokeOpacity="0.5"
          />
          <text
            x={cx} y={cy - 3}
            textAnchor="middle"
            fill={colors.accent}
            fontSize="14"
            fontWeight="700"
            fontFamily="monospace"
          >
            {totalActive}
          </text>
          <text
            x={cx} y={cy + 11}
            textAnchor="middle"
            fill={colors.textMuted}
            fontSize="8"
            fontFamily="monospace"
            opacity="0.6"
          >
            active
          </text>
        </g>

        {/* Member nodes */}
        {pool.members.map((member, i) => {
          const angle = (i / pool.members.length) * Math.PI * 2 - Math.PI / 2;
          const x = cx + Math.cos(angle) * orbitR;
          const y = cy + Math.sin(angle) * orbitR;
          const color = member.healthy ? colors.success : colors.danger;
          const nodeR = member.weight ? 6 + Math.min(member.weight, 10) * 0.5 : 6;

          const errorPct = member.total_requests > 0
            ? Math.round((member.total_errors / member.total_requests) * 100)
            : 0;

          return (
            <g
              key={member.provider_name}
              onMouseEnter={() => onMemberHover?.(member.provider_name)}
              onMouseLeave={() => onMemberHover?.(null)}
              style={{ cursor: "pointer" }}
            >
              <circle
                cx={x} cy={y}
                r={nodeR + 6}
                fill={color}
                opacity="0.06"
              />
              <circle
                cx={x} cy={y}
                r={nodeR}
                fill={color}
                filter="url(#node-shadow)"
              />
              <circle
                cx={x} cy={y}
                r={nodeR * 0.35}
                fill="#fff"
                opacity="0.3"
              />

              <text
                x={x}
                y={y + nodeR + 14}
                textAnchor="middle"
                fill={member.healthy ? colors.text : colors.danger}
                fontSize="10"
                fontWeight="600"
                fontFamily="monospace"
              >
                {member.provider_name}
              </text>

              {/* Stat row: active / total / err% */}
              <text
                x={x}
                y={y + nodeR + 26}
                textAnchor="middle"
                fill={colors.textMuted}
                fontSize="8"
                fontFamily="monospace"
                opacity="0.55"
              >
                {member.active_requests} act · {errorPct}% err
              </text>
            </g>
          );
        })}

        {/* Top-left strategy badge */}
        <rect x={14} y={14} rx="6" ry="6" width={pool.strategy.length * 8 + 24} height="24"
          fill={colors.surface} stroke={colors.border} strokeWidth="0.5" opacity="0.9" />
        <text x={26} y={30} fill={colors.accent} fontSize="10" fontWeight="700" fontFamily="monospace">
          {pool.strategy.replace(/_/g, " ").toUpperCase()}
        </text>

        {/* Total traffic */}
        <rect x={14} y={44} rx="6" ry="6" width="100" height="22"
          fill={colors.surface} stroke={colors.border} strokeWidth="0.5" opacity="0.9" />
        <text x={26} y={59} fill={colors.textMuted} fontSize="9" fontWeight="600" fontFamily="monospace">
          {totalTraffic.toLocaleString()} total
        </text>

        {/* Pool name bottom-right */}
        <text
          x={dims.width - 16}
          y={dims.height - 12}
          textAnchor="end"
          fill={colors.textMuted}
          fontSize="11"
          fontWeight="600"
          fontFamily="monospace"
          opacity="0.4"
        >
          {pool.name}
        </text>
      </svg>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  SVG arc path helper                                                */
/* ------------------------------------------------------------------ */

function describeArc(
  cx: number, cy: number, r: number,
  startAngle: number, endAngle: number,
): string {
  const x1 = cx + r * Math.cos(startAngle);
  const y1 = cy + r * Math.sin(startAngle);
  const x2 = cx + r * Math.cos(endAngle);
  const y2 = cy + r * Math.sin(endAngle);
  const large = endAngle - startAngle > Math.PI ? 1 : 0;
  return `M${x1},${y1} A${r},${r} 0 ${large} 1 ${x2},${y2}`;
}

/* ------------------------------------------------------------------ */
/*  Container size hook                                                */
/* ------------------------------------------------------------------ */

function useContainerSize(
  ref: React.RefObject<HTMLDivElement | null>,
): { width: number; height: number } {
  const [size, setSize] = React.useState({ width: 800, height: 520 });

  React.useEffect(() => {
    const el = ref.current;
    if (!el) return;

    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        setSize({ width: Math.floor(width), height: Math.floor(height) });
      }
    });

    observer.observe(el);
    return () => observer.disconnect();
  }, [ref]);

  return size;
}

/* ------------------------------------------------------------------ */
/*  Theme color hook                                                   */
/* ------------------------------------------------------------------ */

type Colors = Pick<Record<string, string>, "accent" | "success" | "danger" | "text" | "textMuted" | "surface" | "border">;

function readColors(): Colors {
  const s = getComputedStyle(document.documentElement);
  const v = (name: string, fallback: string): string => {
    const raw = s.getPropertyValue(name).trim();
    if (!raw) return fallback;
    if (raw.startsWith("#") || raw.startsWith("hsl") || raw.startsWith("rgb") || raw.startsWith("var(")) return raw;
    return `hsl(${raw})`;
  };
  return {
    accent: v("--accent", "#6AD87A"),
    success: v("--success", "#5CB884"),
    danger: v("--danger", "#C44A4A"),
    text: v("--text", "#EDE8DA"),
    textMuted: v("--text-muted", "#A3B0A4"),
    surface: v("--bg-surface", "#1B2A22"),
    border: v("--border", "#24382E"),
  };
}

function useThemeColors(): Colors {
  const [colors, setColors] = React.useState<Colors>(readColors);

  React.useEffect(() => {
    const update = () => setColors(readColors());

    const observer = new MutationObserver(update);
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });

    const mq = window.matchMedia("(prefers-color-scheme: light)");
    mq.addEventListener("change", update);

    return () => {
      observer.disconnect();
      mq.removeEventListener("change", update);
    };
  }, []);

  return colors;
}

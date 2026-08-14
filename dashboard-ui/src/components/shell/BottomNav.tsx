import { Link, useRouterState } from "@tanstack/react-router";
import { LayoutDashboard, Box, Terminal, Settings, Workflow } from "lucide-react";
import { cn } from "@/lib/utils";

const TABS = [
  { to: "/admin/dashboard/overview", label: "Home", Icon: LayoutDashboard },
  { to: "/admin/dashboard/models", label: "Models", Icon: Box },
  { to: "/admin/dashboard/fallback", label: "Fallback", Icon: Workflow },
  { to: "/admin/dashboard/console", label: "Console", Icon: Terminal },
  { to: "/admin/dashboard/settings", label: "Settings", Icon: Settings },
];

export function BottomNav(): JSX.Element {
  const { location } = useRouterState();
  const path = location.pathname;

  return (
    <nav className="fixed bottom-0 left-0 right-0 z-40 border-t border-border bg-surface/95 backdrop-blur-xl md:hidden safe-area-inset-bottom">
      <div className="flex items-center justify-around h-16">
        {TABS.map(({ to, label, Icon }) => {
          const active = path === to || path.startsWith(to + "/");
          return (
            <Link
              key={to}
              to={to}
              className={cn(
                "flex flex-col items-center justify-center gap-1 w-full h-full text-[11px] font-medium transition-all active:scale-95",
                active ? "text-accent" : "text-muted-foreground active:text-foreground"
              )}
            >
              <Icon className={cn("h-6 w-6 transition-transform", active && "text-accent scale-110")} />
              <span className={cn("transition-all", active && "font-semibold")}>{label}</span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}

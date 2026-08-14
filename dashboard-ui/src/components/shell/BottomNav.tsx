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
    <nav className="fixed bottom-0 left-0 right-0 z-40 border-t border-border bg-surface md:hidden">
      <div className="flex items-center justify-around h-14">
        {TABS.map(({ to, label, Icon }) => {
          const active = path === to || path.startsWith(to + "/");
          return (
            <Link
              key={to}
              to={to}
              className={cn(
                "flex flex-col items-center justify-center gap-0.5 w-full h-full text-[10px] font-medium transition-colors",
                active ? "text-accent" : "text-muted-foreground"
              )}
            >
              <Icon className={cn("h-5 w-5", active && "text-accent")} />
              <span>{label}</span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}

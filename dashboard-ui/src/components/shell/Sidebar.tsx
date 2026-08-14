import * as React from "react";
import { Link, useRouterState, useNavigate } from "@tanstack/react-router";
import {
  BookOpen,
  Box,
  ChartColumn,
  Database,
  History,
  KeyRound,
  LayoutDashboard,
  LockKeyhole,
  LogOut,
  MessageSquareText,
  Monitor,
  Moon,
  Network,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  ShieldCheck,
  Sun,
  Terminal,
  Layers,
  Workflow,
} from "lucide-react";
import { cn } from "@/lib/utils";
import {
  flagOn,
  hasCapability,
  type DashboardConfigResponse,
} from "@/lib/api/dashboard-config";
import { setTheme, toggleTheme, useTheme, type Theme } from "@/lib/theme/theme";
import { useApiKeyState } from "@/lib/auth/useApiKey";
import { useSession } from "@/lib/auth/useSession";
import { clearSession, isLoggedIn, fetchMe } from "@/lib/auth/session";
import { clearApiKey } from "@/lib/auth/storage";

type IconType = React.ComponentType<{ className?: string }>;

interface NavEntry {
  to: string;
  label: string;
  Icon: IconType;
  show?: (cfg: DashboardConfigResponse | undefined) => boolean;
  requiredResource?: string;
  requiredAction?: "read" | "write";
  requiredCapability?: string;
}

const NAV: readonly NavEntry[] = [
  { to: "/admin/dashboard/overview", label: "Overview", Icon: LayoutDashboard, requiredResource: "admin/dashboard" },
  { to: "/admin/dashboard/pools", label: "Pools", Icon: Network, requiredResource: "admin/pools" },
  { to: "/admin/dashboard/guide", label: "Guide", Icon: BookOpen },
  { to: "/admin/dashboard/playground", label: "Playground", Icon: MessageSquareText },
  { to: "/admin/dashboard/models", label: "Models", Icon: Box, requiredResource: "admin/models" },
  { to: "/admin/dashboard/combos", label: "Combos", Icon: Layers, requiredResource: "admin/models" },
  { to: "/admin/dashboard/fallback", label: "Fallback", Icon: Workflow, requiredResource: "admin/models" },
  { to: "/admin/dashboard/audit-logs", label: "Audit Logs", Icon: History, requiredResource: "admin/audit" },
  { to: "/admin/dashboard/console", label: "Live Console", Icon: Terminal, requiredResource: "admin/audit" },
  { to: "/admin/dashboard/usage", label: "Usage", Icon: ChartColumn, requiredResource: "admin/usage" },
  {
    to: "/admin/dashboard/cache",
    label: "Cache",
    Icon: Database,
    show: (cfg) => flagOn(cfg?.CACHE_ENABLED),
    requiredResource: "admin/cache",
  },

  { to: "/admin/dashboard/auth-keys", label: "API Keys", Icon: KeyRound, requiredResource: "admin/keys" },
  { to: "/admin/dashboard/workflows", label: "Workflows", Icon: Workflow, requiredResource: "admin/workflows" },
  {
    to: "/admin/dashboard/guardrails",
    label: "Guardrails",
    Icon: ShieldCheck,
    show: (cfg) => flagOn(cfg?.GUARDRAILS_ENABLED),
    requiredResource: "admin/guardrails",
  },
  { to: "/admin/dashboard/settings", label: "Settings", Icon: Settings, requiredResource: "admin/settings" },
];

export interface SidebarProps {
  config: DashboardConfigResponse | undefined;
  onOpenAuthDialog: () => void;
}

export function Sidebar({
  config,
  onOpenAuthDialog,
}: SidebarProps): JSX.Element {
  const theme = useTheme();
  const apiKey = useApiKeyState();
  const navigate = useNavigate();
  const { location } = useRouterState();
  const path = location.pathname;
  const { loggedIn, user } = useSession();

  const [isCollapsed, setIsCollapsed] = React.useState(() => {
    return localStorage.getItem("aurora_sidebar_collapsed") === "true";
  });

  // Fetch permissions from /auth/me after identity login.
  React.useEffect(() => {
    if (loggedIn && user?.user.id && user.permissions.length === 0) {
      void fetchMe();
    }
  }, [loggedIn, user?.user.id, user?.permissions.length]);

  const canAccess = React.useCallback(
    (entry: NavEntry): boolean => {
      if (!entry.requiredResource) return true;
      // Logged-in identity users check permissions.
      if (loggedIn && user) {
        const denied = user.permissions.some(
          (p) =>
            p.effect === "deny" &&
            (p.resource === "*" || p.resource === entry.requiredResource) &&
            (!entry.requiredAction || p.action === "*" || p.action === entry.requiredAction),
        );
        if (denied) return false;
        return user.permissions.some(
          (p) =>
            p.effect === "allow" &&
            (p.resource === "*" || p.resource === entry.requiredResource) &&
            (!entry.requiredAction || p.action === "*" || p.action === entry.requiredAction),
        );
      }
      // API key / master key users get full access only after server validation.
      if (apiKey.key && apiKey.verified) return true;
      return false;
    },
    [loggedIn, user, apiKey.key, apiKey.verified],
  );

  const handleLogout = () => {
    clearSession();
    clearApiKey();
    navigate({ to: "/admin/dashboard/login" });
  };

  const handleDisconnectKey = () => {
    clearApiKey();
    navigate({ to: "/admin/dashboard/login" });
  };

  const toggleCollapse = () => {
    const next = !isCollapsed;
    setIsCollapsed(next);
    localStorage.setItem("aurora_sidebar_collapsed", String(next));
  };

  return (
    <aside
      className={cn(
        "flex h-full shrink-0 flex-col transition-[width] duration-300 ease-[var(--ease-ios)] glass-chrome z-20",
        isCollapsed ? "w-[72px]" : "w-[var(--spacing-sidebar)]"
      )}
      aria-label="Primary navigation"
    >
      <header className={cn("flex items-center pb-5 pt-6 transition-all duration-300", isCollapsed ? "flex-col gap-2 px-2 justify-center" : "gap-3 px-5")}>
        <span className="grid h-9 w-9 shrink-0 place-items-center  bg-accent text-accent-foreground shadow-lg shadow-accent/25">
          <SidebarLogo />
        </span>
        {!isCollapsed && (
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h1 className="font-display text-lg font-bold tracking-tight text-foreground whitespace-nowrap overflow-hidden">Aurora</h1>
            </div>
          </div>
        )}
      </header>

      <nav className="flex-1 overflow-y-auto px-3 py-2 overflow-x-hidden">
        <ul className="flex flex-col gap-0.5">
          {NAV.filter((entry) => (!entry.show || entry.show(config)) && hasCapability(config, entry.requiredCapability) && canAccess(entry)).map(
            (entry) => {
              const active =
                path === entry.to ||
                path.startsWith(entry.to + "/") ||
                (entry.to.endsWith("/overview") && path === "/admin/dashboard");
              return (
                <li key={entry.to} title={isCollapsed ? entry.label : undefined}>
                  <Link
                    to={entry.to}
                    className={cn(
                      "group flex items-center gap-3 py-2 text-[13px] font-medium transition-all duration-200 ease-[var(--ease-ios)]",
                      "text-muted-foreground hover:bg-surface-hover/60 hover:text-foreground",
                      active && "bg-accent/12 text-accent",
                      isCollapsed ? "px-0 justify-center" : "px-3"
                    )}
                    aria-current={active ? "page" : undefined}
                  >
                    <entry.Icon
                      className={cn(
                        "h-4 w-4 shrink-0 transition-all duration-200 ease-[var(--ease-spring)] group-hover:scale-110",
                        active && "text-accent"
                      )}
                    />
                    {!isCollapsed && <span className="truncate whitespace-nowrap">{entry.label}</span>}
                  </Link>
                </li>
              );
            },
          )}
        </ul>
      </nav>

      <footer className={cn("flex flex-col gap-2.5 border-t border-border/30 py-4 transition-all duration-300", isCollapsed ? "px-2 items-center" : "px-4")}>


        {!isCollapsed && <ThemeToggle theme={theme} />}
        {!isCollapsed && <MobileThemeButton theme={theme} />}


        {loggedIn && user ? (
          <div className={cn(
            "flex items-center py-2 text-[12px] font-medium border border-border/30",
            isCollapsed ? "px-0 justify-center gap-1" : "px-3 gap-2"
          )}>
            {!isCollapsed && (
              <span className="flex-1 truncate text-muted-foreground">
                {user.user.display_name || user.user.email}
              </span>
            )}
            <button
              type="button"
              onClick={handleLogout}
              title="Sign out"
              className="shrink-0 p-1 text-muted-foreground hover:text-foreground hover:bg-surface-hover/60 transition-colors"
            >
              <LogOut className="h-3.5 w-3.5" />
            </button>
            <SidebarCollapseButton isCollapsed={isCollapsed} onToggle={toggleCollapse} />
          </div>
        ) : apiKey.key && !isLoggedIn() ? (
          <div className={cn(
            "flex items-center py-2 text-[12px] font-medium border border-border/30 bg-surface/30",
            isCollapsed ? "px-0 justify-center gap-1" : "px-3 gap-2"
          )}>
            {!isCollapsed && (
              <span className="flex-1 truncate text-muted-foreground">
                Verified admin key
              </span>
            )}
            <button
              type="button"
              onClick={handleDisconnectKey}
              title="Disconnect key"
              className="shrink-0 p-1 text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors"
            >
              <LogOut className="h-3.5 w-3.5" />
            </button>
            <SidebarCollapseButton isCollapsed={isCollapsed} onToggle={toggleCollapse} />
          </div>
        ) : (
          <div className={cn("flex items-center", isCollapsed ? "gap-1" : "gap-2")}>
            <button
              type="button"
              onClick={onOpenAuthDialog}
              title={isCollapsed ? "Enter API key" : undefined}
              className={cn(
                "flex items-center py-2.5 text-[12px] font-medium transition-all duration-200 ease-[var(--ease-ios)]",
                "text-muted-foreground bg-background/40 hover:bg-surface-hover/60 hover:text-foreground border border-border/30",
                isCollapsed ? "px-0 w-10 justify-center shrink-0" : "px-3 gap-2.5 flex-1"
              )}
            >
              <LockKeyhole className="h-4 w-4 shrink-0" aria-hidden />
              {!isCollapsed && <span className="truncate whitespace-nowrap">Enter API key</span>}
            </button>
            <SidebarCollapseButton isCollapsed={isCollapsed} onToggle={toggleCollapse} />
          </div>
        )}
      </footer>
    </aside>
  );
}

function SidebarLogo(): JSX.Element {
  return (
    <img
      src="/navbar-logo.svg"
      alt="Aurora"
      className="h-6 w-6 object-contain"
      draggable={false}
    />
  );
}


interface SidebarCollapseButtonProps {
  isCollapsed: boolean;
  onToggle: () => void;
}

function SidebarCollapseButton({ isCollapsed, onToggle }: SidebarCollapseButtonProps): JSX.Element {
  return (
    <button
      type="button"
      onClick={onToggle}
      title={isCollapsed ? "Expand sidebar" : "Collapse sidebar"}
      aria-label={isCollapsed ? "Expand sidebar" : "Collapse sidebar"}
      className={cn(
        "grid h-8 w-8 place-items-center text-muted-foreground transition-all duration-200 ease-[var(--ease-ios)]",
        "hover:bg-surface-hover/60 hover:text-foreground",
      )}
    >
      {isCollapsed ? <PanelLeftOpen className="h-4 w-4 shrink-0" /> : <PanelLeftClose className="h-4 w-4 shrink-0" />}
    </button>
  );
}

interface ThemeToggleProps {
  theme: Theme;
}

function ThemeToggle({ theme }: ThemeToggleProps): JSX.Element {
  const choices: ReadonlyArray<{ value: Theme; Icon: IconType; label: string }> = [
    { value: "light", Icon: Sun, label: "Light theme" },
    { value: "system", Icon: Monitor, label: "System theme" },
    { value: "dark", Icon: Moon, label: "Dark theme" },
  ];
  return (
    <div
      className="hidden grid-cols-3 overflow-hidden bg-background/30 p-0.5 md:grid border border-border/40"
      role="radiogroup"
      aria-label="Color theme"
    >
      {choices.map(({ value, Icon, label }) => {
        const active = theme === value;
        return (
          <button
            key={value}
            type="button"
            role="radio"
            aria-checked={active}
            aria-label={label}
            title={label}
            onClick={() => setTheme(value)}
            className={cn(
              "grid h-7 place-items-center text-muted-foreground transition-all duration-200 ease-[var(--ease-ios)] hover:text-foreground",
              active && "bg-surface text-foreground border border-border/40",
            )}
          >
            <Icon className={cn("h-3.5 w-3.5", active && "scale-110")} />
          </button>
        );
      })}
    </div>
  );
}

function MobileThemeButton({ theme }: ThemeToggleProps): JSX.Element {
  const Icon = theme === "light" ? Sun : theme === "dark" ? Moon : Monitor;
  const label =
    theme === "light"
      ? "Light theme"
      : theme === "dark"
        ? "Dark theme"
        : "System theme";
  return (
    <button
      type="button"
      onClick={toggleTheme}
      title={label}
      aria-label={label}
      className={cn(
        "grid h-8 w-8 place-items-center self-end border border-border text-muted-foreground transition-colors md:hidden",
        "hover:bg-surface-hover hover:text-foreground",
      )}
    >
      <Icon className="h-4 w-4" />
    </button>
  );
}

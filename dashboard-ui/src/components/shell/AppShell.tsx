import * as React from "react";
import { Outlet, useRouterState, useNavigate } from "@tanstack/react-router";
import { Sidebar } from "./Sidebar";
import { AuthDialog } from "./AuthDialog";
import { useDashboardConfig } from "@/lib/api/useDashboardConfig";
import { flagOn, hasCapability } from "@/lib/api/dashboard-config";
import { isApiKeyVerified, markApiKeyVerified, onAuthStale } from "@/lib/auth/storage";
import { useApplyThemeAttr } from "@/lib/theme/theme";
import { isLoggedIn, setSessionFromUserInfo } from "@/lib/auth/session";
import { getApiKey } from "@/lib/auth/storage";
import { apiFetch, ApiError } from "@/lib/api/client";
import { withBasePath } from "@/lib/basepath";
import { Loader2 } from "lucide-react";
import { TenantScopeProvider } from "@/lib/tenant/tenant-scope";

/**
 * Application shell.
 * - Applies the persisted theme to <html data-theme>.
 * - Hosts sidebar + page outlet.
 * - Owns the auth dialog: opens on auth-stale event or sidebar click.
 * - Loads /admin/api/v1/dashboard/config once and feeds feature flags into
 *   the sidebar so Budgets/Guardrails entries match the legacy dashboard.
 * - Renders a minimal layout for the login page (no sidebar).
 */
type AuthStatus = "loading" | "authenticated" | "unauthenticated" | "no-identity";

export function AppShell(): JSX.Element {
  useApplyThemeAttr();
  const navigate = useNavigate();
  const location = useRouterState({ select: (s) => s.location });

  const isLoginPage =
    location.pathname === "/admin/dashboard/login" ||
    location.pathname === "/admin/dashboard/login/" ||
    location.pathname === "/admin/dashboard/oidc-callback";

  // Auth guard: probe /auth/me before rendering any page content.
  // This prevents the flash-of-content before the 401 redirect.
  const [authStatus, setAuthStatus] = React.useState<AuthStatus>("loading");
  const probeDone = React.useRef(false);

  React.useEffect(() => {
    if (probeDone.current) return;
    if (isLoginPage) { setAuthStatus("authenticated"); return; }

    // Have already validated stored credentials — let them through immediately.
    if (isLoggedIn() || isApiKeyVerified()) {
      setAuthStatus("authenticated");
      probeDone.current = true;
      return;
    }

    if (getApiKey()) {
      apiFetch("/admin/api/v1/dashboard/config")
        .then(() => {
          markApiKeyVerified();
          setAuthStatus("authenticated");
        })
        .catch(() => setAuthStatus("unauthenticated"))
        .finally(() => { probeDone.current = true; });
      return;
    }

    // No credentials — probe /auth/me to determine identity state.
    fetch(withBasePath("/admin/api/v1/auth/me"), { credentials: "same-origin" })
      .then((res) => {
        if (res.ok) {
          // Already authenticated — populate session data.
          setAuthStatus("authenticated");
          res.json().then((data) => {
            if (data?.user) {
              setSessionFromUserInfo({
                user: data.user,
                roles: data.roles ?? [],
                permissions: data.permissions ?? [],
              });
            }
          }).catch(() => {});
          return;
        }
        if (res.status === 503) {
          // Identity service is disabled — show AuthDialog.
          setAuthStatus("no-identity");
          return;
        }
        // 401 — identity enabled, redirect to login.
        setAuthStatus("unauthenticated");
      })
      .catch(() => {
        setAuthStatus("unauthenticated");
      })
      .finally(() => { probeDone.current = true; });
  }, [isLoginPage]);

  // Redirect once auth status is determined.
  React.useEffect(() => {
    if (authStatus === "unauthenticated") {
      navigate({ to: "/admin/dashboard/login" });
    }
  }, [authStatus, navigate]);

  const config = useDashboardConfig();
  const identityEnabled = flagOn(config.data?.IDENTITY_ENABLED) && hasCapability(config.data, "identity");
  const [authOpen, setAuthOpen] = React.useState(false);
  const [needsAuth, setNeedsAuth] = React.useState(false);

  // If identity is enabled and already logged in on login page, redirect to overview.
  React.useEffect(() => {
    if (isLoginPage && identityEnabled && isLoggedIn()) {
      navigate({ to: "/admin/dashboard/overview" });
    }
  }, [isLoginPage, identityEnabled, navigate]);

  // Reset auth dialog on login pages and when leaving them.
  React.useEffect(() => {
    setAuthOpen(false);
    setNeedsAuth(false);
  }, [isLoginPage]);

  React.useEffect(
    () =>
      onAuthStale(() => {
        setNeedsAuth(true);
        if (identityEnabled) {
          navigate({ to: "/admin/dashboard/login" });
        } else {
          setAuthOpen(true);
        }
      }),
    [identityEnabled, navigate],
  );

  React.useEffect(() => {
    if (config.error instanceof ApiError && config.error.status === 401) {
      if (identityEnabled) {
        navigate({ to: "/admin/dashboard/login" });
      } else {
        setNeedsAuth(true);
        setAuthOpen(true);
      }
    }
  }, [config.error, identityEnabled, navigate]);

  // While auth is being resolved, show a loading indicator.
  if (authStatus === "loading" || authStatus === "unauthenticated") {
    return (
      <div className="flex h-full min-h-screen items-center justify-center bg-background">
        <Loader2 className="h-6 w-6 animate-spin text-accent" />
      </div>
    );
  }

  if (isLoginPage) {
    return (
      <div className="min-h-screen bg-background text-foreground">
        <Outlet />
      </div>
    );
  }

  return (
    <TenantScopeProvider>
      <div className="flex h-full min-h-screen bg-background text-foreground selection:bg-accent/20 selection:text-foreground">
        <Sidebar
          config={config.data}
          onOpenAuthDialog={() => {
            setNeedsAuth(false);
            setAuthOpen(true);
          }}
        />
        <main className="flex min-w-0 flex-1 flex-col overflow-x-hidden p-6 sm:p-8 lg:p-10 mx-auto w-full transition-all duration-300 ease-[var(--ease-ios)]">
          <div className="mx-auto w-full max-w-9xl flex-1">
            <Outlet />
          </div>
        </main>
        <AuthDialog
          open={authOpen}
          needsAuth={needsAuth}
          identityEnabled={identityEnabled}
          onOpenChange={(next) => {
            setAuthOpen(next);
            if (!next) setNeedsAuth(false);
          }}
        />
      </div>
    </TenantScopeProvider>
  );
}

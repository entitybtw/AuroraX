import {
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
  RouterProvider,
} from "@tanstack/react-router";
import { AppShell } from "@/components/shell/AppShell";
import { RequirePermission } from "@/components/shell/RequirePermission";
import { LoginPage } from "@/routes/login";
import { OIDCCallbackPage } from "@/routes/oidc-callback";
import { OverviewPage } from "@/routes/overview";
import { GuidePage } from "@/routes/guide";
import { PlaygroundPage } from "@/routes/playground";
import { ModelsPage } from "@/routes/models";
import { AuditLogsPage } from "@/routes/audit-logs";
import { UsagePage } from "@/routes/usage";
import { AuthKeysPage } from "@/routes/auth-keys";
import { WorkflowsPage } from "@/routes/workflows";
import { GuardrailsPage } from "@/routes/guardrails";
import { CachePage } from "@/routes/cache";
import { SettingsPage } from "@/routes/settings";
import { PoolsPage } from "@/routes/pools";

import { ConsolePage } from "@/routes/console";
import { CombosPage } from "@/routes/combos";

import { getBasePath } from "@/lib/basepath";

const rootRoute = createRootRoute({ component: AppShell });

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/admin/dashboard/login",
  component: LoginPage,
});

const oidcCallbackRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/admin/dashboard/oidc-callback",
  component: OIDCCallbackPage,
});

const indexRedirect = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  beforeLoad: () => {
    throw redirect({ to: "/admin/dashboard/overview" });
  },
});

const dashboardLayout = createRoute({
  getParentRoute: () => rootRoute,
  path: "/admin/dashboard",
  beforeLoad: ({ location }) => {
    if (
      location.pathname === "/admin/dashboard" ||
      location.pathname === "/admin/dashboard/"
    ) {
      throw redirect({ to: "/admin/dashboard/overview" });
    }
  },
});

function protectedComponent(Component: () => JSX.Element, resource?: string, capability?: string): () => JSX.Element {
  return function ProtectedRoute(): JSX.Element {
    return (
      <RequirePermission resource={resource} capability={capability}>
        <Component />
      </RequirePermission>
    );
  };
}

const overviewRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "overview",
  component: protectedComponent(OverviewPage, "admin/dashboard"),
});


const guideRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "guide",
  component: protectedComponent(GuidePage, "admin/dashboard"),
});

const playgroundRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "playground",
  component: PlaygroundPage,
});

const modelsRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "models",
  component: protectedComponent(ModelsPage, "admin/models"),
});

const auditLogsRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "audit-logs",
  component: protectedComponent(AuditLogsPage, "admin/audit"),
});

const consoleRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "console",
  component: protectedComponent(ConsolePage, "admin/audit"),
});

const combosRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "combos",
  component: protectedComponent(CombosPage, "admin/models"),
});

const cliToolsRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "cli-tools",
  beforeLoad: () => {
    throw redirect({ to: "/admin/dashboard/guide" });
  },
});

const usageRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "usage",
  component: protectedComponent(UsagePage, "admin/usage"),
});


const authKeysRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "auth-keys",
  component: protectedComponent(AuthKeysPage, "admin/keys"),
});


const workflowsRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "workflows",
  component: protectedComponent(WorkflowsPage, "admin/workflows"),
});

const guardrailsRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "guardrails",
  component: protectedComponent(GuardrailsPage, "admin/guardrails"),
});

const cacheRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "cache",
  component: protectedComponent(CachePage, "admin/cache"),
});

const settingsRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "settings",
  component: protectedComponent(SettingsPage, "admin/settings"),
});



const poolsRoute = createRoute({
  getParentRoute: () => dashboardLayout,
  path: "pools",
  component: protectedComponent(PoolsPage, "admin/pools"),
});

const routeTree = rootRoute.addChildren([
  indexRedirect,
  loginRoute,
  oidcCallbackRoute,
  dashboardLayout.addChildren([
    overviewRoute,
    poolsRoute,

    guideRoute,
    playgroundRoute,
    modelsRoute,
    combosRoute,
    auditLogsRoute,
    consoleRoute,
    cliToolsRoute,
    usageRoute,
    authKeysRoute,
    workflowsRoute,
    guardrailsRoute,
    cacheRoute,
    settingsRoute,
  ]),
]);

const router = createRouter({
  routeTree,
  defaultPreload: "intent",
  defaultPreloadStaleTime: 0,
  basepath: getBasePath(),
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

export function AppRouter(): JSX.Element {
  return <RouterProvider router={router} />;
}

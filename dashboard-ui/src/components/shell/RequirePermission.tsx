import { ForbiddenPage } from "./ForbiddenPage";
import { hasCapability } from "@/lib/api/dashboard-config";
import { useDashboardConfig } from "@/lib/api/useDashboardConfig";
import { useApiKeyState } from "@/lib/auth/useApiKey";
import { useSession } from "@/lib/auth/useSession";

interface RequirePermissionProps {
  resource?: string | undefined;
  action?: "read" | "write";
  capability?: string | undefined;
  children: JSX.Element;
}

export function RequirePermission({ resource, action = "read", capability, children }: RequirePermissionProps): JSX.Element {
  const apiKey = useApiKeyState();
  const { data: config } = useDashboardConfig();
  const { loggedIn, user } = useSession();

  if (!hasCapability(config, capability)) return <ForbiddenPage />;
  if (!resource) return children;
  if (apiKey.key && apiKey.verified) return children;
  if (!loggedIn || !user) return <ForbiddenPage />;

  const denied = user.permissions.some(
    (permission) =>
      permission.effect === "deny" &&
      (permission.resource === "*" || permission.resource === resource) &&
      (permission.action === "*" || permission.action === action),
  );
  if (denied) return <ForbiddenPage />;

  const allowed = user.permissions.some(
    (permission) =>
      permission.effect === "allow" &&
      (permission.resource === "*" || permission.resource === resource) &&
      (permission.action === "*" || permission.action === action),
  );
  return allowed ? children : <ForbiddenPage />;
}

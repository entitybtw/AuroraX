import { ForbiddenPage } from "./ForbiddenPage";
import { FeatureDisabledPage } from "./FeatureDisabledPage";
import { hasCapability } from "@/lib/api/dashboard-config";
import { useDashboardConfig } from "@/lib/api/useDashboardConfig";
import { useApiKeyState } from "@/lib/auth/useApiKey";
import { useSession } from "@/lib/auth/useSession";
import { extractHiddenFeatures, hiddenFeatureSet } from "@/lib/features/features";

interface RequirePermissionProps {
  resource?: string | undefined;
  action?: "read" | "write";
  capability?: string | undefined;
  /** Operator feature id; when hidden, the page shows a disabled banner (data intact). */
  featureId?: string | undefined;
  children: JSX.Element;
}

export function RequirePermission({ resource, action = "read", capability, featureId, children }: RequirePermissionProps): JSX.Element {
  const apiKey = useApiKeyState();
  const { data: config } = useDashboardConfig();
  const { loggedIn, user } = useSession();

  if (featureId && hiddenFeatureSet(extractHiddenFeatures(config)).has(featureId)) {
    return <FeatureDisabledPage featureId={featureId} />;
  }
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

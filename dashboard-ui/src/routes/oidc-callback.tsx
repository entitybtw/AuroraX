import * as React from "react";
import { useNavigate, useRouterState } from "@tanstack/react-router";
import { ShieldCheck } from "lucide-react";
import { apiFetch, ApiError } from "@/lib/api/client";
import { setSessionFromLogin, type LoginResponse } from "@/lib/auth/session";
import { ensureLocaleSynced, t } from "@/lib/i18n";


export function OIDCCallbackPage(): JSX.Element {
  const navigate = useNavigate();
  const loc = useRouterState({ select: (s) => s.location });
  const search = typeof loc.search === "string" ? loc.search : "";
  const params = Object.fromEntries(new URLSearchParams(search).entries());
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    const code = params.code;
    const state = params.state;

    if (!code) {
      setError("Missing authorization code. Sign in again.");
      return;
    }

    const exchange = async () => {
      try {
        const resp = await apiFetch<LoginResponse>(
          "/admin/api/v1/auth/oidc/callback",
          {
            method: "POST",
            json: { code, state },
            credentials: "same-origin",
          },
        );

        setSessionFromLogin(resp);
        await navigate({ to: "/admin/dashboard/overview" });
        // Pick up the persisted UI language now that the session exists.
        void ensureLocaleSynced();
      } catch (err) {
        if (err instanceof ApiError) {
          setError(err.message);
        } else {
          setError("Could not complete sign in. Try again.");
        }
      }
    };

    exchange();
  }, []);

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm text-center space-y-4">
        <div className="mx-auto grid h-12 w-12 place-items-center rounded-2xl bg-accent text-accent-foreground shadow-lg shadow-accent/25">
          <ShieldCheck className="h-6 w-6" />
        </div>
        {error ? (
          <>
            <h1 className="text-lg font-semibold">{t("Sign in failed")}</h1>
            <p className="text-sm text-destructive">{error}</p>
            <button
              type="button"
              onClick={() => navigate({ to: "/admin/dashboard/login" })}
              className="text-sm text-accent hover:underline"
            >{t("Back to login")}</button>
          </>
        ) : (
          <>
            <h1 className="text-lg font-semibold">{t("Completing sign in")}</h1>
            <p className="text-sm text-muted-foreground">{t("Redirecting to dashboard…")}</p>
          </>
        )}
      </div>
    </div>
  );
}

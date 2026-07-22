import * as React from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { LockKeyhole, Mail, LogIn, ShieldCheck, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiError } from "@/lib/api/client";
import { markApiKeyVerified, setApiKey } from "@/lib/auth/storage";
import { setSessionFromLogin, type LoginResponse } from "@/lib/auth/session";

interface OIDCProviderInfo {
  name: string;
  display_name: string;
  enabled: boolean;
}

interface MeProbeResponse {
  identity_enabled?: boolean;
  oidc_providers?: OIDCProviderInfo[];
}

export function LoginPage(): JSX.Element {
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  // Probe /auth/me (auth-skipped) to determine identity state without needing
  // the authenticated dashboard config endpoint.
  const [probeLoading, setProbeLoading] = React.useState(true);
  const [identityEnabled, setIdentityEnabled] = React.useState(false);
  const [oidcProviders, setOidcProviders] = React.useState<OIDCProviderInfo[]>([]);
  const oidcEnabled = oidcProviders.some((p) => p.enabled);

  React.useEffect(() => {
    fetch("/admin/api/v1/auth/me", { credentials: "same-origin" })
      .then((res) => {
        if (res.status === 401) {
          // Identity enabled, not authenticated.
          return res.json().then((data: MeProbeResponse) => {
            setIdentityEnabled(true);
            setOidcProviders(data.oidc_providers ?? []);
          });
        }
        if (res.status === 503) {
          // Identity disabled.
          setIdentityEnabled(false);
          setOidcProviders([]);
          return;
        }
        // 200 — already authenticated, redirect.
        navigate({ to: "/admin/dashboard/overview" });
      })
      .catch(() => {
        setIdentityEnabled(false);
      })
      .finally(() => setProbeLoading(false));
  }, [navigate]);

  const [tab, setTab] = React.useState<"sso" | "master">("master");
  const [tabInitialized, setTabInitialized] = React.useState(false);

  React.useEffect(() => {
    if (!probeLoading && !tabInitialized) {
      setTab(identityEnabled ? "sso" : "master");
      setTabInitialized(true);
    }
  }, [probeLoading, identityEnabled, tabInitialized]);

  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [masterKey, setMasterKey] = React.useState("");
  const [error, setError] = React.useState("");
  const [submitting, setSubmitting] = React.useState(false);
  const [sessionDone, setSessionDone] = React.useState(false);

  React.useEffect(() => {
    if (sessionDone) {
      navigate({ to: "/admin/dashboard/overview" });
    }
  }, [sessionDone, navigate]);

  const handleSSOLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email.trim() || !password.trim()) {
      setError("Email and password are required.");
      return;
    }
    setSubmitting(true);
    setError("");

    try {
      const resp = await apiFetch<LoginResponse>(
        "/admin/api/v1/auth/login",
        {
          method: "POST",
          json: { email: email.trim(), password },
          credentials: "same-origin",
        },
      );

      setSessionFromLogin(resp);
      queryClient.invalidateQueries({ queryKey: ["dashboard"] });
      setSessionDone(true);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Could not connect to the server.");
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleMasterKeyLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!masterKey.trim()) {
      setError("Enter a valid API key or master key.");
      return;
    }
    setSubmitting(true);
    setError("");

    setApiKey(masterKey);
    try {
      await apiFetch("/admin/api/v1/dashboard/config");
      markApiKeyVerified();
      await navigate({ to: "/admin/dashboard/overview" });
    } catch (err) {
      setApiKey("");
      if (err instanceof ApiError && err.status === 401) {
        setError("The key was rejected by the server.");
      } else {
        setError("Could not verify the key. Check that aurora is running.");
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleOIDCLogin = async (providerName: string) => {
    setSubmitting(true);
    setError("");
    try {
      const redirectUri = `${window.location.origin}/admin/dashboard/oidc-callback`;
      const resp = await apiFetch<{ auth_url: string; state: string; provider: string }>(
        `/admin/api/v1/auth/oidc/${providerName}`,
        { query: { redirect_uri: redirectUri } },
      );
      window.location.href = resp.auth_url;
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Could not contact the identity provider.");
      }
      setSubmitting(false);
    }
  };

  if (probeLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <Loader2 className="h-6 w-6 animate-spin text-accent" />
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="text-center space-y-2">
          <div className="mx-auto grid h-12 w-12 place-items-center rounded-2xl bg-accent text-accent-foreground shadow-lg shadow-accent/25">
            <ShieldCheck className="h-6 w-6" />
          </div>
          <h1 className="font-serif text-[34px] font-normal leading-tight tracking-tight text-foreground">Aurora Gateway</h1>
          <p className="text-sm text-muted-foreground">
            Sign in to access the admin dashboard
          </p>
        </div>

        {identityEnabled && (
          <div className="flex  bg-surface p-0.5">
            <button
              type="button"
              onClick={() => { setTab("sso"); setError(""); }}
              className={`flex-1 rounded-lg px-3 py-2 text-sm font-medium transition-all ${tab === "sso"
                  ? "bg-background text-foreground"
                  : "text-muted-foreground hover:text-foreground"
                }`}
            >
              SSO / Email
            </button>
            <button
              type="button"
              onClick={() => { setTab("master"); setError(""); }}
              className={`flex-1 rounded-lg px-3 py-2 text-sm font-medium transition-all ${tab === "master"
                  ? "bg-background text-foreground"
                  : "text-muted-foreground hover:text-foreground"
                }`}
            >
              Master Key
            </button>
          </div>
        )}

        {tab === "sso" ? (
          <form onSubmit={handleSSOLogin} className="space-y-4">
            <div className="space-y-3">
              <div className="relative">
                <Mail className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  type="email"
                  autoComplete="email"
                  autoFocus
                  placeholder="Email"
                  aria-label="Email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="pl-9"
                />
              </div>
              <div className="relative">
                <LockKeyhole className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  type="password"
                  autoComplete="current-password"
                  placeholder="Password"
                  aria-label="Password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="pl-9"
                />
              </div>
            </div>

            {error ? (
              <p className="text-xs text-destructive" role="alert">{error}</p>
            ) : null}

            <Button type="submit" disabled={submitting} className="w-full">
              <LogIn className="h-4 w-4" />
              <span>{submitting ? "Signing in…" : "Sign in"}</span>
            </Button>

            {oidcEnabled && oidcProviders.length > 0 && (
              <div className="space-y-2">
                <div className="relative">
                  <div className="absolute inset-0 flex items-center">
                    <span className="w-full border-t border-border/40" />
                  </div>
                  <div className="relative flex justify-center text-xs uppercase">
                    <span className="bg-background px-2 text-muted-foreground">
                      Or continue with SSO
                    </span>
                  </div>
                </div>
                {oidcProviders
                  .filter((p) => p.enabled)
                  .map((p) => (
                    <Button
                      key={p.name}
                      type="button"
                      variant="outline"
                      onClick={() => handleOIDCLogin(p.name)}
                      className="w-full"
                    >
                      <ShieldCheck className="h-4 w-4" />
                      <span>{p.display_name || p.name}</span>
                    </Button>
                  ))}
              </div>
            )}
          </form>
        ) : (
          <form onSubmit={handleMasterKeyLogin} className="space-y-4">
            <div className="relative">
              <LockKeyhole className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                type="password"
                autoComplete="current-password"
                autoFocus
                placeholder="Master key or bearer token"
                aria-label="API key"
                value={masterKey}
                onChange={(e) => setMasterKey(e.target.value)}
                className="pl-9"
              />
            </div>

            {error ? (
              <p className="text-xs text-destructive" role="alert">{error}</p>
            ) : null}

            <Button type="submit" disabled={submitting} className="w-full">
              <LogIn className="h-4 w-4" />
              <span>{submitting ? "Checking…" : "Unlock dashboard"}</span>
            </Button>
          </form>
        )}
      </div>
    </div>
  );
}

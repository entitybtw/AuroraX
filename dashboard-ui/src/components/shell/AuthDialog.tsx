import * as React from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { Check, LockKeyhole, LogIn } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiError } from "@/lib/api/client";
import { markApiKeyVerified, setApiKey } from "@/lib/auth/storage";
import { useApiKey } from "@/lib/auth/useApiKey";
import { useSession } from "@/lib/auth/useSession";

/**
 * API key entry dialog. Opens when:
 *   - operator clicks "Enter API key" / "Change API key" in the sidebar, or
 *   - any apiFetch() observes a 401 (auth-stale event).
 *
 * Mirrors legacy auth-dialog markup at
 *   internal/admin/dashboard/templates/layout.html lines 88–131.
 */
export interface AuthDialogProps {
  open: boolean;
  needsAuth: boolean;
  identityEnabled: boolean;
  onOpenChange: (open: boolean) => void;
}

export function AuthDialog({
  open,
  needsAuth,
  identityEnabled,
  onOpenChange,
}: AuthDialogProps): JSX.Element {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const stored = useApiKey();
  const { loggedIn } = useSession();
  const [value, setValue] = React.useState(stored);
  const [error, setError] = React.useState("");
  const [submitting, setSubmitting] = React.useState(false);

  React.useEffect(() => {
    if (open) {
      setValue(stored);
      setError("");
    }
  }, [open, stored]);

  const submit = async (event: React.FormEvent): Promise<void> => {
    event.preventDefault();
    if (!value.trim()) {
      setError("Enter a valid API key to continue.");
      return;
    }

    setSubmitting(true);
    setError("");
    setApiKey(value);
    try {
      await apiFetch("/admin/api/v1/dashboard/config");
      markApiKeyVerified();
      await queryClient.invalidateQueries();
      onOpenChange(false);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setError("The saved key was rejected by the server.");
      } else {
        setError("Could not verify the key. Check that aurora is running.");
      }
    } finally {
      setSubmitting(false);
    }
  };

  const title = needsAuth ? "Dashboard locked" : "Change API key";
  const cta = needsAuth ? "Unlock dashboard" : "Save API key";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            Requires a master key or operator API key stored in this browser.
          </DialogDescription>
        </DialogHeader>

        {identityEnabled && !loggedIn && (
          <Button
            type="button"
            variant="default"
            className="w-full"
            onClick={() => {
              onOpenChange(false);
              navigate({ to: "/admin/dashboard/login" });
            }}
          >
            <LogIn className="h-4 w-4" />
            <span>Sign in with SSO</span>
          </Button>
        )}

        <div className="relative">
          <div className="absolute inset-0 flex items-center">
            <span className="w-full border-t border-border/40" />
          </div>
          <div className="relative flex justify-center text-xs uppercase">
            <span className="bg-background px-2 text-muted-foreground">
              Or use a key
            </span>
          </div>
        </div>

        <form onSubmit={submit} className="space-y-3">
          <div className="relative">
            <LockKeyhole
              className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
              aria-hidden
            />
            <Input
              type="password"
              autoComplete="current-password"
              autoFocus
              placeholder="Master key or bearer token"
              aria-label="API key"
              value={value}
              onChange={(event) => setValue(event.target.value)}
              className="pl-9"
            />
          </div>
          {error ? (
            <p className="text-xs text-destructive" role="alert">
              {error}
            </p>
          ) : null}
          <DialogFooter>
            <Button type="submit" disabled={submitting}>
              <Check className="h-4 w-4" aria-hidden />
              <span>{submitting ? "Checking…" : cta}</span>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

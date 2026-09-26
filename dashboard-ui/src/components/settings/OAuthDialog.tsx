import { useEffect, useState, useRef, useCallback } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Surface,
  Pill,
  CodeBlock,
} from "@/components/ui/surface";
import { Loader2, CheckCircle, AlertCircle, Copy, ExternalLink, WifiOff, Unlink2, Link, Link2 } from "lucide-react";
import { cn } from "@/lib/utils";

interface OAuthDialogProps {
  providerName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onComplete?: () => void;
}

export function OAuthDialog({
  providerName,
  open,
  onOpenChange,
  onComplete,
}: OAuthDialogProps) {
  const [step, setStep] = useState<"idle" | "waiting" | "polling" | "success" | "error">(
    "idle"
  );
  const [userCode, setUserCode] = useState("");
  const [verificationUri, setVerificationUri] = useState("");
  const [expiresIn, setExpiresIn] = useState(0);
  const [pollInterval, setPollInterval] = useState(5);
  const [error, setError] = useState("");
  const [accountInfo, setAccountInfo] = useState<{
    email: string | undefined;
    account_id: string | undefined;
    expires_at: string | undefined;
  } | null>(null);
  const [unlinking, setUnlinking] = useState(false);
  const [alreadyLinked, setAlreadyLinked] = useState(false);
  const [checkingStatus, setCheckingStatus] = useState(true);
  const [verificationBase, setVerificationBase] = useState("");
  const [grant, setGrant] = useState<"device" | "authorization_code">("device");
  const [authorizeUrl, setAuthorizeUrl] = useState("");
  const [pkceState, setPkceState] = useState("");
  const [manualCode, setManualCode] = useState("");
  const [completing, setCompleting] = useState(false);
  const pollTimerRef = useRef<NodeJS.Timeout | null>(null);
  const startTimeRef = useRef<number>(0);
  const stepRef = useRef<"idle" | "waiting" | "polling" | "success" | "error">("idle");

  // Resolve verification URI: absolute URLs pass through; relative paths are
  // expanded with the extension-supplied base (never a hardcoded origin).
  const fullVerificationUri = useCallback(() => {
    if (!verificationUri) return "";
    if (verificationUri.startsWith("http://") || verificationUri.startsWith("https://")) {
      return verificationUri;
    }
    if (!verificationBase) return verificationUri;
    const path = verificationUri.startsWith("/") ? verificationUri : `/${verificationUri}`;
    return `${verificationBase.replace(/\/$/, "")}${path}`;
  }, [verificationUri, verificationBase]);

  // On dialog open: check status first, then decide whether to start a new flow.
  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setCheckingStatus(true);

    const checkStatus = async () => {
      try {
        const { fetchOAuthStatus, fetchOAuthFlow } = await import("@/lib/api/oauth");
        const status = await fetchOAuthStatus(providerName);
        if (cancelled) return;
        try {
          const flow = await fetchOAuthFlow(providerName);
          if (!cancelled) {
            setGrant(flow.authorization_code ? "authorization_code" : "device");
          }
        } catch {
          /* older backend without /flow — keep device */
        }
        if (status.has_token && !status.expired) {
          setAlreadyLinked(true);
          setAccountInfo({
            email: status.email ?? undefined,
            account_id: status.account_id ?? undefined,
            expires_at: status.expires_at ?? undefined,
          });
          setStep("success");
        } else {
          setAlreadyLinked(false);
          setAccountInfo(null);
        }
      } catch {
        if (!cancelled) {
          setAlreadyLinked(false);
          setAccountInfo(null);
        }
      } finally {
        if (!cancelled) setCheckingStatus(false);
      }
    };
    checkStatus();
    return () => {
      cancelled = true;
    };
  }, [open, providerName]);

  // Copy to clipboard helper
  const copyToClipboard = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text);
      // Could add toast here
    } catch (e) {
      console.error(`Failed to copy ${label}:`, e);
    }
  };

  const handleUnlink = async () => {
    setUnlinking(true);
    try {
      const { clearOAuthToken } = await import("@/lib/api/oauth");
      await clearOAuthToken(providerName);
      setStep("idle");
      setAccountInfo(null);
      onComplete?.();
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : "Failed to unlink account";
      setError(message);
      setStep("error");
    } finally {
      setUnlinking(false);
    }
  };

  // Start the device flow or authorization-code + PKCE authorize step.
  const handleStart = async () => {
    setStep("waiting");
    setError("");
    try {
      // Dynamic import to avoid SSR issues
      const oauth = await import("@/lib/api/oauth");

      if (grant === "authorization_code") {
        const res = await oauth.startOAuthAuthorize(providerName);
        setAuthorizeUrl(res.authorize_url);
        setPkceState(res.state);
        setExpiresIn(res.expires_in);
        setManualCode("");
        startTimeRef.current = Date.now();
        stepRef.current = "polling";
        setStep("polling");
        startPolling();
        return;
      }

      const res = await oauth.startOAuthDeviceFlow(providerName);
      // Backend doesn't return device_code, but starts background polling
      setUserCode(res.user_code);
      setVerificationUri(res.verification_uri_complete);
      setVerificationBase(res.verification_base ?? "");
      setExpiresIn(res.expires_in);
      setPollInterval(res.interval);
      startTimeRef.current = Date.now();
      stepRef.current = "polling";
      setStep("polling");
      startPolling();
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : "Failed to start OAuth flow";
      setError(message);
      setStep("error");
    }
  };

  // Exchange a pasted CODE#STATE or full callback URL (manual redirect).
  const handleCompleteAuthorize = async () => {
    if (!manualCode.trim()) return;
    setCompleting(true);
    setError("");
    try {
      const { completeOAuthAuthorize } = await import("@/lib/api/oauth");
      const raw = manualCode.trim();
      const body = raw.includes("://") || raw.includes("code=")
        ? { url: raw }
        : raw.includes("#")
          ? { code_and_state: raw }
          : { code: raw, state: pkceState };
      await completeOAuthAuthorize(providerName, body);
      setManualCode("");
      // Poll status to pick up email/account.
      const { fetchOAuthStatus } = await import("@/lib/api/oauth");
      const status = await fetchOAuthStatus(providerName);
      stepRef.current = "success";
      setStep("success");
      setAccountInfo({
        email: status.email ?? undefined,
        account_id: status.account_id ?? undefined,
        expires_at: status.expires_at ?? undefined,
      });
      onComplete?.();
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : "Failed to complete authorization";
      setError(message);
      setStep("error");
    } finally {
      setCompleting(false);
    }
  };

  // Poll for token using status endpoint (backend handles device_code internally)
  const startPolling = useCallback(async () => {
    // Track the step via a local flag that isn't captured from a stale closure.
    let active = true;
    stepRef.current = "polling";

    const poll = async () => {
      if (!active || stepRef.current !== "polling") {
        return;
      }

      try {
        const { fetchOAuthStatus } = await import("@/lib/api/oauth");
        const status = await fetchOAuthStatus(providerName);

        if (status.has_token && !status.expired) {
          active = false;
          stepRef.current = "success";
          setStep("success");
          setAccountInfo({
            email: status.email ?? undefined,
            account_id: status.account_id ?? undefined,
            expires_at: status.expires_at ?? undefined,
          });
          onComplete?.();
          return;
        }

        if (status.expired) {
          active = false;
          stepRef.current = "error";
          setError("Token expired. Please start a new authorization.");
          setStep("error");
          return;
        }

        // Check expiry
        const elapsed = (Date.now() - startTimeRef.current) / 1000;
        if (elapsed >= expiresIn) {
          active = false;
          stepRef.current = "error";
          setError("Authorization timed out. Please try again.");
          setStep("error");
          return;
        }

        // Continue polling
        pollTimerRef.current = setTimeout(poll, pollInterval * 1000);
      } catch (e: unknown) {
        const message = e instanceof Error ? e.message : "Polling error";
        // Don't error out on transient errors, just retry
        console.error("OAuth poll error:", message);
        if (!active || stepRef.current !== "polling") {
          return;
        }
        pollTimerRef.current = setTimeout(poll, pollInterval * 1000);
      }
    };

    await poll();
  }, [providerName, pollInterval, expiresIn, onComplete]);

  // Cleanup on unmount or step change
  useEffect(() => {
    return () => {
      if (pollTimerRef.current) clearTimeout(pollTimerRef.current);
    };
  }, []);

  // Reset when dialog opens; auto-start a device flow only after the status
  // check completes and the account is NOT already linked.
  useEffect(() => {
    if (!open) {
      // Cleanup on close
      if (pollTimerRef.current) clearTimeout(pollTimerRef.current);
      stepRef.current = "idle";
      setStep("idle");
      setUserCode("");
      setVerificationUri("");
      setError("");
      setAccountInfo(null);
      setAlreadyLinked(false);
      setCheckingStatus(true);
      setAuthorizeUrl("");
      setPkceState("");
      setManualCode("");
      setGrant("device");
      return;
    }
    if (!checkingStatus && !alreadyLinked && step === "idle") {
      handleStart();
    }
  }, [open, checkingStatus, alreadyLinked]);

  // Keep stepRef in sync with the step state.
  useEffect(() => {
    stepRef.current = step;
  }, [step]);

  // Time remaining calculation
  const timeRemaining = step === "polling"
    ? Math.max(0, expiresIn - Math.floor((Date.now() - startTimeRef.current) / 1000))
    : expiresIn;

  // Live countdown that updates every second for smooth display
  const [displayTime, setDisplayTime] = useState(expiresIn);
  const intervalRef = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    if (step === "polling") {
      startTimeRef.current = Date.now();
      setDisplayTime(expiresIn);
      intervalRef.current = setInterval(() => {
        setDisplayTime((prev) => Math.max(0, prev - 1));
      }, 1000);
    } else {
      if (intervalRef.current) clearInterval(intervalRef.current);
      setDisplayTime(expiresIn);
    }
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [step, expiresIn]);

  const formatTime = (seconds: number) => {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, "0")}`;
  };

  if (!open) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn(
          "max-w-md sm:max-w-lg",
          step === "success" && "max-w-md"
        )}
      >
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {checkingStatus && (
              <Loader2 className="h-5 w-5 animate-spin text-primary" />
            )}
            {!checkingStatus && step === "polling" && (
              <Loader2 className="h-5 w-5 animate-spin text-primary" />
            )}
            {!checkingStatus && step === "success" && alreadyLinked && (
              <Link className="h-5 w-5 text-success" />
            )}
            {!checkingStatus && step === "success" && !alreadyLinked && (
              <CheckCircle className="h-5 w-5 text-success" />
            )}
            {!checkingStatus && step === "error" && (
              <AlertCircle className="h-5 w-5 text-destructive" />
            )}
            {!checkingStatus && step === "waiting" && (
              <WifiOff className="h-5 w-5 text-muted-foreground" />
            )}
            {checkingStatus
              ? "Checking status..."
              : step === "success"
                ? alreadyLinked
                  ? "Account Linked"
                  : "Authorized!"
                : "Link OAuth Account"}
          </DialogTitle>
          <DialogDescription>
            {checkingStatus && "Checking OAuth status for this provider..."}
            {!checkingStatus && step === "success" && alreadyLinked && (
              "This provider is already linked to an OAuth account."
            )}
            {!checkingStatus && step === "waiting" && "Starting device authorization flow..."}
            {!checkingStatus && step === "polling" && grant === "authorization_code" && (
              <>
                Open the authorize link, then paste the redirect result below.
                <br />
                <span className="font-mono text-lg text-primary">{formatTime(displayTime)}</span>
                {" "}
                <span className="text-xs text-muted-foreground">remaining</span>
              </>
            )}
            {!checkingStatus && step === "polling" && grant !== "authorization_code" && (
              <>
                Authorize in browser, then wait for confirmation.
                <br />
                <span className="font-mono text-lg text-primary">{formatTime(displayTime)}</span>
                {" "}
                <span className="text-xs text-muted-foreground">remaining</span>
              </>
            )}
            {!checkingStatus && step === "success" && !alreadyLinked && "Your OAuth account is now linked."}
            {!checkingStatus && step === "error" && error}
          </DialogDescription>
        </DialogHeader>

        {step === "polling" && (
          <Surface variant="subtle" className="p-4">
            <div className="grid gap-3 text-sm">
              <div>
                <label className="text-xs text-muted-foreground block mb-1">User Code</label>
                <div className="flex items-center gap-2 mt-1">
                  <CodeBlock className="flex-1 text-lg font-mono tracking-widest">
                    {userCode}
                  </CodeBlock>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => copyToClipboard(userCode, "user code")}
                    aria-label="Copy user code"
                  >
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              </div>

              <div>
                <label className="text-xs text-muted-foreground block mb-1">Verification URL</label>
                <div className="flex items-center gap-2 mt-1">
                  <Input
                    readOnly
                    value={fullVerificationUri()}
                    className="text-xs truncate"
                  />
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => copyToClipboard(fullVerificationUri(), "verification URL")}
                    aria-label="Copy verification URL"
                  >
                    <Copy className="h-4 w-4" />
                  </Button>
                  <a
                    href={fullVerificationUri()}
                    target="_blank"
                    rel="noopener noreferrer"
                    aria-label="Open in browser"
                  >
                    <Button variant="ghost" size="icon">
                      <ExternalLink className="h-4 w-4" />
                    </Button>
                  </a>
                </div>
              </div>
            </div>

            <div className="mt-4 h-2 bg-muted rounded-full overflow-hidden">
              <div
                className="h-full bg-primary transition-all duration-300"
                style={{
                  width: `${Math.max(0, (1 - timeRemaining / expiresIn) * 100)}%`,
                }}
              />
            </div>
            <p className="text-xs text-muted-foreground text-center mt-1">
              Auto-polling every {pollInterval}s • Expires in {formatTime(timeRemaining)}
            </p>
          </Surface>
        )}

        {step === "success" && (
          <Surface variant="subtle" className="p-4">
            <div className="grid gap-2 text-sm">
              {accountInfo?.email && (
                <div className="flex items-center gap-2">
                  <span className="text-muted-foreground">Email:</span>
                  <span className="font-medium">{accountInfo.email}</span>
                </div>
              )}
              {accountInfo?.account_id && (
                <div className="flex items-center gap-2">
                  <span className="text-muted-foreground">Account ID:</span>
                  <CodeBlock className="text-xs">{accountInfo.account_id}</CodeBlock>
                </div>
              )}
              {accountInfo?.expires_at && (
                <div className="flex items-center gap-2">
                  <span className="text-muted-foreground">Token expires:</span>
                  <span>
                    {new Date(accountInfo.expires_at).toLocaleString()}
                  </span>
                </div>
              )}
              <Pill tone="success">Standard plan enabled</Pill>
            </div>
            <div className="flex items-center gap-2 mt-3 pt-3 border-t">
              <Button
                variant="outline"
                size="sm"
                onClick={handleStart}
                className="flex-1"
              >
                <Link2 className="mr-1.5 h-3.5 w-3.5" />
                Relink Account
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleUnlink}
                disabled={unlinking}
                className="flex-1 text-destructive hover:bg-destructive/10"
              >
                {unlinking ? (
                  <>
                    <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                    Unlinking...
                  </>
                ) : (
                  <>
                    <Unlink2 className="mr-1.5 h-3.5 w-3.5" />
                    Unlink
                  </>
                )}
              </Button>
            </div>
            <div className="flex justify-end mt-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => onOpenChange(false)}
              >
                Done
              </Button>
            </div>
          </Surface>
        )}

        {step === "error" && (
          <Surface variant="elevated" className="p-4 border-destructive/30">
            <p className="text-destructive">{error}</p>
            <p className="text-xs text-muted-foreground mt-1">
              {grant === "authorization_code"
                ? "The authorize session may have expired. Start a new authorization."
                : "The device code may have expired. Try starting a new authorization."}
            </p>
          </Surface>
        )}

        <DialogFooter className="gap-2">
        {step === "polling" && grant === "authorization_code" && (
          <Surface variant="subtle" className="p-4">
            <div className="grid gap-3 text-sm">
              <div>
                <label className="text-xs text-muted-foreground block mb-1">Authorize URL</label>
                <div className="flex items-center gap-2 mt-1">
                  <Input
                    readOnly
                    value={authorizeUrl}
                    className="text-xs truncate"
                  />
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => copyToClipboard(authorizeUrl, "authorize URL")}
                    aria-label="Copy authorize URL"
                  >
                    <Copy className="h-4 w-4" />
                  </Button>
                  <a
                    href={authorizeUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    aria-label="Open authorize URL in browser"
                  >
                    <Button variant="ghost" size="icon">
                      <ExternalLink className="h-4 w-4" />
                    </Button>
                  </a>
                </div>
              </div>

              <div>
                <label className="text-xs text-muted-foreground block mb-1">
                  Redirect result (CODE#STATE or full callback URL)
                </label>
                <div className="flex items-center gap-2 mt-1">
                  <Input
                    value={manualCode}
                    onChange={(e) => setManualCode(e.target.value)}
                    placeholder="Paste code#state or callback URL"
                    className="text-xs"
                    autoComplete="off"
                  />
                  <Button
                    variant="default"
                    size="sm"
                    onClick={handleCompleteAuthorize}
                    disabled={completing || !manualCode.trim()}
                  >
                    {completing ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      "Complete"
                    )}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground mt-1.5">
                  After authorize, the browser may land on a localhost callback that
                  does not open — copy the full URL (or CODE#STATE) and paste it here.
                </p>
              </div>
            </div>

            <div className="mt-4 h-2 bg-muted rounded-full overflow-hidden">
              <div
                className="h-full bg-primary transition-all duration-300"
                style={{
                  width: `${Math.max(0, (1 - timeRemaining / expiresIn) * 100)}%`,
                }}
              />
            </div>
            <p className="text-xs text-muted-foreground text-center mt-1">
              Session expires in {formatTime(timeRemaining)}
            </p>
          </Surface>
        )}

        {step === "polling" && grant !== "authorization_code" && (
            <Button variant="secondary" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
          )}
          {step === "error" && (
            <Button variant="default" onClick={handleStart}>
              Try Again
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
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
  const [polling, setPolling] = useState(false);
  const [accountInfo, setAccountInfo] = useState<{
    email: string | undefined;
    account_id: string | undefined;
    expires_at: string | undefined;
  } | null>(null);
  const [unlinking, setUnlinking] = useState(false);
  const [alreadyLinked, setAlreadyLinked] = useState(false);
  const pollTimerRef = useRef<NodeJS.Timeout | null>(null);
  const startTimeRef = useRef<number>(0);

  // Format verification URI for display (add https://opencode.ai if relative)
  const fullVerificationUri = useCallback(() => {
    if (verificationUri.startsWith("http")) return verificationUri;
    return `https://opencode.ai${verificationUri}`;
  }, [verificationUri]);

  // Check if account is already linked when dialog opens
  useEffect(() => {
    if (!open) return;
    const checkStatus = async () => {
      try {
        const { fetchOAuthStatus } = await import("@/lib/api/oauth");
        const status = await fetchOAuthStatus(providerName);
        if (status.has_token && !status.expired) {
          setAlreadyLinked(true);
          setAccountInfo({
            email: status.email ?? undefined,
            account_id: status.account_id ?? undefined,
            expires_at: status.expires_at ?? undefined,
          });
        } else {
          setAlreadyLinked(false);
          setAccountInfo(null);
        }
      } catch {
        setAlreadyLinked(false);
      }
    };
    checkStatus();
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

  // Start the device flow
  const handleStart = async () => {
    setStep("waiting");
    setError("");
    try {
      // Dynamic import to avoid SSR issues
      const { startOAuthDeviceFlow } = await import("@/lib/api/oauth");
      const res = await startOAuthDeviceFlow(providerName);
      // Backend doesn't return device_code, but starts background polling
      setUserCode(res.user_code);
      setVerificationUri(res.verification_uri_complete);
      setExpiresIn(res.expires_in);
      setPollInterval(res.interval);
      startTimeRef.current = Date.now();
      setStep("polling");
      startPolling();
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : "Failed to start OAuth flow";
      setError(message);
      setStep("error");
    }
  };

  // Poll for token using status endpoint (backend handles device_code internally)
  const startPolling = useCallback(async () => {
    if (polling) return;
    setPolling(true);

    const poll = async () => {
      if (step !== "polling") {
        setPolling(false);
        return;
      }

      try {
        const { fetchOAuthStatus } = await import("@/lib/api/oauth");
        const status = await fetchOAuthStatus(providerName);

        if (status.has_token && !status.expired) {
          setStep("success");
          setAccountInfo({
            email: status.email ?? undefined,
            account_id: status.account_id ?? undefined,
            expires_at: status.expires_at ?? undefined,
          });
          setPolling(false);
          onComplete?.();
          return;
        }

        if (status.expired) {
          setError("Token expired. Please start a new authorization.");
          setStep("error");
          setPolling(false);
          return;
        }

        // Check expiry
        const elapsed = (Date.now() - startTimeRef.current) / 1000;
        if (elapsed >= expiresIn) {
          setError("Authorization timed out. Please try again.");
          setStep("error");
          setPolling(false);
          return;
        }

        // Continue polling
        pollTimerRef.current = setTimeout(poll, pollInterval * 1000);
      } catch (e: unknown) {
        const message = e instanceof Error ? e.message : "Polling error";
        // Don't error out on transient errors, just retry
        console.error("OAuth poll error:", message);
        pollTimerRef.current = setTimeout(poll, pollInterval * 1000);
      }
    };

    await poll();
  }, [step, polling, providerName, pollInterval, expiresIn]);

  // Cleanup on unmount or step change
  useEffect(() => {
    return () => {
      if (pollTimerRef.current) clearTimeout(pollTimerRef.current);
    };
  }, []);

  // Reset when dialog opens
  useEffect(() => {
    if (open && step === "idle") {
      handleStart();
    } else if (!open) {
      // Cleanup on close
      if (pollTimerRef.current) clearTimeout(pollTimerRef.current);
      setStep("idle");
      setUserCode("");
      setVerificationUri("");
      setError("");
      setPolling(false);
      setAccountInfo(null);
    }
  }, [open]);

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
            {step === "polling" && (
              <Loader2 className="h-5 w-5 animate-spin text-primary" />
            )}
            {step === "success" && (
              <CheckCircle className="h-5 w-5 text-success" />
            )}
            {step === "error" && (
              <AlertCircle className="h-5 w-5 text-destructive" />
            )}
            {step === "waiting" && (
              <WifiOff className="h-5 w-5 text-muted-foreground" />
            )}
            {step === "success" ? "Authorized!" : alreadyLinked ? "Account Linked" : "Link OpenCode Account"}
          </DialogTitle>
          <DialogDescription>
            {alreadyLinked && step === "idle" && (
              <>
                <Link className="mr-1.5 h-4 w-4 text-primary" />
                Account already linked to OpenCode.
              </>
            )}
            {step === "waiting" && "Starting device authorization flow..."}
            {step === "polling" && (
              <>
                Authorize in browser, then wait for confirmation.
                <br />
                <span className="font-mono text-lg text-primary">{formatTime(displayTime)}</span>
                {" "}
                <span className="text-xs text-muted-foreground">remaining</span>
              </>
            )}
            {step === "success" && "Your OpenCode account is now linked."}
            {step === "error" && error}
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

        {step === "success" && accountInfo && (
          <Surface variant="subtle" className="p-4">
            <div className="grid gap-2 text-sm">
              {accountInfo.email && (
                <div className="flex items-center gap-2">
                  <span className="text-muted-foreground">Email:</span>
                  <span className="font-medium">{accountInfo.email}</span>
                </div>
              )}
              {accountInfo.account_id && (
                <div className="flex items-center gap-2">
                  <span className="text-muted-foreground">Account ID:</span>
                  <CodeBlock className="text-xs">{accountInfo.account_id}</CodeBlock>
                </div>
              )}
              {accountInfo.expires_at && (
                <div className="flex items-center gap-2">
                  <span className="text-muted-foreground">Token expires:</span>
                  <span>
                    {new Date(accountInfo.expires_at).toLocaleString()}
                  </span>
                </div>
              )}
              <Pill tone="success">Free tier enabled</Pill>
            </div>
            <div className="flex items-center gap-2 mt-3 pt-3 border-t">
              <Button
                variant="outline"
                size="sm"
                onClick={handleUnlink}
                disabled={unlinking}
                className="flex-1"
              >
                {unlinking ? (
                  <>
                    <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                    Unlinking...
                  </>
                ) : (
                  <>
                    <Unlink2 className="mr-1.5 h-3.5 w-3.5" />
                    Unlink Account
                  </>
                )}
              </Button>
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
              The device code may have expired. Try starting a new authorization.
            </p>
          </Surface>
        )}

        <DialogFooter className="gap-2">
          {alreadyLinked && step === "idle" && (
            <Button variant="outline" onClick={handleStart} className="flex-1">
              <Link2 className="mr-1.5 h-3.5 w-3.5" />
              Relink Account
            </Button>
          )}
          {step === "polling" && (
            <Button variant="secondary" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
          )}
          {step === "error" && (
            <Button variant="default" onClick={handleStart}>
              Try Again
            </Button>
          )}
          {step === "success" && (
            <Button variant="default" onClick={() => onOpenChange(false)}>
              Done
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
"use client";

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
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import {
  ToggleField,
  Surface,
  Pill,
  codeBlock,
} from "@/components/ui/surface";
import { Loader2, CheckCircle, AlertCircle, Copy, ExternalLink, WifiOff } from "lucide-react";
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
  const [deviceCode, setDeviceCode] = useState("");
  const [userCode, setUserCode] = useState("");
  const [verificationUri, setVerificationUri] = useState("");
  const [expiresIn, setExpiresIn] = useState(0);
  const [interval, setInterval] = useState(5);
  const [error, setError] = useState("");
  const [polling, setPolling] = useState(false);
  const [accountInfo, setAccountInfo] = useState<{
    email?: string;
    account_id?: string;
    expires_at?: string;
  } | null>(null);
  const pollTimerRef = useRef<NodeJS.Timeout | null>(null);
  const startTimeRef = useRef<number>(0);

  // Format verification URI for display (add https://example.com if relative)
  const fullVerificationUri = useCallback(() => {
    if (verificationUri.startsWith("http")) return verificationUri;
    return `https://example.com${verificationUri}`;
  }, [verificationUri]);

  // Copy to clipboard helper
  const copyToClipboard = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text);
      // Could add toast here
    } catch (e) {
      console.error(`Failed to copy ${label}:`, e);
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
      setDeviceCode(res.device_code);
      setUserCode(res.user_code);
      setVerificationUri(res.verification_uri_complete);
      setExpiresIn(res.expires_in);
      setInterval(res.interval);
      startTimeRef.current = Date.now();
      setStep("polling");
      startPolling();
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : "Failed to start OAuth flow";
      setError(message);
      setStep("error");
    }
  };

  // Poll for token
  const startPolling = useCallback(async () => {
    if (polling) return;
    setPolling(true);

    const poll = async () => {
      if (!deviceCode || step !== "polling") {
        setPolling(false);
        return;
      }

      try {
        const { pollOAuthToken } = await import("@/lib/api/oauth");
        const res = await pollOAuthToken(providerName, deviceCode, interval, expiresIn);

        if (res.status === "authorized") {
          setStep("success");
          if (res.access_token) {
            // Fetch account info
            const { fetchOAuthStatus } = await import("@/lib/api/oauth");
            const status = await fetchOAuthStatus(providerName);
            setAccountInfo({
              email: status.email,
              account_id: status.account_id,
              expires_at: status.expires_at,
            });
          }
          setPolling(false);
          onComplete?.();
          return;
        }

        if (res.status === "error") {
          setError(res.error || "Authorization failed");
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
        pollTimerRef.current = setTimeout(poll, interval * 1000);
      } catch (e: unknown) {
        const message = e instanceof Error ? e.message : "Polling error";
        // Don't error out on transient errors, just retry
        console.error("OAuth poll error:", message);
        pollTimerRef.current = setTimeout(poll, interval * 1000);
      }
    };

    await poll();
  }, [deviceCode, step, polling, providerName, interval, expiresIn]);

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
      setDeviceCode("");
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
            {step === "success" ? "Authorized!" : "Link upstream Account"}
          </DialogTitle>
          <DialogDescription>
            {step === "waiting" && "Starting device authorization flow..."}
            {step === "polling" && (
              <>
                Authorize in browser, then wait for confirmation.
                <br />
                Time remaining: <strong>{formatTime(timeRemaining)}</strong>
              </>
            )}
            {step === "success" && "Your upstream account is now linked."}
            {step === "error" && error}
          </DialogDescription>
        </DialogHeader>

        {step === "polling" && (
          <Surface variant="subtle" className="p-4">
            <div className="grid gap-3 text-sm">
              <div>
                <Label className="text-xs text-muted-foreground">User Code</Label>
                <div className="flex items-center gap-2 mt-1">
                  <codeBlock className="flex-1 text-lg font-mono tracking-widest">
                    {userCode}
                  </codeBlock>
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
                <Label className="text-xs text-muted-foreground">Verification URL</Label>
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
              Auto-polling every {interval}s • Expires in {formatTime(timeRemaining)}
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
                  <codeBlock className="text-xs">{accountInfo.account_id}</codeBlock>
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
          </Surface>
        )}

        {step === "error" && (
          <Surface variant="destructive" className="p-4 border-destructive/30">
            <p className="text-destructive">{error}</p>
            <p className="text-xs text-muted-foreground mt-1">
              The device code may have expired. Try starting a new authorization.
            </p>
          </Surface>
        )}

        <DialogFooter className="gap-2">
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
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Surface } from "@/components/ui/surface";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ToggleField } from "@/components/ui/toggle-field";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  CpuIcon,
  GlobeIcon,
  RefreshCwIcon,
  PlusIcon,
  Trash2Icon,
  CheckCircleIcon,
  XCircleIcon,
  ZapIcon,
  ServerIcon,
  AlertTriangleIcon,
  ShieldAlertIcon,
  ArrowRightIcon,
} from "lucide-react";
import { apiFetch } from "@/lib/api/client";

// --- Types ---

interface SidecarProxy {
  ip: string;
  port: number;
  enabled: boolean;
}

interface SidecarSettings {
  enabled: boolean;
  port: number;
  inject_tools: boolean;
  default_auth: string;
  user_agent: string;
  base_url: string;
  max_attempts: number;
  retry_delay_ms: number;
  bind_ips: string[];
  proxies: SidecarProxy[];
}

interface EnsureRulesResult {
  provider: string;
  added: string[];
  already: string[];
  headers: { name: string; mode: string; prefix?: string; length?: number; value?: string }[];
}

// --- API hooks ---

function useSidecarStatus() {
  return useQuery({
    queryKey: ["sidecar"],
    queryFn: () =>
      apiFetch<{ running: boolean; settings: SidecarSettings; proxies: SidecarProxy[] }>(
        "/admin/api/v1/sidecar",
      ),
    refetchInterval: 5000,
  });
}

function useSidecarMutation() {
  const qc = useQueryClient();
  const invalidate = () => qc.invalidateQueries({ queryKey: ["sidecar"] });

  const updateSettings = useMutation({
    mutationFn: (data: SidecarSettings) =>
      apiFetch("/admin/api/v1/sidecar", { method: "PUT", json: data }),
    onSuccess: invalidate,
  });

  const ensureRules = useMutation({
    mutationFn: (provider: string) =>
      apiFetch<{ status: string; data: EnsureRulesResult }>(
        `/admin/api/v1/sessionhub/providers/${encodeURIComponent(provider)}/ensure-opencode`,
        { method: "POST" },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessionhub"] });
      qc.invalidateQueries({ queryKey: ["sidecar"] });
    },
  });

  return { updateSettings, ensureRules };
}

// --- Subcomponents ---

function StatusBadge({ running }: { running: boolean }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ${
        running ? "bg-emerald-500/10 text-emerald-500" : "bg-muted text-muted-foreground"
      }`}
    >
      {running ? <CheckCircleIcon className="h-3 w-3" /> : <XCircleIcon className="h-3 w-3" />}
      {running ? "Running" : "Stopped"}
    </span>
  );
}

function RiskNotice() {
  return (
    <div className="flex items-start gap-3 rounded-lg border border-amber-500/30 bg-amber-500/5 p-4">
      <ShieldAlertIcon className="h-5 w-5 shrink-0 text-amber-500 mt-0.5" />
      <div className="text-sm">
        <p className="font-medium text-amber-600 dark:text-amber-400">Use at your own risk</p>
        <p className="text-muted-foreground mt-1 leading-snug">
          The sidecar emulates the upstream client fingerprint to reach the free tier.
          Upstream hardening can break it at any time, and using it may violate the
          provider&apos;s terms of service. Review the documentation before enabling.
        </p>
      </div>
    </div>
  );
}

function BindIPEditor({
  bindIPs,
  onChange,
}: {
  bindIPs: string[];
  onChange: (ips: string[]) => void;
}) {
  const [newIP, setNewIP] = useState("");

  const addIP = () => {
    const ip = newIP.trim();
    if (ip && !bindIPs.includes(ip)) {
      onChange([...bindIPs, ip]);
      setNewIP("");
    }
  };

  const removeIP = (ip: string) => {
    onChange(bindIPs.filter((i) => i !== ip));
  };

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap gap-1.5">
        {bindIPs.map((ip) => (
          <span
            key={ip}
            className="inline-flex items-center gap-1 rounded-md bg-primary/10 px-2 py-1 text-xs font-mono text-primary"
          >
            {ip}
            <button
              type="button"
              onClick={() => removeIP(ip)}
              className="ml-0.5 rounded-full p-0.5 hover:bg-primary/20 transition-colors"
              aria-label={`Remove ${ip}`}
            >
              <Trash2Icon className="h-3 w-3" />
            </button>
          </span>
        ))}
        {bindIPs.length === 0 && (
          <span className="text-xs text-muted-foreground">No bind IPs configured</span>
        )}
      </div>
      <div className="flex gap-2">
        <Input
          value={newIP}
          onChange={(e) => setNewIP(e.target.value)}
          placeholder="Add IP address..."
          className="flex-1 h-11 sm:h-8 text-sm sm:text-xs font-mono"
          onKeyDown={(e) => e.key === "Enter" && addIP()}
        />
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={addIP}
          disabled={!newIP.trim()}
          className="h-11 sm:h-8 px-3"
        >
          <PlusIcon className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  );
}

// --- Session hub rule opt-in dialog ---

function SessionHubRulesDialog({
  open,
  onOpenChange,
  onConfirm,
  pending,
  result,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  pending: boolean;
  result: EnsureRulesResult | null;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg mx-4 sm:mx-auto max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <ShieldAlertIcon className="h-5 w-5 text-amber-500" />
            Add upstream header rules?
          </DialogTitle>
          <DialogDescription>
            The sidecar replicates the upstream client fingerprint. The Session Hub must send
            matching <code className="rounded bg-muted px-1 py-0.5 text-xs">x-opencode-*</code>{" "}
            headers for the free tier to accept requests.
          </DialogDescription>
        </DialogHeader>

        {!result ? (
          <div className="flex flex-col gap-3 text-sm">
            <p className="text-muted-foreground">
              This will add the following rules to each upstream provider. Existing rules are
              never overwritten — only missing ones are added.
            </p>
            <div className="rounded-lg border border-border/60 divide-y divide-border/40">
              {[
                ["x-opencode-session", "map or generate", "ses_ · 28 chars"],
                ["x-opencode-client", "static", "cli"],
                ["x-opencode-request", "generate", "msg_ · 28 chars"],
                ["x-opencode-project", "static", "global"],
              ].map(([name, mode, detail]) => (
                <div key={name} className="flex items-center justify-between gap-3 px-3 py-2">
                  <code className="text-xs font-medium">{name}</code>
                  <div className="text-right text-xs text-muted-foreground">
                    <div>{mode}</div>
                    <div className="font-mono">{detail}</div>
                  </div>
                </div>
              ))}
            </div>
            <p className="text-xs text-muted-foreground">
              You can customise these later in the Session Hub tab. The sidecar will warn you if
              values are changed from the defaults.
            </p>
          </div>
        ) : (
          <div className="flex flex-col gap-3 text-sm">
            <p className="font-medium text-emerald-500">Rules applied.</p>
            {result.added.length > 0 && (
              <div>
                <p className="text-xs font-medium text-muted-foreground mb-1">Added</p>
                <div className="flex flex-wrap gap-1.5">
                  {result.added.map((h) => (
                    <span
                      key={h}
                      className="rounded bg-emerald-500/10 px-2 py-0.5 text-xs font-mono text-emerald-500"
                    >
                      {h}
                    </span>
                  ))}
                </div>
              </div>
            )}
            {result.already.length > 0 && (
              <div>
                <p className="text-xs font-medium text-muted-foreground mb-1">Already present (kept)</p>
                <div className="flex flex-wrap gap-1.5">
                  {result.already.map((h) => (
                    <span
                      key={h}
                      className="rounded bg-muted px-2 py-0.5 text-xs font-mono text-muted-foreground"
                    >
                      {h}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        <DialogFooter className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          {!result ? (
            <>
              <Button variant="outline" onClick={() => onOpenChange(false)} className="w-full sm:w-auto">
                Not now
              </Button>
              <Button onClick={onConfirm} disabled={pending} className="w-full sm:w-auto gap-1.5">
                {pending ? (
                  <RefreshCwIcon className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <ArrowRightIcon className="h-3.5 w-3.5" />
                )}
                Add rules
              </Button>
            </>
          ) : (
            <Button onClick={() => onOpenChange(false)} className="w-full sm:w-auto">
              Done
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// --- Main component ---

export function SidecarTab(): JSX.Element {
  const { data: status, isLoading } = useSidecarStatus();
  const { updateSettings, ensureRules } = useSidecarMutation();
  const [form, setForm] = useState<SidecarSettings | null>(null);
  const [rulesDialogOpen, setRulesDialogOpen] = useState(false);
  const [rulesResult, setRulesResult] = useState<EnsureRulesResult | null>(null);
  const [rulesProvider, setRulesProvider] = useState("opencode-zen");

  const settings = form ?? status?.settings;
  const proxies = status?.proxies ?? [];

  const handleSave = () => {
    if (settings) {
      updateSettings.mutate(settings);
    }
  };

  const update = <K extends keyof SidecarSettings>(key: K, value: SidecarSettings[K]) => {
    if (!settings) return;
    setForm({ ...settings, [key]: value });
  };

  const openRulesDialog = () => {
    setRulesResult(null);
    setRulesDialogOpen(true);
  };

  const confirmEnsureRules = () => {
    ensureRules.mutate(rulesProvider, {
      onSuccess: (res) => setRulesResult(res.data),
    });
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12 text-muted-foreground text-sm">
        Loading sidecar configuration...
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="flex items-center justify-center py-12 text-muted-foreground text-sm">
        Failed to load sidecar settings.
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4 sm:gap-6">
      <RiskNotice />

      {/* Overview */}
      <Surface id="sidecar-overview" className="p-4 sm:p-6 scroll-mt-20">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="flex items-start gap-3">
            <div className="rounded-lg bg-primary/10 p-2">
              <CpuIcon className="h-5 w-5 text-primary" />
            </div>
            <div>
              <h3 className="text-base font-semibold">Sidecar</h3>
              <p className="text-sm text-muted-foreground mt-1">
                Bun-based TLS proxy for free-tier-tier access. Works for both free tier and
                opencode-go providers. Requires the Bun runtime.
              </p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <StatusBadge running={status?.running ?? false} />
            <Button
              variant="outline"
              onClick={handleSave}
              disabled={!form || updateSettings.isPending}
              className="gap-1.5 h-11 sm:h-9"
            >
              <RefreshCwIcon className={`h-3.5 w-3.5 ${updateSettings.isPending ? "animate-spin" : ""}`} />
              Save
            </Button>
          </div>
        </div>
      </Surface>

      {/* Session Hub integration */}
      <Surface id="sidecar-sessionhub" className="p-4 sm:p-6 scroll-mt-20">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="flex items-start gap-3">
            <div className="rounded-lg bg-rose-500/10 p-2">
              <ShieldAlertIcon className="h-5 w-5 text-rose-500" />
            </div>
            <div>
              <h3 className="text-base font-semibold">Session Hub headers</h3>
              <p className="text-sm text-muted-foreground mt-1">
                upstream providers need matching <code className="rounded bg-muted px-1 py-0.5 text-xs">x-opencode-*</code>{" "}
                headers. Add them to the Session Hub so the fingerprint check passes.
              </p>
            </div>
          </div>
          <Button onClick={openRulesDialog} className="gap-1.5 h-11 sm:h-9 w-full sm:w-auto">
            <ArrowRightIcon className="h-3.5 w-3.5" />
            Configure rules
          </Button>
        </div>

        <div className="mt-4 flex flex-col gap-1.5 sm:flex-row sm:items-center sm:gap-3">
          <label className="text-sm font-medium shrink-0">Target provider</label>
          <Input
            value={rulesProvider}
            onChange={(e) => setRulesProvider(e.target.value)}
            className="h-11 sm:h-8 text-sm sm:text-xs font-mono sm:max-w-xs"
            placeholder="opencode-zen"
          />
        </div>

        <div className="mt-3 flex items-start gap-2 rounded-md bg-muted/40 p-3 text-xs text-muted-foreground">
          <AlertTriangleIcon className="h-4 w-4 shrink-0 mt-0.5" />
          <span>
            Rules are merged, never overwritten. If you customise a value the sidecar will flag
            it as non-default so you know the fingerprint may no longer match.
          </span>
        </div>
      </Surface>

      {/* Core settings */}
      <Surface id="sidecar-core" className="p-4 sm:p-6 scroll-mt-20">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="flex items-start gap-3">
            <div className="rounded-lg bg-blue-500/10 p-2">
              <ZapIcon className="h-5 w-5 text-blue-500" />
            </div>
            <div>
              <h3 className="text-base font-semibold">Core Settings</h3>
              <p className="text-sm text-muted-foreground mt-1">
                Enable/disable the sidecar and configure base behavior.
              </p>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <StatusBadge running={settings.enabled} />
          </div>
        </div>

        <div className="mt-6 grid grid-cols-1 gap-4 xl:grid-cols-2">
          <ToggleField
            label="Enable sidecar"
            checked={settings.enabled}
            onCheckedChange={(v) => update("enabled", v)}
            description="Route upstream traffic through the Bun TLS proxy"
          />
          <ToggleField
            label="Inject upstream tools"
            checked={settings.inject_tools}
            onCheckedChange={(v) => update("inject_tools", v)}
            description="Automatically add the 11 upstream tools to free-tier requests"
          />
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Listen port</label>
            <Input
              type="number"
              value={settings.port}
              onChange={(e) => update("port", Number(e.target.value) || 8090)}
              className="h-11 sm:h-9 font-mono text-sm"
              min={1}
              max={65535}
            />
            <p className="text-xs text-muted-foreground">Sidecar HTTP port (default 8090)</p>
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Upstream base URL</label>
            <Input
              value={settings.base_url}
              onChange={(e) => update("base_url", e.target.value)}
              className="h-11 sm:h-9 font-mono text-sm"
              placeholder="https://opencode.ai/zen/v1"
            />
            <p className="text-xs text-muted-foreground">upstream API base URL (free tier or go)</p>
          </div>
        </div>
      </Surface>

      {/* Auth & retry */}
      <Surface id="sidecar-auth" className="p-4 sm:p-6 scroll-mt-20">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="flex items-start gap-3">
            <div className="rounded-lg bg-amber-500/10 p-2">
              <GlobeIcon className="h-5 w-5 text-amber-500" />
            </div>
            <div>
              <h3 className="text-base font-semibold">Authentication & Retry</h3>
              <p className="text-sm text-muted-foreground mt-1">
                Default auth header and retry behavior for failed requests.
              </p>
            </div>
          </div>
        </div>

        <div className="mt-6 grid grid-cols-1 gap-4 xl:grid-cols-2">
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Default Authorization</label>
            <Input
              value={settings.default_auth}
              onChange={(e) => update("default_auth", e.target.value)}
              className="h-11 sm:h-9 font-mono text-sm"
              placeholder="Bearer public"
            />
            <p className="text-xs text-muted-foreground">Fallback when no provider auth is set</p>
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">User-Agent</label>
            <Input
              value={settings.user_agent}
              onChange={(e) => update("user_agent", e.target.value)}
              className="h-11 sm:h-9 font-mono text-sm"
              placeholder="opencode/1.18.31 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14"
            />
            <p className="text-xs text-muted-foreground">Upstream User-Agent header</p>
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Max retry attempts</label>
            <Input
              type="number"
              value={settings.max_attempts}
              onChange={(e) => update("max_attempts", Number(e.target.value) || 4)}
              className="h-11 sm:h-9 font-mono text-sm"
              min={1}
              max={10}
            />
            <p className="text-xs text-muted-foreground">Retry on 403/429 (1-10)</p>
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Retry delay (ms)</label>
            <Input
              type="number"
              value={settings.retry_delay_ms}
              onChange={(e) => update("retry_delay_ms", Number(e.target.value) || 750)}
              className="h-11 sm:h-9 font-mono text-sm"
              min={100}
              max={5000}
            />
            <p className="text-xs text-muted-foreground">Base delay between retries (100-5000ms)</p>
          </div>
        </div>
      </Surface>

      {/* Multi-IP */}
      <Surface id="sidecar-multiip" className="p-4 sm:p-6 scroll-mt-20">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="flex items-start gap-3">
            <div className="rounded-lg bg-violet-500/10 p-2">
              <ServerIcon className="h-5 w-5 text-violet-500" />
            </div>
            <div>
              <h3 className="text-base font-semibold">Multi-IP Bind Proxies</h3>
              <p className="text-sm text-muted-foreground mt-1">
                Egress from specific IPs via per-IP CONNECT proxies. Each IP gets its own Go
                proxy process on the loopback interface.
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">
              {proxies.length} proxy{proxies.length !== 1 ? "ies" : ""}
            </span>
          </div>
        </div>

        <div className="mt-6">
          <label className="text-sm font-medium mb-2 block">Bind IP addresses</label>
          <BindIPEditor
            bindIPs={settings.bind_ips ?? []}
            onChange={(ips) => update("bind_ips", ips)}
          />
          <p className="text-xs text-muted-foreground mt-2">
            Add local IPs to enable per-IP egress. The entrypoint starts a Go CONNECT proxy for
            each (ports 8981+). Providers with a matching{" "}
            <code className="mx-0.5 rounded bg-muted px-1 py-0.5 text-[11px]">bind_ip</code>
            will route through the corresponding proxy.
          </p>
        </div>

        {proxies.length > 0 && (
          <div className="mt-4">
            <label className="text-sm font-medium mb-2 block">Active proxies</label>
            <div className="rounded-lg border border-border/60 overflow-x-auto">
              <table className="w-full text-sm min-w-[320px]">
                <thead>
                  <tr className="border-b border-border/60 bg-muted/30">
                    <th className="px-3 py-2 text-left font-medium text-muted-foreground">IP Address</th>
                    <th className="px-3 py-2 text-left font-medium text-muted-foreground">Proxy Port</th>
                    <th className="px-3 py-2 text-left font-medium text-muted-foreground">Status</th>
                  </tr>
                </thead>
                <tbody>
                  {proxies.map((p) => (
                    <tr key={p.ip} className="border-b border-border/40 last:border-0">
                      <td className="px-3 py-2 font-mono text-xs whitespace-nowrap">{p.ip}</td>
                      <td className="px-3 py-2 font-mono text-xs">{p.port}</td>
                      <td className="px-3 py-2">
                        <StatusBadge running={p.enabled} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </Surface>

      <SessionHubRulesDialog
        open={rulesDialogOpen}
        onOpenChange={setRulesDialogOpen}
        onConfirm={confirmEnsureRules}
        pending={ensureRules.isPending}
        result={rulesResult}
      />
    </div>
  );
}

import { useEffect, useState } from "react";
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
  SparklesIcon,
  DownloadIcon,
  UploadIcon,
} from "lucide-react";
import { apiFetch } from "@/lib/api/client";
import {
  fetchExtensions,
  importExtension,
  deleteExtension,
  exportExtensionUrl,
  applyExtension,
  ensureExtensionHeaders,
  type Extension,
} from "@/lib/api/extensions";
import { SidecarPresetImportDialog } from "@/components/settings/SidecarPresetImportDialog";

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
  inject_tool_types?: string[] | undefined;
  default_auth: string;
  user_agent: string;
  base_url: string;
  max_attempts: number;
  retry_delay_ms: number;
  bind_ips: string[];
  proxies: SidecarProxy[];
  tools_path?: string | undefined;
  oauth_server?: string | undefined;
  oauth_client_id?: string | undefined;
  oauth_verification_base?: string | undefined;
}

interface EnsureRulesResult {
  provider: string;
  added: string[];
  already: string[];
  headers: {
    name: string;
    mode: string;
    prefix?: string | undefined;
    length?: number | undefined;
    value?: string | undefined;
    charset?: string | undefined;
    values?: string[] | undefined;
  }[];
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

interface SessionHubRule {
  name: string;
  enabled: boolean;
  headers: { name: string; mode: string; prefix?: string; length?: number; value?: string; charset?: string }[];
}

function useSessionHubRules() {
  return useQuery({
    queryKey: ["sessionhub", "providers"],
    queryFn: () =>
      apiFetch<{ status: string; data: SessionHubRule[] }>("/admin/api/v1/sessionhub/providers").then(
        (r) => r.data,
      ),
    refetchInterval: 8000,
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
    mutationFn: ({
      provider,
      headers,
    }: {
      provider: string;
      headers: EnsureRulesResult["headers"];
    }) =>
      apiFetch<{ status: string; data: EnsureRulesResult }>(
        `/admin/api/v1/sessionhub/providers/${encodeURIComponent(provider)}/ensure-headers`,
        { method: "POST", json: { headers } },
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
          The sidecar reproduces an upstream client signature (TLS fingerprint, headers and tool
          schema) so requests look like they come from the official client. Upstream hardening can
          break it at any time, and some of these techniques may violate a provider&apos;s terms of
          service.
        </p>
        <p className="text-muted-foreground mt-2 leading-snug">
          Import extensions only from sources you trust. Third-party extensions are unreviewed
          modifications and may contain harmful instructions, data-exfiltration URLs, or
          configuration that violates a provider&apos;s rules. Review an extension&apos;s JSON
          before importing and keep a copy of anything you import.
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
  headers,
  extensionName,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  pending: boolean;
  result: EnsureRulesResult | null;
  headers?: EnsureRulesResult["headers"] | undefined;
  extensionName?: string | undefined;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg mx-4 sm:mx-auto max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <ShieldAlertIcon className="h-5 w-5 text-amber-500" />
            Add header rules from extension?
          </DialogTitle>
          <DialogDescription>
            {extensionName ? `The "${extensionName}" extension ` : "This extension "}
            supplies header rules the Session Hub must install for the upstream to accept requests.
          </DialogDescription>
        </DialogHeader>

        {!result ? (
          <div className="flex flex-col gap-3 text-sm">
            <p className="text-muted-foreground">
              This will add the following rules to the selected target. Existing rules are never
              overwritten — only missing ones are added.
            </p>
            <div className="rounded-lg border border-border/60 divide-y divide-border/40">
              {(headers ?? []).length === 0 ? (
                <div className="px-3 py-2 text-xs text-muted-foreground">
                  This extension does not declare header rules.
                </div>
              ) : (
                (headers ?? []).map((h) => (
                  <div key={h.name} className="flex items-center justify-between gap-3 px-3 py-2">
                    <code className="text-xs font-medium">{h.name}</code>
                    <div className="text-right text-xs text-muted-foreground">
                      <div>{h.mode}</div>
                      <div className="font-mono">
                        {h.prefix ?? ""}
                        {h.length ? ` + ${h.length}` : ""}
                        {h.value ? ` · ${h.value}` : ""}
                      </div>
                    </div>
                  </div>
                ))
              )}
            </div>
            <p className="text-xs text-muted-foreground">
              You can edit rules any time in the Session Hub tab; non-default values are flagged
              there.
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
  const [rulesProvider, setRulesProvider] = useState("");
  const [extensions, setExtensions] = useState<Extension[]>([]);
  const [selectedExtensionId, setSelectedExtensionId] = useState("");
  const [lastApplyState, setLastApplyState] = useState<{ ok: boolean; message: string; added: string[]; kept: string[] } | null>(null);
  const [importOpen, setImportOpen] = useState(false);
  const [importMode, setImportMode] = useState<"json" | "url">("json");
  const [importJSON, setImportJSON] = useState("");
  const [importURL, setImportURL] = useState("");
  const [importBusy, setImportBusy] = useState(false);
  const [importError, setImportError] = useState("");
  const [extensionsError, setExtensionsError] = useState("");
  const [storeURL, setStoreURL] = useState("");
  const [storeBusy, setStoreBusy] = useState(false);
  const [storeError, setStoreError] = useState("");
  const [storeResults, setStoreResults] = useState<
    Array<{ id: string; name: string; tagline?: string | undefined; url: string }>
  >([]);

  const { data: sessionHubRules } = useSessionHubRules();

  const selectedExtension = extensions.find((p) => p.id === selectedExtensionId);
  const boundRule = (sessionHubRules ?? []).find((r) => r.name === rulesProvider);
  const ruleBound = Boolean(boundRule);

  // Load imported extensions from the gateway (no built-ins).
  const loadExtensions = () => {
    fetchExtensions()
      .then((list) => {
        setExtensions(list);
        setExtensionsError("");
        setSelectedExtensionId((current) =>
          current && list.some((p) => p.id === current) ? current : list[0]?.id ?? "",
        );
      })
      .catch((err) => setExtensionsError(err instanceof Error ? err.message : "Failed to load extensions"));
  };

  useEffect(() => {
    loadExtensions();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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
    if (!selectedExtension) return;
    ensureRules.mutate(
      { provider: rulesProvider, headers: selectedExtension.headers ?? [] },
      {
        onSuccess: (res) => setRulesResult(res.data),
      },
    );
  };

  // Apply an extension: push its sidecar settings, then install its header
  // rules into the Session Hub for the chosen target. Both steps report into
  // the result banner.
  const handleApplyExtension = () => {
    if (!settings || !selectedExtension) return;
    const target = rulesProvider.trim();
    if (!target) return;

    const nextSettings: SidecarSettings = {
      ...settings,
      enabled: true,
      inject_tools: selectedExtension.inject_tools ?? settings.inject_tools,
      inject_tool_types: selectedExtension.inject_tool_types ?? settings.inject_tool_types ?? [],
      default_auth: selectedExtension.default_auth ?? settings.default_auth,
      base_url: selectedExtension.base_url ?? settings.base_url,
      user_agent: selectedExtension.user_agent ?? settings.user_agent,
      max_attempts: selectedExtension.max_attempts ?? settings.max_attempts,
      retry_delay_ms: selectedExtension.retry_delay_ms ?? settings.retry_delay_ms,
      oauth_server:
        selectedExtension.oauth?.server ??
        selectedExtension.settings?.oauth_server ??
        settings.oauth_server,
      oauth_client_id:
        selectedExtension.oauth?.client_id ??
        selectedExtension.settings?.oauth_client_id ??
        settings.oauth_client_id,
      oauth_verification_base:
        selectedExtension.oauth?.verification_base ??
        selectedExtension.settings?.oauth_verification_base ??
        settings.oauth_verification_base,
    };
    setForm(nextSettings);
    updateSettings.mutate(nextSettings);

    applyExtension(selectedExtension.id)
      .then((res) => {
        if (res.tools_path || res.oauth) {
          setForm((prev) =>
            prev
              ? {
                  ...prev,
                  tools_path: res.tools_path ?? prev.tools_path,
                  oauth_server: res.oauth?.server ?? prev.oauth_server,
                  oauth_client_id: res.oauth?.client_id ?? prev.oauth_client_id,
                  oauth_verification_base:
                    res.oauth?.verification_base ?? prev.oauth_verification_base,
                }
              : prev,
          );
        }
        return ensureExtensionHeaders(target, res.headers);
      })
      .then((res) =>
        setLastApplyState({
          ok: true,
          message: `Applied "${selectedExtension.name}" to ${target}.`,
          added: res.added,
          kept: res.already,
        }),
      )
      .catch((err) =>
        setLastApplyState({
          ok: false,
          message: `Could not apply extension: ${err instanceof Error ? err.message : String(err)}`,
          added: [],
          kept: [],
        }),
      );
  };

  const handleImport = () => {
    setImportBusy(true);
    setImportError("");
    importExtension(importMode === "url" ? { url: importURL.trim() } : { json: importJSON })
      .then((ext) => {
        loadExtensions();
        setSelectedExtensionId(ext.id);
        setImportOpen(false);
        setImportJSON("");
        setImportURL("");
      })
      .catch((err) => setImportError(err instanceof Error ? err.message : "Import failed"))
      .finally(() => setImportBusy(false));
  };

  const handleExport = (ext: Extension) => {
    apiFetch<Extension>(`/admin/api/v1/sidecar/extensions/${encodeURIComponent(ext.id)}/export`)
      .then((data) => {
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `${ext.id}.extension.json`;
        a.click();
        URL.revokeObjectURL(url);
      })
      .catch(() => {
        window.open(exportExtensionUrl(ext.id), "_blank");
      });
  };

  const handleDeleteExtension = (ext: Extension) => {
    if (ext.builtin) return;
    deleteExtension(ext.id).then(loadExtensions);
  };

  const handleBrowseStore = () => {
    const base = storeURL.trim();
    if (!base) return;
    setStoreBusy(true);
    setStoreError("");
    setStoreResults([]);
    apiFetch<{ extensions?: Array<{ id: string; name: string; tagline?: string; raw_url?: string }> }>(
      `/admin/api/v1/sidecar/extensions/store/browse?url=${encodeURIComponent(base)}`,
      { method: "POST", json: { url: base } },
    )
      .then((res) => {
        const list = (res as { extensions?: Array<{ id: string; name: string; tagline?: string; raw_url?: string }> })
          .extensions ?? [];
        setStoreResults(
          list.map((e) => ({
            id: e.id,
            name: e.name,
            ...(e.tagline != null ? { tagline: e.tagline } : {}),
            url: e.raw_url ?? `${base.replace(/\/$/, "")}/api/v1/extensions/${encodeURIComponent(e.id)}/raw`,
          })),
        );
        if (list.length === 0) setStoreError("No extensions found at this store.");
      })
      .catch((err) => setStoreError(err instanceof Error ? err.message : "Store fetch failed"))
      .finally(() => setStoreBusy(false));
  };

  const handleInstallFromStore = (url: string, id: string) => {
    setStoreBusy(true);
    setStoreError("");
    importExtension({ url })
      .then(() => {
        loadExtensions();
        setSelectedExtensionId(id);
      })
      .catch((err) => setStoreError(err instanceof Error ? err.message : "Install failed"))
      .finally(() => setStoreBusy(false));
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
                Optional Bun-based TLS proxy that reproduces extension-supplied upstream client
                signatures (TLS fingerprint, headers, tool schema). Configured through extensions;
                requires the Bun runtime.
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

      {/* Quick setup: preset + target */}
      <Surface id="sidecar-setup" className="p-4 sm:p-6 scroll-mt-20">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="flex items-start gap-3">
            <div className="rounded-lg bg-primary/10 p-2">
              <SparklesIcon className="h-5 w-5 text-primary" />
            </div>
            <div>
              <h3 className="text-base font-semibold">Quick setup</h3>
              <p className="text-sm text-muted-foreground mt-1">
                Pick an imported extension and a target. Applying it turns on the sidecar with the
                right settings and installs the matching Session Hub headers in one step.
              </p>
            </div>
          </div>
          {ruleBound && (
            <span className="hidden items-center gap-1.5 rounded-full border border-success/30 bg-success/10 px-2.5 py-1 text-[11px] font-medium text-success sm:inline-flex">
              <CheckCircleIcon className="h-3 w-3" />
              {rulesProvider} configured
            </span>
          )}
        </div>

        {extensionsError ? (
          <p className="mt-4 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
            {extensionsError}
          </p>
        ) : null}

        {/* Extension chooser */}
        <div className="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {extensions.map((ext) => {
            const active = ext.id === selectedExtensionId;
            return (
              <button
                key={ext.id}
                type="button"
                onClick={() => setSelectedExtensionId(ext.id)}
                className={`flex flex-col gap-1 rounded-lg border p-4 text-left transition-colors ${
                  active
                    ? "border-primary/50 bg-primary/5"
                    : "border-border/40 bg-surface hover:bg-surface-hover/30"
                }`}
              >
                <span className="flex items-center gap-2">
                  {ext.ui?.accent ? (
                    <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: ext.ui.accent }} />
                  ) : (
                    <span className={`h-2.5 w-2.5 rounded-full ${active ? "bg-primary" : "bg-muted-foreground/30"}`} />
                  )}
                  <span className="text-sm font-medium text-foreground">{ext.name}</span>
                </span>
                <span className="text-xs text-muted-foreground">{ext.tagline}</span>
              </button>
            );
          })}
          {extensions.length === 0 && !extensionsError ? (
            <div className="col-span-full rounded-lg border border-dashed border-border/60 bg-surface/50 p-6 text-center text-sm text-muted-foreground">
              No extensions installed. Import extension JSON or install from a store below.
            </div>
          ) : null}
        </div>

        {/* Import / export / store toolbar */}
        <div className="mt-3 flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={() => setImportOpen(true)} className="h-10 gap-1.5 sm:h-9">
            <UploadIcon className="h-3.5 w-3.5" />
            Import extension
          </Button>
          {selectedExtension ? (
            <>
              <Button variant="outline" size="sm" onClick={() => handleExport(selectedExtension)} className="h-10 gap-1.5 sm:h-9">
                <DownloadIcon className="h-3.5 w-3.5" />
                Export {selectedExtension.name}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => handleDeleteExtension(selectedExtension)}
                className="h-10 gap-1.5 border-destructive/40 text-destructive hover:bg-destructive/10 sm:h-9"
              >
                <Trash2Icon className="h-3.5 w-3.5" />
                Delete
              </Button>
            </>
          ) : null}
        </div>

        {/* Store browse */}
        <div className="mt-3 rounded-lg border border-border/40 bg-background/40 p-3">
          <p className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
            Browse store
          </p>
          <div className="mt-2 flex flex-col gap-2 sm:flex-row">
            <Input
              value={storeURL}
              onChange={(e) => setStoreURL(e.target.value)}
              placeholder="https://store.example.com"
              className="h-11 sm:h-9 font-mono text-sm flex-1"
            />
            <Button
              variant="outline"
              size="sm"
              onClick={handleBrowseStore}
              disabled={storeBusy || !storeURL.trim()}
              className="h-11 gap-1.5 sm:h-9"
            >
              {storeBusy ? <RefreshCwIcon className="h-3.5 w-3.5 animate-spin" /> : <GlobeIcon className="h-3.5 w-3.5" />}
              Browse
            </Button>
          </div>
          {storeError ? (
            <p className="mt-2 text-xs text-destructive">{storeError}</p>
          ) : null}
          {storeResults.length > 0 ? (
            <div className="mt-2 flex flex-col gap-1.5">
              {storeResults.map((r) => (
                <div
                  key={r.id}
                  className="flex flex-col gap-1 rounded-md border border-border/40 px-3 py-2 sm:flex-row sm:items-center sm:justify-between"
                >
                  <div className="min-w-0">
                    <span className="block truncate text-sm font-medium text-foreground">{r.name}</span>
                    {r.tagline ? (
                      <span className="block truncate text-xs text-muted-foreground">{r.tagline}</span>
                    ) : null}
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleInstallFromStore(r.url, r.id)}
                    disabled={storeBusy}
                    className="h-9 gap-1.5 sm:h-8"
                  >
                    <DownloadIcon className="h-3.5 w-3.5" />
                    Install
                  </Button>
                </div>
              ))}
            </div>
          ) : null}
        </div>

        {selectedExtension ? (
          <>
            <p className="mt-4 text-sm text-muted-foreground">{selectedExtension.description}</p>
            {selectedExtension.ui?.help ? (
              <p className="mt-2 text-xs text-muted-foreground">{selectedExtension.ui.help}</p>
            ) : null}

            {(selectedExtension.requirements?.length ?? 0) > 0 ? (
              <div className="mt-3 rounded-md border border-amber-500/30 bg-amber-500/5 p-3">
                <p className="text-[11px] font-bold uppercase tracking-wider text-amber-600 dark:text-amber-400">
                  Requirements
                </p>
                <ul className="mt-1 list-disc pl-4 text-xs text-muted-foreground">
                  {selectedExtension.requirements?.map((req) => (
                    <li key={req}>{req}</li>
                  ))}
                </ul>
              </div>
            ) : null}

            {(selectedExtension.headers?.length ?? 0) > 0 ? (
              <div className="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-2">
                {selectedExtension.headers?.map((h) => (
                  <div
                    key={h.name}
                    className="flex flex-col gap-0.5 rounded-md border border-border/40 bg-background/40 px-3 py-2"
                  >
                    <span className="font-mono text-xs font-medium text-foreground">{h.name}</span>
                    <span className="text-[11px] text-muted-foreground">
                      {h.mode}
                      {h.prefix ? ` · ${h.prefix}` : ""}
                      {h.length ? ` + ${h.length}` : ""}
                      {h.charset ? ` ${h.charset}` : ""}
                      {h.value ? ` · ${h.value}` : ""}
                    </span>
                  </div>
                ))}
              </div>
            ) : null}

            {/* Dynamic ui.fields */}
            {(selectedExtension.ui?.fields?.length ?? 0) > 0 ? (
              <div className="mt-4 rounded-lg border border-border/40 bg-background/40 p-3">
                <p className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
                  Extension settings
                </p>
                <div className="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-2">
                  {selectedExtension.ui?.fields?.map((field) => (
                    <label key={field.key} className="flex flex-col gap-1.5">
                      <span className="text-sm font-medium">{field.label}</span>
                      {field.type === "boolean" ? (
                        <ToggleField
                          label=""
                          checked={
                            (selectedExtension.settings?.[field.key] ??
                              field.default ??
                              "false") === "true"
                          }
                          onCheckedChange={(v) => {
                            /* informational until apply wires free-form settings */
                            void v;
                          }}
                          description={field.description}
                        />
                      ) : field.type === "select" && field.options?.length ? (
                        <select
                          className="h-11 sm:h-9 rounded-lg border border-border/60 bg-surface px-3 text-sm text-foreground"
                          defaultValue={field.default ?? field.options[0]}
                          disabled
                        >
                          {field.options.map((opt) => (
                            <option key={opt} value={opt}>
                              {opt}
                            </option>
                          ))}
                        </select>
                      ) : (
                        <Input
                          type={field.secret ? "password" : field.type === "number" ? "number" : "text"}
                          defaultValue={field.default ?? ""}
                          disabled
                          className="h-11 sm:h-9 font-mono text-sm"
                          placeholder={field.secret ? "••••••••" : field.default}
                        />
                      )}
                      {field.description ? (
                        <span className="text-xs text-muted-foreground">{field.description}</span>
                      ) : null}
                    </label>
                  ))}
                </div>
                <p className="mt-2 text-xs text-muted-foreground">
                  Fields are applied when the extension is applied.
                </p>
              </div>
            ) : null}

            <div className="mt-5 flex flex-col gap-3 sm:flex-row sm:items-end">
              <div className="flex flex-1 flex-col gap-1.5">
                <label className="text-sm font-medium">Target pool or provider</label>
                <Input
                  value={rulesProvider}
                  onChange={(e) => setRulesProvider(e.target.value)}
                  className="h-11 font-mono text-sm sm:h-9 sm:max-w-xs"
                  placeholder="my-pool"
                />
                <p className="text-xs text-muted-foreground">
                  The Session Hub rule is bound to this name (a pool applies to all its members).
                </p>
              </div>
              <Button
                onClick={handleApplyExtension}
                disabled={!rulesProvider.trim() || updateSettings.isPending}
                className="h-11 w-full gap-1.5 sm:h-9 sm:w-auto"
              >
                {updateSettings.isPending ? (
                  <RefreshCwIcon className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <SparklesIcon className="h-3.5 w-3.5" />
                )}
                Apply extension
              </Button>
              <Button
                variant="outline"
                onClick={openRulesDialog}
                className="h-11 w-full gap-1.5 sm:h-9 sm:w-auto"
              >
                <ArrowRightIcon className="h-3.5 w-3.5" />
                Review changes
              </Button>
            </div>
          </>
        ) : null}

        {lastApplyState && (
          <div className="mt-3 flex items-start gap-2 rounded-md border border-border/40 bg-muted/30 p-3 text-xs">
            {lastApplyState.ok ? (
              <CheckCircleIcon className="mt-0.5 h-4 w-4 shrink-0 text-success" />
            ) : (
              <XCircleIcon className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
            )}
            <div className="text-muted-foreground">
              <span className="font-medium text-foreground">{lastApplyState.message}</span>
              {lastApplyState.added.length > 0 && (
                <div className="mt-1">
                  Added:{" "}
                  {lastApplyState.added.map((h) => (
                    <code key={h} className="mr-1 rounded bg-muted px-1 py-0.5 font-mono">
                      {h}
                    </code>
                  ))}
                </div>
              )}
              {lastApplyState.kept.length > 0 && (
                <div className="mt-1">
                  Kept existing:{" "}
                  {lastApplyState.kept.map((h) => (
                    <code key={h} className="mr-1 rounded bg-muted px-1 py-0.5 font-mono">
                      {h}
                    </code>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        <div className="mt-3 flex items-start gap-2 rounded-md bg-muted/40 p-3 text-xs text-muted-foreground">
          <AlertTriangleIcon className="h-4 w-4 shrink-0 mt-0.5" />
          <span className="flex flex-wrap items-center gap-1">
            Existing rules are never overwritten — only missing headers are added. Edit the rules
            any time in the{" "}
            <a
              href="#sessionhub"
              className="font-medium text-foreground underline decoration-border/60 underline-offset-2 hover:text-accent"
            >
              Session Hub
            </a>{" "}
            tab.
          </span>
        </div>

        <SidecarPresetImportDialog
          open={importOpen}
          onOpenChange={setImportOpen}
          mode={importMode}
          setMode={setImportMode}
          jsonValue={importJSON}
          setJSONValue={setImportJSON}
          urlValue={importURL}
          setURLValue={setImportURL}
          onConfirm={handleImport}
          pending={importBusy}
          error={importError}
        />
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
            description="Route matching provider traffic through the Bun TLS proxy"
          />
          <ToggleField
            label="Inject tool schema"
            checked={settings.inject_tools}
            onCheckedChange={(v) => update("inject_tools", v)}
            description="Add the preset's bundled tool schema when the request has none"
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
              placeholder="https://upstream.example.com/v1"
            />
            <p className="text-xs text-muted-foreground">Extension / operator upstream base URL</p>
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
              placeholder="Mozilla/5.0 ..."
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
        headers={selectedExtension?.headers ?? []}
        extensionName={selectedExtension?.name}
      />
    </div>
  );
}

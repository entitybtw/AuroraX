import { Surface, SectionHeader } from "@/components/ui/surface";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useModels } from "@/lib/api/useModels";
import { useProviderStatus } from "@/lib/api/useProviders";
import { modelDisplayName } from "@/lib/api/models-types";
import { RuntimeStatusBadge, useSettings } from "./SettingsContext";
import {
  ServerIcon, ArrowRightLeftIcon,
  SaveIcon, GaugeIcon,
} from "lucide-react";
import { useEffect, useState } from "react";

function formatToggle(enabled: boolean): string {
  return enabled ? "Enabled" : "Disabled";
}

function parseCommaList(value: string): string[] {
  return value
    .split(",")
    .map(item => item.trim())
    .filter(Boolean);
}

function uniqueStrings(values: string[]): string[] {
  return [...new Set(values.map(value => value.trim()).filter(Boolean))].sort((a, b) => a.localeCompare(b));
}

function toggleString(values: string[], value: string, checked: boolean): string[] {
  const normalized = value.trim();
  if (!normalized) return values;
  const without = values.filter(item => item !== normalized);
  return checked ? uniqueStrings([...without, normalized]) : without;
}

function StatusChip({ enabled }: { enabled: boolean }): JSX.Element {
  return (
    <span
      className={`inline-flex items-center border px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-widest ${enabled
        ? "border-success/30 bg-success/10 text-success"
        : "border-border/60 bg-background/70 text-muted-foreground"
        }`}
    >
      {enabled ? "Active" : "Off"}
    </span>
  );
}

export function GeneralTab(): JSX.Element {
  const {
    config, dashboardSettings, setDashboardSettings,
    passthroughProvidersText, setPassthroughProvidersText,
    timezoneOverride, handleTimezoneChange,
    handleAdminEndpointToggle, handleDashboardSettingsSave,
    mutations,
  } = useSettings();

  const runtimeSettings = config?.settings as any;

  const runtimeFeature = (key: string) => config?.runtime_features?.find((f: { key: string }) => f.key.toLowerCase() === key);
  const isRuntimeFeatureEnabled = (feature: { configured?: boolean; status?: string } | undefined) => {
    const status = feature?.status?.toLowerCase();
    return Boolean(feature?.configured || status === "enabled" || status === "active");
  };
  const runtimeFeatureConfigured = (key: string) => isRuntimeFeatureEnabled(runtimeFeature(key));
  const tokenSaver = dashboardSettings.token_saver;
  const providerStatusQuery = useProviderStatus();
  const modelsQuery = useModels();
  const availableProviders = uniqueStrings([
    ...(providerStatusQuery.data?.providers ?? []).flatMap(provider => [provider.name, provider.type]),
    ...(modelsQuery.data ?? []).flatMap(model => [model.provider_name ?? "", model.provider_type ?? ""]),
  ]);
  const availableModels = uniqueStrings((modelsQuery.data ?? []).map(model => modelDisplayName(model)));

  const adminEndpointsEnabled = Boolean(runtimeSettings?.client.admin_endpoints_enabled);
  const adminUIEnabled = Boolean(runtimeSettings?.client.admin_ui_enabled);
  const passthroughRoutesEnabled = runtimeFeatureConfigured("passthrough") || Boolean(runtimeSettings?.client.enable_passthrough_routes);

  const [detectedTz, setDetectedTz] = useState("UTC");

  useEffect(() => {
    try {
      setDetectedTz(Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC");
    } catch {
      setDetectedTz("UTC");
    }
  }, []);

  return (
    <div className="flex flex-col gap-6">
      <Surface id="client-settings" className="p-6 scroll-mt-20">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="border border-border/40 bg-background/80 p-2">
                <ServerIcon className="h-4 w-4 text-accent" />
              </div>
              <SectionHeader
                title="Client Settings"
                subtitle="Gateway and dashboard surface controls that shape how the admin UI and OpenAI-compatible surface are exposed."
              />
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <RuntimeStatusBadge featureKey="passthrough" />
              <RuntimeStatusBadge featureKey="auth_keys" />
              <StatusChip enabled={adminEndpointsEnabled || adminUIEnabled || passthroughRoutesEnabled} />
            </div>
          </div>
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Passthrough routes</div>
              <div className="mt-2 text-[14px] font-medium text-foreground">{formatToggle(passthroughRoutesEnabled)}</div>
              <div className="mt-2 text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Enabled passthrough providers</div>
              <Input placeholder="openai, anthropic" className="w-full mt-1" value={passthroughProvidersText} onChange={e => setPassthroughProvidersText(e.target.value)} />
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="flex items-center justify-between">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Anthropic ingress</div>
                <StatusChip enabled={dashboardSettings.client.enable_anthropic_ingress ?? false} />
              </div>
              <div className="flex items-center gap-3 mt-1">
                <input type="checkbox" checked={dashboardSettings.client.enable_anthropic_ingress ?? false} onChange={e => setDashboardSettings({ ...dashboardSettings, client: { ...dashboardSettings.client, enable_anthropic_ingress: e.target.checked } })} className="h-4 w-4 rounded border-border text-accent focus:ring-accent" />
                <span className="text-[14px] font-medium text-foreground">Enable Anthropic ingress</span>
              </div>
              <div className="mt-1 text-[12px] text-muted-foreground">Expose /v1/messages endpoint for Anthropic-format chat completions. Allows Anthropic SDK clients and Claude Code CLI to route through the gateway. Requires restart.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Admin API</div>
              <div className="flex items-center gap-3 mt-1">
                <input type="checkbox" checked={dashboardSettings.client.admin_endpoints_enabled ?? adminEndpointsEnabled} onChange={e => handleAdminEndpointToggle(e.target.checked)} className="h-4 w-4 rounded border-border text-accent focus:ring-accent" />
                <span className="text-[14px] font-medium text-foreground">Enable admin API endpoints</span>
              </div>
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground mt-2">Admin UI</div>
              <div className="flex items-center gap-3 mt-1">
                <input type="checkbox" checked={dashboardSettings.client.admin_ui_enabled ?? adminUIEnabled} onChange={e => setDashboardSettings({ ...dashboardSettings, client: { ...dashboardSettings.client, admin_ui_enabled: e.target.checked } })} className="h-4 w-4 rounded border-border text-accent focus:ring-accent" />
                <span className="text-[14px] font-medium text-foreground">Enable admin dashboard UI</span>
              </div>
              <div className="mt-1 text-[12px] text-muted-foreground">Requires restart.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Server port & base path</div>
              <div className="grid grid-cols-2 gap-3 mt-1">
                <div>
                  <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Port</label>
                  <Input placeholder="e.g. 4000" className="w-full mt-1" value={dashboardSettings.client.port ?? ""} onChange={e => setDashboardSettings({ ...dashboardSettings, client: { ...dashboardSettings.client, port: e.target.value } })} />
                </div>
                <div>
                  <label className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Base path</label>
                  <Input placeholder="e.g. /api/v1" className="w-full mt-1" value={dashboardSettings.client.base_path ?? ""} onChange={e => setDashboardSettings({ ...dashboardSettings, client: { ...dashboardSettings.client, base_path: e.target.value } })} />
                </div>
              </div>
              <div className="mt-1 text-[12px] text-muted-foreground">Changes take effect after server restart.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Debug endpoints</div>
              <div className="flex items-center gap-3 mt-1">
                <input type="checkbox" checked={dashboardSettings.client.swagger_enabled ?? false} onChange={e => setDashboardSettings({ ...dashboardSettings, client: { ...dashboardSettings.client, swagger_enabled: e.target.checked } })} className="h-4 w-4 rounded border-border text-accent focus:ring-accent" />
                <span className="text-[14px] font-medium text-foreground">Swagger / OpenAPI docs</span>
              </div>
              <div className="flex items-center gap-3 mt-1">
                <input type="checkbox" checked={dashboardSettings.client.pprof_enabled ?? false} onChange={e => setDashboardSettings({ ...dashboardSettings, client: { ...dashboardSettings.client, pprof_enabled: e.target.checked } })} className="h-4 w-4 rounded border-border text-accent focus:ring-accent" />
                <span className="text-[14px] font-medium text-foreground">pprof profiling</span>
              </div>
              <div className="mt-1 text-[12px] text-muted-foreground">Requires restart.</div>
            </div>
          </div>
          <div className="flex items-center gap-3 mt-2 border-t border-border/50 pt-4">
            <Button onClick={handleDashboardSettingsSave} disabled={mutations.saveDashboardSettingsMutation.isPending}>
              <SaveIcon className="mr-2 h-4 w-4" />
              {mutations.saveDashboardSettingsMutation.isPending ? "Saving..." : "Save Client Settings"}
            </Button>
            {mutations.saveDashboardSettingsMutation.isSuccess && <span className="text-[13px] font-medium text-success">Saved</span>}
            {mutations.saveDashboardSettingsMutation.isError && <span className="text-[13px] font-medium text-destructive">{mutations.saveDashboardSettingsMutation?.error?.message}</span>}
          </div>
        </div>
      </Surface>

      <Surface id="compatibility-settings" className="p-6 scroll-mt-20">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="border border-border/40 bg-background/80 p-2">
                <ArrowRightLeftIcon className="h-4 w-4 text-accent" />
              </div>
              <SectionHeader
                title="Compatibility"
                subtitle="Request conversion controls that shape how provider model lists and aliases are exposed through the API surface."
              />
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <RuntimeStatusBadge featureKey="models" />
              <RuntimeStatusBadge featureKey="aliases" />
              <RuntimeStatusBadge featureKey="fallback" />
            </div>
          </div>
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Model compatibility mode</div>
              <select className="field-input w-full mt-1" value={dashboardSettings.client.configured_provider_models_mode} onChange={e => setDashboardSettings({ ...dashboardSettings, client: { ...dashboardSettings.client, configured_provider_models_mode: e.target.value } })}>
                <option value="fallback">Fallback (use configured list if upstream fails)</option>
                <option value="allowlist">Allowlist (use configured list exclusively)</option>
              </select>
              <div className="mt-1 text-[12px] text-muted-foreground">Controls how configured provider model lists are applied to discovery.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Alias projection</div>
              <div className="flex items-center gap-3 mt-1">
                <input type="checkbox" checked={!dashboardSettings.client.keep_only_aliases_at_models_endpoint} onChange={e => setDashboardSettings({ ...dashboardSettings, client: { ...dashboardSettings.client, keep_only_aliases_at_models_endpoint: !e.target.checked } })} className="h-4 w-4 rounded border-border text-accent focus:ring-accent" />
                <span className="text-[14px] font-medium text-foreground">Include concrete models</span>
              </div>
              <div className="mt-1 text-[12px] text-muted-foreground">Whether /v1/models keeps provider catalog entries in addition to aliases.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">OpenAI passthrough alias</div>
              <div className="flex items-center gap-3 mt-1">
                <input type="checkbox" checked={dashboardSettings.client.allow_passthrough_v1_alias} onChange={e => setDashboardSettings({ ...dashboardSettings, client: { ...dashboardSettings.client, allow_passthrough_v1_alias: e.target.checked } })} className="h-4 w-4 rounded border-border text-accent focus:ring-accent" />
                <span className="text-[14px] font-medium text-foreground">Allow /p/{"{provider}"}/v1</span>
              </div>
              <div className="mt-1 text-[12px] text-muted-foreground">Lets <code>/p/{"{provider}"}/v1/...</code> behave like the canonical <code>/p/{"{provider}"}/...</code> path.</div>
            </div>
            <div className="border border-border/60 bg-surface-hover/20 p-4">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Compatibility status</div>
              <div className="mt-2 text-[14px] font-medium text-foreground">Handled by the gateway</div>
              <div className="mt-1 text-[12px] text-muted-foreground">aurora already normalizes requests and responses server-side, so no separate toggle panel exists yet.</div>
            </div>
          </div>
          <div className="flex items-center gap-3 mt-2 border-t border-border/50 pt-4">
            <Button onClick={handleDashboardSettingsSave} disabled={mutations.saveDashboardSettingsMutation.isPending}>
              <SaveIcon className="mr-2 h-4 w-4" />
              {mutations.saveDashboardSettingsMutation.isPending ? "Saving..." : "Save Compatibility Settings"}
            </Button>
            {mutations.saveDashboardSettingsMutation.isSuccess && <span className="text-[13px] font-medium text-success">Saved</span>}
            {mutations.saveDashboardSettingsMutation.isError && <span className="text-[13px] font-medium text-destructive">{mutations.saveDashboardSettingsMutation?.error?.message}</span>}
          </div>
        </div>
      </Surface>

      <Surface id="token-saver-settings" className="p-6 scroll-mt-20">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="border border-border/40 bg-background/80 p-2">
                <GaugeIcon className="h-4 w-4 text-accent" />
              </div>
              <SectionHeader
                title="Aurora Token Saver"
                subtitle="Policy-driven prompt and tool-output compression with an optional concise response profile. Disabled by default and scoped by endpoint, model, and provider."
              />
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <RuntimeStatusBadge featureKey="token_saver" />
              <StatusChip enabled={tokenSaver.enabled} />
            </div>
          </div>
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={tokenSaver.enabled}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, enabled: e.target.checked } })}
                  className="h-4 w-4 rounded border-border text-accent focus:ring-accent"
                />
                <span className="text-[14px] font-medium text-foreground">Enable Token Saver</span>
              </div>
              <div className="mt-1 text-[12px] text-muted-foreground">Applies deterministic compression before provider dispatch when the request matches this policy.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={tokenSaver.apply_streaming}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, apply_streaming: e.target.checked } })}
                  className="h-4 w-4 rounded border-border text-accent focus:ring-accent"
                />
                <span className="text-[14px] font-medium text-foreground">Apply to streaming requests</span>
              </div>
              <div className="mt-1 text-[12px] text-muted-foreground">When disabled, streaming chat completions bypass compression.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">On error</div>
              <select className="field-input w-full mt-1" value={tokenSaver.on_error} onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, on_error: e.target.value } })}>
                <option value="allow">Allow original request</option>
                <option value="block">Block request</option>
              </select>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={tokenSaver.output_enabled}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, output_enabled: e.target.checked } })}
                  className="h-4 w-4 rounded border-border text-accent focus:ring-accent"
                />
                <span className="text-[14px] font-medium text-foreground">Concise output profile</span>
              </div>
              <div className="mt-1 text-[12px] text-muted-foreground">Adds a safe concise-response instruction except when JSON response format is requested.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Output level</div>
              <select className="field-input w-full mt-1" value={tokenSaver.output_level} onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, output_level: e.target.value } })}>
                <option value="lite">Lite</option>
                <option value="full">Full</option>
                <option value="ultra">Ultra</option>
                <option value="wenyan">Wenyan</option>
              </select>
              <div className="mt-1 text-[12px] text-muted-foreground">Controls response profile verbosity. Full is the balanced default; ultra and wenyan are more aggressive.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={tokenSaver.emit_headers}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, emit_headers: e.target.checked } })}
                  className="h-4 w-4 rounded border-border text-accent focus:ring-accent"
                />
                <span className="text-[14px] font-medium text-foreground">Emit observability headers</span>
              </div>
              <div className="mt-1 text-[12px] text-muted-foreground">Headers include only status and character counts, never prompt content.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={tokenSaver.audit_enabled}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, audit_enabled: e.target.checked } })}
                  className="h-4 w-4 rounded border-border text-accent focus:ring-accent"
                />
                <span className="text-[14px] font-medium text-foreground">Audit logging</span>
              </div>
              <div className="mt-1 text-[12px] text-muted-foreground">Logs token saver decisions to the audit trail. Disabled by default; enable for compliance visibility.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30 xl:col-span-2">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Endpoint scope</div>
              <Input value={tokenSaver.endpoints.join(", ")} onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, endpoints: parseCommaList(e.target.value) } })} />
              <div className="mt-1 text-[12px] text-muted-foreground">Comma-separated endpoints. Use chat_completions for OpenAI-compatible chat requests.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-3 transition-colors hover:bg-surface-hover/30 xl:col-span-2">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Model scope</div>
                  <div className="mt-1 text-[12px] text-muted-foreground">Select exact provider/model inventory entries. Empty include means all models are eligible unless excluded.</div>
                </div>
                <span className="text-[11px] text-muted-foreground">{modelsQuery.isLoading ? "Loading models…" : `${availableModels.length} models`}</span>
              </div>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                <Input placeholder="Include models manually" value={tokenSaver.model_include.join(", ")} onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, model_include: parseCommaList(e.target.value) } })} />
                <Input placeholder="Exclude models manually" value={tokenSaver.model_exclude.join(", ")} onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, model_exclude: parseCommaList(e.target.value) } })} />
              </div>
              <div className="max-h-44 overflow-auto border border-border/40 bg-background/40 p-2">
                {availableModels.length === 0 ? (
                  <div className="p-2 text-[12px] text-muted-foreground">No model inventory loaded yet. Use Runtime Refresh or enter model selectors manually.</div>
                ) : availableModels.map(model => (
                  <label key={model} className="flex items-center justify-between gap-3 px-2 py-1.5 text-[12px] hover:bg-surface-hover/40">
                    <span className="font-mono text-foreground/90">{model}</span>
                    <div className="flex items-center gap-4">
                      <span className="flex items-center gap-1"><input type="checkbox" checked={tokenSaver.model_include.includes(model)} onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, model_include: toggleString(tokenSaver.model_include, model, e.target.checked) } })} /> include</span>
                      <span className="flex items-center gap-1"><input type="checkbox" checked={tokenSaver.model_exclude.includes(model)} onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, model_exclude: toggleString(tokenSaver.model_exclude, model, e.target.checked) } })} /> exclude</span>
                    </div>
                  </label>
                ))}
              </div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-3 transition-colors hover:bg-surface-hover/30 xl:col-span-2">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Provider scope</div>
                  <div className="mt-1 text-[12px] text-muted-foreground">Select provider names or provider types discovered from provider status and model inventory.</div>
                </div>
                <span className="text-[11px] text-muted-foreground">{providerStatusQuery.isLoading ? "Loading providers…" : `${availableProviders.length} providers/types`}</span>
              </div>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                <Input placeholder="Include providers manually" value={tokenSaver.provider_include.join(", ")} onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, provider_include: parseCommaList(e.target.value) } })} />
                <Input placeholder="Exclude providers manually" value={tokenSaver.provider_exclude.join(", ")} onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, provider_exclude: parseCommaList(e.target.value) } })} />
              </div>
              <div className="max-h-40 overflow-auto border border-border/40 bg-background/40 p-2">
                {availableProviders.length === 0 ? (
                  <div className="p-2 text-[12px] text-muted-foreground">No provider inventory loaded yet. Use Runtime Refresh or enter provider selectors manually.</div>
                ) : availableProviders.map(provider => (
                  <label key={provider} className="flex items-center justify-between gap-3 px-2 py-1.5 text-[12px] hover:bg-surface-hover/40">
                    <span className="font-mono text-foreground/90">{provider}</span>
                    <div className="flex items-center gap-4">
                      <span className="flex items-center gap-1"><input type="checkbox" checked={tokenSaver.provider_include.includes(provider)} onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, provider_include: toggleString(tokenSaver.provider_include, provider, e.target.checked) } })} /> include</span>
                      <span className="flex items-center gap-1"><input type="checkbox" checked={tokenSaver.provider_exclude.includes(provider)} onChange={e => setDashboardSettings({ ...dashboardSettings, token_saver: { ...tokenSaver, provider_exclude: toggleString(tokenSaver.provider_exclude, provider, e.target.checked) } })} /> exclude</span>
                    </div>
                  </label>
                ))}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-3 mt-2 border-t border-border/50 pt-4">
            <Button onClick={handleDashboardSettingsSave} disabled={mutations.saveDashboardSettingsMutation.isPending}>
              <SaveIcon className="mr-2 h-4 w-4" />
              {mutations.saveDashboardSettingsMutation.isPending ? "Saving..." : "Save Token Saver Settings"}
            </Button>
            {mutations.saveDashboardSettingsMutation.isSuccess && <span className="text-[13px] font-medium text-success">Saved</span>}
            {mutations.saveDashboardSettingsMutation.isError && <span className="text-[13px] font-medium text-destructive">{mutations.saveDashboardSettingsMutation?.error?.message}</span>}
          </div>
        </div>
      </Surface>


      <Surface className="p-6">
        <div className="flex flex-col gap-6">
          <SectionHeader
            title="Timezone"
            subtitle="Day-based analytics, charts, and date filters use your effective timezone. Usage and audit logs keep UTC in the hover title while rendering row timestamps in your effective timezone."
          />
          <div className="flex flex-col gap-4 max-w-sm">
            <div className="flex flex-col gap-2">
              <label className="text-sm font-medium text-muted-foreground">Timezone Override</label>
              <select
                className="field-input"
                value={timezoneOverride}
                onChange={(e) => handleTimezoneChange(e.target.value)}
              >
                <option value="">Automatic ({detectedTz})</option>
                <option value="UTC">UTC</option>
                <option value="America/New_York">America/New_York</option>
                <option value="America/Chicago">America/Chicago</option>
                <option value="America/Denver">America/Denver</option>
                <option value="America/Los_Angeles">America/Los_Angeles</option>
                <option value="Europe/London">Europe/London</option>
                <option value="Europe/Paris">Europe/Paris</option>
                <option value="Asia/Tokyo">Asia/Tokyo</option>
                <option value="Asia/Shanghai">Asia/Shanghai</option>
                <option value="Australia/Sydney">Australia/Sydney</option>
              </select>
            </div>
            {timezoneOverride && (
              <Button variant="outline" size="sm" onClick={() => handleTimezoneChange("")} className="w-max">
                Use Browser Timezone
              </Button>
            )}
          </div>
        </div>
      </Surface>
    </div>
  );
}

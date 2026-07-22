import { Surface, SectionHeader } from "@/components/ui/surface";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { RuntimeStatusBadge, useSettings } from "./SettingsContext";
import { ActivityIcon, GaugeIcon, SaveIcon } from "lucide-react";

export function NetworkingTab(): JSX.Element {
  const { dashboardSettings, setDashboardSettings, handleDashboardSettingsSave, mutations } = useSettings();

  return (
    <div className="flex flex-col gap-6">
      <Surface id="server-surface" className="p-6 scroll-mt-20">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="border border-border/40 bg-background/80 p-2">
                <ActivityIcon className="h-4 w-4 text-accent" />
              </div>
              <SectionHeader
                title="Server Surface"
                subtitle="Inbound listener and debug endpoints exposed by this gateway runtime. Some values require restart when changed in config."
              />
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <RuntimeStatusBadge featureKey="pprof" />
            </div>
          </div>
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-4">
            <div className="border border-border/40 bg-surface p-4 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Port</div>
              <div className="mt-2 font-mono text-[14px] font-medium text-foreground">{dashboardSettings.client.port || "8080"}</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Base path</div>
              <div className="mt-2 font-mono text-[14px] font-medium text-foreground">{dashboardSettings.client.base_path || "/"}</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Swagger UI</div>
              <div className="mt-2 text-[14px] font-medium text-foreground">{dashboardSettings.client.swagger_enabled ? "Enabled" : "Disabled"}</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">PProf</div>
              <div className="mt-2 text-[14px] font-medium text-foreground">{dashboardSettings.client.pprof_enabled ? "Enabled" : "Disabled"}</div>
            </div>
          </div>
        </div>
      </Surface>

      <Surface id="proxy-settings" className="p-6 scroll-mt-20">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="border border-border/40 bg-background/80 p-2">
                <ActivityIcon className="h-4 w-4 text-accent" />
              </div>
              <SectionHeader
                title="Proxy"
                subtitle="Global outbound proxy configuration for the gateway."
              />
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <RuntimeStatusBadge featureKey="passthrough" />
              <RuntimeStatusBadge featureKey="pprof" />
            </div>
          </div>
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">HTTP proxy</div>
              <Input
                type="text"
                className="w-full mt-1"
                placeholder="http://proxy.example.com:8080"
                value={dashboardSettings.proxy.http_proxy}
                onChange={e => setDashboardSettings({ ...dashboardSettings, proxy: { ...dashboardSettings.proxy, http_proxy: e.target.value } })}
              />
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">HTTPS proxy</div>
              <Input
                type="text"
                className="w-full mt-1"
                placeholder="https://proxy.example.com:8443"
                value={dashboardSettings.proxy.https_proxy}
                onChange={e => setDashboardSettings({ ...dashboardSettings, proxy: { ...dashboardSettings.proxy, https_proxy: e.target.value } })}
              />
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">No proxy</div>
              <Input
                type="text"
                className="w-full mt-1"
                placeholder="localhost, 127.0.0.1, .local"
                value={dashboardSettings.proxy.no_proxy}
                onChange={e => setDashboardSettings({ ...dashboardSettings, proxy: { ...dashboardSettings.proxy, no_proxy: e.target.value } })}
              />
              <div className="mt-1 text-[12px] text-muted-foreground">Comma-separated hosts to bypass the proxy.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={dashboardSettings.proxy.proxy_auth_enabled}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, proxy: { ...dashboardSettings.proxy, proxy_auth_enabled: e.target.checked } })}
                  className="h-4 w-4 rounded border-border text-accent focus:ring-accent"
                />
                <span className="text-[14px] font-medium text-foreground">Proxy auth required</span>
              </div>
              <div className="mt-1 text-[12px] text-muted-foreground">Toggle if your proxy requires username/password authentication.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30 xl:col-span-2">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">CA certificate (PEM)</div>
              <textarea
                className="field-input w-full mt-1 font-mono text-[12px] leading-relaxed min-h-[100px] resize-y"
                placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----"
                value={dashboardSettings.proxy.ca_cert_pem}
                onChange={e => setDashboardSettings({ ...dashboardSettings, proxy: { ...dashboardSettings.proxy, ca_cert_pem: e.target.value } })}
              />
              <div className="mt-1 text-[12px] text-muted-foreground">Custom CA certificate for proxy TLS interception. Leave empty to use system trust store.</div>
            </div>
          </div>
          <div className="flex items-center gap-3 mt-2 border-t border-border/50 pt-4">
            <Button onClick={handleDashboardSettingsSave} disabled={mutations.saveDashboardSettingsMutation.isPending}>
              <SaveIcon className="mr-2 h-4 w-4" />
              {mutations.saveDashboardSettingsMutation.isPending ? "Saving..." : "Save Proxy Settings"}
            </Button>
            {mutations.saveDashboardSettingsMutation.isSuccess && <span className="text-[13px] font-medium text-success">Saved</span>}
            {mutations.saveDashboardSettingsMutation.isError && <span className="text-[13px] font-medium text-destructive">{mutations.saveDashboardSettingsMutation?.error?.message}</span>}
          </div>
        </div>
      </Surface>

      <Surface id="performance-settings" className="p-6 scroll-mt-20">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="border border-border/40 bg-background/80 p-2">
                <GaugeIcon className="h-4 w-4 text-accent" />
              </div>
              <SectionHeader
                title="Performance"
                subtitle="Timeout, retry, and circuit breaker settings for HTTP requests."
              />
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <RuntimeStatusBadge featureKey="metrics" />
            </div>
          </div>
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">HTTP timeout</div>
              <Input
                type="number"
                min={0}
                className="w-full mt-1"
                value={dashboardSettings.performance.http_timeout_seconds}
                onChange={e => setDashboardSettings({ ...dashboardSettings, performance: { ...dashboardSettings.performance, http_timeout_seconds: parseInt(e.target.value) || 0 } })}
              />
              <div className="mt-1 text-[12px] text-muted-foreground">Seconds</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Response header timeout</div>
              <Input
                type="number"
                min={0}
                className="w-full mt-1"
                value={dashboardSettings.performance.http_response_header_timeout_seconds}
                onChange={e => setDashboardSettings({ ...dashboardSettings, performance: { ...dashboardSettings.performance, http_response_header_timeout_seconds: parseInt(e.target.value) || 0 } })}
              />
              <div className="mt-1 text-[12px] text-muted-foreground">Seconds</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Workflow refresh interval</div>
              <Input
                type="number"
                min={0}
                className="w-full mt-1"
                value={dashboardSettings.performance.workflow_refresh_interval_seconds}
                onChange={e => setDashboardSettings({ ...dashboardSettings, performance: { ...dashboardSettings.performance, workflow_refresh_interval_seconds: parseInt(e.target.value) || 0 } })}
              />
              <div className="mt-1 text-[12px] text-muted-foreground">Seconds</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Body size limit</div>
              <Input
                type="text"
                className="w-full mt-1"
                placeholder="10M"
                value={dashboardSettings.client.body_size_limit}
                onChange={e => setDashboardSettings({ ...dashboardSettings, client: { ...dashboardSettings.client, body_size_limit: e.target.value } })}
              />
              <div className="mt-1 text-[12px] text-muted-foreground">Max request/response body size (e.g. 10M, 50M)</div>
            </div>
          </div>

          <div className="border-t border-border/50 pt-4">
            <h4 className="font-semibold text-[15px] tracking-tight text-foreground mb-3">Resilience</h4>
            <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Max retries</div>
                <Input
                  type="number"
                  min={0}
                  className="w-full mt-1"
                  value={dashboardSettings.performance.retry_max_retries}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, performance: { ...dashboardSettings.performance, retry_max_retries: parseInt(e.target.value) || 0 } })}
                />
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Initial backoff</div>
                <Input
                  type="number"
                  min={0}
                  className="w-full mt-1"
                  value={dashboardSettings.performance.retry_initial_backoff_milliseconds}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, performance: { ...dashboardSettings.performance, retry_initial_backoff_milliseconds: parseInt(e.target.value) || 0 } })}
                />
                <div className="mt-1 text-[12px] text-muted-foreground">Milliseconds</div>
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Max backoff</div>
                <Input
                  type="number"
                  min={0}
                  className="w-full mt-1"
                  value={dashboardSettings.performance.retry_max_backoff_milliseconds}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, performance: { ...dashboardSettings.performance, retry_max_backoff_milliseconds: parseInt(e.target.value) || 0 } })}
                />
                <div className="mt-1 text-[12px] text-muted-foreground">Milliseconds</div>
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Backoff factor</div>
                <Input
                  type="number"
                  step={0.1}
                  min={0}
                  className="w-full mt-1"
                  value={dashboardSettings.performance.retry_backoff_factor}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, performance: { ...dashboardSettings.performance, retry_backoff_factor: parseFloat(e.target.value) || 0 } })}
                />
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Jitter factor</div>
                <Input
                  type="number"
                  step={0.01}
                  min={0}
                  max={1}
                  className="w-full mt-1"
                  value={dashboardSettings.performance.retry_jitter_factor}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, performance: { ...dashboardSettings.performance, retry_jitter_factor: parseFloat(e.target.value) || 0 } })}
                />
              </div>
            </div>
          </div>

          <div className="border-t border-border/50 pt-4">
            <h4 className="font-semibold text-[15px] tracking-tight text-foreground mb-3">Circuit Breaker</h4>
            <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Failure threshold</div>
                <Input
                  type="number"
                  min={0}
                  className="w-full mt-1"
                  value={dashboardSettings.performance.circuit_breaker_failure_threshold}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, performance: { ...dashboardSettings.performance, circuit_breaker_failure_threshold: parseInt(e.target.value) || 0 } })}
                />
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Success threshold</div>
                <Input
                  type="number"
                  min={0}
                  className="w-full mt-1"
                  value={dashboardSettings.performance.circuit_breaker_success_threshold}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, performance: { ...dashboardSettings.performance, circuit_breaker_success_threshold: parseInt(e.target.value) || 0 } })}
                />
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Timeout</div>
                <Input
                  type="number"
                  min={0}
                  className="w-full mt-1"
                  value={dashboardSettings.performance.circuit_breaker_timeout_milliseconds}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, performance: { ...dashboardSettings.performance, circuit_breaker_timeout_milliseconds: parseInt(e.target.value) || 0 } })}
                />
                <div className="mt-1 text-[12px] text-muted-foreground">Milliseconds</div>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-3 mt-2 border-t border-border/50 pt-4">
            <Button onClick={handleDashboardSettingsSave} disabled={mutations.saveDashboardSettingsMutation.isPending}>
              <SaveIcon className="mr-2 h-4 w-4" />
              {mutations.saveDashboardSettingsMutation.isPending ? "Saving..." : "Save Performance Settings"}
            </Button>
            {mutations.saveDashboardSettingsMutation.isSuccess && <span className="text-[13px] font-medium text-success">Saved</span>}
            {mutations.saveDashboardSettingsMutation.isError && <span className="text-[13px] font-medium text-destructive">{mutations.saveDashboardSettingsMutation?.error?.message}</span>}
          </div>
        </div>
      </Surface>
    </div>
  );
}

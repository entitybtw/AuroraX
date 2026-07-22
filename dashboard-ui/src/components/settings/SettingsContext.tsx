import { createContext, useContext, useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useDashboardConfig } from "@/lib/api/useDashboardConfig";
import { flagOn } from "@/lib/api/dashboard-config";
import { apiFetch } from "@/lib/api/client";
import type { DashboardSettingsFormState, DashboardSettingsSaveResponse, RuntimeRefreshResponse } from "./types";

export function StatusChip({ enabled }: { enabled: boolean }): JSX.Element {
  return (
    <span className={`inline-flex items-center border px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-widest ${enabled ? "border-success/30 bg-success/10 text-success" : "border-border/60 bg-background/70 text-muted-foreground"}`}>
      {enabled ? "Active" : "Off"}
    </span>
  );
}

export function RuntimeStatusBadge({ featureKey }: { featureKey: string }): JSX.Element | null {
  const ctx = useSettings();
  const feature = ctx.config?.runtime_features?.find((item: { key: string }) => item.key === featureKey);
  if (!feature) return null;

  const status = String(feature.status ?? (feature.configured ? "enabled" : "disabled")).toLowerCase();
  const enabled = status === "enabled" || status === "active";
  const degraded = status === "degraded";
  const cls = enabled
    ? "border-success/30 bg-success/10 text-success"
    : degraded
      ? "border-warning/30 bg-warning/10 text-warning"
      : "border-border/60 bg-background/70 text-muted-foreground";

  return (
    <span
      className={`inline-flex items-center border px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-widest ${cls}`}
      title={feature.description ?? feature.label ?? feature.key}
    >
      {status}
    </span>
  );
}

interface SettingsValue {
  config: any;
  runtimeSettings: any;
  dashboardSettings: DashboardSettingsFormState;
  setDashboardSettings: React.Dispatch<React.SetStateAction<DashboardSettingsFormState>>;
  passthroughProvidersText: string;
  setPassthroughProvidersText: React.Dispatch<React.SetStateAction<string>>;
  timezoneOverride: string;
  handleTimezoneChange: (val: string) => void;
  autoRefreshEnabled: boolean;
  toggleAutoRefresh: () => void;
  handleAdminEndpointToggle: (checked: boolean) => void;
  handleDashboardSettingsSave: () => void;
  adminEndpointsEnabled: boolean;
  masterKeyConfigured: boolean;
  auditLogsEnabled: boolean;
  cacheConfigured: boolean;
  metricsEnabled: boolean;
  securityConfigured: boolean;
  guardrailsEnabled: boolean;
  batchGuardrailsEnabled: boolean;
  pricingRecalculationEnabled: boolean;
  mutations: {
    refreshMutation: { isPending: boolean; isError: boolean; isSuccess: boolean; data: RuntimeRefreshResponse | undefined; error: { message: string } | null; mutate: (vars?: any) => void };
    recalculateMutation: { isPending: boolean; isError: boolean; isSuccess: boolean; error: { message: string } | null; mutate: (vars: any) => void };
    saveDashboardSettingsMutation: { isPending: boolean; isError: boolean; isSuccess: boolean; data: DashboardSettingsSaveResponse | undefined; error: { message: string } | null; mutate: (vars: any) => void };
  };
}

const SettingsContext = createContext<SettingsValue | null>(null);

export function useSettings(): SettingsValue {
  const ctx = useContext(SettingsContext);
  if (!ctx) throw new Error("useSettings must be used within SettingsProvider");
  return ctx;
}

function defaults(): DashboardSettingsFormState {
  return {
    client: { body_size_limit: "10M", configured_provider_models_mode: "fallback", keep_only_aliases_at_models_endpoint: false, allow_passthrough_v1_alias: true, enable_anthropic_ingress: false },
    caching: { model_refresh_interval_seconds: 3600, model_list_url: "", exact_cache_enabled: false, exact_cache_ttl_seconds: 3600, exact_cache_redis_key: "", semantic_cache_enabled: false, semantic_similarity_threshold: 0.92, semantic_prompt_similarity_min: 0.72, semantic_ttl_seconds: 3600, semantic_max_conversation_messages: 3, semantic_exclude_system_prompt: false, semantic_embedder_provider: "", semantic_embedder_model: "", semantic_vector_store_type: "", prompt_cache_mode: "auto", prompt_cache_system_prompt: true, prompt_cache_first_message: true, prompt_cache_tools: false, prompt_cache_min_tokens: 1024 },
    logging: { enabled: false, log_bodies: true, log_headers: true, buffer_size: 1000, flush_interval_seconds: 5, retention_days: 30, only_model_interactions: true },
    observability: { metrics_enabled: false, metrics_endpoint: "/metrics" },
    performance: { http_timeout_seconds: 600, http_response_header_timeout_seconds: 600, workflow_refresh_interval_seconds: 60, retry_max_retries: 3, retry_initial_backoff_milliseconds: 1000, retry_max_backoff_milliseconds: 30000, retry_backoff_factor: 2, retry_jitter_factor: 0.1, circuit_breaker_failure_threshold: 5, circuit_breaker_success_threshold: 2, circuit_breaker_timeout_milliseconds: 30000 },
    security: { guardrails_enabled: false, batch_guardrails: false },
    pricing: { enforce_returning_usage_data: true, pricing_recalculation_enabled: true, usage_retention_days: 90 },
    token_saver: { enabled: false, apply_streaming: true, endpoints: ["chat_completions"], output_enabled: false, output_profile: "concise", output_level: "full", emit_headers: true, on_error: "allow", model_include: [], model_exclude: [], provider_include: [], provider_exclude: [], audit_enabled: true },
    proxy: { http_proxy: "", https_proxy: "", no_proxy: "", proxy_auth_enabled: false, ca_cert_pem: "" },
  };
}

export function SettingsProvider({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient();
  const { data: config } = useDashboardConfig();
  const runtimeSettings = config?.settings as any;
  const [dashboardSettings, setDashboardSettings] = useState<DashboardSettingsFormState>(defaults);
  const [passthroughProvidersText, setPassthroughProvidersText] = useState("");
  const [timezoneOverride, setTimezoneOverride] = useState("");
  const [autoRefreshEnabled, setAutoRefreshEnabled] = useState(() => localStorage.getItem("aurora_dashboard_autorefresh") === "true");

  const refreshMutation = useMutation({
    mutationFn: () => apiFetch<RuntimeRefreshResponse>("/admin/api/v1/runtime/refresh", { method: "POST" }),
    onSuccess: () => queryClient.invalidateQueries(),
  });
  const recalculateMutation = useMutation({
    mutationFn: (params: { startDate?: string; endDate?: string; userPath?: string; selector?: string }) => {
      const body: Record<string, string> = {};
      if (params.startDate) body.start_date = params.startDate;
      if (params.endDate) body.end_date = params.endDate;
      if (params.userPath) body.user_path = params.userPath;
      if (params.selector) body.selector = params.selector;
      return apiFetch("/admin/api/v1/usage/recalculate-pricing", { method: "POST", json: body });
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["usage"] }),
  });
  const saveDashboardSettingsMutation = useMutation({
    mutationFn: (payload: DashboardSettingsFormState & { client: DashboardSettingsFormState["client"] & { enabled_passthrough_providers: string[] } }) => apiFetch<DashboardSettingsSaveResponse>("/admin/api/v1/dashboard/settings", { method: "PUT", json: payload }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["dashboard", "config"] }),
  });

  const auditLogsEnabled = flagOn(config?.LOGGING_ENABLED) || Boolean(runtimeSettings?.logging?.enabled);
  const metricsEnabled = Boolean(runtimeSettings?.observability?.metrics_enabled);
  const guardrailsEnabled = flagOn(config?.GUARDRAILS_ENABLED) || Boolean(runtimeSettings?.security?.guardrails_enabled);
  const batchGuardrailsEnabled = Boolean(runtimeSettings?.security?.batch_guardrails);
  const pricingRecalculationEnabled = flagOn(config?.USAGE_PRICING_RECALCULATION_ENABLED) || Boolean(runtimeSettings?.pricing?.pricing_recalculation_enabled);
  const masterKeyConfigured = Boolean(runtimeSettings?.security?.master_key_configured);
  const adminEndpointsEnabled = Boolean(runtimeSettings?.client?.admin_endpoints_enabled);
  const cacheConfigured = flagOn(config?.CACHE_ENABLED) || Boolean(runtimeSettings?.caching?.exact_cache_enabled) || Boolean(runtimeSettings?.caching?.semantic_cache_enabled);
  const securityConfigured = masterKeyConfigured || guardrailsEnabled || batchGuardrailsEnabled || adminEndpointsEnabled;

  useEffect(() => {
    const saved = localStorage.getItem("aurora_timezone_override");
    if (saved) setTimezoneOverride(saved);
  }, []);

  useEffect(() => {
    if (!runtimeSettings) return;
    const next = defaults();
    next.client = { ...next.client, ...runtimeSettings.client };
    next.caching = { ...next.caching, ...runtimeSettings.caching };
    next.logging = { ...next.logging, ...runtimeSettings.logging, enabled: auditLogsEnabled };
    next.observability = { ...next.observability, ...runtimeSettings.observability, metrics_enabled: metricsEnabled };
    next.performance = { ...next.performance, ...runtimeSettings.performance };
    next.security = { ...next.security, ...runtimeSettings.security, guardrails_enabled: guardrailsEnabled, batch_guardrails: batchGuardrailsEnabled };
    next.pricing = { ...next.pricing, ...runtimeSettings.pricing, pricing_recalculation_enabled: pricingRecalculationEnabled };
    next.token_saver = { ...next.token_saver, ...runtimeSettings.token_saver };
    next.proxy = { ...next.proxy, ...runtimeSettings.proxy };
    setDashboardSettings(next);
    setPassthroughProvidersText((runtimeSettings.client?.enabled_passthrough_providers || []).join(", "));
  }, [auditLogsEnabled, batchGuardrailsEnabled, guardrailsEnabled, metricsEnabled, pricingRecalculationEnabled, runtimeSettings]);

  const handleTimezoneChange = (val: string) => {
    setTimezoneOverride(val);
    if (val) localStorage.setItem("aurora_timezone_override", val);
    else localStorage.removeItem("aurora_timezone_override");
  };
  const toggleAutoRefresh = () => {
    const next = !autoRefreshEnabled;
    setAutoRefreshEnabled(next);
    localStorage.setItem("aurora_dashboard_autorefresh", String(next));
    if (next) window.dispatchEvent(new Event("aurora_autorefresh_toggled"));
  };
  const handleAdminEndpointToggle = (checked: boolean) => setDashboardSettings(prev => ({ ...prev, client: { ...prev.client, admin_endpoints_enabled: checked } }));
  const handleDashboardSettingsSave = () => saveDashboardSettingsMutation.mutate({
    ...dashboardSettings,
    client: { ...dashboardSettings.client, enabled_passthrough_providers: passthroughProvidersText.split(",").map(value => value.trim()).filter(Boolean) },
  });

  const value: SettingsValue = {
    config: config as any,
    runtimeSettings,
    dashboardSettings,
    setDashboardSettings,
    passthroughProvidersText,
    setPassthroughProvidersText,
    timezoneOverride,
    handleTimezoneChange,
    autoRefreshEnabled,
    toggleAutoRefresh,
    handleAdminEndpointToggle,
    handleDashboardSettingsSave,
    adminEndpointsEnabled,
    masterKeyConfigured,
    auditLogsEnabled,
    cacheConfigured,
    metricsEnabled,
    securityConfigured,
    guardrailsEnabled,
    batchGuardrailsEnabled,
    pricingRecalculationEnabled,
    mutations: { refreshMutation, recalculateMutation, saveDashboardSettingsMutation },
  };

  return <SettingsContext.Provider value={value}>{children}</SettingsContext.Provider>;
}

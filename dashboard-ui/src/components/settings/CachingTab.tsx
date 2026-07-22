import { Surface, SectionHeader } from "@/components/ui/surface";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { RuntimeStatusBadge, useSettings, StatusChip } from "./SettingsContext";
import { DatabaseIcon, SaveIcon } from "lucide-react";

export function CachingTab(): JSX.Element {
  const { dashboardSettings, setDashboardSettings, handleDashboardSettingsSave, mutations } = useSettings();
  const vectorDetails = [
    ["Backend URL", dashboardSettings.caching.semantic_vector_store_url],
    ["Collection", dashboardSettings.caching.semantic_vector_store_collection],
    ["Table", dashboardSettings.caching.semantic_vector_store_table],
    ["Namespace", dashboardSettings.caching.semantic_vector_store_namespace],
    ["Class", dashboardSettings.caching.semantic_vector_store_class],
    ["Dimension", dashboardSettings.caching.semantic_vector_store_dimension ? String(dashboardSettings.caching.semantic_vector_store_dimension) : ""],
    ["API key", dashboardSettings.caching.semantic_vector_store_api_key_set ? "Configured" : "Not configured"],
  ].filter(([, value]) => value);

  return (
    <div className="flex flex-col gap-6">
      <Surface id="caching-settings" className="p-6 scroll-mt-20">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="border border-border/40 bg-background/80 p-2">
                <DatabaseIcon className="h-4 w-4 text-accent" />
              </div>
              <SectionHeader
                title="Caching"
                subtitle="Model cache, exact cache, and semantic cache configuration for the gateway."
              />
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <RuntimeStatusBadge featureKey="model_cache" />
              <RuntimeStatusBadge featureKey="exact_cache" />
              <RuntimeStatusBadge featureKey="semantic_cache" />
              <StatusChip enabled={dashboardSettings.caching.exact_cache_enabled || dashboardSettings.caching.semantic_cache_enabled} />
            </div>
          </div>
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Model refresh interval</div>
              <Input
                type="number"
                min={0}
                max={86400}
                className="w-full mt-1"
                value={dashboardSettings.caching.model_refresh_interval_seconds}
                onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, model_refresh_interval_seconds: parseInt(e.target.value) || 0 } })}
              />
              <div className="mt-1 text-[12px] text-muted-foreground">Seconds between model list refreshes (0–86400)</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Model list URL</div>
              <Input
                type="text"
                className="w-full mt-1"
                placeholder="https://..."
                value={dashboardSettings.caching.model_list_url}
                onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, model_list_url: e.target.value } })}
              />
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Model list local path</div>
              <div className="mt-1 truncate font-mono text-[13px] text-foreground" title={dashboardSettings.caching.model_list_local_path || "Not configured"}>{dashboardSettings.caching.model_list_local_path || "Not configured"}</div>
              <div className="mt-1 text-[12px] text-muted-foreground">Loaded first when present; URL is the fallback/sync source.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">User overrides path</div>
              <div className="mt-1 truncate font-mono text-[13px] text-foreground" title={dashboardSettings.caching.model_list_user_overrides_path || "Not configured"}>{dashboardSettings.caching.model_list_user_overrides_path || "Not configured"}</div>
              <div className="mt-1 text-[12px] text-muted-foreground">Operator pricing/model metadata overrides applied on top of the registry.</div>
            </div>
            <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30 xl:col-span-2">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Model cache backend</div>
              <div className="mt-1 text-[14px] font-medium text-foreground">{dashboardSettings.caching.model_cache_backend || "Not configured"}</div>
              <div className="grid grid-cols-1 gap-2 pt-2 text-[12px] md:grid-cols-3">
                <div><span className="text-muted-foreground">Local dir: </span><code>{dashboardSettings.caching.model_cache_local_dir || "—"}</code></div>
                <div><span className="text-muted-foreground">Redis key: </span><code>{dashboardSettings.caching.model_cache_redis_key || "—"}</code></div>
                <div><span className="text-muted-foreground">Redis TTL: </span><code>{dashboardSettings.caching.model_cache_redis_ttl_seconds || "—"}</code></div>
              </div>
            </div>
          </div>
          <div className="border-t border-border/50 pt-4">
            <h4 className="font-semibold text-[15px] tracking-tight text-foreground mb-3">Exact Cache</h4>
            <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="flex items-center gap-3 mt-1">
                  <input
                    type="checkbox"
                    checked={dashboardSettings.caching.exact_cache_enabled}
                    onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, exact_cache_enabled: e.target.checked } })}
                    className="h-4 w-4 rounded border-border text-accent focus:ring-accent"
                  />
                  <span className="text-[14px] font-medium text-foreground">Enabled</span>
                </div>
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">TTL</div>
                <Input
                  type="number"
                  min={0}
                  className="w-full mt-1"
                  value={dashboardSettings.caching.exact_cache_ttl_seconds}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, exact_cache_ttl_seconds: parseInt(e.target.value) || 0 } })}
                />
                <div className="mt-1 text-[12px] text-muted-foreground">Seconds</div>
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Redis key</div>
                <Input
                  type="text"
                  className="w-full mt-1"
                  placeholder="cache:exact"
                  value={dashboardSettings.caching.exact_cache_redis_key}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, exact_cache_redis_key: e.target.value } })}
                />
              </div>
            </div>
          </div>
          <div className="border-t border-border/50 pt-4">
            <h4 className="font-semibold text-[15px] tracking-tight text-foreground mb-3">Semantic Cache</h4>
            <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="flex items-center gap-3 mt-1">
                  <input
                    type="checkbox"
                    checked={dashboardSettings.caching.semantic_cache_enabled}
                    onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, semantic_cache_enabled: e.target.checked } })}
                    className="h-4 w-4 rounded border-border text-accent focus:ring-accent"
                  />
                  <span className="text-[14px] font-medium text-foreground">Enabled</span>
                </div>
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Similarity threshold</div>
                <Input
                  type="number"
                  step={0.01}
                  min={0}
                  max={1}
                  className="w-full mt-1"
                  value={dashboardSettings.caching.semantic_similarity_threshold}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, semantic_similarity_threshold: parseFloat(e.target.value) || 0 } })}
                />
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Prompt similarity min</div>
                <Input
                  type="number"
                  step={0.01}
                  min={0}
                  max={1}
                  className="w-full mt-1"
                  value={dashboardSettings.caching.semantic_prompt_similarity_min}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, semantic_prompt_similarity_min: parseFloat(e.target.value) || 0 } })}
                />
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">TTL</div>
                <Input
                  type="number"
                  min={0}
                  className="w-full mt-1"
                  value={dashboardSettings.caching.semantic_ttl_seconds}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, semantic_ttl_seconds: parseInt(e.target.value) || 0 } })}
                />
                <div className="mt-1 text-[12px] text-muted-foreground">Seconds</div>
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Max conversation messages</div>
                <Input
                  type="number"
                  min={0}
                  className="w-full mt-1"
                  value={dashboardSettings.caching.semantic_max_conversation_messages}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, semantic_max_conversation_messages: parseInt(e.target.value) || 0 } })}
                />
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="flex items-center gap-3 mt-1">
                  <input
                    type="checkbox"
                    checked={dashboardSettings.caching.semantic_exclude_system_prompt}
                    onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, semantic_exclude_system_prompt: e.target.checked } })}
                    className="h-4 w-4 rounded border-border text-accent focus:ring-accent"
                  />
                  <span className="text-[14px] font-medium text-foreground">Exclude system prompt</span>
                </div>
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Embedder provider</div>
                <Input
                  type="text"
                  className="w-full mt-1"
                  placeholder="openai"
                  value={dashboardSettings.caching.semantic_embedder_provider}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, semantic_embedder_provider: e.target.value } })}
                />
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Embedder model</div>
                <Input
                  type="text"
                  className="w-full mt-1"
                  placeholder="text-embedding-3-small"
                  value={dashboardSettings.caching.semantic_embedder_model}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, semantic_embedder_model: e.target.value } })}
                />
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Vector store type</div>
                <Input
                  type="text"
                  className="w-full mt-1"
                  placeholder="redis"
                  value={dashboardSettings.caching.semantic_vector_store_type}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, semantic_vector_store_type: e.target.value } })}
                />
              </div>
              {vectorDetails.length > 0 && (
                <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30 xl:col-span-3">
                  <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Resolved vector backend</div>
                  <div className="grid grid-cols-1 gap-2 md:grid-cols-2 xl:grid-cols-3">
                    {vectorDetails.map(([label, value]) => (
                      <div key={label} className="flex items-center justify-between gap-3 text-[12px]">
                        <span className="text-muted-foreground">{label}</span>
                        <code className="max-w-[62%] truncate border border-border/40 bg-background/60 px-2 py-1 text-[11px] text-foreground" title={value}>{value}</code>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
          <div className="border-t border-border/50 pt-4 mt-4">
            <h4 className="font-semibold text-[15px] tracking-tight text-foreground mb-1">Provider Prompt Caching</h4>
            <p className="text-[13px] text-muted-foreground mb-3">
              Sends cache_control directives to upstream providers (Anthropic, OpenAI, etc.) so they cache
              prompt segments across requests. This reduces token spend at the provider level — separate from
              the gateway-level response caching above.
            </p>
            <div className="grid grid-cols-1 gap-4 xl:grid-cols-4">
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Mode</div>
                <select
                  className="mt-1 w-full border border-border bg-background px-2 py-1.5 text-[13px] text-foreground focus:outline-none focus:ring-1 focus:ring-accent"
                  value={dashboardSettings.caching.prompt_cache_mode}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, prompt_cache_mode: e.target.value } })}
                >
                  <option value="auto">Auto</option>
                  <option value="manual">Manual</option>
                  <option value="off">Off</option>
                </select>
                <div className="mt-1 text-[12px] text-muted-foreground">Auto injects breakpoints; Manual uses only explicit request cache_control</div>
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="flex items-center gap-3 mt-1">
                  <input
                    type="checkbox"
                    checked={dashboardSettings.caching.prompt_cache_system_prompt}
                    onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, prompt_cache_system_prompt: e.target.checked } })}
                    className="h-4 w-4 rounded border-border text-accent focus:ring-accent"
                  />
                  <span className="text-[14px] font-medium text-foreground">System prompt cache</span>
                </div>
                <div className="mt-1 text-[12px] text-muted-foreground">Mark system prompt content for caching (auto mode)</div>
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="flex items-center gap-3 mt-1">
                  <input
                    type="checkbox"
                    checked={dashboardSettings.caching.prompt_cache_first_message}
                    onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, prompt_cache_first_message: e.target.checked } })}
                    className="h-4 w-4 rounded border-border text-accent focus:ring-accent"
                  />
                  <span className="text-[14px] font-medium text-foreground">First message cache</span>
                </div>
                <div className="mt-1 text-[12px] text-muted-foreground">Mark first user message for caching (auto mode)</div>
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="flex items-center gap-3 mt-1">
                  <input
                    type="checkbox"
                    checked={dashboardSettings.caching.prompt_cache_tools}
                    onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, prompt_cache_tools: e.target.checked } })}
                    className="h-4 w-4 rounded border-border text-accent focus:ring-accent"
                  />
                  <span className="text-[14px] font-medium text-foreground">Tools cache</span>
                </div>
                <div className="mt-1 text-[12px] text-muted-foreground">Mark tool definitions for caching (auto mode)</div>
              </div>
              <div className="border border-border/40 bg-surface p-4 flex flex-col gap-2 transition-colors hover:bg-surface-hover/30">
                <div className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Min tokens before cache</div>
                <Input
                  type="number"
                  min={0}
                  className="w-full mt-1"
                  value={dashboardSettings.caching.prompt_cache_min_tokens}
                  onChange={e => setDashboardSettings({ ...dashboardSettings, caching: { ...dashboardSettings.caching, prompt_cache_min_tokens: parseInt(e.target.value) || 0 } })}
                />
                <div className="mt-1 text-[12px] text-muted-foreground">Minimum cumulative tokens before inserting a cache breakpoint</div>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-3 mt-2 border-t border-border/50 pt-4">
            <Button onClick={handleDashboardSettingsSave} disabled={mutations.saveDashboardSettingsMutation.isPending}>
              <SaveIcon className="mr-2 h-4 w-4" />
              {mutations.saveDashboardSettingsMutation.isPending ? "Saving..." : "Save Cache Settings"}
            </Button>
            {mutations.saveDashboardSettingsMutation.isSuccess && <span className="text-[13px] font-medium text-success">Saved</span>}
            {mutations.saveDashboardSettingsMutation.isError && <span className="text-[13px] font-medium text-destructive">{mutations.saveDashboardSettingsMutation?.error?.message}</span>}
          </div>
        </div>
      </Surface>
    </div>
  );
}

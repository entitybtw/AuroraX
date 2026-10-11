import * as React from "react";
import { DateRangeSelect } from "@/components/overview/DateRangeSelect";
import { ThroughputChart } from "@/components/charts/ThroughputChart";
import { TokenUsageChart } from "@/components/charts/TokenUsageChart";
import { PageHeader } from "@/components/ui/page-header";
import { CodeBlock, EmptyState, Pill, SectionHeader, Surface } from "@/components/ui/surface";
import { DataTable, TableWrap, Td, Th } from "@/components/ui/data-table";
import { ApiError } from "@/lib/api/client";
import { useDateRange } from "@/lib/date-picker/useDateRange";
import { useCacheOverview } from "@/lib/api/useUsage";
import { useCacheDebug } from "@/lib/api/useCache";
import { useDashboardConfig } from "@/lib/api/useDashboardConfig";
import { flagOn } from "@/lib/api/dashboard-config";
import type { UsageQueryFilters } from "@/lib/api/usage-types";
import { formatCost, formatRequests, formatTokens } from "@/lib/format/numbers";
import { t } from "@/lib/i18n";


const DEFAULT_CHAT_BODY = {
  model: "gpt-4o-mini",
  messages: [{ role: "user", content: "Summarize the cache policy for this request." }],
};

export function CachePage(): JSX.Element {
  const config = useDashboardConfig();
  const cacheEnabled = flagOn(config.data?.CACHE_ENABLED);
  const range = useDateRange("30d");
  const [path, setPath] = React.useState("/v1/chat/completions");
  const [cacheType, setCacheType] = React.useState("both");
  const [ttl, setTTL] = React.useState("");
  const [threshold, setThreshold] = React.useState("");
  const [promptSimilarity, setPromptSimilarity] = React.useState("");
  const [bodyText, setBodyText] = React.useState(JSON.stringify(DEFAULT_CHAT_BODY, null, 2));
  const [localDebugError, setLocalDebugError] = React.useState<string>("");

  const filters = React.useMemo<UsageQueryFilters>(
    () => ({ startDate: range.startDate, endDate: range.endDate, interval: "daily" }),
    [range.endDate, range.startDate],
  );

  const overview = useCacheOverview(filters);
  const debugMutation = useCacheDebug();

  const caching = config.data?.settings?.caching;
  const semanticEnabled = caching?.semantic_cache_enabled ?? false;
  const configuredSimilarity = caching?.semantic_similarity_threshold ?? 0;
  const configuredPromptSimilarity = caching?.semantic_prompt_similarity_min ?? 0;

  const summary = overview.data?.summary;
  const daily = overview.data?.daily ?? [];

  const submitDebug = React.useCallback(
    (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      let parsedBody: unknown;
      try {
        parsedBody = JSON.parse(bodyText);
      } catch (error) {
        setLocalDebugError(error instanceof Error ? error.message : "Invalid JSON payload");
        return;
      }
      setLocalDebugError("");
      const headers: Record<string, string> = { "X-Cache-Type": cacheType };
      if (ttl.trim() !== "") headers["X-Cache-TTL"] = ttl.trim();
      if (threshold.trim() !== "") headers["X-Cache-Semantic-Threshold"] = threshold.trim();
      if (promptSimilarity.trim() !== "") headers["X-Cache-Prompt-Similarity"] = promptSimilarity.trim();
      debugMutation.mutate({ method: "POST", path, headers, body: parsedBody });
    },
    [bodyText, cacheType, debugMutation, path, promptSimilarity, threshold, ttl],
  );

  if (!cacheEnabled) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title={t("Cache")} subtitle={t("Response cache is disabled in this gateway runtime.")} />
        <EmptyState
          title={t("Caching is disabled")}
          description={t("Enable the response cache in runtime configuration to view cache analytics and run request-level cache introspection.")}
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={t("Cache")}
        subtitle={t("Inspect response cache performance, exact vs semantic hit mix, and debug cache decisions for specific requests.")}
        actions={<DateRangeSelect range={range} />}
      />

      <Surface className="grid grid-cols-1 gap-px bg-border sm:grid-cols-2 xl:grid-cols-4">
        <div className="bg-surface p-6">
          <div className="text-[12px] font-bold uppercase tracking-wider text-muted-foreground">{t("Total hits")}</div>
          <div className="mt-4 text-[40px] font-bold tracking-tight text-foreground">{formatRequests(summary?.total_hits)}</div>
        </div>
        <div className="bg-surface p-6">
          <div className="text-[12px] font-bold uppercase tracking-wider text-muted-foreground">{t("Exact hits")}</div>
          <div className="mt-4 text-[40px] font-bold tracking-tight text-foreground">{formatRequests(summary?.exact_hits)}</div>
        </div>
        <div className="bg-surface p-6">
          <div className="text-[12px] font-bold uppercase tracking-wider text-muted-foreground">{t("Semantic hits")}</div>
          <div className="mt-4 text-[40px] font-bold tracking-tight text-foreground">{formatRequests(summary?.semantic_hits)}</div>
        </div>
        <div className="bg-surface p-6">
          <div className="text-[12px] font-bold uppercase tracking-wider text-muted-foreground">{t("Estimated savings")}</div>
          <div className="mt-4 text-[40px] font-bold tracking-tight text-success">{formatCost(summary?.total_saved_cost)}</div>
        </div>
      </Surface>

      <Surface className="p-5">
        <SectionHeader
          title={t("Semantic Cache Policy")}
          subtitle={t("A reply is replayed only when both gates pass: embedding similarity meets the vector threshold AND the new prompt's wording is close to the cached one.")}
        />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <Metric
            label={t("Semantic cache")}
            value={semanticEnabled ? t("Enabled") : t("Disabled")}
          />
          <Metric
            label={t("Vector similarity ≥")}
            value={configuredSimilarity > 0 ? configuredSimilarity.toFixed(2) : "—"}
          />
          <Metric
            label={t("Prompt similarity ≥")}
            value={configuredPromptSimilarity > 0 ? configuredPromptSimilarity.toFixed(2) : t("0.72 (default)")}
          />
        </div>
        <p className="mt-4 text-xs text-muted-foreground">{t("Tune via ")}<code>SEMANTIC_CACHE_THRESHOLD</code>{t(" and ")}<code>SEMANTIC_CACHE_PROMPT_SIMILARITY</code>{t(" in")}<code> .aurora.local</code>{t(". Raise prompt similarity toward 1.0 for stricter matching (closer to exact wording).")}</p>
      </Surface>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
        <div className="xl:col-span-2">
          <TokenUsageChart daily={[]} cacheDaily={daily} isLoading={overview.isLoading} />
        </div>
        <ThroughputChart daily={[]} cacheDaily={daily} isLoading={overview.isLoading} />
      </div>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
        <Surface className="p-5">
          <SectionHeader
            title={t("Cache Decision Debugger")}
            subtitle={t("Dry-run cache introspection for a single request. This does not read from or write to the live cache.")}
          />
          <form className="grid gap-4 md:grid-cols-2" onSubmit={submitDebug}>
            <div className="flex flex-col gap-2">
              <label htmlFor="cache-path" className="text-sm font-medium">{t("Path")}</label>
              <select id="cache-path" className="field-input" value={path} onChange={(e) => setPath(e.target.value)}>
                <option value="/v1/chat/completions">{t("/v1/chat/completions")}</option>
                <option value="/v1/responses">{t("/v1/responses")}</option>
                <option value="/v1/embeddings">{t("/v1/embeddings")}</option>
              </select>
            </div>
            <div className="flex flex-col gap-2">
              <label htmlFor="cache-type" className="text-sm font-medium">{t("Cache type")}</label>
              <select id="cache-type" className="field-input" value={cacheType} onChange={(e) => setCacheType(e.target.value)}>
                <option value="both">both</option>
                <option value="exact">exact</option>
                <option value="semantic">semantic</option>
              </select>
            </div>
            <div className="flex flex-col gap-2">
              <label htmlFor="cache-ttl" className="text-sm font-medium">{t("X-Cache-TTL")}</label>
              <input id="cache-ttl" className="field-input" placeholder={t("e.g. 3600")} value={ttl} onChange={(e) => setTTL(e.target.value)} />
            </div>
            <div className="flex flex-col gap-2">
              <label htmlFor="cache-threshold" className="text-sm font-medium">{t("X-Cache-Semantic-Threshold")}</label>
              <input id="cache-threshold" className="field-input" placeholder={t("e.g. 0.92")} value={threshold} onChange={(e) => setThreshold(e.target.value)} />
            </div>
            <div className="flex flex-col gap-2">
              <label htmlFor="cache-prompt-similarity" className="text-sm font-medium">{t("X-Cache-Prompt-Similarity")}</label>
              <input
                id="cache-prompt-similarity"
                className="field-input"
                placeholder={t("e.g. 0.72")}
                value={promptSimilarity}
                onChange={(e) => setPromptSimilarity(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2 md:col-span-2">
              <label htmlFor="cache-body" className="text-sm font-medium">{t("Request body")}</label>
              <textarea
                id="cache-body"
                className="field-input min-h-[240px] resize-y font-mono text-[13px]"
                value={bodyText}
                onChange={(e) => setBodyText(e.target.value)}
              />
            </div>
            <div className="md:col-span-2 flex items-center gap-3">
              <button
                type="submit"
                disabled={debugMutation.isPending}
                className="inline-flex items-center bg-accent px-4 py-2 text-[12px] font-bold uppercase tracking-widest text-accent-foreground transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {debugMutation.isPending ? t("Inspecting...") : t("Inspect cache decision")}
              </button>
              <Pill tone="muted">{t("Dry run only")}</Pill>
            </div>
          </form>
          {localDebugError ? (
            <div className="mt-4 border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {localDebugError}
            </div>
          ) : null}
          {debugMutation.error ? <CacheDebugErrorPanel error={debugMutation.error} /> : null}
        </Surface>

        <Surface className="p-5">
          <SectionHeader
            title={t("Debug Result")}
            subtitle={t("Effective hashes, TTLs, threshold, and cacheability for the supplied request.")}
          />
          {debugMutation.data ? (
            <div className="flex flex-col gap-4">
              <div className="flex flex-wrap gap-2">
                <Pill tone={debugMutation.data.cacheable ? "success" : "warning"}>
                  {debugMutation.data.cacheable ? t("Cacheable") : t("Not cacheable")}
                </Pill>
                <Pill tone="accent">{debugMutation.data.cache_type || "both"}</Pill>
                {debugMutation.data.streaming ? <Pill tone="warning">{t("Streaming")}</Pill> : <Pill tone="muted">{t("Non-streaming")}</Pill>}
                {debugMutation.data.miss_reason ? <Pill tone="muted">{debugMutation.data.miss_reason}</Pill> : null}
              </div>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <Metric label={t("Exact TTL")} value={debugMutation.data.exact_ttl_seconds ? `${debugMutation.data.exact_ttl_seconds}s` : "—"} />
                <Metric label={t("Semantic TTL")} value={debugMutation.data.semantic_ttl_seconds ? `${debugMutation.data.semantic_ttl_seconds}s` : "—"} />
                <Metric label={t("Semantic threshold")} value={debugMutation.data.semantic_threshold?.toString() || "—"} />
                <Metric
                  label={t("Prompt similarity min")}
                  value={debugMutation.data.prompt_similarity_threshold?.toString() || "—"}
                />
                <Metric label={t("Embedder")} value={debugMutation.data.embedder_identity || "—"} />
              </div>
              <div className="flex flex-col gap-2">
                <div className="text-sm font-medium text-foreground">{t("Exact cache key hash")}</div>
                <CodeBlock>{debugMutation.data.exact_cache_key || "—"}</CodeBlock>
              </div>
              <div className="flex flex-col gap-2">
                <div className="text-sm font-medium text-foreground">{t("Semantic params hash")}</div>
                <CodeBlock>{debugMutation.data.semantic_params_hash || "—"}</CodeBlock>
              </div>
              <div className="flex flex-col gap-2">
                <div className="text-sm font-medium text-foreground">{t("Semantic cache key hash")}</div>
                <CodeBlock>{debugMutation.data.semantic_cache_key || "—"}</CodeBlock>
              </div>
            </div>
          ) : (
            <EmptyState
              title={t("No debug result yet")}
              description={t("Run the cache debugger to inspect exact and semantic cache decisions for a specific request payload.")}
              className="mt-2"
            />
          )}
        </Surface>
      </div>

      <Surface className="p-5">
        <SectionHeader title={t("Daily Cache Activity")} subtitle={t("Exact and semantic hit counts plus cached token volume for the selected period.")} />
        <TableWrap>
          <DataTable>
            <thead>
              <tr>
                <Th>{t("Date")}</Th>
                <Th className="text-right">{t("Hits")}</Th>
                <Th className="hidden md:table-cell text-right">{t("Exact")}</Th>
                <Th className="hidden md:table-cell text-right">{t("Semantic")}</Th>
                <Th className="hidden sm:table-cell text-right">{t("Input Tokens")}</Th>
                <Th className="hidden sm:table-cell text-right">{t("Output Tokens")}</Th>
                <Th className="text-right">{t("Saved Cost")}</Th>
              </tr>
            </thead>
            <tbody>
              {daily.length === 0 ? (
                <tr>
                  <Td colSpan={7} className="py-8 text-center text-sm text-muted-foreground">{t("No cache activity found for this period.")}</Td>
                </tr>
              ) : (
                daily.map((row) => (
                  <tr key={row.date}>
                    <Td>{row.date}</Td>
                    <Td className="text-right">{formatRequests(row.hits)}</Td>
                    <Td className="hidden md:table-cell text-right">{formatRequests(row.exact_hits)}</Td>
                    <Td className="hidden md:table-cell text-right">{formatRequests(row.semantic_hits)}</Td>
                    <Td className="hidden sm:table-cell text-right">{formatTokens(row.input_tokens)}</Td>
                    <Td className="hidden sm:table-cell text-right">{formatTokens(row.output_tokens)}</Td>
                    <Td className="text-right">{formatCost(row.saved_cost)}</Td>
                  </tr>
                ))
              )}
            </tbody>
          </DataTable>
        </TableWrap>
      </Surface>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }): JSX.Element {
  return (
    <div className="border border-border/40 bg-background/40 px-4 py-3">
      <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</div>
      <div className="mt-2 text-sm font-medium text-foreground break-all">{value}</div>
    </div>
  );
}

function CacheDebugErrorPanel({ error }: { error: Error }): JSX.Element {
	const isApiError = error instanceof ApiError;
	const body = isApiError ? formatErrorBody(error.body) : "";
	return (
		<div className="mt-4 border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
			<div className="font-semibold">{error.message}</div>
			{isApiError ? (
				<div className="mt-3 grid gap-3">
					<div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
						<Metric label={t("HTTP Status")} value={String(error.status)} />
						<Metric label={t("Request URL")} value={error.url} />
					</div>
					{error.status === 404 ? (
						<div className="border border-destructive/20 bg-background/40 px-3 py-2 text-xs leading-6 text-foreground/80">{t("The UI is reaching the cache debug endpoint, but the server responded with 404. That usually means the running aurora process does not have `POST /admin/api/v1/cache/debug` mounted yet.")}</div>
					) : null}
					<div>
						<div className="mb-2 text-xs font-semibold uppercase tracking-wider text-foreground/70">{t("Response body")}</div>
						<CodeBlock className="border-destructive/20 bg-background/60 text-[12px] text-foreground">{body}</CodeBlock>
					</div>
				</div>
			) : null}
		</div>
	);
}

function formatErrorBody(body: unknown): string {
	if (typeof body === "string") {
		return body;
	}
	if (body == null) {
		return "(empty response body)";
	}
	try {
		return JSON.stringify(body, null, 2);
	} catch {
		return String(body);
	}
}

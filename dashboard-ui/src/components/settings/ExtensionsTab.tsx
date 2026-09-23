import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Surface } from "@/components/ui/surface";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ToggleField } from "@/components/ui/toggle-field";
import { Switch } from "@/components/ui/switch";
import {
  AlertTriangleIcon,
  CheckCircleIcon,
  DownloadIcon,
  GlobeIcon,
  PlugZapIcon,
  PuzzleIcon,
  RefreshCwIcon,
  SparklesIcon,
  Trash2Icon,
  UploadIcon,
  XCircleIcon,
} from "lucide-react";
import { apiFetch } from "@/lib/api/client";
import {
  applyExtension,
  deleteExtension,
  ensureExtensionHeaders,
  exportExtensionUrl,
  fetchExtensions,
  fullApplyExtension,
  importExtension,
  unapplyExtension,
  updateExtensionConfig,
  type Extension,
} from "@/lib/api/extensions";
import { SidecarPresetImportDialog } from "@/components/settings/SidecarPresetImportDialog";

const TAG_STYLES: Record<string, string> = {
  theme: "border-emerald-500/30 bg-emerald-500/10 text-emerald-500",
  sessionhub: "border-blue-500/30 bg-blue-500/10 text-blue-500",
  sidecar: "border-orange-500/30 bg-orange-500/10 text-orange-500",
  ui: "border-purple-500/30 bg-purple-500/10 text-purple-500",
  oauth: "border-teal-500/30 bg-teal-500/10 text-teal-500",
  providers: "border-indigo-500/30 bg-indigo-500/10 text-indigo-500",
  official: "border-border/60 bg-muted text-muted-foreground",
};

function tagStyle(tag: string): string {
  return TAG_STYLES[tag.toLowerCase()] ?? "border-border/50 bg-surface/60 text-muted-foreground";
}

function isThemeOnly(ext: Extension): boolean {
  if (ext.type === "theme") return true;
  return (ext.tags ?? []).some((t) => t.toLowerCase() === "theme");
}

function fieldValue(
  ext: Extension,
  key: string,
  draft: Record<string, string>,
  fallback?: string | undefined,
): string {
  if (key in draft) return draft[key] ?? "";
  return ext.config?.[key] ?? ext.settings?.[key] ?? fallback ?? "";
}

function normalizeHex(value: string): string {
  const v = value.trim();
  if (/^#[0-9a-fA-F]{6}$/.test(v)) return v;
  if (/^#[0-9a-fA-F]{3}$/.test(v)) return v;
  return "#000000";
}

interface ApplyResult {
  ok: boolean;
  message: string;
  added: string[];
  kept: string[];
}

export function ExtensionsTab(): JSX.Element {
  const qc = useQueryClient();
  const {
    data: extensions = [],
    isLoading,
    error: loadError,
  } = useQuery({
    queryKey: ["extensions"],
    queryFn: fetchExtensions,
  });

  const [selectedId, setSelectedId] = useState("");
  const [configDraft, setConfigDraft] = useState<Record<string, string>>({});
  const [target, setTarget] = useState("");
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<ApplyResult | null>(null);

  const [importOpen, setImportOpen] = useState(false);
  const [importMode, setImportMode] = useState<"json" | "url">("json");
  const [importJSON, setImportJSON] = useState("");
  const [importURL, setImportURL] = useState("");
  const [importBusy, setImportBusy] = useState(false);
  const [importError, setImportError] = useState("");

  const [storeURL, setStoreURL] = useState("");
  const [storeBusy, setStoreBusy] = useState(false);
  const [storeError, setStoreError] = useState("");
  const [storeResults, setStoreResults] = useState<
    Array<{ id: string; name: string; tagline?: string | undefined; url: string }>
  >([]);

  const selected = useMemo(
    () => extensions.find((e) => e.id === selectedId) ?? extensions[0],
    [extensions, selectedId],
  );

  useEffect(() => {
    if (!selected) {
      setConfigDraft({});
      return;
    }
    const draft: Record<string, string> = {};
    for (const f of selected.ui?.fields ?? []) {
      draft[f.key] = fieldValue(selected, f.key, {}, f.default);
    }
    setConfigDraft(draft);
    setResult(null);
  }, [selected?.id]); // eslint-disable-line react-hooks/exhaustive-deps

  const invalidateExtensionQueries = async () => {
    await qc.invalidateQueries({ queryKey: ["extensions"] });
    await qc.invalidateQueries({ queryKey: ["extensions", "ui"] });
  };

  const fields = selected?.ui?.fields ?? [];
  const themeOnly = selected ? isThemeOnly(selected) : false;
  const hasThemeUI =
    Boolean(selected?.ui?.theme) ||
    themeOnly ||
    fields.some((f) => f.key.startsWith("--") || f.type === "color");

  const setField = (key: string, value: string) => {
    setConfigDraft((prev) => ({ ...prev, [key]: value }));
  };

  const handleSaveConfig = async () => {
    if (!selected) return;
    setBusy(true);
    try {
      await updateExtensionConfig(selected.id, configDraft);
      await qc.invalidateQueries({ queryKey: ["extensions"] });
      if (hasThemeUI) {
        await qc.invalidateQueries({ queryKey: ["extensions", "ui"] });
      }
      setResult({
        ok: true,
        message: `Saved settings for "${selected.name}".`,
        added: [],
        kept: [],
      });
    } catch (err) {
      setResult({
        ok: false,
        message: `Could not save settings: ${err instanceof Error ? err.message : String(err)}`,
        added: [],
        kept: [],
      });
    } finally {
      setBusy(false);
    }
  };

  const handleApply = async () => {
    if (!selected) return;
    setBusy(true);
    const targetName = target.trim();
    try {
      const res = await applyExtension(selected.id);
      let added: string[] = [];
      let kept: string[] = [];
      if (!themeOnly && targetName && res.headers.length > 0) {
        const ensure = await ensureExtensionHeaders(targetName, res.headers);
        added = ensure.added;
        kept = ensure.already;
      }
      await invalidateExtensionQueries();
      setResult({
        ok: true,
        message: themeOnly
          ? `Applied theme "${selected.name}".`
          : targetName
            ? `Applied "${selected.name}" to ${targetName}.`
            : `Applied "${selected.name}".`,
        added,
        kept,
      });
    } catch (err) {
      setResult({
        ok: false,
        message: `Could not apply extension: ${err instanceof Error ? err.message : String(err)}`,
        added: [],
        kept: [],
      });
    } finally {
      setBusy(false);
    }
  };

  const handleFullApply = async () => {
    if (!selected) return;
    setBusy(true);
    const targetName = themeOnly ? "" : target.trim();
    try {
      const res = await fullApplyExtension(selected.id, targetName || undefined);
      let added: string[] = [];
      let kept: string[] = [];
      if (targetName && res.headers.length > 0) {
        const ensure = await ensureExtensionHeaders(targetName, res.headers);
        added = ensure.added;
        kept = ensure.already;
      }
      await invalidateExtensionQueries();
      setResult({
        ok: true,
        message: targetName
          ? `Applied "${selected.name}" everywhere (headers → ${targetName}).`
          : `Applied "${selected.name}" everywhere.`,
        added,
        kept,
      });
    } catch (err) {
      setResult({
        ok: false,
        message: `Could not full-apply extension: ${err instanceof Error ? err.message : String(err)}`,
        added: [],
        kept: [],
      });
    } finally {
      setBusy(false);
    }
  };

  const handleToggle = async (next: boolean) => {
    if (!selected) return;
    if (next) {
      await handleApply();
      return;
    }
    setBusy(true);
    try {
      await unapplyExtension(selected.id);
      await invalidateExtensionQueries();
      setResult({
        ok: true,
        message: `Disabled "${selected.name}".`,
        added: [],
        kept: [],
      });
    } catch (err) {
      setResult({
        ok: false,
        message: `Could not disable extension: ${err instanceof Error ? err.message : String(err)}`,
        added: [],
        kept: [],
      });
    } finally {
      setBusy(false);
    }
  };

  const handleImport = () => {
    setImportBusy(true);
    setImportError("");
    importExtension(importMode === "url" ? { url: importURL.trim() } : { json: importJSON })
      .then((ext) => {
        qc.invalidateQueries({ queryKey: ["extensions"] });
        setSelectedId(ext.id);
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

  const handleDelete = (ext: Extension) => {
    if (ext.builtin) return;
    deleteExtension(ext.id).then(() => {
      if (selected?.id === ext.id) setSelectedId("");
      qc.invalidateQueries({ queryKey: ["extensions"] });
      qc.invalidateQueries({ queryKey: ["extensions", "ui"] });
    });
  };

  const handleBrowseStore = () => {
    const base = storeURL.trim();
    if (!base) return;
    setStoreBusy(true);
    setStoreError("");
    setStoreResults([]);
    apiFetch<{
      extensions?: Array<{ id: string; name: string; tagline?: string; raw_url?: string }>;
    }>(`/admin/api/v1/sidecar/extensions/store/browse?url=${encodeURIComponent(base)}`, {
      method: "POST",
      json: { url: base },
    })
      .then((res) => {
        const list = res.extensions ?? [];
        setStoreResults(
          list.map((e) => ({
            id: e.id,
            name: e.name,
            ...(e.tagline != null ? { tagline: e.tagline } : {}),
            url:
              e.raw_url ??
              `${base.replace(/\/$/, "")}/api/v1/extensions/${encodeURIComponent(e.id)}/raw`,
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
        qc.invalidateQueries({ queryKey: ["extensions"] });
        setSelectedId(id);
      })
      .catch((err) => setStoreError(err instanceof Error ? err.message : "Install failed"))
      .finally(() => setStoreBusy(false));
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12 text-muted-foreground text-sm">
        Loading extensions...
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4 sm:gap-6">
      {/* Header + toolbar */}
      <Surface id="extensions-list" className="p-4 sm:p-6 scroll-mt-20">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="flex items-start gap-3">
            <div className="rounded-lg bg-primary/10 p-2">
              <PuzzleIcon className="h-5 w-5 text-primary" />
            </div>
            <div>
              <h3 className="text-base font-semibold">Extensions</h3>
              <p className="text-sm text-muted-foreground mt-1">
                Install and apply extensions — sidecar presets, themes, session hub headers, and
                UI contributions.
              </p>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" onClick={() => setImportOpen(true)} className="h-10 gap-1.5 sm:h-9">
              <UploadIcon className="h-3.5 w-3.5" />
              Import
            </Button>
            {selected ? (
              <>
                <Button variant="outline" size="sm" onClick={() => handleExport(selected)} className="h-10 gap-1.5 sm:h-9">
                  <DownloadIcon className="h-3.5 w-3.5" />
                  Export
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleDelete(selected)}
                  disabled={selected.builtin || busy}
                  className="h-10 gap-1.5 border-destructive/40 text-destructive hover:bg-destructive/10 sm:h-9"
                >
                  <Trash2Icon className="h-3.5 w-3.5" />
                  Delete
                </Button>
              </>
            ) : null}
          </div>
        </div>

        {loadError ? (
          <p className="mt-4 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
            {loadError instanceof Error ? loadError.message : "Failed to load extensions"}
          </p>
        ) : null}

        {/* Extension cards */}
        <div className="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {extensions.map((ext) => {
            const active = selected?.id === ext.id;
            return (
              <button
                key={ext.id}
                type="button"
                onClick={() => setSelectedId(ext.id)}
                className={`flex flex-col gap-2 rounded-lg border p-4 text-left transition-colors ${
                  active
                    ? "border-primary/50 bg-primary/5"
                    : "border-border/40 bg-surface hover:bg-surface-hover/30"
                }`}
              >
                <span className="flex items-start justify-between gap-2">
                  <span className="flex min-w-0 items-center gap-2">
                    {ext.ui?.accent ? (
                      <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: ext.ui.accent }} />
                    ) : (
                      <span className={`h-2.5 w-2.5 shrink-0 rounded-full ${active ? "bg-primary" : "bg-muted-foreground/30"}`} />
                    )}
                    <span className="truncate text-sm font-medium text-foreground">{ext.name}</span>
                  </span>
                  <span
                    className={`shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${
                      ext.applied
                        ? "bg-emerald-500/10 text-emerald-500"
                        : "bg-muted text-muted-foreground"
                    }`}
                  >
                    {ext.applied ? "Active" : "Inactive"}
                  </span>
                </span>
                <span className="text-xs text-muted-foreground line-clamp-2">{ext.tagline}</span>
                {(ext.tags?.length ?? 0) > 0 ? (
                  <span className="flex flex-wrap gap-1">
                    {(ext.tags ?? []).map((tag) => (
                      <span
                        key={tag}
                        className={`rounded-full border px-1.5 py-0.5 text-[10px] font-medium ${tagStyle(tag)}`}
                      >
                        {tag}
                      </span>
                    ))}
                  </span>
                ) : null}
              </button>
            );
          })}
          {extensions.length === 0 && !loadError ? (
            <div className="col-span-full rounded-lg border border-dashed border-border/60 bg-surface/50 p-6 text-center text-sm text-muted-foreground">
              No extensions installed. Import extension JSON or install from a store below.
            </div>
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
              {storeBusy ? (
                <RefreshCwIcon className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <GlobeIcon className="h-3.5 w-3.5" />
              )}
              Browse
            </Button>
          </div>
          {storeError ? <p className="mt-2 text-xs text-destructive">{storeError}</p> : null}
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

      {/* Details panel */}
      {selected ? (
        <Surface id="extensions-detail" className="p-4 sm:p-6 scroll-mt-20">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex min-w-0 items-start gap-3">
              <div className="rounded-lg bg-primary/10 p-2">
                <PlugZapIcon className="h-5 w-5 text-primary" />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="text-base font-semibold truncate">{selected.name}</h3>
                  <span
                    className={`rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${
                      selected.applied
                        ? "bg-emerald-500/10 text-emerald-500"
                        : "bg-muted text-muted-foreground"
                    }`}
                  >
                    {selected.applied ? "Active" : "Inactive"}
                  </span>
                </div>
                <p className="text-sm text-muted-foreground mt-1">{selected.tagline}</p>
              </div>
            </div>
            <div className="flex items-center gap-3 shrink-0">
              <span className="text-xs text-muted-foreground">
                {selected.applied ? "Disable" : "Enable"}
              </span>
              <Switch
                checked={Boolean(selected.applied)}
                onCheckedChange={(v) => void handleToggle(v)}
                disabled={busy}
                aria-label={`Toggle ${selected.name}`}
              />
            </div>
          </div>

          <div className="flex flex-wrap gap-1.5 mt-3">
            {(selected.tags ?? []).map((tag) => (
              <span
                key={tag}
                className={`rounded-full border px-2 py-0.5 text-[11px] font-medium ${tagStyle(tag)}`}
              >
                {tag}
              </span>
            ))}
          </div>

          {selected.description ? (
            <p className="mt-4 text-sm text-muted-foreground">{selected.description}</p>
          ) : null}
          {selected.ui?.help ? (
            <p className="mt-2 text-xs text-muted-foreground">{selected.ui.help}</p>
          ) : null}

          {(selected.requirements?.length ?? 0) > 0 ? (
            <div className="mt-3 rounded-md border border-amber-500/30 bg-amber-500/5 p-3">
              <p className="text-[11px] font-bold uppercase tracking-wider text-amber-600 dark:text-amber-400">
                Requirements
              </p>
              <ul className="mt-1 list-disc pl-4 text-xs text-muted-foreground">
                {selected.requirements?.map((req) => (
                  <li key={req}>{req}</li>
                ))}
              </ul>
            </div>
          ) : null}

          {(selected.headers?.length ?? 0) > 0 ? (
            <div className="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-2">
              {selected.headers?.map((h) => (
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

          {/* Editable ui.fields */}
          {fields.length > 0 ? (
            <div className="mt-4 rounded-lg border border-border/40 bg-background/40 p-3">
              <p className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
                Extension settings
              </p>
              <div className="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-2">
                {fields.map((field) => {
                  const value = fieldValue(selected, field.key, configDraft, field.default);
                  if (field.type === "boolean") {
                    return (
                      <ToggleField
                        key={field.key}
                        label={field.label}
                        checked={value === "true"}
                        onCheckedChange={(v) => setField(field.key, v ? "true" : "false")}
                        description={field.description}
                      />
                    );
                  }
                  if (field.type === "select" && field.options?.length) {
                    return (
                      <label key={field.key} className="flex flex-col gap-1.5">
                        <span className="text-sm font-medium">{field.label}</span>
                        <select
                          className="h-11 sm:h-9 rounded-lg border border-border/60 bg-surface px-3 text-sm text-foreground"
                          value={value || field.options[0] || ""}
                          onChange={(e) => setField(field.key, e.target.value)}
                        >
                          {field.options.map((opt) => (
                            <option key={opt} value={opt}>
                              {opt}
                            </option>
                          ))}
                        </select>
                        {field.description ? (
                          <span className="text-xs text-muted-foreground">{field.description}</span>
                        ) : null}
                      </label>
                    );
                  }
                  if (field.type === "color") {
                    return (
                      <label key={field.key} className="flex flex-col gap-1.5">
                        <span className="text-sm font-medium">{field.label}</span>
                        <div className="flex items-center gap-2">
                          <input
                            type="color"
                            value={normalizeHex(value || field.default || "#000000")}
                            onChange={(e) => setField(field.key, e.target.value)}
                            className="h-11 sm:h-9 w-12 rounded-lg border border-border/60 bg-surface p-1 cursor-pointer"
                            aria-label={`${field.label} color`}
                          />
                          <Input
                            value={value}
                            onChange={(e) => setField(field.key, e.target.value)}
                            className="h-11 sm:h-9 font-mono text-sm"
                            placeholder={field.default || "#000000"}
                          />
                        </div>
                        {field.description ? (
                          <span className="text-xs text-muted-foreground">{field.description}</span>
                        ) : null}
                      </label>
                    );
                  }
                  return (
                    <label key={field.key} className="flex flex-col gap-1.5">
                      <span className="text-sm font-medium">{field.label}</span>
                      <Input
                        type={
                          field.secret ? "password" : field.type === "number" ? "number" : "text"
                        }
                        value={value}
                        onChange={(e) => setField(field.key, e.target.value)}
                        className="h-11 sm:h-9 font-mono text-sm"
                        placeholder={field.secret ? "••••••••" : field.default}
                      />
                      {field.description ? (
                        <span className="text-xs text-muted-foreground">{field.description}</span>
                      ) : null}
                    </label>
                  );
                })}
              </div>
              <div className="mt-3 flex justify-end">
                <Button
                  size="sm"
                  onClick={handleSaveConfig}
                  disabled={busy}
                  className="h-10 gap-1.5 sm:h-9"
                >
                  {busy ? (
                    <RefreshCwIcon className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <CheckCircleIcon className="h-3.5 w-3.5" />
                  )}
                  Save settings
                </Button>
              </div>
            </div>
          ) : null}

          {/* Target + apply */}
          <div className="mt-5 flex flex-col gap-3 sm:flex-row sm:items-end">
            {!themeOnly ? (
              <div className="flex flex-1 flex-col gap-1.5">
                <label className="text-sm font-medium">Target pool or provider</label>
                <Input
                  value={target}
                  onChange={(e) => setTarget(e.target.value)}
                  className="h-11 font-mono text-sm sm:h-9 sm:max-w-xs"
                  placeholder="my-pool"
                />
                <p className="text-xs text-muted-foreground">
                  Session Hub rules install against this name (a pool applies to all members).
                  Optional for themes.
                </p>
              </div>
            ) : (
              <div className="flex flex-1 flex-col gap-1.5">
                <label className="text-sm font-medium">Theme</label>
                <p className="text-xs text-muted-foreground">
                  This extension only contributes theme / UI. Apply to activate it — no Session Hub
                  target required.
                </p>
              </div>
            )}
            <Button
              onClick={handleApply}
              disabled={busy}
              className="h-11 w-full gap-1.5 sm:h-9 sm:w-auto"
            >
              {busy ? (
                <RefreshCwIcon className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <SparklesIcon className="h-3.5 w-3.5" />
              )}
              Apply
            </Button>
            <Button
              variant="outline"
              onClick={handleFullApply}
              disabled={busy}
              className="h-11 w-full gap-1.5 sm:h-9 sm:w-auto"
            >
              <PlugZapIcon className="h-3.5 w-3.5" />
              Apply everywhere
            </Button>
          </div>

          {result ? (
            <div className="mt-3 flex items-start gap-2 rounded-md border border-border/40 bg-muted/30 p-3 text-xs">
              {result.ok ? (
                <CheckCircleIcon className="mt-0.5 h-4 w-4 shrink-0 text-success" />
              ) : (
                <XCircleIcon className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
              )}
              <div className="text-muted-foreground">
                <span className="font-medium text-foreground">{result.message}</span>
                {result.added.length > 0 ? (
                  <div className="mt-1">
                    Added:{" "}
                    {result.added.map((h) => (
                      <code key={h} className="mr-1 rounded bg-muted px-1 py-0.5 font-mono">
                        {h}
                      </code>
                    ))}
                  </div>
                ) : null}
                {result.kept.length > 0 ? (
                  <div className="mt-1">
                    Kept existing:{" "}
                    {result.kept.map((h) => (
                      <code key={h} className="mr-1 rounded bg-muted px-1 py-0.5 font-mono">
                        {h}
                      </code>
                    ))}
                  </div>
                ) : null}
              </div>
            </div>
          ) : null}

          {selected.headers?.length ? (
            <div className="mt-3 flex items-start gap-2 rounded-md bg-muted/40 p-3 text-xs text-muted-foreground">
              <AlertTriangleIcon className="h-4 w-4 shrink-0 mt-0.5" />
              <span className="flex flex-wrap items-center gap-1">
                Existing Session Hub rules are never overwritten — only missing headers are added
                when a target is set.
              </span>
            </div>
          ) : null}
        </Surface>
      ) : null}
    </div>
  );
}

import * as React from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch } from "@/lib/api/client";
import {
  applyExtension,
  extensionFileRef,
  extensionFileText,
  importExtension,
  type Extension,
  type ExtensionFile,
} from "@/lib/api/extensions";
import {
  ArrowDownIcon,
  ArrowUpIcon,
  FileJsonIcon,
  RefreshCwIcon,
  ReplaceIcon,
  SaveIcon,
  SearchIcon,
  Wand2Icon,
  XIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";

const MATCH_CAP = 5000;

export interface MatchRange {
  ranges: Array<[number, number]>;
  error?: string | undefined;
}

/** Locate every occurrence of `query` in `text` (literal or regex). */
export function findMatches(
  text: string,
  query: string,
  opts: { caseSensitive: boolean; regex: boolean },
): MatchRange {
  if (!query) return { ranges: [] };
  if (opts.regex) {
    let re: RegExp;
    try {
      re = new RegExp(query, opts.caseSensitive ? "g" : "gi");
    } catch (err) {
      return { ranges: [], error: err instanceof Error ? err.message : String(err) };
    }
    const ranges: Array<[number, number]> = [];
    let m: RegExpExecArray | null;
    while ((m = re.exec(text)) !== null) {
      if (m[0].length === 0) {
        re.lastIndex += 1;
        continue;
      }
      ranges.push([m.index, m.index + m[0].length]);
      if (ranges.length >= MATCH_CAP) break;
    }
    return { ranges };
  }
  const haystack = opts.caseSensitive ? text : text.toLowerCase();
  const needle = opts.caseSensitive ? query : query.toLowerCase();
  const ranges: Array<[number, number]> = [];
  let from = 0;
  while (ranges.length < MATCH_CAP) {
    const at = haystack.indexOf(needle, from);
    if (at === -1) break;
    ranges.push([at, at + needle.length]);
    from = at + needle.length;
  }
  return { ranges };
}

/** 1-based line/column for a character offset. */
export function lineColAt(text: string, index: number): { line: number; column: number } {
  const upto = text.slice(0, Math.max(0, Math.min(index, text.length)));
  const line = upto.split("\n").length;
  const column = upto.length - upto.lastIndexOf("\n");
  return { line, column };
}

export interface JsonValidation {
  ok: boolean;
  message?: string | undefined;
  line?: number | undefined;
  column?: number | undefined;
}

/** Parse-check JSON and surface the error position as line/column. */
export function validateJson(text: string): JsonValidation {
  try {
    JSON.parse(text);
    return { ok: true };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    const lc = /line (\d+) column (\d+)/.exec(message);
    if (lc) return { ok: false, message, line: Number(lc[1]), column: Number(lc[2]) };
    const pos = /position (\d+)/.exec(message);
    if (pos) {
      const at = lineColAt(text, Number(pos[1]));
      return { ok: false, message, line: at.line, column: at.column };
    }
    return { ok: false, message };
  }
}

interface ExtensionJsonEditorProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  extensionId: string;
  extensionName?: string | undefined;
  onSaved?: (() => void) | undefined;
  /** When set, edit only this `files` entry (e.g. an addon .go source)
   *  instead of the whole manifest. Refs are resolved by the gateway, so the
   *  file always opens with its real content. */
  fileKey?: string | null | undefined;
}

/**
 * Raw JSON editor for an installed extension: pretty-printed document with a
 * built-in find/replace bar (literal or regex, case toggle, match counter),
 * live JSON validation with line/column errors, and a format action. Save
 * re-imports the document server-side (operator config, applied state and
 * source are preserved by the import endpoint) and re-applies a currently
 * active extension so edited files/addons reload immediately.
 *
 * All colors come from theme CSS variables, so applied themes tint the editor.
 */
export function ExtensionJsonEditor({
  open,
  onOpenChange,
  extensionId,
  extensionName,
  onSaved,
  fileKey = null,
}: ExtensionJsonEditorProps): JSX.Element {
  const qc = useQueryClient();
  const taRef = React.useRef<HTMLTextAreaElement | null>(null);
  const gutterRef = React.useRef<HTMLDivElement | null>(null);
  const findRef = React.useRef<HTMLInputElement | null>(null);

  const [text, setText] = React.useState("");
  const [original, setOriginal] = React.useState("");
  const [doc, setDoc] = React.useState<Extension | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [loadError, setLoadError] = React.useState("");
  const [saving, setSaving] = React.useState(false);
  const [saveError, setSaveError] = React.useState("");

  const [findOpen, setFindOpen] = React.useState(false);
  const [find, setFind] = React.useState("");
  const [replace, setReplace] = React.useState("");
  const [caseSensitive, setCaseSensitive] = React.useState(false);
  const [useRegex, setUseRegex] = React.useState(false);
  const [matchIndex, setMatchIndex] = React.useState(0);
  /** Index to select once the (recomputed) match list settles after an edit. */
  const [pendingSelect, setPendingSelect] = React.useState<number | null>(null);
  const [cursor, setCursor] = React.useState({ line: 1, column: 1 });

  const fileMode = Boolean(fileKey);
  const validation = React.useMemo(() => {
    if (!fileMode) return validateJson(text);
    // Companion sources are arbitrary text; only .json files must parse.
    if (fileKey?.toLowerCase().endsWith(".json")) return validateJson(text);
    if (text.trim() === "") return { ok: false, message: "File is empty." };
    return { ok: true };
  }, [text, fileMode, fileKey]);
  const matchResult = React.useMemo(
    () => findMatches(text, find, { caseSensitive, regex: useRegex }),
    [text, find, caseSensitive, useRegex],
  );
  const ranges = matchResult.ranges;
  const dirty = text !== original;

  const lineCount = React.useMemo(() => text.split("\n").length, [text]);

  const updateCursor = () => {
    const ta = taRef.current;
    if (!ta) return;
    setCursor(lineColAt(ta.value, ta.selectionStart));
  };

  const selectRange = (idx: number) => {
    const ta = taRef.current;
    const range = ranges[idx];
    if (!ta || !range) return;
    setMatchIndex(idx);
    ta.focus();
    ta.setSelectionRange(range[0], range[1]);
    // wrap="off" makes one logical line one visual row, so line math is exact.
    const lineHeight = 20;
    const line = lineColAt(ta.value, range[0]).line - 1;
    const nextTop = Math.max(0, line * lineHeight - ta.clientHeight / 2);
    ta.scrollTop = nextTop;
    if (gutterRef.current) gutterRef.current.scrollTop = nextTop;
    setCursor(lineColAt(ta.value, range[0]));
  };

  // Load the current document every time the editor opens.
  React.useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setLoading(true);
    setLoadError("");
    setSaveError("");
    apiFetch<Extension>(`/admin/api/v1/sidecar/extensions/${encodeURIComponent(extensionId)}`)
      .then((ext) => {
        if (cancelled) return;
        setDoc(ext);
        let loaded = "";
        if (fileKey) {
          const entry = ext.files?.[fileKey] as ExtensionFile | undefined;
          loaded = extensionFileText(entry);
          if (loaded === "") {
            throw new Error(
              `Content of "${fileKey}" is not loaded — the gateway could not fetch its ref. ` +
                "Check the extension source, or re-import the extension.",
            );
          }
        } else {
          loaded = JSON.stringify(ext, null, 2);
        }
        setText(loaded);
        setOriginal(loaded);
        setMatchIndex(0);
        setFind("");
        setReplace("");
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoadError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, extensionId, fileKey]);

  // Keep the selected match in range and materialise deferred selections
  // (replace changes the text, so the new match list only exists after render).
  React.useEffect(() => {
    if (pendingSelect === null) return;
    if (ranges.length === 0) {
      setPendingSelect(null);
      setMatchIndex(0);
      return;
    }
    const idx = Math.min(pendingSelect, ranges.length - 1);
    selectRange(idx);
    setPendingSelect(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingSelect, ranges]);

  const gotoMatch = (idx: number) => {
    if (ranges.length === 0) return;
    const wrapped = ((idx % ranges.length) + ranges.length) % ranges.length;
    selectRange(wrapped);
  };

  const replaceCurrent = () => {
    const range = ranges[Math.min(matchIndex, ranges.length - 1)];
    if (!range) return;
    const [start, end] = range;
    setText(text.slice(0, start) + replace + text.slice(end));
    setPendingSelect(Math.min(matchIndex, ranges.length - 1));
  };

  const replaceAll = () => {
    if (ranges.length === 0) return;
    let out = "";
    let last = 0;
    for (const [start, end] of ranges) {
      out += text.slice(last, start) + replace;
      last = end;
    }
    out += text.slice(last);
    setText(out);
    setMatchIndex(0);
    setPendingSelect(0);
  };

  const formatDocument = () => {
    if (!validation.ok) return;
    const pretty = JSON.stringify(JSON.parse(text), null, 2);
    setText(pretty);
    setMatchIndex(0);
    setPendingSelect(0);
  };

  const openFind = () => {
    setFindOpen(true);
    window.requestAnimationFrame(() => findRef.current?.focus());
  };

  const handleSave = async () => {
    if (!validation.ok || loading || saving) return;
    let parsed: Extension;
    if (fileKey) {
      // File mode: patch the loaded manifest's files entry with the edited
      // content and re-import the whole document.
      if (!doc) return;
      const entry = (doc.files?.[fileKey] ?? "") as ExtensionFile;
      const ref = extensionFileRef(entry);
      const files: Record<string, ExtensionFile> = { ...doc.files };
      files[fileKey] = ref ? { ref, content: text } : text;
      parsed = { ...doc, files };
    } else {
      try {
        parsed = JSON.parse(text) as Extension;
      } catch {
        setSaveError("Document is not valid JSON.");
        return;
      }
    }
    if (parsed.id !== extensionId) {
      setSaveError(`"id" must remain "${extensionId}" — use Import to create a new extension.`);
      return;
    }
    setSaving(true);
    setSaveError("");
    try {
      await importExtension({ json: JSON.stringify(parsed) });
      if (parsed.applied) {
        try {
          await applyExtension(extensionId);
        } catch {
          // Non-fatal: the document is saved; the operator can re-apply from
          // the details panel if the runtime reload failed.
        }
      }
      await qc.invalidateQueries({ queryKey: ["extensions"] });
      await qc.invalidateQueries({ queryKey: ["extensions", "ui"] });
      onSaved?.();
      onOpenChange(false);
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const handleOpenChange = (next: boolean) => {
    if (!next && dirty) {
      const ok =
        typeof window !== "undefined" && typeof window.confirm === "function"
          ? window.confirm("Discard unsaved changes to the extension JSON?")
          : true;
      if (!ok) return;
    }
    onOpenChange(next);
  };

  const onTextareaKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    const mod = e.metaKey || e.ctrlKey;
    if (mod && e.key.toLowerCase() === "f") {
      e.preventDefault();
      openFind();
      return;
    }
    if (mod && e.key.toLowerCase() === "h") {
      e.preventDefault();
      openFind();
      return;
    }
    if (mod && e.key.toLowerCase() === "s") {
      e.preventDefault();
      void handleSave();
      return;
    }
    if (e.key === "Escape" && findOpen) {
      e.preventDefault();
      setFindOpen(false);
    }
  };

  const onFindKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      gotoMatch(e.shiftKey ? matchIndex - 1 : matchIndex + 1);
    } else if (e.key === "Escape") {
      e.preventDefault();
      setFindOpen(false);
      taRef.current?.focus();
    }
  };

  const matchLabel =
    find.trim() === ""
      ? ""
      : matchResult.error
        ? "invalid pattern"
        : ranges.length === 0
          ? "0/0"
          : `${Math.min(matchIndex + 1, ranges.length)}/${ranges.length}`;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <FileJsonIcon className="h-5 w-5 text-primary" />
            {fileMode ? `Edit ${fileKey}` : "Edit JSON"}
            {extensionName ? ` — ${extensionName}` : ""}
          </DialogTitle>
          <DialogDescription>
            {fileMode
              ? "Companion source file referenced by the extension manifest. Saving re-imports the manifest with the updated file — refs are kept so updates from source still work. Press Ctrl+S to save."
              : "Raw extension document with find and replace. Saving re-imports it — applied state, operator settings and sync source are preserved. Press Ctrl+F to search, Ctrl+H for replace, Ctrl+S to save."}
          </DialogDescription>
        </DialogHeader>

        {/* Toolbar */}
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => openFind()}
            className="h-8 gap-1.5"
            title="Find (Ctrl+F)"
          >
            <SearchIcon className="h-3.5 w-3.5" />
            Find &amp; Replace
          </Button>
          {!fileMode ? (
            <Button
              variant="outline"
              size="sm"
              onClick={formatDocument}
              disabled={!validation.ok}
              className="h-8 gap-1.5"
              title="Prettify the document"
            >
              <Wand2Icon className="h-3.5 w-3.5" />
              Format
            </Button>
          ) : null}
          <span
            className={cn(
              "rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide",
              validation.ok
                ? "bg-emerald-500/10 text-emerald-500"
                : "bg-destructive/10 text-destructive",
            )}
          >
            {validation.ok
              ? "Valid JSON"
              : `Invalid${validation.line ? ` · line ${validation.line}, col ${validation.column ?? "?"}` : ""}`}
          </span>
          {dirty ? (
            <span className="rounded-full bg-amber-500/10 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-amber-500">
              Unsaved
            </span>
          ) : null}
        </div>

        {/* Find / replace bar */}
        {findOpen ? (
          <div className="flex flex-col gap-2 rounded-lg border border-border/50 bg-background/40 p-2">
            <div className="flex flex-wrap items-center gap-2">
              <Input
                ref={findRef}
                value={find}
                onChange={(e) => {
                  setFind(e.target.value);
                  setMatchIndex(0);
                  setPendingSelect(e.target.value ? 0 : null);
                }}
                onKeyDown={onFindKeyDown}
                placeholder="Find"
                aria-label="Find"
                className="h-8 w-full min-w-0 flex-1 font-mono text-xs sm:w-56"
              />
              <span className="min-w-14 font-mono text-xs text-muted-foreground" aria-live="polite">
                {matchLabel}
              </span>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => gotoMatch(matchIndex - 1)}
                disabled={ranges.length === 0}
                aria-label="Previous match"
                title="Previous match (Shift+Enter)"
              >
                <ArrowUpIcon className="h-3.5 w-3.5" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => gotoMatch(matchIndex + 1)}
                disabled={ranges.length === 0}
                aria-label="Next match"
                title="Next match (Enter)"
              >
                <ArrowDownIcon className="h-3.5 w-3.5" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className={cn("h-8 w-8", caseSensitive && "bg-accent/15 text-accent")}
                onClick={() => {
                  setCaseSensitive((v) => !v);
                  setMatchIndex(0);
                }}
                aria-label="Match case"
                aria-pressed={caseSensitive}
                title="Match case"
              >
                <span className="text-xs font-bold">Aa</span>
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className={cn("h-8 w-8", useRegex && "bg-accent/15 text-accent")}
                onClick={() => {
                  setUseRegex((v) => !v);
                  setMatchIndex(0);
                }}
                aria-label="Regular expression"
                aria-pressed={useRegex}
                title="Regular expression"
              >
                <span className="text-xs font-bold">.*</span>
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => setFindOpen(false)}
                aria-label="Close find bar"
                title="Close (Esc)"
              >
                <XIcon className="h-3.5 w-3.5" />
              </Button>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Input
                value={replace}
                onChange={(e) => setReplace(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    replaceCurrent();
                  } else if (e.key === "Escape") {
                    e.preventDefault();
                    setFindOpen(false);
                    taRef.current?.focus();
                  }
                }}
                placeholder="Replace with"
                aria-label="Replace with"
                className="h-8 w-full min-w-0 flex-1 font-mono text-xs sm:w-56"
              />
              <Button
                variant="outline"
                size="sm"
                className="h-8 gap-1.5"
                onClick={replaceCurrent}
                disabled={ranges.length === 0}
              >
                <ReplaceIcon className="h-3.5 w-3.5" />
                Replace
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="h-8 gap-1.5"
                onClick={replaceAll}
                disabled={ranges.length === 0}
              >
                Replace all
              </Button>
            </div>
            {matchResult.error ? (
              <p className="text-xs text-destructive">{matchResult.error}</p>
            ) : null}
          </div>
        ) : null}

        {loadError ? (
          <p className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
            {loadError}
          </p>
        ) : null}

        {/* Editor */}
        <div className="flex h-[45vh] min-h-[260px] overflow-hidden rounded-lg border border-border/50 bg-background/40">
          <div
            ref={gutterRef}
            aria-hidden
            className="w-11 shrink-0 overflow-hidden border-r border-border/40 bg-surface/50 pt-2 text-right font-mono text-xs leading-5 text-muted-foreground/60 select-none"
          >
            {Array.from({ length: lineCount }, (_, i) => (
              <div key={i} className="pr-2">
                {i + 1}
              </div>
            ))}
          </div>
          <textarea
            ref={taRef}
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={onTextareaKeyDown}
            onSelect={updateCursor}
            onKeyUp={updateCursor}
            onClick={updateCursor}
            onScroll={() => {
              if (gutterRef.current && taRef.current) {
                gutterRef.current.scrollTop = taRef.current.scrollTop;
              }
            }}
            spellCheck={false}
            wrap="off"
            aria-label={fileMode ? `File ${fileKey}` : "Extension JSON"}
            placeholder={loading ? "Loading…" : "{}"}
            className="h-full min-w-0 flex-1 resize-none bg-transparent px-3 pt-2 font-mono text-xs leading-5 text-foreground caret-accent outline-none placeholder:text-muted-foreground/50"
          />
        </div>

        {/* Status bar */}
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 font-mono text-[11px] text-muted-foreground">
          <span>
            Ln {cursor.line}, Col {cursor.column} · {lineCount} lines · {text.length} chars
          </span>
          {find.trim() && !matchResult.error ? <span>{matchLabel} matches</span> : null}
        </div>

        {saveError ? (
          <p className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
            {saveError}
          </p>
        ) : null}

        <DialogFooter className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button
            variant="outline"
            onClick={() => handleOpenChange(false)}
            disabled={saving}
            className="w-full sm:w-auto"
          >
            Close
          </Button>
          <Button
            onClick={() => void handleSave()}
            disabled={!validation.ok || loading || saving || !dirty || Boolean(loadError)}
            className="w-full gap-1.5 sm:w-auto"
          >
            {saving ? (
              <RefreshCwIcon className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <SaveIcon className="h-3.5 w-3.5" />
            )}
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

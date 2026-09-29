import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { FileJsonIcon, LinkIcon, UploadIcon, RefreshCwIcon, AlertTriangleIcon, FileCodeIcon } from "lucide-react";

interface SidecarPresetImportDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mode: "json" | "url";
  setMode: (mode: "json" | "url") => void;
  jsonValue: string;
  setJSONValue: (value: string) => void;
  urlValue: string;
  setURLValue: (value: string) => void;
  onConfirm: () => void;
  pending: boolean;
  error: string;
  /** `files` entries the gateway still needs (refs it could not fetch). */
  missingFiles?: string[];
  /** Uploaded content per missing file key. */
  uploadedFiles?: Record<string, string>;
  onUploadFile?: (key: string, content: string) => void;
}

/**
 * Import a sidecar extension either by pasting its JSON or by pointing at a URL.
 * The dialog is mobile-first: full-width tabs, a scrollable textarea and a
 * stacked footer.
 */
export function SidecarPresetImportDialog({
  open,
  onOpenChange,
  mode,
  setMode,
  jsonValue,
  setJSONValue,
  urlValue,
  setURLValue,
  onConfirm,
  pending,
  error,
  missingFiles = [],
  uploadedFiles = {},
  onUploadFile,
}: SidecarPresetImportDialogProps): JSX.Element {
  const needsFiles = missingFiles.length > 0;
  const allUploaded = needsFiles && missingFiles.every((k) => Boolean(uploadedFiles[k]));

  const readFile = (key: string, file: File | undefined) => {
    if (!file || !onUploadFile) return;
    file.text().then((text) => onUploadFile(key, text));
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="mx-4 max-h-[90vh] max-w-lg overflow-y-auto sm:mx-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <UploadIcon className="h-5 w-5 text-primary" />
            Import extension
          </DialogTitle>
          <DialogDescription>
            Paste an extension JSON document or load one from a URL. Imported extensions are stored
            on the gateway and can be applied to any target.
          </DialogDescription>
        </DialogHeader>

        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => setMode("json")}
            className={`flex flex-1 items-center justify-center gap-2 rounded-lg border px-3 py-2.5 text-sm font-medium transition-colors ${
              mode === "json"
                ? "border-primary/50 bg-primary/5 text-foreground"
                : "border-border/40 text-muted-foreground hover:bg-surface-hover/30"
            }`}
          >
            <FileJsonIcon className="h-4 w-4" />
            Paste JSON
          </button>
          <button
            type="button"
            onClick={() => setMode("url")}
            className={`flex flex-1 items-center justify-center gap-2 rounded-lg border px-3 py-2.5 text-sm font-medium transition-colors ${
              mode === "url"
                ? "border-primary/50 bg-primary/5 text-foreground"
                : "border-border/40 text-muted-foreground hover:bg-surface-hover/30"
            }`}
          >
            <LinkIcon className="h-4 w-4" />
            From URL
          </button>
        </div>

        {mode === "json" ? (
          <textarea
            value={jsonValue}
            onChange={(e) => setJSONValue(e.target.value)}
            placeholder={'{\n  "id": "my-extension",\n  "name": "My Extension",\n  "base_url": "https://example.test/v1",\n  "headers": []\n}'}
            spellCheck={false}
            className="h-56 w-full resize-y rounded-lg border border-border/50 bg-background/40 p-3 font-mono text-xs text-foreground focus-visible:border-accent/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/15"
          />
        ) : (
          <div className="flex flex-col gap-2">
            <Input
              value={urlValue}
              onChange={(e) => setURLValue(e.target.value)}
              placeholder="https://example.test/my-preset.json"
              className="font-mono"
            />
            <p className="text-xs text-muted-foreground">
              The gateway fetches the URL server-side (http/https only, 1 MB limit).
            </p>
          </div>
        )}

        <div className="flex items-start gap-2 rounded-md border border-amber-500/30 bg-amber-500/5 p-3 text-xs text-muted-foreground">
          <AlertTriangleIcon className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" />
          <span>
            Only import extensions from sources you trust. Community extensions are unreviewed
            modifications and may be harmful or conflict with a provider&apos;s rules.
          </span>
        </div>

        {needsFiles ? (
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 p-3">
            <p className="flex items-center gap-2 text-sm font-medium text-foreground">
              <FileCodeIcon className="h-4 w-4 text-amber-500" />
              This extension needs {missingFiles.length} companion file
              {missingFiles.length > 1 ? "s" : ""}
            </p>
            <p className="mt-1 text-xs text-muted-foreground">
              The JSON references source files that were not provided. Upload{" "}
              {missingFiles.length > 1 ? "each file" : "the file"} to continue the import.
            </p>
            <div className="mt-3 flex flex-col gap-2">
              {missingFiles.map((key) => (
                <div
                  key={key}
                  className="flex flex-col gap-1.5 rounded-md border border-border/40 bg-background/40 px-3 py-2 sm:flex-row sm:items-center sm:justify-between"
                >
                  <span className="min-w-0 truncate font-mono text-xs text-foreground" title={key}>
                    {key}
                  </span>
                  <div className="flex items-center gap-2">
                    {uploadedFiles[key] ? (
                      <span className="text-xs text-emerald-500">uploaded</span>
                    ) : null}
                    <label className="cursor-pointer rounded-md border border-border/50 px-2.5 py-1 text-xs font-medium text-foreground transition-colors hover:bg-surface-hover">
                      Choose file
                      <input
                        type="file"
                        className="hidden"
                        aria-label={`Upload ${key}`}
                        onChange={(e) => readFile(key, e.target.files?.[0])}
                      />
                    </label>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ) : null}

        {error ? (
          <p className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
            {error}
          </p>
        ) : null}

        <DialogFooter className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button variant="outline" onClick={() => onOpenChange(false)} className="w-full sm:w-auto">
            Cancel
          </Button>
          <Button
            onClick={onConfirm}
            disabled={
              pending ||
              (needsFiles ? !allUploaded : mode === "json" ? !jsonValue.trim() : !urlValue.trim())
            }
            className="w-full gap-1.5 sm:w-auto"
          >
            {pending ? <RefreshCwIcon className="h-3.5 w-3.5 animate-spin" /> : <UploadIcon className="h-3.5 w-3.5" />}
            Import
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

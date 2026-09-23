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
import { FileJsonIcon, LinkIcon, UploadIcon, RefreshCwIcon, AlertTriangleIcon } from "lucide-react";

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
}: SidecarPresetImportDialogProps): JSX.Element {
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
            Only import extensions from sources you trust. Third-party extensions are unreviewed
            modifications and may be harmful or violate a provider&apos;s rules.
          </span>
        </div>

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
            disabled={pending || (mode === "json" ? !jsonValue.trim() : !urlValue.trim())}
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

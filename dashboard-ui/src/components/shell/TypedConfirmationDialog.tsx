import * as React from "react";
import { AlertTriangle } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

/**
 * Typed-confirmation dialog ("Type RESET to continue").
 * Functional port of internal/admin/dashboard/templates/typed-confirmation-dialog.html.
 *
 * Confirm button stays disabled until the input matches `requiredText`
 * (case-insensitive, trimmed). The destructive action only fires when the
 * button is clicked, never on Enter inside the input.
 */
export interface TypedConfirmationDialogProps {
  open: boolean;
  title: string;
  description?: React.ReactNode;
  requiredText: string;
  confirmLabel?: string;
  loading?: boolean;
  errorMessage?: string;
  onConfirm: () => void;
  onOpenChange: (open: boolean) => void;
}

export function TypedConfirmationDialog({
  open,
  title,
  description,
  requiredText,
  confirmLabel = "Confirm",
  loading = false,
  errorMessage,
  onConfirm,
  onOpenChange,
}: TypedConfirmationDialogProps): JSX.Element {
  const [value, setValue] = React.useState("");

  React.useEffect(() => {
    if (open) setValue("");
  }, [open]);

  const matches =
    value.trim().toLocaleLowerCase() ===
    requiredText.trim().toLocaleLowerCase();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <div className="flex items-start gap-3">
            <span className="grid h-9 w-9 shrink-0 place-items-center border border-warning/30 bg-warning/10 text-warning">
              <AlertTriangle className="h-4 w-4" aria-hidden />
            </span>
            <div className="flex-1">
              <DialogTitle>{title}</DialogTitle>
              {description ? (
                <DialogDescription className="mt-1">
                  {description}
                </DialogDescription>
              ) : null}
            </div>
          </div>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (matches && !loading) onConfirm();
          }}
          className="space-y-3"
        >
          <p className="text-xs text-muted-foreground">
            Type{" "}
            <code className="rounded bg-surface-hover px-1.5 py-0.5 font-mono text-foreground">
              {requiredText}
            </code>{" "}
            to confirm.
          </p>
          <Input
            value={value}
            onChange={(e) => setValue(e.target.value)}
            autoFocus
            autoComplete="off"
            spellCheck={false}
          />
          {errorMessage ? (
            <p className="text-xs text-destructive" role="alert">
              {errorMessage}
            </p>
          ) : null}
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => onOpenChange(false)}
              disabled={loading}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              variant="destructive"
              disabled={!matches || loading}
            >
              {loading ? "Working…" : confirmLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

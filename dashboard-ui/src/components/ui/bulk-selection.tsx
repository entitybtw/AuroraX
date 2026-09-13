import { Button } from "@/components/ui/button";
import { SquareIcon, CheckSquareIcon, MinusIcon } from "lucide-react";

export function SelectCheckbox({ checked, indeterminate, onClick, title }: {
  checked: boolean;
  indeterminate?: boolean;
  onClick: () => void;
  title?: string;
}) {
  return (
    <button onClick={onClick} className="shrink-0 p-0.5 hover:bg-border/20 transition-colors" title={title}>
      {checked ? (
        <CheckSquareIcon className="h-4 w-4 text-accent" />
      ) : indeterminate ? (
        <div className="h-4 w-4 border-2 border-accent bg-accent/20 flex items-center justify-center"><MinusIcon className="h-2.5 w-2.5 text-accent" /></div>
      ) : (
        <SquareIcon className="h-4 w-4 text-muted-foreground" />
      )}
    </button>
  );
}

export function BulkActionsBar({ count, onClear, children }: {
  count: number;
  onClear: () => void;
  children: React.ReactNode;
}) {
  if (count === 0) return null;
  return (
    <div className="border border-accent/30 bg-accent/5 px-3 sm:px-4 py-2.5">
      <div className="flex items-center gap-3 mb-2">
        <span className="text-[12px] font-medium text-accent">{count} selected</span>
        <div className="ml-auto">
          <Button size="sm" variant="ghost" onClick={onClear} className="text-[11px] h-7">Clear</Button>
        </div>
      </div>
      <div className="flex items-center gap-1.5 overflow-x-auto pb-1 -mb-1">
        {children}
      </div>
    </div>
  );
}

import * as React from "react";
import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { CalendarIcon, ChevronDownIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import type { DateRangePreset, UseDateRangeResult } from "@/lib/date-picker/useDateRange";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export interface DateRangeSelectProps {
  range: UseDateRangeResult;
  onChange?: () => void;
}

const PRESETS: ReadonlyArray<{ value: Exclude<DateRangePreset, "custom">; label: string }> = [
  { value: "7d", label: "7 days" },
  { value: "30d", label: "30 days" },
  { value: "90d", label: "90 days" },
];

export function DateRangeSelect({ range, onChange }: DateRangeSelectProps): JSX.Element {
  const [isOpen, setIsOpen] = React.useState(false);
  const [customStart, setCustomStart] = React.useState(range.startDate);
  const [customEnd, setCustomEnd] = React.useState(range.endDate);

  // Sync internal state when range changes externally
  React.useEffect(() => {
    if (range.preset !== "custom") {
      setCustomStart(range.startDate);
      setCustomEnd(range.endDate);
    }
  }, [range.preset, range.startDate, range.endDate]);

  const handleApplyCustom = () => {
    range.setCustom(customStart, customEnd);
    if (onChange) onChange();
    setIsOpen(false);
  };

  const activeLabel = range.preset === "custom"
    ? `${range.startDate} → ${range.endDate}`
    : PRESETS.find(p => p.value === range.preset)!.label;

  return (
    <DropdownMenu.Root open={isOpen} onOpenChange={setIsOpen}>
      <DropdownMenu.Trigger asChild>
        <Button variant="outline" size="sm" className="h-10 px-3.5 bg-surface/50 border-border/60 hover:bg-surface-hover/80 transition-all">
          <CalendarIcon className="mr-2 h-4 w-4 text-muted-foreground" />
          <span className="font-semibold">{activeLabel}</span>
          <ChevronDownIcon className="ml-2 h-3.5 w-3.5 text-muted-foreground opacity-70" />
        </Button>
      </DropdownMenu.Trigger>

      <DropdownMenu.Portal>
        <DropdownMenu.Content
          align="end"
          sideOffset={8}
          className="z-50 w-72  glass p-3.5 shadow-2xl animate-in fade-in-0 zoom-in-95 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95"
        >
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <span className="px-1 text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Presets</span>
              <div className="grid grid-cols-3 gap-1.5">
                {PRESETS.map((opt) => {
                  const active = range.preset === opt.value;
                  return (
                    <button
                      key={opt.value}
                      onClick={() => {
                        range.setPreset(opt.value);
                        if (onChange) onChange();
                        setIsOpen(false);
                      }}
                      className={cn(
                        "px-2 py-1.5 text-[12px] font-semibold transition-all duration-200",
                        active
                          ? "bg-accent/15 text-accent border border-accent/30"
                          : "bg-background/50 text-muted-foreground hover:bg-surface-hover/80 hover:text-foreground border border-border/40"
                      )}
                    >
                      {opt.label}
                    </button>
                  );
                })}
              </div>
            </div>

            <div className="my-0.5 h-px w-full bg-border/40" />

            <div className="flex flex-col gap-2">
              <span className="px-1 text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Custom Range</span>
              <div className="flex flex-col gap-3">
                <div className="grid grid-cols-2 gap-2">
                  <div className="flex flex-col gap-1.5">
                    <label className="text-[11px] font-medium text-muted-foreground">Start Date</label>
                    <Input
                      type="date"
                      value={customStart}
                      onChange={(e) => setCustomStart(e.target.value)}
                      max={customEnd}
                      className="h-9 px-2 text-[12px]"
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <label className="text-[11px] font-medium text-muted-foreground">End Date</label>
                    <Input
                      type="date"
                      value={customEnd}
                      onChange={(e) => setCustomEnd(e.target.value)}
                      min={customStart}
                      className="h-9 px-2 text-[12px]"
                    />
                  </div>
                </div>
                <Button
                  onClick={handleApplyCustom}
                  disabled={!customStart || !customEnd || customStart > customEnd}
                  className="w-full h-9 text-[13px] mt-1"
                >
                  Apply Custom Range
                </Button>
              </div>
            </div>
          </div>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}

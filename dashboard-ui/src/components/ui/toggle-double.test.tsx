import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToggleField } from "@/components/ui/toggle-field";

/**
 * The switch must fire onCheckedChange exactly once per click — a double-fire
 * here would flip visibility toggles back and forth in one gesture.
 *
 * Note: label→control activation (clicking the row text) is a browser feature
 * happy-dom does not implement, so only direct switch clicks are asserted.
 */
describe("ToggleField click handling", () => {
  it("switch click fires exactly once", () => {
    const onCheckedChange = vi.fn();
    render(<ToggleField label="Visible" checked onCheckedChange={onCheckedChange} aria-label="Toggle X" />);
    fireEvent.click(screen.getByRole("switch"));
    expect(onCheckedChange).toHaveBeenCalledTimes(1);
    expect(onCheckedChange).toHaveBeenCalledWith(false);
  });

  it("does not attach a second handler to the switch", () => {
    const onCheckedChange = vi.fn();
    const { rerender } = render(
      <ToggleField label="Visible" checked onCheckedChange={onCheckedChange} aria-label="Toggle X" />,
    );
    rerender(<ToggleField label="Visible" checked onCheckedChange={onCheckedChange} aria-label="Toggle X" />);
    fireEvent.click(screen.getByRole("switch"));
    expect(onCheckedChange).toHaveBeenCalledTimes(1);
  });
});

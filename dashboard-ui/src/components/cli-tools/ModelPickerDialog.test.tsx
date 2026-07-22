import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ModelPickerDialog } from "./ModelPickerDialog";
import type { ModelInventoryItem } from "@/lib/api/models-types";

const models: ModelInventoryItem[] = [
  { provider_name: "anthropic", provider_type: "anthropic", model: { id: "claude-sonnet" } },
  { provider_name: "openai", provider_type: "openai", model: { id: "gpt-4o" } },
];

describe("ModelPickerDialog", () => {
  it("renders grouped models and selects a model", () => {
    const onSelect = vi.fn();
    const onClose = vi.fn();

    render(
      <ModelPickerDialog
        open
        title="Select model"
        selectedModel=""
        models={models}
        onSelect={onSelect}
        onClose={onClose}
      />,
    );

    expect(screen.getByText("anthropic")).toBeInTheDocument();
    fireEvent.click(screen.getByText("claude-sonnet"));

    expect(onSelect).toHaveBeenCalledWith("anthropic/claude-sonnet");
    expect(onClose).toHaveBeenCalled();
  });

  it("filters by search query", () => {
    render(
      <ModelPickerDialog
        open
        title="Select model"
        selectedModel=""
        models={models}
        onSelect={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByPlaceholderText("Search provider or model..."), {
      target: { value: "gpt" },
    });

    expect(screen.getByText("gpt-4o")).toBeInTheDocument();
    expect(screen.queryByText("claude-sonnet")).not.toBeInTheDocument();
  });
});

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CLIModelFieldGrid } from "./CLIModelFieldGrid";
import type { CLIModelField } from "@/lib/api/cli-tools";
import type { ModelInventoryItem } from "@/lib/api/models-types";

const fields: CLIModelField[] = [
  { key: "ANTHROPIC_DEFAULT_HAIKU_MODEL", label: "Haiku default", description: "Fast model" },
  { key: "ANTHROPIC_DEFAULT_SONNET_MODEL", label: "Sonnet default" },
];

const models: ModelInventoryItem[] = [
  { provider_name: "anthropic", provider_type: "anthropic", model: { id: "claude-haiku" } },
];

describe("CLIModelFieldGrid", () => {
  it("renders one control per model field and supports manual edits", () => {
    const onChange = vi.fn();

    render(
      <CLIModelFieldGrid
        modelFields={fields}
        modelOverrides={{}}
        fallbackModel="fallback/model"
        models={models}
        onChange={onChange}
        selectedModels={undefined}
        onSelectedModelsChange={undefined}
      />,
    );

    fireEvent.change(screen.getByLabelText("Haiku default"), {
      target: { value: "anthropic/claude-haiku" },
    });

    expect(screen.getByText("Sonnet default")).toBeInTheDocument();
    expect(onChange).toHaveBeenCalledWith("ANTHROPIC_DEFAULT_HAIKU_MODEL", "anthropic/claude-haiku");
  });

  it("selects an inventory model from the dropdown for a field", () => {
    const onChange = vi.fn();

    render(
      <CLIModelFieldGrid
        modelFields={fields}
        modelOverrides={{}}
        fallbackModel="fallback/model"
        models={models}
        onChange={onChange}
        selectedModels={undefined}
        onSelectedModelsChange={undefined}
      />,
    );

    fireEvent.change(screen.getByLabelText("Haiku default"), {
      target: { value: "anthropic/claude-haiku" },
    });

    expect(onChange).toHaveBeenCalledWith("ANTHROPIC_DEFAULT_HAIKU_MODEL", "anthropic/claude-haiku");
  });

  it("clears populated overrides with the fallback dropdown option", () => {
    const onChange = vi.fn();

    render(
      <CLIModelFieldGrid
        modelFields={fields}
        modelOverrides={{ ANTHROPIC_DEFAULT_HAIKU_MODEL: "anthropic/claude-haiku" }}
        fallbackModel="fallback/model"
        models={models}
        onChange={onChange}
        selectedModels={undefined}
        onSelectedModelsChange={undefined}
      />,
    );

    fireEvent.change(screen.getByLabelText("Haiku default"), {
      target: { value: "" },
    });

    expect(onChange).toHaveBeenCalledWith("ANTHROPIC_DEFAULT_HAIKU_MODEL", "");
  });
});
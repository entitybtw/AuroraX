import type { CLIPreviewRequest } from "@/lib/api/cli-tools";

export interface CLIModelFormState {
  base_url: string;
  api_key: string;
  model: string;
  model_overrides: Record<string, string>;
  selectedModels?: string[];
}

export function compactModelOverrides(
  overrides: Readonly<Record<string, string>>,
): Record<string, string> | undefined {
  const compacted = Object.entries(overrides).reduce<Record<string, string>>(
    (accumulator, [key, value]) => {
      const trimmedKey = key.trim();
      const trimmedValue = value.trim();
      if (!trimmedKey || !trimmedValue) {
        return accumulator;
      }
      return { ...accumulator, [trimmedKey]: trimmedValue };
    },
    {},
  );

  return Object.keys(compacted).length > 0 ? compacted : undefined;
}

export function buildCLIPreviewRequest(
  form: Readonly<CLIModelFormState>,
): CLIPreviewRequest {
  const modelOverrides = compactModelOverrides(form.model_overrides);
  return {
    base_url: form.base_url,
    api_key: form.api_key,
    model: form.model,
    ...(modelOverrides ? { model_overrides: modelOverrides } : {}),
    ...(form.selectedModels && form.selectedModels.length > 0 ? { models: form.selectedModels } : {}),
  };
}

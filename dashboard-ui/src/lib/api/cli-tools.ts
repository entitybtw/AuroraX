import { z } from "zod";
import { apiFetch } from "./client";

const CLIModelFieldSchema = z.object({
  key: z.string(),
  label: z.string(),
  description: z.string().optional(),
  default_model: z.string().optional(),
  multi: z.boolean().optional(),
});

const CLIToolSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  config_path: z.string().optional(),
  can_apply: z.boolean(),
  config_type: z.string().optional(),
  color: z.string().optional(),
  docs_url: z.string().optional(),
  default_command: z.string().optional(),
  notes: z.array(z.string()).optional(),
  model_fields: z.array(CLIModelFieldSchema).optional(),
});
const CLIToolPresetSchema = z.object({
  id: z.string(),
  label: z.string(),
  description: z.string().optional(),
  tool_id: z.string(),
  base_url: z.string().optional(),
  model: z.string().optional(),
  model_overrides: z.record(z.string()).optional(),
  models: z.array(z.string()).optional(),
  api_key_placeholder: z.string().optional(),
});
const CLIToolsResponseSchema = z.object({
  tools: z.array(CLIToolSchema),
  presets: z.array(CLIToolPresetSchema).optional(),
});
const CLIPreviewResponseSchema = z.object({
  tool: CLIToolSchema,
  snippets: z.record(z.string()),
  masked_key: z.string().optional(),
});
const CLIApplyResponseSchema = z.object({
  applied: z.boolean(),
  path: z.string(),
  backup_path: z.string().optional(),
});

export type CLIModelField = z.infer<typeof CLIModelFieldSchema>;
export type CLITool = z.infer<typeof CLIToolSchema>;
export type CLIToolPreset = z.infer<typeof CLIToolPresetSchema>;
export interface CLIPreviewRequest {
  base_url: string;
  api_key: string;
  model: string;
  model_overrides?: Record<string, string>;
  models?: string[];
}
export type CLIPreviewResponse = z.infer<typeof CLIPreviewResponseSchema>;
export type CLIApplyResponse = z.infer<typeof CLIApplyResponseSchema>;

export async function fetchCLITools(): Promise<{ tools: CLITool[]; presets: CLIToolPreset[] }> {
  const data = await apiFetch<z.infer<typeof CLIToolsResponseSchema>>("/admin/api/v1/cli-tools", { schema: CLIToolsResponseSchema });
  return { tools: data.tools, presets: data.presets ?? [] };
}

export function previewCLITool(tool: string, payload: CLIPreviewRequest): Promise<CLIPreviewResponse> {
  return apiFetch<CLIPreviewResponse>(`/admin/api/v1/cli-tools/${encodeURIComponent(tool)}/preview`, { method: "POST", json: payload, schema: CLIPreviewResponseSchema });
}

export function applyCLITool(tool: string, payload: CLIPreviewRequest): Promise<CLIApplyResponse> {
  return apiFetch<CLIApplyResponse>(`/admin/api/v1/cli-tools/${encodeURIComponent(tool)}/apply`, { method: "POST", json: payload, schema: CLIApplyResponseSchema });
}

/** Restores the tool config from the `.aurora.bak` written on the last apply. */
export function resetCLITool(tool: string): Promise<CLIApplyResponse> {
  return apiFetch<CLIApplyResponse>(`/admin/api/v1/cli-tools/${encodeURIComponent(tool)}/reset`, { method: "POST", schema: CLIApplyResponseSchema });
}

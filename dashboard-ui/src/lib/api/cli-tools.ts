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
const CLIToolsResponseSchema = z.object({ tools: z.array(CLIToolSchema) });
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
export interface CLIPreviewRequest {
  base_url: string;
  api_key: string;
  model: string;
  model_overrides?: Record<string, string>;
  models?: string[];
}
export type CLIPreviewResponse = z.infer<typeof CLIPreviewResponseSchema>;
export type CLIApplyResponse = z.infer<typeof CLIApplyResponseSchema>;

export async function fetchCLITools(): Promise<CLITool[]> {
  const data = await apiFetch<z.infer<typeof CLIToolsResponseSchema>>("/admin/api/v1/cli-tools", { schema: CLIToolsResponseSchema });
  return data.tools;
}

export function previewCLITool(tool: string, payload: CLIPreviewRequest): Promise<CLIPreviewResponse> {
  return apiFetch<CLIPreviewResponse>(`/admin/api/v1/cli-tools/${encodeURIComponent(tool)}/preview`, { method: "POST", json: payload, schema: CLIPreviewResponseSchema });
}

export function applyCLITool(tool: string, payload: CLIPreviewRequest): Promise<CLIApplyResponse> {
  return apiFetch<CLIApplyResponse>(`/admin/api/v1/cli-tools/${encodeURIComponent(tool)}/apply`, { method: "POST", json: payload, schema: CLIApplyResponseSchema });
}

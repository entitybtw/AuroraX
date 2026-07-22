import { z } from "zod";
import { apiFetch } from "./client";

const ConsoleEventSchema = z.object({
  id: z.string(),
  time: z.string(),
  level: z.string(),
  kind: z.string(),
  message: z.string(),
  request_id: z.string().optional(),
  method: z.string().optional(),
  path: z.string().optional(),
  status: z.number().optional(),
  model: z.string().optional(),
  provider: z.string().optional(),
  duration_ms: z.number().optional(),
  fallback: z.object({ target_model: z.string().optional() }).optional(),
});
const ConsoleRecentResponseSchema = z.object({
  events: z.array(ConsoleEventSchema),
  total: z.number().int().optional().default(0),
});

export type ConsoleEvent = z.infer<typeof ConsoleEventSchema>;

export interface ConsolePage {
  events: ConsoleEvent[];
  total: number;
}

export async function fetchConsoleRecent(limit = 200, offset = 0): Promise<ConsolePage> {
  const data = await apiFetch<z.infer<typeof ConsoleRecentResponseSchema>>("/admin/api/v1/console/recent", {
    query: { limit, offset },
    schema: ConsoleRecentResponseSchema,
  });
  return { events: data.events, total: data.total };
}

import { z } from "zod";

export const AuditWorkflowFeaturesSchema = z.object({
  cache: z.boolean().optional().default(false),
  audit: z.boolean().optional().default(false),
  usage: z.boolean().optional().default(false),
  budget: z.boolean().optional().default(false),
  guardrails: z.boolean().optional().default(false),
  fallback: z.boolean().optional().default(false),
}).passthrough();
export type AuditWorkflowFeatures = z.infer<typeof AuditWorkflowFeaturesSchema>;

export const AuditFailoverSnapshotSchema = z.object({
  used: z.boolean().optional().default(false),
  target_model: z.string().optional().default(""),
}).passthrough();
export type AuditFailoverSnapshot = z.infer<typeof AuditFailoverSnapshotSchema>;

export const AuditUsageSummarySchema = z.object({
  input_tokens: z.number().optional(),
  output_tokens: z.number().optional(),
  total_tokens: z.number().optional(),
  total_input_tokens: z.number().optional(), // Legacy support if needed
  total_output_tokens: z.number().optional(), // Legacy support if needed
  total_cost: z.number().nullable().optional(),
  total_input_cost: z.number().nullable().optional(),
  total_output_cost: z.number().nullable().optional(),
  total_cache_read_input_tokens: z.number().optional(),
  total_cache_creation_input_tokens: z.number().optional(),
  total_cached_input_cost: z.number().nullable().optional(),
}).passthrough();
export type AuditUsageSummary = z.infer<typeof AuditUsageSummarySchema>;

export const AuditEntryDataSchema = z.object({
  user_agent: z.string().optional().default(""),
  workflow_features: AuditWorkflowFeaturesSchema.optional(),
  failover: AuditFailoverSnapshotSchema.optional(),
  temperature: z.number().nullable().optional(),
  max_tokens: z.number().nullable().optional(),
  error_message: z.string().optional().default(""),
  error_code: z.string().optional().default(""),
  request_headers: z.record(z.string()).optional(),
  response_headers: z.record(z.string()).optional(),
  request_body: z.unknown().optional(),
  response_body: z.unknown().optional(),
  request_body_too_big_to_handle: z.boolean().optional().default(false),
  response_body_too_big_to_handle: z.boolean().optional().default(false),
}).passthrough();
export type AuditEntryData = z.infer<typeof AuditEntryDataSchema>;

export const AuditEntrySchema = z.object({
  id: z.string(),
  timestamp: z.string().optional(),
  method: z.string().optional().default(""),
  path: z.string().optional().default(""),
  status_code: z.number().optional(),
  duration_ns: z.number().optional(),
  model: z.string().optional().default(""),
  requested_model: z.string().optional().default(""),
  resolved_model: z.string().optional().default(""),
  provider: z.string().optional().default(""),
  provider_name: z.string().optional().default(""),
  user_path: z.string().optional().default(""),
  request_id: z.string().optional().default(""),
  client_ip: z.string().optional().default(""),
  auth_key_id: z.string().optional().default(""),
  auth_method: z.string().optional().default(""),
  alias_used: z.boolean().optional().default(false),
  workflow_version_id: z.string().optional().default(""),
  cache_type: z.string().optional().default(""),
  stream: z.boolean().optional().default(false),
  error_type: z.string().optional().default(""),
  cache_hit: z.boolean().optional().default(false),
  cache_mode: z.string().optional().default(""),
  failover_target: z.string().optional().default(""),
  request_body: z.string().optional(),
  response_body: z.string().optional(),
  data: AuditEntryDataSchema.optional(),
  usage: AuditUsageSummarySchema.optional(),
  // Some legacy responses keep these flat; tolerate both shapes.
  input_tokens: z.number().optional(),
  output_tokens: z.number().optional(),
  total_tokens: z.number().optional(),
  input_cost: z.number().nullable().optional(),
  output_cost: z.number().nullable().optional(),
  total_cost: z.number().nullable().optional(),
  costs_calculation_caveat: z.string().optional().default(""),
}).passthrough();
export type AuditEntry = z.infer<typeof AuditEntrySchema>;

export const AuditLogStatsBucketSchema = z.object({
  label: z.string(),
  count: z.number().int().nonnegative(),
}).passthrough();
export type AuditLogStatsBucket = z.infer<typeof AuditLogStatsBucketSchema>;

export const AuditLogStatsSchema = z.object({
  visible: z.number().int().nonnegative().optional().default(0),
  status: z.array(AuditLogStatsBucketSchema).optional().default([]),
  methods: z.array(AuditLogStatsBucketSchema).optional().default([]),
  providers: z.array(AuditLogStatsBucketSchema).optional().default([]),
  models: z.array(AuditLogStatsBucketSchema).optional().default([]),
  errors: z.array(AuditLogStatsBucketSchema).optional().default([]),
  auth_methods: z.array(AuditLogStatsBucketSchema).optional().default([]),
  error_count: z.number().int().nonnegative().optional().default(0),
  stream_count: z.number().int().nonnegative().optional().default(0),
  failover_count: z.number().int().nonnegative().optional().default(0),
  total_duration_ms: z.number().nonnegative().optional().default(0),
}).passthrough();
export type AuditLogStats = z.infer<typeof AuditLogStatsSchema>;

export const AuditLogPageSchema = z.object({
  entries: z.array(AuditEntrySchema),
  total: z.number(),
  offset: z.number(),
  limit: z.number(),
  stats: AuditLogStatsSchema.optional(),
}).passthrough();
export type AuditLogPage = z.infer<typeof AuditLogPageSchema>;

export interface AuditQueryParams {
  start_date?: string;
  end_date?: string;
  search?: string;
  method?: string;
  status_code?: string;
  stream?: string;
  requested_model?: string;
  provider?: string;
  path?: string;
  user_path?: string;
  tenant?: string;
  error_type?: string;
  sort?: string;
  offset?: number;
  limit?: number;
  include_stats?: string;
}

export const ConversationMessageSchema = z.object({
  uid: z.string().optional(),
  role: z.string(),
  roleLabel: z.string().optional(),
  roleClass: z.string().optional(),
  text: z.string().optional(),
  timestamp: z.string().optional(),
  isAnchor: z.boolean().optional(),
}).passthrough();
export type ConversationMessage = z.infer<typeof ConversationMessageSchema>;

export const ConversationResponseSchema = z.object({
  anchor_id: z.string().optional().default(""),
  entries: z.array(AuditEntrySchema).optional().default([]),
  messages: z.array(ConversationMessageSchema).optional().default([]),
}).passthrough();
export type ConversationResponse = z.infer<typeof ConversationResponseSchema>;

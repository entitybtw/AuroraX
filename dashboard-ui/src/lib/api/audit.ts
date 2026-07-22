import { apiFetch } from "./client";
import {
  AuditLogPageSchema,
  ConversationResponseSchema,
  type AuditLogPage,
  type AuditQueryParams,
  type ConversationResponse,
} from "./audit-types";

export function fetchAuditLog(params: AuditQueryParams): Promise<AuditLogPage> {
  const query: Record<string, string | number | boolean | undefined | null> = {};
  if (params.start_date) query.start_date = params.start_date;
  if (params.end_date) query.end_date = params.end_date;
  if (params.search) query.search = params.search;
  if (params.method) query.method = params.method;
  if (params.status_code) query.status_code = params.status_code;
  if (params.stream) query.stream = params.stream;
  if (params.requested_model) query.requested_model = params.requested_model;
  if (params.provider) query.provider = params.provider;
  if (params.path) query.path = params.path;
  if (params.user_path) query.user_path = params.user_path;
  if (params.tenant) query.tenant = params.tenant;
  if (params.error_type) query.error_type = params.error_type;
  if (params.sort) query.sort = params.sort;
  if (params.offset !== undefined) query.offset = params.offset;
  if (params.limit !== undefined) query.limit = params.limit;
  if (params.include_stats) query.include_stats = params.include_stats;
  return apiFetch("/admin/api/v1/audit/log", {
    query,
    schema: AuditLogPageSchema,
  }) as Promise<AuditLogPage>;
}

export function exportAuditLogCsv(params: Omit<AuditQueryParams, "offset" | "limit">): Promise<string> {
  const query: Record<string, string | number | boolean | undefined | null> = { format: "csv" };
  if (params.start_date) query.start_date = params.start_date;
  if (params.end_date) query.end_date = params.end_date;
  if (params.search) query.search = params.search;
  if (params.method) query.method = params.method;
  if (params.status_code) query.status_code = params.status_code;
  if (params.stream) query.stream = params.stream;
  if (params.requested_model) query.requested_model = params.requested_model;
  if (params.provider) query.provider = params.provider;
  if (params.path) query.path = params.path;
  if (params.user_path) query.user_path = params.user_path;
  if (params.tenant) query.tenant = params.tenant;
  if (params.error_type) query.error_type = params.error_type;
  if (params.sort) query.sort = params.sort;
  return apiFetch("/admin/api/v1/audit/log/export", {
    query,
  }) as Promise<string>;
}

export function fetchConversation(entryId: string): Promise<ConversationResponse> {
  return apiFetch(`/admin/api/v1/audit/conversation`, {
    query: { log_id: entryId },
    schema: ConversationResponseSchema,
  }) as Promise<ConversationResponse>;
}

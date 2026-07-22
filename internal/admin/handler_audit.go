package admin

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"aurora/internal/audit_logging"
	"aurora/internal/core"
	"aurora/internal/usage"
)

// maxAuditLogLimit caps the page size accepted by the audit log endpoint and
// matches the value documented in the @Param limit annotation below.
const maxAuditLogLimit = 100
const maxAuditLogExportPageSize = 100

var allowedAuditSortFields = map[string]struct{}{
	"timestamp":           {},
	"duration_ns":         {},
	"status_code":         {},
	"requested_model":     {},
	"resolved_model":      {},
	"provider":            {},
	"provider_name":       {},
	"method":              {},
	"path":                {},
	"user_path":           {},
	"error_type":          {},
	"request_id":          {},
	"auth_key_id":         {},
	"auth_method":         {},
	"stream":              {},
	"cache_type":          {},
	"workflow_version_id": {},
}

// AuditLog handles GET /admin/api/v1/audit/log
//
// @Summary      Get paginated audit log entries
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        days         query     int     false  "Number of days (default 30)"
// @Param        start_date   query     string  false  "Start date (YYYY-MM-DD)"
// @Param        end_date     query     string  false  "End date (YYYY-MM-DD)"
// @Param        requested_model  query     string  false  "Filter by requested model selector"
// @Param        provider     query     string  false  "Filter by provider name or provider type"
// @Param        method       query     string  false  "Filter by HTTP method"
// @Param        path         query     string  false  "Filter by request path"
// @Param        user_path    query     string  false  "Filter by tracked user path subtree"
// @Param        error_type   query     string  false  "Filter by error type"
// @Param        status_code  query     int     false  "Filter by status code"
// @Param        stream       query     bool    false  "Filter by stream mode (true/false)"
// @Param        search       query     string  false  "Search across request_id/requested_model/provider/method/path/error_type/error_message"
// @Param        limit        query     int     false  "Page size (default 25, max 100)"
// @Param        offset       query     int     false  "Offset for pagination"
// @Success      200  {object}  auditLogListResponse
// @Failure      400  {object}  core.GatewayError
// @Failure      401  {object}  core.GatewayError
// @Router       /admin/api/v1/audit/log [get]
func (h *Handler) AuditLog(c *echo.Context) error {
	params, err := parseAuditLogQuery(c, true)
	if err != nil {
		return handleError(c, err)
	}

	if h.auditReader == nil {
		return c.JSON(http.StatusOK, auditLogListResponse{
			Entries: []auditLogEntryResponse{},
		})
	}

	result, err := h.auditReader.GetLogs(c.Request().Context(), params)
	if err != nil {
		return handleError(c, err)
	}
	if result == nil {
		result = &auditlog.LogListResult{Entries: []auditlog.LogEntry{}}
	}
	if result.Entries == nil {
		result.Entries = []auditlog.LogEntry{}
	}

	response, err := h.auditLogResponse(c.Request().Context(), result)
	if err != nil {
		return handleError(c, err)
	}
	return c.JSON(http.StatusOK, response)
}

func parseAuditLogQuery(c *echo.Context, allowPagination bool) (auditlog.LogQueryParams, error) {
	dateRange, err := parseDateRangeParams(c)
	if err != nil {
		return auditlog.LogQueryParams{}, err
	}
	userPath, err := normalizeUserPathQueryParam("user_path", c.QueryParam("user_path"))
	if err != nil {
		return auditlog.LogQueryParams{}, err
	}

	requestedModel := c.QueryParam("requested_model")
	if requestedModel == "" {
		requestedModel = c.QueryParam("model")
	}
	sortParam, err := normalizeAuditSortQuery(c.QueryParam("sort"))
	if err != nil {
		return auditlog.LogQueryParams{}, err
	}

	params := auditlog.LogQueryParams{
		QueryParams:    auditlog.QueryParams{StartDate: dateRange.StartDate, EndDate: dateRange.EndDate},
		RequestedModel: requestedModel,
		Provider:       c.QueryParam("provider"),
		Method:         strings.ToUpper(c.QueryParam("method")),
		Path:           c.QueryParam("path"),
		UserPath:       userPath,
		AuthKeyID:      strings.TrimSpace(c.QueryParam("auth_key_id")),
		ErrorType:      c.QueryParam("error_type"),
		Search:         c.QueryParam("search"),
		Sort:           sortParam,
	}

	if sc := c.QueryParam("status_code"); sc != "" {
		parsed, err := strconv.Atoi(sc)
		if err != nil {
			return auditlog.LogQueryParams{}, core.NewInvalidRequestError("invalid status_code, expected integer", nil)
		}
		params.StatusCode = &parsed
	}
	if stream := c.QueryParam("stream"); stream != "" {
		parsed, err := strconv.ParseBool(stream)
		if err != nil {
			return auditlog.LogQueryParams{}, core.NewInvalidRequestError("invalid stream value, expected true or false", nil)
		}
		params.Stream = &parsed
	}
	if includeStats := c.QueryParam("include_stats"); includeStats != "" {
		parsed, err := strconv.ParseBool(includeStats)
		if err != nil {
			return auditlog.LogQueryParams{}, core.NewInvalidRequestError("invalid include_stats value, expected true or false", nil)
		}
		params.IncludeStats = parsed
	}
	if allowPagination {
		if l := c.QueryParam("limit"); l != "" {
			parsed, err := strconv.Atoi(l)
			if err != nil || parsed <= 0 {
				return auditlog.LogQueryParams{}, core.NewInvalidRequestError("invalid limit, expected positive integer", nil)
			}
			if parsed > maxAuditLogLimit {
				return auditlog.LogQueryParams{}, core.NewInvalidRequestError("invalid limit parameter: limit must be between 1 and 100", nil)
			}
			params.Limit = parsed
		}
		if o := c.QueryParam("offset"); o != "" {
			parsed, err := strconv.Atoi(o)
			if err != nil || parsed < 0 {
				return auditlog.LogQueryParams{}, core.NewInvalidRequestError("invalid offset, expected non-negative integer", nil)
			}
			params.Offset = parsed
		}
	}
	return params, nil
}

func normalizeAuditSortQuery(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-timestamp", nil
	}
	field := trimmed
	if strings.HasPrefix(field, "-") {
		field = strings.TrimPrefix(field, "-")
	}
	field = strings.TrimSpace(field)
	if _, ok := allowedAuditSortFields[field]; !ok || field == "" {
		return "", core.NewInvalidRequestError("invalid sort parameter", nil)
	}
	if strings.HasPrefix(trimmed, "-") {
		return "-" + field, nil
	}
	return field, nil
}

func (h *Handler) AuditLogExport(c *echo.Context) error {
	params, err := parseAuditLogQuery(c, false)
	if err != nil {
		return handleError(c, err)
	}
	params.Limit = maxAuditLogExportPageSize
	params.Offset = 0
	params.IncludeStats = false

	format := strings.ToLower(strings.TrimSpace(c.QueryParam("format")))
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "jsonl" {
		return handleError(c, core.NewInvalidRequestError("invalid format, expected csv or jsonl", nil))
	}

	if format == "jsonl" {
		return h.writeAuditJSONLExportStreaming(c, params)
	}
	return h.writeAuditCSVExportStreaming(c, params)
}

// maxAuditExportRows caps any single export at one million rows. Combined
// with the per-page hard cap and a context deadline, this stops a malicious
// (or careless) caller from OOM-ing the gateway by repeatedly requesting
// `?days=365&format=jsonl`.
const maxAuditExportRows = 1_000_000

func (h *Handler) writeAuditCSVExportStreaming(c *echo.Context, params auditlog.LogQueryParams) error {
	res := c.Response()
	res.Header().Set("Content-Type", "text/csv; charset=utf-8")
	res.Header().Set("Content-Disposition", `attachment; filename="aurora-audit-log.csv"`)
	writer := csv.NewWriter(res)
	if err := writer.Write([]string{"id", "timestamp", "duration_ns", "requested_model", "resolved_model", "provider", "provider_name", "status_code", "request_id", "auth_key_id", "auth_method", "method", "path", "user_path", "stream", "error_type", "error_message"}); err != nil {
		return err
	}

	if h.auditReader == nil {
		writer.Flush()
		return writer.Error()
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Minute)
	defer cancel()

	params.Limit = maxAuditLogExportPageSize
	params.Offset = 0
	written := 0

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		result, err := h.auditReader.GetLogs(ctx, params)
		if err != nil {
			return err
		}
		if result == nil || len(result.Entries) == 0 {
			break
		}
		for _, entry := range result.Entries {
			if written >= maxAuditExportRows {
				writer.Flush()
				return writer.Error()
			}
			if err := writer.Write([]string{
				sanitizeCSVCell(entry.ID),
				entry.Timestamp.Format(time.RFC3339Nano),
				strconv.FormatInt(entry.DurationNs, 10),
				sanitizeCSVCell(entry.RequestedModel),
				sanitizeCSVCell(entry.ResolvedModel),
				sanitizeCSVCell(entry.Provider),
				sanitizeCSVCell(entry.ProviderName),
				strconv.Itoa(entry.StatusCode),
				sanitizeCSVCell(entry.RequestID),
				sanitizeCSVCell(entry.AuthKeyID),
				sanitizeCSVCell(entry.AuthMethod),
				sanitizeCSVCell(entry.Method),
				sanitizeCSVCell(entry.Path),
				sanitizeCSVCell(entry.UserPath),
				strconv.FormatBool(entry.Stream),
				sanitizeCSVCell(entry.ErrorType),
				sanitizeCSVCell(safeAuditErrorMessage(entry)),
			}); err != nil {
				return err
			}
			written++
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
		params.Offset += params.Limit
		if result.Total > 0 && params.Offset >= result.Total {
			break
		}
		if len(result.Entries) < params.Limit {
			break
		}
	}
	writer.Flush()
	return writer.Error()
}

func (h *Handler) writeAuditJSONLExportStreaming(c *echo.Context, params auditlog.LogQueryParams) error {
	res := c.Response()
	res.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	res.Header().Set("Content-Disposition", `attachment; filename="aurora-audit-log.jsonl"`)

	if h.auditReader == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Minute)
	defer cancel()

	encoder := json.NewEncoder(res)
	params.Limit = maxAuditLogExportPageSize
	params.Offset = 0
	written := 0

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		result, err := h.auditReader.GetLogs(ctx, params)
		if err != nil {
			return err
		}
		if result == nil || len(result.Entries) == 0 {
			break
		}
		for _, entry := range result.Entries {
			if written >= maxAuditExportRows {
				return nil
			}
			row := map[string]any{
				"id": entry.ID, "timestamp": entry.Timestamp, "duration_ns": entry.DurationNs,
				"requested_model": entry.RequestedModel, "resolved_model": entry.ResolvedModel,
				"provider": entry.Provider, "provider_name": entry.ProviderName, "status_code": entry.StatusCode,
				"request_id": entry.RequestID, "auth_key_id": entry.AuthKeyID, "auth_method": entry.AuthMethod,
				"method": entry.Method, "path": entry.Path, "user_path": entry.UserPath, "stream": entry.Stream,
				"error_type": entry.ErrorType, "error_message": safeAuditErrorMessage(entry),
			}
			if err := encoder.Encode(row); err != nil {
				return err
			}
			written++
		}
		if flusher, ok := res.(http.Flusher); ok {
			flusher.Flush()
		}
		params.Offset += params.Limit
		if result.Total > 0 && params.Offset >= result.Total {
			break
		}
		if len(result.Entries) < params.Limit {
			break
		}
	}
	return nil
}

func sanitizeCSVCell(value string) string {
	if value == "" {
		return ""
	}
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	}
	if value[0] == '\t' || value[0] == '\r' || value[0] == '\n' {
		return "'" + value
	}
	return value
}

// secretRedactor catches API key prefixes (OpenAI sk-, Anthropic sk-ant-,
// generic Bearer tokens, x-api-key headers, AWS keys) that upstream
// providers occasionally echo into error messages. Without this, an
// audit-log export can leak partial credentials of the configured
// providers downstream.
var secretRedactor = regexp.MustCompile(
	`(?i)` +
		`(bearer\s+[A-Za-z0-9_\-\.]{6,}|` +
		`sk-(?:ant-)?[A-Za-z0-9_\-]{10,}|` +
		`AKIA[0-9A-Z]{16}|` +
		`x-api-key:\s*[A-Za-z0-9_\-]{6,}|` +
		`api[_-]?key["'\s:=]+[A-Za-z0-9_\-]{12,})`,
)

func redactAuditSecrets(value string) string {
	if value == "" {
		return ""
	}
	return secretRedactor.ReplaceAllString(value, "[REDACTED]")
}

func safeAuditErrorMessage(entry auditlog.LogEntry) string {
	if entry.Data == nil {
		return ""
	}
	return redactAuditSecrets(entry.Data.ErrorMessage)
}

func (h *Handler) auditLogResponse(ctx context.Context, result *auditlog.LogListResult) (*auditLogListResponse, error) {
	if result == nil {
		return &auditLogListResponse{Entries: []auditLogEntryResponse{}}, nil
	}

	response := &auditLogListResponse{
		Entries: h.auditEntriesResponse(ctx, result.Entries, "audit log"),
		Total:   result.Total,
		Limit:   result.Limit,
		Offset:  result.Offset,
		Sort:    result.Sort,
		Stats:   result.Stats,
	}

	return response, nil
}

func (h *Handler) auditEntriesResponse(ctx context.Context, entries []auditlog.LogEntry, source string) []auditLogEntryResponse {
	response := make([]auditLogEntryResponse, len(entries))
	for i := range entries {
		response[i].LogEntry = entries[i]
	}

	if h.usageReader == nil || len(entries) == 0 {
		return response
	}

	requestIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.RequestID != "" {
			requestIDs = append(requestIDs, entry.RequestID)
		}
	}
	if len(requestIDs) == 0 {
		return response
	}

	entriesByRequestID, err := h.usageReader.GetUsageByRequestIDs(ctx, requestIDs)
	if err != nil {
		slog.Warn("failed to enrich audit entries with usage", "source", source, "error", err, "request_count", len(requestIDs))
		return response
	}

	summaries := usage.SummarizeUsageByRequestID(entriesByRequestID)
	for i := range response {
		if summary, ok := summaries[response[i].RequestID]; ok {
			response[i].Usage = summary
		}
	}
	return response
}

// AuditConversation handles GET /admin/api/v1/audit/conversation
//
// @Summary      Get conversation thread around an audit log entry
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        log_id  query     string  true   "Anchor audit log entry ID"
// @Param        limit   query     int     false  "Max entries in thread (default 40, max 50)"
// @Success      200  {object}  auditConversationResponse
// @Failure      400  {object}  core.GatewayError
// @Failure      401  {object}  core.GatewayError
// @Router       /admin/api/v1/audit/conversation [get]
func (h *Handler) AuditConversation(c *echo.Context) error {
	// Validate request shape before the disabled-reader fast path so callers
	// always get a 400 for missing/invalid params, regardless of whether
	// audit logging is configured.
	logID := strings.TrimSpace(c.QueryParam("log_id"))
	if logID == "" {
		return handleError(c, core.NewInvalidRequestError("log_id is required", nil))
	}

	limit := 40
	if l := c.QueryParam("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err != nil {
			return handleError(c, core.NewInvalidRequestError("invalid limit, expected integer", nil))
		}
		if parsed < 1 || parsed > 50 {
			return handleError(c, core.NewInvalidRequestError("invalid limit parameter: limit must be between 1 and 50", nil))
		}
		limit = parsed
	}

	if h.auditReader == nil {
		return c.JSON(http.StatusOK, auditConversationResponse{
			AnchorID: logID,
			Entries:  []auditLogEntryResponse{},
		})
	}

	result, err := h.auditReader.GetConversation(c.Request().Context(), logID, limit)
	if err != nil {
		return handleError(c, err)
	}
	if result == nil {
		result = &auditlog.ConversationResult{
			AnchorID: logID,
			Entries:  []auditlog.LogEntry{},
		}
	}
	if result.Entries == nil {
		result.Entries = []auditlog.LogEntry{}
	}

	return c.JSON(http.StatusOK, auditConversationResponse{
		AnchorID: result.AnchorID,
		Entries:  h.auditEntriesResponse(c.Request().Context(), result.Entries, "audit conversation"),
	})
}

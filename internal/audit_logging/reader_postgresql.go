package auditlog

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQLReader implements Reader for PostgreSQL databases.
type PostgreSQLReader struct {
	pool *pgxpool.Pool
}

// NewPostgreSQLReader creates a new PostgreSQL audit log reader.
func NewPostgreSQLReader(pool *pgxpool.Pool) (*PostgreSQLReader, error) {
	if pool == nil {
		return nil, fmt.Errorf("connection pool is required")
	}
	return &PostgreSQLReader{pool: pool}, nil
}

// GetLogs returns a paginated list of audit log entries.
func (r *PostgreSQLReader) GetLogs(ctx context.Context, params LogQueryParams) (*LogListResult, error) {
	limit, offset := clampLimitOffset(params.Limit, params.Offset)
	sortSpec, err := normalizeAuditSort(params.Sort)
	if err != nil {
		return nil, err
	}

	conditions, args, argIdx := pgDateRangeConditions(params.QueryParams, 1)
	userPath, err := normalizeAuditUserPathFilter(params.UserPath)
	if err != nil {
		return nil, err
	}

	if params.RequestedModel != "" {
		conditions = append(conditions, fmt.Sprintf("requested_model ILIKE $%d ESCAPE '\\'", argIdx))
		args = append(args, "%"+escapeLikeWildcards(params.RequestedModel)+"%")
		argIdx++
	}
	if params.Provider != "" {
		conditions = append(conditions, fmt.Sprintf("(provider ILIKE $%d ESCAPE '\\' OR provider_name ILIKE $%d ESCAPE '\\')", argIdx, argIdx+1))
		args = append(args, "%"+escapeLikeWildcards(params.Provider)+"%", "%"+escapeLikeWildcards(params.Provider)+"%")
		argIdx += 2
	}
	if params.Method != "" {
		conditions = append(conditions, fmt.Sprintf("method = $%d", argIdx))
		args = append(args, params.Method)
		argIdx++
	}
	if params.Path != "" {
		conditions = append(conditions, fmt.Sprintf("path ILIKE $%d ESCAPE '\\'", argIdx))
		args = append(args, "%"+escapeLikeWildcards(params.Path)+"%")
		argIdx++
	}
	if userPath != "" {
		conditions = append(conditions, auditUserPathSQLPredicate(
			userPath,
			fmt.Sprintf("user_path = $%d", argIdx),
			fmt.Sprintf("user_path LIKE $%d ESCAPE '\\'", argIdx+1),
		))
		args = append(args, userPath, auditUserPathSubtreePattern(userPath))
		argIdx += 2
	}
	if params.ErrorType != "" {
		conditions = append(conditions, fmt.Sprintf("error_type ILIKE $%d ESCAPE '\\'", argIdx))
		args = append(args, "%"+escapeLikeWildcards(params.ErrorType)+"%")
		argIdx++
	}
	if params.AuthKeyID != "" {
		conditions = append(conditions, fmt.Sprintf("auth_key_id = $%d", argIdx))
		args = append(args, params.AuthKeyID)
		argIdx++
	}
	if params.StatusCode != nil {
		conditions = append(conditions, fmt.Sprintf("status_code = $%d", argIdx))
		args = append(args, *params.StatusCode)
		argIdx++
	}
	if params.Stream != nil {
		conditions = append(conditions, fmt.Sprintf("stream = $%d", argIdx))
		args = append(args, *params.Stream)
		argIdx++
	}
	if params.Search != "" {
		s := "%" + escapeLikeWildcards(params.Search) + "%"
		conditions = append(conditions, fmt.Sprintf("(request_id ILIKE $%d ESCAPE '\\' OR auth_key_id ILIKE $%d ESCAPE '\\' OR requested_model ILIKE $%d ESCAPE '\\' OR provider ILIKE $%d ESCAPE '\\' OR provider_name ILIKE $%d ESCAPE '\\' OR method ILIKE $%d ESCAPE '\\' OR path ILIKE $%d ESCAPE '\\' OR user_path ILIKE $%d ESCAPE '\\' OR error_type ILIKE $%d ESCAPE '\\' OR data->>'error_message' ILIKE $%d ESCAPE '\\')",
			argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4, argIdx+5, argIdx+6, argIdx+7, argIdx+8, argIdx+9))
		args = append(args, s, s, s, s, s, s, s, s, s, s)
		argIdx += 10
	}

	where := buildWhereClause(conditions)

	var total int
	countQuery := `SELECT COUNT(*) FROM audit_logs` + where
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count audit log entries: %w", err)
	}

	dataQuery := fmt.Sprintf(`SELECT id, timestamp, duration_ns, requested_model, resolved_model, provider, provider_name, alias_used, workflow_version_id, cache_type, status_code, request_id, auth_key_id, auth_method,
		client_ip, method, path, user_path, stream, error_type, data
		FROM audit_logs%s%s LIMIT $%d OFFSET $%d`, where, auditSQLOrderBy(sortSpec), argIdx, argIdx+1)
	dataArgs := append(append([]any(nil), args...), limit, offset)

	rows, err := r.pool.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	entries := make([]LogEntry, 0)
	for rows.Next() {
		var e LogEntry
		var dataJSON *string
		var providerName *string
		var workflowVersionID *string
		var cacheType *string
		var authKeyID *string
		var authMethod *string
		var userPath *string

		if err := rows.Scan(&e.ID, &e.Timestamp, &e.DurationNs, &e.RequestedModel, &e.ResolvedModel, &e.Provider, &providerName, &e.AliasUsed, &workflowVersionID, &cacheType, &e.StatusCode,
			&e.RequestID, &authKeyID, &authMethod, &e.ClientIP, &e.Method, &e.Path, &userPath, &e.Stream, &e.ErrorType, &dataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan audit log row: %w", err)
		}
		if workflowVersionID != nil {
			e.WorkflowVersionID = *workflowVersionID
		}
		if authKeyID != nil {
			e.AuthKeyID = *authKeyID
		}
		if authMethod != nil {
			e.AuthMethod = *authMethod
		}
		if cacheType != nil {
			e.CacheType = cleanCacheType(*cacheType)
		}
		if providerName != nil {
			e.ProviderName = displayAuditProviderName(*providerName, e.Provider)
		} else {
			e.ProviderName = displayAuditProviderName("", e.Provider)
		}
		if userPath != nil {
			e.UserPath = *userPath
		}

		if dataJSON != nil && *dataJSON != "" {
			var data LogData
			if err := json.Unmarshal([]byte(*dataJSON), &data); err != nil {
				slog.Warn("failed to unmarshal audit data JSON", "id", e.ID, "error", err)
			} else {
				e.Data = redactLogData(&data)
			}
		}

		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit log rows: %w", err)
	}

	var stats *LogStats
	if params.IncludeStats {
		stats, err = r.postgreSQLLogStats(ctx, where, args)
		if err != nil {
			return nil, err
		}
	}

	return &LogListResult{
		Entries: entries,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		Sort:    sortSpec.Public,
		Stats:   stats,
	}, nil
}

func (r *PostgreSQLReader) postgreSQLLogStats(ctx context.Context, where string, args []any) (*LogStats, error) {
	stats := &LogStats{
		StatusBuckets:    []BucketCount{},
		MethodBuckets:    []BucketCount{},
		ProviderBuckets:  []BucketCount{},
		ModelBuckets:     []BucketCount{},
		ErrorTypeBuckets: []BucketCount{},
	}

	query := `SELECT COUNT(*),
		COALESCE(SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status_code >= 300 AND status_code < 400 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status_code >= 400 OR error_type <> '' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN stream THEN 1 ELSE 0 END), 0),
		COALESCE(MIN(duration_ns), 0),
		COALESCE(AVG(duration_ns), 0),
		COALESCE(MAX(duration_ns), 0)
		FROM audit_logs` + where
	if err := r.pool.QueryRow(ctx, query, args...).Scan(
		&stats.Total,
		&stats.SuccessCount,
		&stats.RedirectCount,
		&stats.ClientErrorCount,
		&stats.ServerErrorCount,
		&stats.ErrorCount,
		&stats.StreamCount,
		&stats.MinDurationNs,
		&stats.AvgDurationNs,
		&stats.MaxDurationNs,
	); err != nil {
		return nil, fmt.Errorf("failed to query audit log stats: %w", err)
	}

	var err error
	stats.StatusBuckets, err = r.postgreSQLBucketCounts(ctx, `SELECT CASE WHEN status_code >= 500 THEN '5xx' WHEN status_code >= 400 THEN '4xx' WHEN status_code >= 300 THEN '3xx' WHEN status_code >= 200 THEN '2xx' ELSE 'unknown' END AS label, COUNT(*) FROM audit_logs`+where+` GROUP BY label`, args, 0)
	if err != nil {
		return nil, err
	}
	stats.MethodBuckets, err = r.postgreSQLBucketCounts(ctx, `SELECT COALESCE(NULLIF(method, ''), 'unknown') AS label, COUNT(*) FROM audit_logs`+where+` GROUP BY label`, args, 8)
	if err != nil {
		return nil, err
	}
	stats.ProviderBuckets, err = r.postgreSQLBucketCounts(ctx, `SELECT COALESCE(NULLIF(provider_name, ''), NULLIF(provider, ''), 'unknown') AS label, COUNT(*) FROM audit_logs`+where+` GROUP BY label`, args, 8)
	if err != nil {
		return nil, err
	}
	stats.ModelBuckets, err = r.postgreSQLBucketCounts(ctx, `SELECT COALESCE(NULLIF(requested_model, ''), 'unknown') AS label, COUNT(*) FROM audit_logs`+where+` GROUP BY label`, args, 8)
	if err != nil {
		return nil, err
	}
	errorTypeWhere := where
	if errorTypeWhere == "" {
		errorTypeWhere = " WHERE error_type <> ''"
	} else {
		errorTypeWhere += " AND error_type <> ''"
	}
	stats.ErrorTypeBuckets, err = r.postgreSQLBucketCounts(ctx, `SELECT error_type AS label, COUNT(*) FROM audit_logs`+errorTypeWhere+` GROUP BY label`, args, 8)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *PostgreSQLReader) postgreSQLBucketCounts(ctx context.Context, query string, args []any, limit int) ([]BucketCount, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit log bucket counts: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var label string
		var count int
		if err := rows.Scan(&label, &count); err != nil {
			return nil, fmt.Errorf("failed to scan audit log bucket count: %w", err)
		}
		counts[label] += count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit log bucket counts: %w", err)
	}
	return bucketCountsFromMap(counts, limit), nil
}

// GetLogByID returns a single audit log entry by ID.
func (r *PostgreSQLReader) GetLogByID(ctx context.Context, id string) (*LogEntry, error) {
	query := `SELECT id, timestamp, duration_ns, requested_model, resolved_model, provider, provider_name, alias_used, workflow_version_id, cache_type, status_code, request_id, auth_key_id, auth_method,
		client_ip, method, path, user_path, stream, error_type, data
		FROM audit_logs WHERE id::text = $1 LIMIT 1`

	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit log by id: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	entry, err := scanPostgreSQLLogEntry(rows)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// GetConversation returns a linear conversation thread around a seed log entry.
func (r *PostgreSQLReader) GetConversation(ctx context.Context, logID string, limit int) (*ConversationResult, error) {
	return buildConversationThread(ctx, logID, limit, r.GetLogByID, r.findByResponseID, r.findByPreviousResponseID)
}

// AggregateAuthKeyDaily groups audit rows for one auth key by local calendar day.
// Uses a single GROUP BY query keyed on `timestamp AT TIME ZONE $tz` so the
// database does the bucketing and we never have to fetch the heavy `data` column.
func (r *PostgreSQLReader) AggregateAuthKeyDaily(ctx context.Context, authKeyID string, start, end time.Time, location *time.Location) (*AuthKeyDailyAggregate, error) {
	authKeyID = trimmedAuthKeyID(authKeyID)
	if authKeyID == "" {
		return dailyAggregateFromBuckets(start, end, location, nil), nil
	}
	if location == nil {
		location = time.UTC
	}
	startBoundary := dailyAggregateInclusiveStart(start, location).UTC()
	endBoundary := dailyAggregateExclusiveEnd(end, location).UTC()

	query := `
SELECT to_char((timestamp AT TIME ZONE 'UTC' AT TIME ZONE $1)::date, 'YYYY-MM-DD') AS bucket_date,
       COUNT(*) AS requests,
       COUNT(*) FILTER (WHERE status_code >= 400 OR COALESCE(error_type, '') <> '') AS errors,
       COUNT(*) FILTER (WHERE cache_type = 'exact') AS exact_hits,
       COUNT(*) FILTER (WHERE cache_type = 'semantic') AS semantic_hits
FROM audit_logs
WHERE auth_key_id = $2 AND timestamp >= $3 AND timestamp < $4
GROUP BY bucket_date
ORDER BY bucket_date`
	rows, err := r.pool.Query(ctx, query, location.String(), authKeyID, startBoundary, endBoundary)
	if err != nil {
		return nil, fmt.Errorf("aggregate auth key daily: %w", err)
	}
	defer rows.Close()

	observed := make(map[string]AuthKeyDailyBucket)
	for rows.Next() {
		var bucket AuthKeyDailyBucket
		if err := rows.Scan(&bucket.Date, &bucket.Requests, &bucket.Errors, &bucket.ExactHits, &bucket.SemanticHits); err != nil {
			return nil, fmt.Errorf("aggregate auth key daily: scan row: %w", err)
		}
		observed[bucket.Date] = bucket
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("aggregate auth key daily: iterate rows: %w", err)
	}
	return dailyAggregateFromBuckets(start, end, location, observed), nil
}

func pgDateRangeConditions(params QueryParams, argIdx int) (conditions []string, args []any, nextIdx int) {
	nextIdx = argIdx
	if !params.StartDate.IsZero() {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", nextIdx))
		args = append(args, params.StartDate.UTC())
		nextIdx++
	}
	if !params.EndDate.IsZero() {
		conditions = append(conditions, fmt.Sprintf("timestamp < $%d", nextIdx))
		args = append(args, params.EndDate.AddDate(0, 0, 1).UTC())
		nextIdx++
	}
	return conditions, args, nextIdx
}

func (r *PostgreSQLReader) findByResponseID(ctx context.Context, responseID string) (*LogEntry, error) {
	query := `SELECT id, timestamp, duration_ns, requested_model, resolved_model, provider, provider_name, alias_used, workflow_version_id, cache_type, status_code, request_id, auth_key_id, auth_method,
		client_ip, method, path, user_path, stream, error_type, data
		FROM audit_logs
		WHERE data->'response_body'->>'id' = $1
		ORDER BY timestamp ASC
		LIMIT 1`

	rows, err := r.pool.Query(ctx, query, responseID)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit log by response id: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	return scanPostgreSQLLogEntry(rows)
}

func (r *PostgreSQLReader) findByPreviousResponseID(ctx context.Context, previousResponseID string) (*LogEntry, error) {
	query := `SELECT id, timestamp, duration_ns, requested_model, resolved_model, provider, provider_name, alias_used, workflow_version_id, cache_type, status_code, request_id, auth_key_id, auth_method,
		client_ip, method, path, user_path, stream, error_type, data
		FROM audit_logs
		WHERE data->'request_body'->>'previous_response_id' = $1
		ORDER BY timestamp ASC
		LIMIT 1`

	rows, err := r.pool.Query(ctx, query, previousResponseID)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit log by previous_response_id: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	return scanPostgreSQLLogEntry(rows)
}

func scanPostgreSQLLogEntry(rows interface {
	Scan(dest ...any) error
}) (*LogEntry, error) {
	var e LogEntry
	var dataJSON *string
	var providerName *string
	var workflowVersionID *string
	var cacheType *string
	var authKeyID *string
	var authMethod *string
	var userPath *string

	if err := rows.Scan(&e.ID, &e.Timestamp, &e.DurationNs, &e.RequestedModel, &e.ResolvedModel, &e.Provider, &providerName, &e.AliasUsed, &workflowVersionID, &cacheType, &e.StatusCode,
		&e.RequestID, &authKeyID, &authMethod, &e.ClientIP, &e.Method, &e.Path, &userPath, &e.Stream, &e.ErrorType, &dataJSON); err != nil {
		return nil, fmt.Errorf("failed to scan audit log row: %w", err)
	}
	if workflowVersionID != nil {
		e.WorkflowVersionID = *workflowVersionID
	}
	if authKeyID != nil {
		e.AuthKeyID = *authKeyID
	}
	if authMethod != nil {
		e.AuthMethod = *authMethod
	}
	if cacheType != nil {
		e.CacheType = cleanCacheType(*cacheType)
	}
	if providerName != nil {
		e.ProviderName = displayAuditProviderName(*providerName, e.Provider)
	} else {
		e.ProviderName = displayAuditProviderName("", e.Provider)
	}
	if userPath != nil {
		e.UserPath = *userPath
	}

	if dataJSON != nil && *dataJSON != "" {
		var data LogData
		if err := json.Unmarshal([]byte(*dataJSON), &data); err != nil {
			slog.Warn("failed to unmarshal audit data JSON", "id", e.ID, "error", err)
		} else {
			e.Data = redactLogData(&data)
		}
	}

	return &e, nil
}

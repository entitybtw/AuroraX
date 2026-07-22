package auditlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"
)

const sqliteTimestampBoundaryLayout = "2006-01-02T15:04:05"

// SQLiteReader implements Reader for SQLite databases.
type SQLiteReader struct {
	db *sql.DB
}

// NewSQLiteReader creates a new SQLite audit log reader.
func NewSQLiteReader(db *sql.DB) (*SQLiteReader, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	return &SQLiteReader{db: db}, nil
}

// GetLogs returns a paginated list of audit log entries.
func (r *SQLiteReader) GetLogs(ctx context.Context, params LogQueryParams) (*LogListResult, error) {
	limit, offset := clampLimitOffset(params.Limit, params.Offset)
	sortSpec, err := normalizeAuditSort(params.Sort)
	if err != nil {
		return nil, err
	}

	conditions, args := sqliteDateRangeConditions(params.QueryParams)
	userPath, err := normalizeAuditUserPathFilter(params.UserPath)
	if err != nil {
		return nil, err
	}

	if params.RequestedModel != "" {
		conditions = append(conditions, "requested_model LIKE ? ESCAPE '\\'")
		args = append(args, "%"+escapeLikeWildcards(params.RequestedModel)+"%")
	}
	if params.Provider != "" {
		conditions = append(conditions, "(provider LIKE ? ESCAPE '\\' OR provider_name LIKE ? ESCAPE '\\')")
		args = append(args, "%"+escapeLikeWildcards(params.Provider)+"%", "%"+escapeLikeWildcards(params.Provider)+"%")
	}
	if params.Method != "" {
		conditions = append(conditions, "method = ?")
		args = append(args, params.Method)
	}
	if params.Path != "" {
		conditions = append(conditions, "path LIKE ? ESCAPE '\\'")
		args = append(args, "%"+escapeLikeWildcards(params.Path)+"%")
	}
	if userPath != "" {
		conditions = append(conditions, auditUserPathSQLPredicate(userPath, "user_path = ?", "user_path LIKE ? ESCAPE '\\'"))
		args = append(args, userPath, auditUserPathSubtreePattern(userPath))
	}
	if params.ErrorType != "" {
		conditions = append(conditions, "error_type LIKE ? ESCAPE '\\'")
		args = append(args, "%"+escapeLikeWildcards(params.ErrorType)+"%")
	}
	if params.AuthKeyID != "" {
		conditions = append(conditions, "auth_key_id = ?")
		args = append(args, params.AuthKeyID)
	}
	if params.StatusCode != nil {
		conditions = append(conditions, "status_code = ?")
		args = append(args, *params.StatusCode)
	}
	if params.Stream != nil {
		conditions = append(conditions, "stream = ?")
		if *params.Stream {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if params.Search != "" {
		s := "%" + escapeLikeWildcards(params.Search) + "%"
		conditions = append(conditions, `(request_id LIKE ? ESCAPE '\' OR auth_key_id LIKE ? ESCAPE '\' OR requested_model LIKE ? ESCAPE '\' OR provider LIKE ? ESCAPE '\' OR provider_name LIKE ? ESCAPE '\' OR method LIKE ? ESCAPE '\' OR path LIKE ? ESCAPE '\' OR user_path LIKE ? ESCAPE '\' OR error_type LIKE ? ESCAPE '\' OR json_extract(data, '$.error_message') LIKE ? ESCAPE '\')`)
		args = append(args, s, s, s, s, s, s, s, s, s, s)
	}

	where := buildWhereClause(conditions)

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs" + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count audit log entries: %w", err)
	}

	dataQuery := `SELECT id, timestamp, duration_ns, requested_model, resolved_model, provider, provider_name, alias_used, workflow_version_id, cache_type, status_code, request_id, auth_key_id, auth_method,
		client_ip, method, path, user_path, stream, error_type, data
		FROM audit_logs` + where + auditSQLOrderBy(sortSpec) + ` LIMIT ? OFFSET ?`
	dataArgs := append(append([]any(nil), args...), limit, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	entries := make([]LogEntry, 0)
	for rows.Next() {
		var e LogEntry
		var ts string
		var providerName sql.NullString
		var aliasUsedInt int
		var streamInt int
		var dataJSON *string
		var workflowVersionID sql.NullString
		var cacheType sql.NullString
		var authKeyID sql.NullString
		var authMethod sql.NullString
		var userPath sql.NullString

		if err := rows.Scan(&e.ID, &ts, &e.DurationNs, &e.RequestedModel, &e.ResolvedModel, &e.Provider, &providerName, &aliasUsedInt, &workflowVersionID, &cacheType, &e.StatusCode,
			&e.RequestID, &authKeyID, &authMethod, &e.ClientIP, &e.Method, &e.Path, &userPath, &streamInt, &e.ErrorType, &dataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan audit log row: %w", err)
		}

		e.AliasUsed = aliasUsedInt == 1
		e.Stream = streamInt == 1
		e.Timestamp = parseSQLTimestamp(ts, e.ID)
		if workflowVersionID.Valid {
			e.WorkflowVersionID = workflowVersionID.String
		}
		if authKeyID.Valid {
			e.AuthKeyID = authKeyID.String
		}
		if authMethod.Valid {
			e.AuthMethod = authMethod.String
		}
		if cacheType.Valid {
			e.CacheType = cleanCacheType(cacheType.String)
		}
		if providerName.Valid {
			e.ProviderName = displayAuditProviderName(providerName.String, e.Provider)
		} else {
			e.ProviderName = displayAuditProviderName("", e.Provider)
		}
		if userPath.Valid {
			e.UserPath = userPath.String
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
		stats, err = r.sqliteLogStats(ctx, where, args)
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

func (r *SQLiteReader) sqliteLogStats(ctx context.Context, where string, args []any) (*LogStats, error) {
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
		COALESCE(SUM(CASE WHEN stream = 1 THEN 1 ELSE 0 END), 0),
		COALESCE(MIN(duration_ns), 0),
		COALESCE(AVG(duration_ns), 0),
		COALESCE(MAX(duration_ns), 0)
		FROM audit_logs` + where
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
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
	stats.StatusBuckets, err = r.sqliteBucketCounts(ctx, `SELECT CASE WHEN status_code >= 500 THEN '5xx' WHEN status_code >= 400 THEN '4xx' WHEN status_code >= 300 THEN '3xx' WHEN status_code >= 200 THEN '2xx' ELSE 'unknown' END AS label, COUNT(*) FROM audit_logs`+where+` GROUP BY label`, args, 0)
	if err != nil {
		return nil, err
	}
	stats.MethodBuckets, err = r.sqliteBucketCounts(ctx, `SELECT COALESCE(NULLIF(method, ''), 'unknown') AS label, COUNT(*) FROM audit_logs`+where+` GROUP BY label`, args, 8)
	if err != nil {
		return nil, err
	}
	stats.ProviderBuckets, err = r.sqliteBucketCounts(ctx, `SELECT COALESCE(NULLIF(provider_name, ''), NULLIF(provider, ''), 'unknown') AS label, COUNT(*) FROM audit_logs`+where+` GROUP BY label`, args, 8)
	if err != nil {
		return nil, err
	}
	stats.ModelBuckets, err = r.sqliteBucketCounts(ctx, `SELECT COALESCE(NULLIF(requested_model, ''), 'unknown') AS label, COUNT(*) FROM audit_logs`+where+` GROUP BY label`, args, 8)
	if err != nil {
		return nil, err
	}
	errorTypeWhere := where
	if errorTypeWhere == "" {
		errorTypeWhere = " WHERE error_type <> ''"
	} else {
		errorTypeWhere += " AND error_type <> ''"
	}
	stats.ErrorTypeBuckets, err = r.sqliteBucketCounts(ctx, `SELECT error_type AS label, COUNT(*) FROM audit_logs`+errorTypeWhere+` GROUP BY label`, args, 8)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (r *SQLiteReader) sqliteBucketCounts(ctx context.Context, query string, args []any, limit int) ([]BucketCount, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
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
func (r *SQLiteReader) GetLogByID(ctx context.Context, id string) (*LogEntry, error) {
	query := `SELECT id, timestamp, duration_ns, requested_model, resolved_model, provider, provider_name, alias_used, workflow_version_id, cache_type, status_code, request_id, auth_key_id, auth_method,
		client_ip, method, path, user_path, stream, error_type, data
		FROM audit_logs WHERE id = ? LIMIT 1`

	rows, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit log by id: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	entry, err := scanSQLiteLogEntry(rows)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// GetConversation returns a linear conversation thread around a seed log entry.
func (r *SQLiteReader) GetConversation(ctx context.Context, logID string, limit int) (*ConversationResult, error) {
	limit = clampConversationLimit(limit)

	anchor, err := r.GetLogByID(ctx, logID)
	if err != nil {
		return nil, err
	}
	if anchor == nil {
		return &ConversationResult{
			AnchorID: logID,
			Entries:  []LogEntry{},
		}, nil
	}

	thread := []*LogEntry{anchor}
	seen := map[string]struct{}{anchor.ID: {}}

	// Walk backwards through previous_response_id links.
	current := anchor
	for len(thread) < limit {
		prevID := extractPreviousResponseID(current)
		if prevID == "" {
			break
		}
		parent, err := r.findByResponseID(ctx, prevID)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			break
		}
		if _, ok := seen[parent.ID]; ok {
			break
		}
		thread = append(thread, parent)
		seen[parent.ID] = struct{}{}
		current = parent
	}

	// Walk forwards via entries whose previous_response_id points to current response id.
	current = anchor
	for len(thread) < limit {
		respID := extractResponseID(current)
		if respID == "" {
			break
		}
		child, err := r.findByPreviousResponseID(ctx, respID)
		if err != nil {
			return nil, err
		}
		if child == nil {
			break
		}
		if _, ok := seen[child.ID]; ok {
			break
		}
		thread = append(thread, child)
		seen[child.ID] = struct{}{}
		current = child
	}

	sort.Slice(thread, func(i, j int) bool {
		return thread[i].Timestamp.Before(thread[j].Timestamp)
	})

	entries := make([]LogEntry, 0, len(thread))
	for _, entry := range thread {
		if entry != nil {
			entries = append(entries, *entry)
		}
	}

	return &ConversationResult{
		AnchorID: anchor.ID,
		Entries:  entries,
	}, nil
}

// AggregateAuthKeyDaily groups audit rows for one auth key by local calendar day.
// SQLite stores timestamps as ISO-8601 strings with offsets; rather than trying
// to coerce dates inside SQL, we fetch the minimal columns we need and bucket
// in Go using the caller's location.
func (r *SQLiteReader) AggregateAuthKeyDaily(ctx context.Context, authKeyID string, start, end time.Time, location *time.Location) (*AuthKeyDailyAggregate, error) {
	authKeyID = trimmedAuthKeyID(authKeyID)
	if authKeyID == "" {
		return dailyAggregateFromBuckets(start, end, location, nil), nil
	}
	if location == nil {
		location = time.UTC
	}
	startBoundary := dailyAggregateInclusiveStart(start, location)
	endBoundary := dailyAggregateExclusiveEnd(end, location)

	rows, err := r.db.QueryContext(ctx, `
SELECT timestamp, status_code, COALESCE(error_type, ''), COALESCE(cache_type, '')
FROM audit_logs
WHERE auth_key_id = ? AND timestamp >= ? AND timestamp < ?`,
		authKeyID,
		sqliteTimestampBoundary(startBoundary),
		sqliteTimestampBoundary(endBoundary),
	)
	if err != nil {
		return nil, fmt.Errorf("aggregate auth key daily: %w", err)
	}
	defer rows.Close()

	observed := make(map[string]AuthKeyDailyBucket)
	for rows.Next() {
		var ts string
		var statusCode int
		var errorType string
		var cacheType string
		if err := rows.Scan(&ts, &statusCode, &errorType, &cacheType); err != nil {
			return nil, fmt.Errorf("aggregate auth key daily: scan row: %w", err)
		}
		parsed := parseSQLTimestamp(ts, "")
		key := parsed.In(location).Format("2006-01-02")
		bucket := observed[key]
		bucket.Requests++
		if statusCode >= 400 || errorType != "" {
			bucket.Errors++
		}
		switch cleanCacheType(cacheType) {
		case CacheTypeExact:
			bucket.ExactHits++
		case CacheTypeSemantic:
			bucket.SemanticHits++
		}
		observed[key] = bucket
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("aggregate auth key daily: iterate rows: %w", err)
	}
	return dailyAggregateFromBuckets(start, end, location, observed), nil
}

func sqliteDateRangeConditions(params QueryParams) (conditions []string, args []any) {
	if !params.StartDate.IsZero() {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, sqliteTimestampBoundary(params.StartDate))
	}
	if !params.EndDate.IsZero() {
		conditions = append(conditions, "timestamp < ?")
		args = append(args, sqliteTimestampBoundary(params.EndDate.AddDate(0, 0, 1)))
	}
	return conditions, args
}

func sqliteTimestampBoundary(t time.Time) string {
	return t.UTC().Format(sqliteTimestampBoundaryLayout)
}

func parseSQLTimestamp(ts string, entryID string) time.Time {
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", ts); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02T15:04:05Z", ts); err == nil {
		return t
	}

	slog.Warn("failed to parse audit timestamp", "id", entryID, "raw_timestamp", ts)
	return time.Time{}
}

func (r *SQLiteReader) findByResponseID(ctx context.Context, responseID string) (*LogEntry, error) {
	query := `SELECT id, timestamp, duration_ns, requested_model, resolved_model, provider, provider_name, alias_used, workflow_version_id, cache_type, status_code, request_id, auth_key_id, auth_method,
		client_ip, method, path, user_path, stream, error_type, data
		FROM audit_logs
		WHERE json_extract(data, '$.response_body.id') = ?
		ORDER BY timestamp ASC
		LIMIT 1`

	rows, err := r.db.QueryContext(ctx, query, responseID)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit log by response id: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	return scanSQLiteLogEntry(rows)
}

func (r *SQLiteReader) findByPreviousResponseID(ctx context.Context, previousResponseID string) (*LogEntry, error) {
	query := `SELECT id, timestamp, duration_ns, requested_model, resolved_model, provider, provider_name, alias_used, workflow_version_id, cache_type, status_code, request_id, auth_key_id, auth_method,
		client_ip, method, path, user_path, stream, error_type, data
		FROM audit_logs
		WHERE json_extract(data, '$.request_body.previous_response_id') = ?
		ORDER BY timestamp ASC
		LIMIT 1`

	rows, err := r.db.QueryContext(ctx, query, previousResponseID)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit log by previous_response_id: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	return scanSQLiteLogEntry(rows)
}

func scanSQLiteLogEntry(rows *sql.Rows) (*LogEntry, error) {
	var e LogEntry
	var ts string
	var providerName sql.NullString
	var aliasUsedInt int
	var streamInt int
	var dataJSON *string
	var workflowVersionID sql.NullString
	var cacheType sql.NullString
	var authKeyID sql.NullString
	var authMethod sql.NullString
	var userPath sql.NullString

	if err := rows.Scan(&e.ID, &ts, &e.DurationNs, &e.RequestedModel, &e.ResolvedModel, &e.Provider, &providerName, &aliasUsedInt, &workflowVersionID, &cacheType, &e.StatusCode,
		&e.RequestID, &authKeyID, &authMethod, &e.ClientIP, &e.Method, &e.Path, &userPath, &streamInt, &e.ErrorType, &dataJSON); err != nil {
		return nil, fmt.Errorf("failed to scan audit log row: %w", err)
	}

	e.AliasUsed = aliasUsedInt == 1
	e.Stream = streamInt == 1
	e.Timestamp = parseSQLTimestamp(ts, e.ID)
	if workflowVersionID.Valid {
		e.WorkflowVersionID = workflowVersionID.String
	}
	if authKeyID.Valid {
		e.AuthKeyID = authKeyID.String
	}
	if authMethod.Valid {
		e.AuthMethod = authMethod.String
	}
	if cacheType.Valid {
		e.CacheType = cleanCacheType(cacheType.String)
	}
	if providerName.Valid {
		e.ProviderName = displayAuditProviderName(providerName.String, e.Provider)
	} else {
		e.ProviderName = displayAuditProviderName("", e.Provider)
	}
	if userPath.Valid {
		e.UserPath = userPath.String
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

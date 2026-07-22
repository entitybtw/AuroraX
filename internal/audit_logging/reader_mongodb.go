package auditlog

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"aurora/internal/core"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoDBReader implements Reader for MongoDB.
type MongoDBReader struct {
	collection *mongo.Collection
}

type mongoRow struct {
	ID                string    `bson:"_id"`
	Timestamp         time.Time `bson:"timestamp"`
	DurationNs        int64     `bson:"duration_ns"`
	RequestedModel    string    `bson:"requested_model"`
	LegacyModel       string    `bson:"model"`
	ResolvedModel     string    `bson:"resolved_model"`
	Provider          string    `bson:"provider"`
	ProviderName      string    `bson:"provider_name"`
	AliasUsed         bool      `bson:"alias_used"`
	WorkflowVersionID string    `bson:"workflow_version_id"`
	CacheType         string    `bson:"cache_type"`
	StatusCode        int       `bson:"status_code"`
	RequestID         string    `bson:"request_id"`
	AuthKeyID         string    `bson:"auth_key_id"`
	AuthMethod        string    `bson:"auth_method"`
	ClientIP          string    `bson:"client_ip"`
	Method            string    `bson:"method"`
	Path              string    `bson:"path"`
	UserPath          string    `bson:"user_path"`
	Stream            bool      `bson:"stream"`
	ErrorType         string    `bson:"error_type"`
	Data              *LogData  `bson:"data"`
}

func (r mongoRow) toLogEntry() *LogEntry {
	return &LogEntry{
		ID:                r.ID,
		Timestamp:         r.Timestamp,
		DurationNs:        r.DurationNs,
		RequestedModel:    pickFirstNonEmpty(r.RequestedModel, r.LegacyModel),
		ResolvedModel:     r.ResolvedModel,
		Provider:          r.Provider,
		ProviderName:      displayAuditProviderName(r.ProviderName, r.Provider),
		AliasUsed:         r.AliasUsed,
		WorkflowVersionID: r.WorkflowVersionID,
		CacheType:         cleanCacheType(r.CacheType),
		StatusCode:        r.StatusCode,
		RequestID:         r.RequestID,
		AuthKeyID:         r.AuthKeyID,
		AuthMethod:        r.AuthMethod,
		ClientIP:          r.ClientIP,
		Method:            r.Method,
		Path:              r.Path,
		UserPath:          r.UserPath,
		Stream:            r.Stream,
		ErrorType:         r.ErrorType,
		Data:              redactLogData(r.Data),
	}
}

func redactLogData(data *LogData) *LogData {
	if data == nil {
		return nil
	}
	clean := *data
	clean.RequestHeaders = RedactHeaders(data.RequestHeaders)
	clean.ResponseHeaders = RedactHeaders(data.ResponseHeaders)
	return &clean
}

// NewMongoDBReader creates a new MongoDB audit log reader.
func NewMongoDBReader(database *mongo.Database) (*MongoDBReader, error) {
	if database == nil {
		return nil, fmt.Errorf("database is required")
	}
	return &MongoDBReader{collection: database.Collection("audit_logs")}, nil
}

func mongoUserPathFilter(userPath string) bson.E {
	regexFilter := bson.D{{
		Key: "user_path",
		Value: bson.D{
			{Key: "$regex", Value: auditUserPathSubtreeRegex(userPath)},
		},
	}}
	if userPath != "/" {
		return regexFilter[0]
	}
	return bson.E{
		Key: "$or",
		Value: bson.A{
			regexFilter,
			bson.D{{Key: "user_path", Value: bson.D{{Key: "$exists", Value: false}}}},
			bson.D{{Key: "user_path", Value: nil}},
		},
	}
}

// GetLogs returns a paginated list of audit log entries.
func (r *MongoDBReader) GetLogs(ctx context.Context, params LogQueryParams) (*LogListResult, error) {
	limit, offset := clampLimitOffset(params.Limit, params.Offset)
	sortSpec, err := normalizeAuditSort(params.Sort)
	if err != nil {
		return nil, err
	}

	matchFilters := bson.D{}

	if tsFilter := mongoDateFilter(params.QueryParams); tsFilter != nil {
		matchFilters = append(matchFilters, bson.E{Key: "timestamp", Value: tsFilter})
	}
	if params.RequestedModel != "" {
		matchFilters = append(matchFilters, bson.E{
			Key: "$or",
			Value: bson.A{
				bson.D{{Key: "requested_model", Value: bson.D{
					{Key: "$regex", Value: regexp.QuoteMeta(params.RequestedModel)},
					{Key: "$options", Value: "i"},
				}}},
				bson.D{{Key: "model", Value: bson.D{
					{Key: "$regex", Value: regexp.QuoteMeta(params.RequestedModel)},
					{Key: "$options", Value: "i"},
				}}},
			},
		})
	}
	if params.Provider != "" {
		regex := bson.D{
			{Key: "$regex", Value: regexp.QuoteMeta(params.Provider)},
			{Key: "$options", Value: "i"},
		}
		matchFilters = append(matchFilters, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "provider", Value: regex}},
			bson.D{{Key: "provider_name", Value: regex}},
		}})
	}
	if params.Method != "" {
		matchFilters = append(matchFilters, bson.E{Key: "method", Value: params.Method})
	}
	if params.Path != "" {
		matchFilters = append(matchFilters, bson.E{
			Key: "path",
			Value: bson.D{
				{Key: "$regex", Value: regexp.QuoteMeta(params.Path)},
				{Key: "$options", Value: "i"},
			},
		})
	}
	if userPath, err := normalizeAuditUserPathFilter(params.UserPath); err != nil {
		return nil, core.NewInvalidRequestError(err.Error(), err)
	} else if userPath != "" {
		matchFilters = append(matchFilters, mongoUserPathFilter(userPath))
	}
	if params.ErrorType != "" {
		matchFilters = append(matchFilters, bson.E{
			Key: "error_type",
			Value: bson.D{
				{Key: "$regex", Value: regexp.QuoteMeta(params.ErrorType)},
				{Key: "$options", Value: "i"},
			},
		})
	}
	if params.AuthKeyID != "" {
		matchFilters = append(matchFilters, bson.E{Key: "auth_key_id", Value: params.AuthKeyID})
	}
	if params.StatusCode != nil {
		matchFilters = append(matchFilters, bson.E{Key: "status_code", Value: *params.StatusCode})
	}
	if params.Stream != nil {
		matchFilters = append(matchFilters, bson.E{Key: "stream", Value: *params.Stream})
	}
	if params.Search != "" {
		pattern := regexp.QuoteMeta(params.Search)
		regex := bson.D{{Key: "$regex", Value: pattern}, {Key: "$options", Value: "i"}}
		matchFilters = append(matchFilters, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "request_id", Value: regex}},
			bson.D{{Key: "auth_key_id", Value: regex}},
			bson.D{{Key: "requested_model", Value: regex}},
			bson.D{{Key: "model", Value: regex}},
			bson.D{{Key: "provider", Value: regex}},
			bson.D{{Key: "provider_name", Value: regex}},
			bson.D{{Key: "method", Value: regex}},
			bson.D{{Key: "path", Value: regex}},
			bson.D{{Key: "user_path", Value: regex}},
			bson.D{{Key: "error_type", Value: regex}},
			bson.D{{Key: "data.error_message", Value: regex}},
		}})
	}

	pipeline := bson.A{}
	if len(matchFilters) > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: matchFilters}})
	}

	sortDirection := 1
	if sortSpec.Desc {
		sortDirection = -1
	}

	pipeline = append(pipeline, bson.D{{Key: "$facet", Value: bson.D{
		{Key: "data", Value: bson.A{
			bson.D{{Key: "$sort", Value: bson.D{{Key: sortSpec.Field, Value: sortDirection}, {Key: "_id", Value: sortDirection}}}},
			bson.D{{Key: "$skip", Value: offset}},
			bson.D{{Key: "$limit", Value: limit}},
		}},
		{Key: "total", Value: bson.A{
			bson.D{{Key: "$count", Value: "count"}},
		}},
	}}})

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate audit logs: %w", err)
	}
	defer cursor.Close(ctx)

	var facetResult struct {
		Data  []mongoRow `bson:"data"`
		Total []struct {
			Count int `bson:"count"`
		} `bson:"total"`
	}

	if cursor.Next(ctx) {
		if err := cursor.Decode(&facetResult); err != nil {
			return nil, fmt.Errorf("failed to decode audit log facet result: %w", err)
		}
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit log cursor: %w", err)
	}

	total := 0
	if len(facetResult.Total) > 0 {
		total = facetResult.Total[0].Count
	}

	entries := make([]LogEntry, 0, len(facetResult.Data))
	for _, row := range facetResult.Data {
		entry := row.toLogEntry()
		if entry != nil {
			entries = append(entries, *entry)
		}
	}

	var stats *LogStats
	if params.IncludeStats {
		stats, err = r.mongoStats(ctx, matchFilters)
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

func (r *MongoDBReader) mongoStats(ctx context.Context, matchFilters bson.D) (*LogStats, error) {
	filter := any(bson.D{})
	if len(matchFilters) > 0 {
		filter = matchFilters
	}
	opts := options.Find().SetProjection(bson.D{
		{Key: "timestamp", Value: 1},
		{Key: "duration_ns", Value: 1},
		{Key: "requested_model", Value: 1},
		{Key: "model", Value: 1},
		{Key: "provider", Value: 1},
		{Key: "provider_name", Value: 1},
		{Key: "status_code", Value: 1},
		{Key: "method", Value: 1},
		{Key: "stream", Value: 1},
		{Key: "error_type", Value: 1},
	})
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit log stats: %w", err)
	}
	defer cursor.Close(ctx)

	stats := &LogStats{
		StatusBuckets:    []BucketCount{},
		MethodBuckets:    []BucketCount{},
		ProviderBuckets:  []BucketCount{},
		ModelBuckets:     []BucketCount{},
		ErrorTypeBuckets: []BucketCount{},
	}
	statusCounts := map[string]int{}
	methodCounts := map[string]int{}
	providerCounts := map[string]int{}
	modelCounts := map[string]int{}
	errorTypeCounts := map[string]int{}
	var durationTotal int64

	for cursor.Next(ctx) {
		var row mongoRow
		if err := cursor.Decode(&row); err != nil {
			return nil, fmt.Errorf("failed to decode audit log stats row: %w", err)
		}
		entry := row.toLogEntry()
		if entry == nil {
			continue
		}
		stats.Total++
		if entry.StatusCode >= 200 && entry.StatusCode < 300 {
			stats.SuccessCount++
		}
		if entry.StatusCode >= 300 && entry.StatusCode < 400 {
			stats.RedirectCount++
		}
		if entry.StatusCode >= 400 && entry.StatusCode < 500 {
			stats.ClientErrorCount++
		}
		if entry.StatusCode >= 500 {
			stats.ServerErrorCount++
		}
		if entry.StatusCode >= 400 || entry.ErrorType != "" {
			stats.ErrorCount++
		}
		if entry.Stream {
			stats.StreamCount++
		}
		if stats.Total == 1 || entry.DurationNs < stats.MinDurationNs {
			stats.MinDurationNs = entry.DurationNs
		}
		if entry.DurationNs > stats.MaxDurationNs {
			stats.MaxDurationNs = entry.DurationNs
		}
		durationTotal += entry.DurationNs
		statusCounts[auditStatusBucket(entry.StatusCode)]++
		methodCounts[pickFirstNonEmpty(entry.Method, "unknown")]++
		providerCounts[pickFirstNonEmpty(entry.ProviderName, entry.Provider, "unknown")]++
		modelCounts[pickFirstNonEmpty(entry.RequestedModel, "unknown")]++
		if entry.ErrorType != "" {
			errorTypeCounts[entry.ErrorType]++
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit log stats rows: %w", err)
	}
	if stats.Total > 0 {
		stats.AvgDurationNs = float64(durationTotal) / float64(stats.Total)
	}
	stats.StatusBuckets = bucketCountsFromMap(statusCounts, 0)
	stats.MethodBuckets = bucketCountsFromMap(methodCounts, 8)
	stats.ProviderBuckets = bucketCountsFromMap(providerCounts, 8)
	stats.ModelBuckets = bucketCountsFromMap(modelCounts, 8)
	stats.ErrorTypeBuckets = bucketCountsFromMap(errorTypeCounts, 8)
	return stats, nil
}

func pickFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// GetLogByID returns a single audit log entry by ID.
func (r *MongoDBReader) GetLogByID(ctx context.Context, id string) (*LogEntry, error) {
	var row mongoRow

	err := r.collection.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&row)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query audit log by id: %w", err)
	}

	return row.toLogEntry(), nil
}

// GetConversation returns a linear conversation thread around a seed log entry.
func (r *MongoDBReader) GetConversation(ctx context.Context, logID string, limit int) (*ConversationResult, error) {
	return buildConversationThread(ctx, logID, limit, r.GetLogByID, r.findByResponseID, r.findByPreviousResponseID)
}

func mongoDateFilter(params QueryParams) bson.D {
	startZero := params.StartDate.IsZero()
	endZero := params.EndDate.IsZero()

	if !startZero && !endZero {
		return bson.D{{Key: "$gte", Value: params.StartDate.UTC()}, {Key: "$lt", Value: params.EndDate.AddDate(0, 0, 1).UTC()}}
	}
	if !startZero {
		return bson.D{{Key: "$gte", Value: params.StartDate.UTC()}}
	}
	if !endZero {
		return bson.D{{Key: "$lt", Value: params.EndDate.AddDate(0, 0, 1).UTC()}}
	}
	return nil
}

func (r *MongoDBReader) findByResponseID(ctx context.Context, responseID string) (*LogEntry, error) {
	return r.findFirstByField(ctx, "data.response_body.id", responseID, "response_id")
}

func (r *MongoDBReader) findByPreviousResponseID(ctx context.Context, previousResponseID string) (*LogEntry, error) {
	return r.findFirstByField(ctx, "data.request_body.previous_response_id", previousResponseID, "previous_response_id")
}

func (r *MongoDBReader) findFirstByField(ctx context.Context, field string, value any, label string) (*LogEntry, error) {
	filter := bson.D{{Key: field, Value: value}}
	opts := options.FindOne().SetSort(bson.D{{Key: "timestamp", Value: 1}})

	var row mongoRow

	if err := r.collection.FindOne(ctx, filter, opts).Decode(&row); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query audit log by %s: %w", label, err)
	}

	return row.toLogEntry(), nil
}

// AggregateAuthKeyDaily groups audit rows for one auth key by local calendar day
// using a MongoDB aggregate pipeline that buckets timestamps with $dateToString.
func (r *MongoDBReader) AggregateAuthKeyDaily(ctx context.Context, authKeyID string, start, end time.Time, location *time.Location) (*AuthKeyDailyAggregate, error) {
	authKeyID = trimmedAuthKeyID(authKeyID)
	if authKeyID == "" {
		return dailyAggregateFromBuckets(start, end, location, nil), nil
	}
	if location == nil {
		location = time.UTC
	}
	startBoundary := dailyAggregateInclusiveStart(start, location).UTC()
	endBoundary := dailyAggregateExclusiveEnd(end, location).UTC()

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "auth_key_id", Value: authKeyID},
			{Key: "timestamp", Value: bson.D{
				{Key: "$gte", Value: startBoundary},
				{Key: "$lt", Value: endBoundary},
			}},
		}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "$dateToString", Value: bson.D{
					{Key: "format", Value: "%Y-%m-%d"},
					{Key: "date", Value: "$timestamp"},
					{Key: "timezone", Value: location.String()},
				}},
			}},
			{Key: "requests", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "errors", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{
					bson.D{{Key: "$or", Value: bson.A{
						bson.D{{Key: "$gte", Value: bson.A{"$status_code", 400}}},
						bson.D{{Key: "$ne", Value: bson.A{bson.D{{Key: "$ifNull", Value: bson.A{"$error_type", ""}}}, ""}}},
					}}},
					1,
					0,
				}},
			}}}},
			{Key: "exact_hits", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$cache_type", "exact"}}}, 1, 0}},
			}}}},
			{Key: "semantic_hits", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$cache_type", "semantic"}}}, 1, 0}},
			}}}},
		}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate auth key daily: %w", err)
	}
	defer cursor.Close(ctx)

	observed := make(map[string]AuthKeyDailyBucket)
	for cursor.Next(ctx) {
		var row struct {
			Date         string `bson:"_id"`
			Requests     int    `bson:"requests"`
			Errors       int    `bson:"errors"`
			ExactHits    int    `bson:"exact_hits"`
			SemanticHits int    `bson:"semantic_hits"`
		}
		if err := cursor.Decode(&row); err != nil {
			return nil, fmt.Errorf("aggregate auth key daily: decode row: %w", err)
		}
		observed[row.Date] = AuthKeyDailyBucket{
			Date:         row.Date,
			Requests:     row.Requests,
			Errors:       row.Errors,
			ExactHits:    row.ExactHits,
			SemanticHits: row.SemanticHits,
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("aggregate auth key daily: iterate rows: %w", err)
	}
	return dailyAggregateFromBuckets(start, end, location, observed), nil
}

package auditlog

import (
	"context"
	"time"
)

// QueryParams specifies the date range for audit log retrieval.
type QueryParams struct {
	StartDate time.Time // Inclusive start (day precision)
	EndDate   time.Time // Inclusive end (day precision)
}

// LogQueryParams specifies query parameters for paginated audit log retrieval.
type LogQueryParams struct {
	QueryParams
	RequestedModel string
	Provider       string // filter by provider name or provider type
	Method         string
	Path           string
	UserPath       string
	AuthKeyID      string // filter by exact managed auth key id
	ErrorType      string
	Search         string
	StatusCode     *int
	Stream         *bool
	Sort           string
	IncludeStats   bool
	Limit          int
	Offset         int
}

type BucketCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type LogStats struct {
	Total            int           `json:"total"`
	SuccessCount     int           `json:"success_count"`
	RedirectCount    int           `json:"redirect_count"`
	ClientErrorCount int           `json:"client_error_count"`
	ServerErrorCount int           `json:"server_error_count"`
	ErrorCount       int           `json:"error_count"`
	StreamCount      int           `json:"stream_count"`
	MinDurationNs    int64         `json:"min_duration_ns,omitempty"`
	AvgDurationNs    float64       `json:"avg_duration_ns,omitempty"`
	MaxDurationNs    int64         `json:"max_duration_ns,omitempty"`
	StatusBuckets    []BucketCount `json:"status_buckets"`
	MethodBuckets    []BucketCount `json:"method_buckets"`
	ProviderBuckets  []BucketCount `json:"provider_buckets"`
	ModelBuckets     []BucketCount `json:"model_buckets"`
	ErrorTypeBuckets []BucketCount `json:"error_type_buckets"`
}

// LogListResult holds a paginated list of audit log entries.
type LogListResult struct {
	Entries []LogEntry `json:"entries"`
	Total   int        `json:"total"`
	Limit   int        `json:"limit"`
	Offset  int        `json:"offset"`
	Sort    string     `json:"sort,omitempty"`
	Stats   *LogStats  `json:"stats,omitempty"`
}

// ConversationResult holds a linear conversation thread centered around an anchor log.
type ConversationResult struct {
	AnchorID string     `json:"anchor_id"`
	Entries  []LogEntry `json:"entries"`
}

// AuthKeyDailyBucket is a single day's aggregate for one managed auth key.
// Date is the calendar day in the caller's timezone (YYYY-MM-DD).
type AuthKeyDailyBucket struct {
	Date         string
	Requests     int
	Errors       int
	ExactHits    int
	SemanticHits int
}

// AuthKeyDailyAggregate is the rollup returned by AggregateAuthKeyDaily.
// Buckets covers every day in [start,end] in calendar order with zero-filled
// gaps so the caller never needs to fill in missing days.
type AuthKeyDailyAggregate struct {
	Buckets      []AuthKeyDailyBucket
	TotalEntries int
	ExactHits    int
	SemanticHits int
	Errors       int
}

// Reader provides read access to audit log data for the admin API.
type Reader interface {
	// GetLogs returns a paginated list of audit log entries with optional filtering.
	GetLogs(ctx context.Context, params LogQueryParams) (*LogListResult, error)

	// GetLogByID returns a single audit log entry by ID.
	// Returns (nil, nil) when no entry exists for the given ID.
	GetLogByID(ctx context.Context, id string) (*LogEntry, error)

	// GetConversation returns a linear conversation thread around a seed log entry.
	// It follows Responses API linkage fields when available:
	// request_body.previous_response_id and response_body.id.
	GetConversation(ctx context.Context, logID string, limit int) (*ConversationResult, error)

	// AggregateAuthKeyDaily returns per-day request / error / cache-hit counts
	// for a single managed auth key, bucketed in the supplied location. start
	// and end must be midnight in that location; the returned slice has one
	// entry per calendar day in [start, end] (inclusive) with zero-filled gaps.
	AggregateAuthKeyDaily(ctx context.Context, authKeyID string, start, end time.Time, location *time.Location) (*AuthKeyDailyAggregate, error)
}

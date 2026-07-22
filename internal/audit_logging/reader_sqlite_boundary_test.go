package auditlog

import (
	"context"
	"testing"
	"time"
)

func TestSQLiteReaderGetLogs_IncludesFractionalStartBoundaryAndExcludesFractionalEndBoundary(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	store, err := NewSQLiteStore(db, 0)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	err = store.WriteBatch(ctx, []*LogEntry{
		{
			ID:             "start-boundary",
			Timestamp:      time.Date(2026, 1, 15, 23, 0, 0, 123_000_000, time.UTC),
			RequestedModel: "gpt-5",
			Provider:       "openai",
		},
		{
			ID:             "inside-range",
			Timestamp:      time.Date(2026, 1, 16, 12, 0, 0, 0, time.UTC),
			RequestedModel: "gpt-5",
			Provider:       "openai",
		},
		{
			ID:             "after-end-boundary",
			Timestamp:      time.Date(2026, 1, 16, 23, 0, 0, 123_000_000, time.UTC),
			RequestedModel: "gpt-5",
			Provider:       "openai",
		},
	})
	if err != nil {
		t.Fatalf("failed to seed audit logs: %v", err)
	}

	reader, err := NewSQLiteReader(db)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("failed to load location: %v", err)
	}

	result, err := reader.GetLogs(ctx, LogQueryParams{
		QueryParams: QueryParams{
			StartDate: time.Date(2026, 1, 16, 0, 0, 0, 0, location),
			EndDate:   time.Date(2026, 1, 16, 0, 0, 0, 0, location),
		},
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("GetLogs returned error: %v", err)
	}

	if result.Total != 2 {
		t.Fatalf("expected 2 logs in range, got %d", result.Total)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 returned entries, got %d", len(result.Entries))
	}
	if result.Entries[0].ID != "inside-range" {
		t.Fatalf("expected latest in-range entry %q, got %q", "inside-range", result.Entries[0].ID)
	}
	if result.Entries[1].ID != "start-boundary" {
		t.Fatalf("expected boundary entry %q, got %q", "start-boundary", result.Entries[1].ID)
	}
}

func TestSQLiteReaderGetLogs_SearchMatchesUserPath(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	store, err := NewSQLiteStore(db, 0)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	if err := store.WriteBatch(ctx, []*LogEntry{
		{
			ID:             "team-match",
			Timestamp:      time.Date(2026, 1, 16, 12, 0, 0, 0, time.UTC),
			RequestedModel: "gpt-5",
			Provider:       "openai",
			UserPath:       "/team/alpha",
		},
		{
			ID:             "other-team",
			Timestamp:      time.Date(2026, 1, 16, 11, 0, 0, 0, time.UTC),
			RequestedModel: "gpt-5",
			Provider:       "openai",
			UserPath:       "/org/beta",
		},
	}); err != nil {
		t.Fatalf("failed to seed audit logs: %v", err)
	}

	reader, err := NewSQLiteReader(db)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	result, err := reader.GetLogs(ctx, LogQueryParams{
		Search: "team/alpha",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("GetLogs returned error: %v", err)
	}

	if result.Total != 1 {
		t.Fatalf("expected 1 log in search result, got %d", result.Total)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 returned entry, got %d", len(result.Entries))
	}
	if result.Entries[0].ID != "team-match" {
		t.Fatalf("expected matching entry %q, got %q", "team-match", result.Entries[0].ID)
	}
}

func TestSQLiteReaderGetLogByIDRedactsStoredHeaders(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	store, err := NewSQLiteStore(db, 0)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	if err := store.WriteBatch(ctx, []*LogEntry{
		{
			ID:             "legacy-secret-headers",
			Timestamp:      time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC),
			RequestedModel: "gpt-5",
			Provider:       "openai",
			Data: &LogData{
				RequestHeaders: map[string]string{
					"Authorization": "Bearer secret",
					"X-Test":        "ok",
				},
				ResponseHeaders: map[string]string{
					"Set-Cookie": "session=secret",
					"Server":     "gateway",
				},
			},
		},
	}); err != nil {
		t.Fatalf("failed to seed audit logs: %v", err)
	}

	reader, err := NewSQLiteReader(db)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	entry, err := reader.GetLogByID(ctx, "legacy-secret-headers")
	if err != nil {
		t.Fatalf("GetLogByID returned error: %v", err)
	}
	if entry == nil || entry.Data == nil {
		t.Fatal("expected entry data")
	}
	if got := entry.Data.RequestHeaders["Authorization"]; got != "[REDACTED]" {
		t.Fatalf("Authorization header = %q, want [REDACTED]", got)
	}
	if got := entry.Data.RequestHeaders["X-Test"]; got != "ok" {
		t.Fatalf("X-Test header = %q, want ok", got)
	}
	if got := entry.Data.ResponseHeaders["Set-Cookie"]; got != "[REDACTED]" {
		t.Fatalf("Set-Cookie header = %q, want [REDACTED]", got)
	}
	if got := entry.Data.ResponseHeaders["Server"]; got != "gateway" {
		t.Fatalf("Server header = %q, want gateway", got)
	}
}

func TestSQLiteReaderGetLogs_SortsByDurationDescending(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	store, err := NewSQLiteStore(db, 0)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	if err := store.WriteBatch(ctx, []*LogEntry{
		{ID: "fast", Timestamp: time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC), DurationNs: 10, RequestedModel: "gpt-5", Provider: "openai"},
		{ID: "slow", Timestamp: time.Date(2026, 1, 16, 10, 1, 0, 0, time.UTC), DurationNs: 300, RequestedModel: "gpt-5", Provider: "openai"},
		{ID: "medium", Timestamp: time.Date(2026, 1, 16, 10, 2, 0, 0, time.UTC), DurationNs: 200, RequestedModel: "gpt-5", Provider: "openai"},
	}); err != nil {
		t.Fatalf("failed to seed audit logs: %v", err)
	}

	reader, err := NewSQLiteReader(db)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	result, err := reader.GetLogs(ctx, LogQueryParams{Sort: "-duration_ns", Limit: 10})
	if err != nil {
		t.Fatalf("GetLogs returned error: %v", err)
	}

	got := []string{result.Entries[0].ID, result.Entries[1].ID, result.Entries[2].ID}
	want := []string{"slow", "medium", "fast"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entry order = %v, want %v", got, want)
		}
	}
	if result.Sort != "-duration_ns" {
		t.Fatalf("Sort = %q, want -duration_ns", result.Sort)
	}
}

func TestSQLiteReaderGetLogs_IncludesStatsForFilteredResult(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	store, err := NewSQLiteStore(db, 0)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	if err := store.WriteBatch(ctx, []*LogEntry{
		{ID: "ok-openai", Timestamp: time.Date(2026, 1, 16, 10, 0, 0, 0, time.UTC), DurationNs: 100, RequestedModel: "gpt-4o", Provider: "openai", ProviderName: "primary-openai", StatusCode: 200, Method: "POST", Stream: true},
		{ID: "rate-limit", Timestamp: time.Date(2026, 1, 16, 11, 0, 0, 0, time.UTC), DurationNs: 200, RequestedModel: "gpt-4o", Provider: "openai", ProviderName: "primary-openai", StatusCode: 429, Method: "POST", ErrorType: "rate_limit_error"},
		{ID: "server-error", Timestamp: time.Date(2026, 1, 16, 12, 0, 0, 0, time.UTC), DurationNs: 300, RequestedModel: "claude", Provider: "anthropic", StatusCode: 503, Method: "GET", ErrorType: "provider_error"},
	}); err != nil {
		t.Fatalf("failed to seed audit logs: %v", err)
	}

	reader, err := NewSQLiteReader(db)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	result, err := reader.GetLogs(ctx, LogQueryParams{Provider: "openai", IncludeStats: true, Limit: 1})
	if err != nil {
		t.Fatalf("GetLogs returned error: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected paginated entries length 1, got %d", len(result.Entries))
	}
	if result.Stats == nil {
		t.Fatal("expected stats")
	}
	if result.Stats.Total != 2 {
		t.Fatalf("stats total = %d, want 2", result.Stats.Total)
	}
	if result.Stats.StreamCount != 1 || result.Stats.ErrorCount != 1 || result.Stats.ClientErrorCount != 1 {
		t.Fatalf("stats = %+v, want stream_count=1 error_count=1 client_error_count=1", result.Stats)
	}
	assertBucketCount(t, result.Stats.ProviderBuckets, "primary-openai", 2)
	assertBucketCount(t, result.Stats.StatusBuckets, "2xx", 1)
	assertBucketCount(t, result.Stats.StatusBuckets, "4xx", 1)
	assertBucketCount(t, result.Stats.ErrorTypeBuckets, "rate_limit_error", 1)
}

func assertBucketCount(t *testing.T, buckets []BucketCount, label string, count int) {
	t.Helper()
	for _, bucket := range buckets {
		if bucket.Label == label {
			if bucket.Count != count {
				t.Fatalf("bucket %q count = %d, want %d", label, bucket.Count, count)
			}
			return
		}
	}
	t.Fatalf("bucket %q not found in %+v", label, buckets)
}

func TestSQLiteReaderGetLogs_SearchMatchesErrorMessage(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	store, err := NewSQLiteStore(db, 0)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	if err := store.WriteBatch(ctx, []*LogEntry{
		{
			ID:             "timeout-match",
			Timestamp:      time.Date(2026, 1, 16, 12, 0, 0, 0, time.UTC),
			RequestedModel: "gpt-5",
			Provider:       "openai",
			ErrorType:      "provider_error",
			Data: &LogData{
				ErrorMessage: `failed to send request: Post "https://api.openai.com/v1/chat/completions": http2: timeout awaiting response headers`,
			},
		},
		{
			ID:             "other-error",
			Timestamp:      time.Date(2026, 1, 16, 11, 0, 0, 0, time.UTC),
			RequestedModel: "gpt-5",
			Provider:       "openai",
			ErrorType:      "provider_error",
			Data: &LogData{
				ErrorMessage: "upstream refused connection",
			},
		},
	}); err != nil {
		t.Fatalf("failed to seed audit logs: %v", err)
	}

	reader, err := NewSQLiteReader(db)
	if err != nil {
		t.Fatalf("failed to create sqlite reader: %v", err)
	}

	result, err := reader.GetLogs(ctx, LogQueryParams{
		Search: "timeout awaiting response headers",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("GetLogs returned error: %v", err)
	}

	if result.Total != 1 {
		t.Fatalf("expected 1 log in search result, got %d", result.Total)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 returned entry, got %d", len(result.Entries))
	}
	if result.Entries[0].ID != "timeout-match" {
		t.Fatalf("expected matching entry %q, got %q", "timeout-match", result.Entries[0].ID)
	}
}

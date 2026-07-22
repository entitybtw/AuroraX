package auditlog

import (
	"strings"
	"time"
)

// fillDailyBuckets returns one bucket per calendar day in [start, end]
// (inclusive, in location). Days present in observed keep their counts;
// days missing are zero-filled so the caller can render a continuous chart.
func fillDailyBuckets(start, end time.Time, location *time.Location, observed map[string]AuthKeyDailyBucket) []AuthKeyDailyBucket {
	if location == nil {
		location = time.UTC
	}
	startLocal := start.In(location)
	endLocal := end.In(location)
	startDay := time.Date(startLocal.Year(), startLocal.Month(), startLocal.Day(), 0, 0, 0, 0, location)
	endDay := time.Date(endLocal.Year(), endLocal.Month(), endLocal.Day(), 0, 0, 0, 0, location)

	days := int(endDay.Sub(startDay).Hours()/24) + 1
	if days <= 0 {
		days = 1
	}
	out := make([]AuthKeyDailyBucket, 0, days)
	cursor := startDay
	for !cursor.After(endDay) {
		key := cursor.Format("2006-01-02")
		if bucket, ok := observed[key]; ok {
			bucket.Date = key
			out = append(out, bucket)
		} else {
			out = append(out, AuthKeyDailyBucket{Date: key})
		}
		cursor = cursor.AddDate(0, 0, 1)
	}
	return out
}

// dailyAggregateFromBuckets totals up an observed map and zero-fills the date
// range. Used by every backend after it has populated `observed`.
func dailyAggregateFromBuckets(start, end time.Time, location *time.Location, observed map[string]AuthKeyDailyBucket) *AuthKeyDailyAggregate {
	agg := &AuthKeyDailyAggregate{}
	for _, b := range observed {
		agg.TotalEntries += b.Requests
		agg.Errors += b.Errors
		agg.ExactHits += b.ExactHits
		agg.SemanticHits += b.SemanticHits
	}
	agg.Buckets = fillDailyBuckets(start, end, location, observed)
	return agg
}

// dailyAggregateExclusiveEnd shifts an inclusive end-of-day to the next
// midnight, which is the boundary value SQL backends use for `timestamp < ?`.
func dailyAggregateExclusiveEnd(end time.Time, location *time.Location) time.Time {
	if location == nil {
		location = time.UTC
	}
	local := end.In(location)
	startOfNext := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location).AddDate(0, 0, 1)
	return startOfNext
}

// dailyAggregateInclusiveStart pins the start time to local midnight so
// "today minus N days" buckets correctly even when callers pass a non-midnight
// value.
func dailyAggregateInclusiveStart(start time.Time, location *time.Location) time.Time {
	if location == nil {
		location = time.UTC
	}
	local := start.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

// trimmedAuthKeyID returns the auth key id with surrounding whitespace removed
// so a stray space (e.g. from URL decoding) does not silently match zero rows.
func trimmedAuthKeyID(authKeyID string) string {
	return strings.TrimSpace(authKeyID)
}

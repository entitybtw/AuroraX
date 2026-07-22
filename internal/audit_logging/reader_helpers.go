package auditlog

import (
	"fmt"
	"sort"
	"strings"
)

func buildWhereClause(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conditions, " AND ")
}

func escapeLikeWildcards(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func clampLimitOffset(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

type auditSortSpec struct {
	Public string
	Field  string
	Desc   bool
}

var auditSortFields = map[string]string{
	"timestamp":           "timestamp",
	"duration_ns":         "duration_ns",
	"status_code":         "status_code",
	"requested_model":     "requested_model",
	"resolved_model":      "resolved_model",
	"provider":            "provider",
	"provider_name":       "provider_name",
	"method":              "method",
	"path":                "path",
	"user_path":           "user_path",
	"error_type":          "error_type",
	"request_id":          "request_id",
	"auth_key_id":         "auth_key_id",
	"auth_method":         "auth_method",
	"stream":              "stream",
	"cache_type":          "cache_type",
	"workflow_version_id": "workflow_version_id",
}

func normalizeAuditSort(value string) (auditSortSpec, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = "-timestamp"
	}
	desc := false
	field := trimmed
	if strings.HasPrefix(field, "-") {
		desc = true
		field = strings.TrimPrefix(field, "-")
	}
	field = strings.TrimSpace(field)
	mapped, ok := auditSortFields[field]
	if !ok || field == "" {
		return auditSortSpec{}, fmt.Errorf("invalid audit sort field %q", value)
	}
	public := field
	if desc {
		public = "-" + public
	}
	return auditSortSpec{Public: public, Field: mapped, Desc: desc}, nil
}

func auditSQLOrderBy(spec auditSortSpec) string {
	direction := "ASC"
	if spec.Desc {
		direction = "DESC"
	}
	return fmt.Sprintf(" ORDER BY %s %s, id %s", spec.Field, direction, direction)
}

func bucketCountsFromMap(counts map[string]int, limit int) []BucketCount {
	if len(counts) == 0 {
		return []BucketCount{}
	}
	normalized := make(map[string]int, len(counts))
	for label, count := range counts {
		label = strings.TrimSpace(label)
		if label == "" {
			label = "unknown"
		}
		normalized[label] += count
	}
	buckets := make([]BucketCount, 0, len(normalized))
	for label, count := range normalized {
		buckets = append(buckets, BucketCount{Label: label, Count: count})
	}
	sort.Slice(buckets, func(i, j int) bool {
		if buckets[i].Count != buckets[j].Count {
			return buckets[i].Count > buckets[j].Count
		}
		return buckets[i].Label < buckets[j].Label
	})
	if limit > 0 && len(buckets) > limit {
		return buckets[:limit]
	}
	return buckets
}

func auditStatusBucket(status int) string {
	if status >= 500 {
		return "5xx"
	}
	if status >= 400 {
		return "4xx"
	}
	if status >= 300 {
		return "3xx"
	}
	if status >= 200 {
		return "2xx"
	}
	return "unknown"
}

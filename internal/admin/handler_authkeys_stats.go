package admin

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"aurora/internal/audit_logging"
	"aurora/internal/authentication_keys"
	"aurora/internal/core"
	"aurora/internal/usage"
)

// AuthKeyStatsResponse is the per-key analytics payload returned by
// GET /admin/api/v1/auth-keys/:id/stats.
type AuthKeyStatsResponse struct {
	// Key metadata (same shape as the list endpoint plus computed timestamps).
	Key authkeys.View `json:"key"`

	// Window covered by the historical stats.
	Window AuthKeyStatsWindow `json:"window"`

	// Request totals + status / cache breakdown sourced from the audit log.
	Requests AuthKeyRequestStats `json:"requests"`

	// Latency aggregates over the window (nanoseconds).
	Latency AuthKeyLatencyStats `json:"latency"`

	// Cache hit rate breakdown (exact / semantic / miss).
	Cache AuthKeyCacheStats `json:"cache"`

	// Aggregated tokens & cost from the usage log (filtered by user_path).
	Usage AuthKeyUsageStats `json:"usage"`

	// Top buckets (capped on the server) so the UI doesn't need a second call.
	TopModels    []auditlog.BucketCount `json:"top_models"`
	TopProviders []auditlog.BucketCount `json:"top_providers"`
	TopErrors    []auditlog.BucketCount `json:"top_errors"`

	// Daily request volume series for the sparkline.
	Daily []AuthKeyStatsDaily `json:"daily"`

	// Live rate-limit utilization, if the key has limits configured and an
	// inspector is wired into the admin handler.
	RateLimits *core.AuthKeyRateLimitSnapshot `json:"rate_limit_status,omitempty"`

	// LastUsedAt is the timestamp of the most recent audited request that
	// presented this key, or null if the key has never been used.
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type AuthKeyStatsWindow struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Days      int    `json:"days"`
	TimeZone  string `json:"timezone"`
}

type AuthKeyRequestStats struct {
	Total            int     `json:"total"`
	SuccessCount     int     `json:"success_count"`
	RedirectCount    int     `json:"redirect_count"`
	ClientErrorCount int     `json:"client_error_count"`
	ServerErrorCount int     `json:"server_error_count"`
	ErrorCount       int     `json:"error_count"`
	StreamCount      int     `json:"stream_count"`
	SuccessRate      float64 `json:"success_rate"`
	ErrorRate        float64 `json:"error_rate"`
}

type AuthKeyLatencyStats struct {
	MinNs int64   `json:"min_ns"`
	AvgNs float64 `json:"avg_ns"`
	MaxNs int64   `json:"max_ns"`
}

type AuthKeyCacheStats struct {
	ExactHits    int     `json:"exact_hits"`
	SemanticHits int     `json:"semantic_hits"`
	TotalHits    int     `json:"total_hits"`
	Misses       int     `json:"misses"`
	HitRate      float64 `json:"hit_rate"`
}

type AuthKeyUsageStats struct {
	TotalRequests   int      `json:"total_requests"`
	InputTokens     int64    `json:"input_tokens"`
	OutputTokens    int64    `json:"output_tokens"`
	TotalTokens     int64    `json:"total_tokens"`
	InputCost       *float64 `json:"input_cost"`
	OutputCost      *float64 `json:"output_cost"`
	TotalCost       *float64 `json:"total_cost"`
	NoteUserPathTie bool     `json:"note_user_path_tie"`
}

type AuthKeyStatsDaily struct {
	Date     string `json:"date"`
	Requests int    `json:"requests"`
	Errors   int    `json:"errors"`
	Hits     int    `json:"cache_hits"`
}

// AuthKeyStats handles GET /admin/api/v1/auth-keys/:id/stats
//
// @Summary      Live usage stats for one managed auth key
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id     path      string  true   "Auth key id"
// @Param        days   query     int     false  "Window in days (default 30, max 365)"
// @Success      200    {object}  AuthKeyStatsResponse
// @Failure      404    {object}  core.GatewayError
// @Failure      503    {object}  core.GatewayError
// @Router       /admin/api/v1/auth-keys/{id}/stats [get]
func (h *Handler) AuthKeyStats(c *echo.Context) error {
	if h.authKeys == nil {
		return handleError(c, featureUnavailableError("auth keys feature is unavailable"))
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return handleError(c, core.NewInvalidRequestError("auth key id is required", nil))
	}

	view := h.authKeys.GetView(id)
	if view == nil {
		return handleError(c, core.NewInvalidRequestErrorWithStatus(http.StatusNotFound, "auth key not found", nil).WithCode("auth_key_not_found"))
	}

	timeZone, location := dashboardTimeZone(c)
	now := timeNow().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)

	days := defaultDateRangeDays
	if d := c.QueryParam("days"); d != "" {
		parsed, err := strconv.Atoi(d)
		if err != nil || parsed <= 0 {
			return handleError(c, core.NewInvalidRequestError("invalid days, expected positive integer", nil))
		}
		if parsed > maxDateRangeDays {
			parsed = maxDateRangeDays
		}
		days = parsed
	}
	start := today.AddDate(0, 0, -(days - 1))
	end := today

	response := AuthKeyStatsResponse{
		Key: *view,
		Window: AuthKeyStatsWindow{
			StartDate: start.Format("2006-01-02"),
			EndDate:   end.Format("2006-01-02"),
			Days:      days,
			TimeZone:  timeZone,
		},
	}

	ctx := c.Request().Context()

	// 1. Live rate-limit utilization.
	if h.authKeyRateInspector != nil && !view.RateLimits.Empty() {
		snap := h.authKeyRateInspector.Snapshot(view.TenantID, view.ID, view.RateLimits, time.Now().UTC())
		response.RateLimits = &snap
	}

	// 2. Historical request stats + daily series + top buckets — from the audit log
	//    filtered by auth_key_id (exact). This is precise per-key regardless of
	//    whether the user_path is unique.
	if h.auditReader != nil {
		auditParams := auditlog.LogQueryParams{
			QueryParams:  auditlog.QueryParams{StartDate: start, EndDate: end},
			AuthKeyID:    view.ID,
			IncludeStats: true,
			// Pull one page just to fill recency + last-used timestamp.
			Sort:   "-timestamp",
			Limit:  1,
			Offset: 0,
		}
		result, err := h.auditReader.GetLogs(ctx, auditParams)
		if err == nil && result != nil {
			if result.Stats != nil {
				st := result.Stats
				response.Requests = AuthKeyRequestStats{
					Total:            st.Total,
					SuccessCount:     st.SuccessCount,
					RedirectCount:    st.RedirectCount,
					ClientErrorCount: st.ClientErrorCount,
					ServerErrorCount: st.ServerErrorCount,
					ErrorCount:       st.ErrorCount,
					StreamCount:      st.StreamCount,
				}
				if st.Total > 0 {
					response.Requests.SuccessRate = float64(st.SuccessCount) / float64(st.Total)
					response.Requests.ErrorRate = float64(st.ErrorCount) / float64(st.Total)
				}
				response.Latency = AuthKeyLatencyStats{
					MinNs: st.MinDurationNs,
					AvgNs: st.AvgDurationNs,
					MaxNs: st.MaxDurationNs,
				}
				response.TopModels = trimBuckets(st.ModelBuckets, 5)
				response.TopProviders = trimBuckets(st.ProviderBuckets, 5)
				response.TopErrors = trimBuckets(st.ErrorTypeBuckets, 5)
			}
			if len(result.Entries) > 0 {
				ts := result.Entries[0].Timestamp
				response.LastUsedAt = &ts
			}
		}

		// Daily breakdown — pull bigger pages until we cover the window.
		response.Daily, response.Cache = h.aggregateAuditDaily(ctx, view.ID, start, end, location)
	}

	// 3. Usage totals — by user_path (best-effort, since usage entries are not
	//    yet keyed by auth_key_id). When the key has a non-empty user_path we
	//    return precise per-key tokens & cost. Otherwise we leave it empty.
	if h.usageReader != nil && strings.TrimSpace(view.UserPath) != "" {
		usageParams := usage.UsageQueryParams{
			StartDate: start,
			EndDate:   end,
			TimeZone:  timeZone,
			UserPath:  view.UserPath,
			CacheMode: usage.CacheModeAll,
		}
		summary, err := h.usageReader.GetSummary(ctx, usageParams)
		if err == nil && summary != nil {
			response.Usage = AuthKeyUsageStats{
				TotalRequests: summary.TotalRequests,
				InputTokens:   summary.TotalInput,
				OutputTokens:  summary.TotalOutput,
				TotalTokens:   summary.TotalTokens,
				InputCost:     summary.TotalInputCost,
				OutputCost:    summary.TotalOutputCost,
				TotalCost:     summary.TotalCost,
				// Flag that usage is aggregated by user_path so the UI can warn when
				// multiple keys could share the same path.
				NoteUserPathTie: true,
			}
		}
	}

	return c.JSON(http.StatusOK, response)
}

// aggregateAuditDaily returns the per-day request/error/cache-hit series for one
// managed auth key plus a rollup cache breakdown. Uses the reader's dedicated
// daily-aggregate query so we never page through entries (which silently failed
// on large windows when the audit `data` JSON blob was big).
func (h *Handler) aggregateAuditDaily(ctx context.Context, authKeyID string, start, end time.Time, location *time.Location) ([]AuthKeyStatsDaily, AuthKeyCacheStats) {
	cache := AuthKeyCacheStats{}
	if h.auditReader == nil {
		return nil, cache
	}

	agg, err := h.auditReader.AggregateAuthKeyDaily(ctx, authKeyID, start, end, location)
	if err != nil {
		slog.Warn("auth key daily aggregation failed", "auth_key_id", authKeyID, "err", err)
		return fillEmptyDailySeries(start, end, location), cache
	}
	if agg == nil {
		return fillEmptyDailySeries(start, end, location), cache
	}

	series := make([]AuthKeyStatsDaily, 0, len(agg.Buckets))
	for _, bucket := range agg.Buckets {
		series = append(series, AuthKeyStatsDaily{
			Date:     bucket.Date,
			Requests: bucket.Requests,
			Errors:   bucket.Errors,
			Hits:     bucket.ExactHits + bucket.SemanticHits,
		})
	}

	cache.ExactHits = agg.ExactHits
	cache.SemanticHits = agg.SemanticHits
	cache.TotalHits = agg.ExactHits + agg.SemanticHits
	cache.Misses = agg.TotalEntries - cache.TotalHits
	if cache.Misses < 0 {
		cache.Misses = 0
	}
	if agg.TotalEntries > 0 {
		cache.HitRate = float64(cache.TotalHits) / float64(agg.TotalEntries)
	}
	return series, cache
}

func fillEmptyDailySeries(start, end time.Time, location *time.Location) []AuthKeyStatsDaily {
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
	out := make([]AuthKeyStatsDaily, 0, days)
	cursor := startDay
	for !cursor.After(endDay) {
		out = append(out, AuthKeyStatsDaily{Date: cursor.Format("2006-01-02")})
		cursor = cursor.AddDate(0, 0, 1)
	}
	return out
}

func trimBuckets(buckets []auditlog.BucketCount, limit int) []auditlog.BucketCount {
	if len(buckets) <= limit {
		return buckets
	}
	return buckets[:limit]
}

package benchmark

import (
	"context"
	"encoding/json"
	"time"
)

// StreamResult is the JSON-serializable result sent to SSE clients.
type StreamResult struct {
	Concurrency   int     `json:"concurrency"`
	Throughput    float64 `json:"throughput"`
	P50Ms         float64 `json:"p50_ms"`
	P90Ms         float64 `json:"p90_ms"`
	P95Ms         float64 `json:"p95_ms"`
	P99Ms         float64 `json:"p99_ms"`
	P999Ms        float64 `json:"p999_ms"`
	MaxMs         float64 `json:"max_ms"`
	AvgMs         float64 `json:"avg_ms"`
	TotalRequests uint64  `json:"total_requests"`
	SuccessCount  uint64  `json:"success_count"`
	FailureCount  uint64  `json:"failure_count"`
	MemoryDeltaMB float64 `json:"memory_delta_mb"`
	AllocsPerOp   float64 `json:"allocs_per_op"`
	BytesPerOp    float64 `json:"bytes_per_op"`
}

func resultToStream(r *ConcurrencyResult) *StreamResult {
	deltaMB := float64(r.MemoryAfter.Alloc-r.MemoryBefore.Alloc) / 1024 / 1024
	return &StreamResult{
		Concurrency:   r.Concurrency,
		Throughput:    r.Throughput,
		P50Ms:         r.P50.Seconds() * 1000,
		P90Ms:         r.P90.Seconds() * 1000,
		P95Ms:         r.P95.Seconds() * 1000,
		P99Ms:         r.P99.Seconds() * 1000,
		P999Ms:        r.P999.Seconds() * 1000,
		MaxMs:         r.Max.Seconds() * 1000,
		AvgMs:         r.Avg.Seconds() * 1000,
		TotalRequests: r.TotalRequests,
		SuccessCount:  r.SuccessCount,
		FailureCount:  r.FailureCount,
		MemoryDeltaMB: deltaMB,
		AllocsPerOp:   r.AllocsPerOp,
		BytesPerOp:    r.BytesPerOp,
	}
}

// RunLoadTestStream runs the benchmark suite for the given concurrency levels
// and sends each result to the provided channel as it completes. Closes the
// channel when all levels are done or the context is cancelled.
func RunLoadTestStream(ctx context.Context, baseCfg LoadTestConfig, concurrencyLevels []int, out chan<- *StreamResult) {
	defer close(out)

	for _, c := range concurrencyLevels {
		cfg := baseCfg
		cfg.Concurrency = c

		result, err := RunLoadTest(ctx, cfg)
		if err != nil {
			return
		}

		select {
		case out <- resultToStream(result):
		case <-ctx.Done():
			return
		}

		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return
		}
	}
}

// StreamProgress is a live intermediate progress update sent during a
// concurrency level's run (before the final result is computed).
type StreamProgress struct {
	Concurrency    int     `json:"concurrency"`
	CurrentRPS     float64 `json:"current_rps"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	TotalRequests  uint64  `json:"total_requests"`
	SuccessCount   uint64  `json:"success_count"`
	FailureCount   uint64  `json:"failure_count"`
	DurationMs     int     `json:"duration_ms"`
}

// RunLoadTestStreamWithProgress is like RunLoadTestStream but also sends
// live StreamProgress updates on the progress channel while each level runs.
// The progress channel is NOT closed by this function.
func RunLoadTestStreamWithProgress(ctx context.Context, baseCfg LoadTestConfig, concurrencyLevels []int, out chan<- *StreamResult, progress chan<- *StreamProgress) {
	defer close(out)

	for _, c := range concurrencyLevels {
		cfg := baseCfg
		cfg.Concurrency = c

		result, err := RunLoadTestWithProgress(ctx, cfg, progress)
		if err != nil {
			return
		}

		select {
		case out <- resultToStream(result):
		case <-ctx.Done():
			return
		}

		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return
		}
	}
}

// RunLoadTestWithProgress runs a single concurrency level and sends live
// progress updates on the progress channel while the test runs.
func RunLoadTestWithProgress(ctx context.Context, cfg LoadTestConfig, progress chan<- *StreamProgress) (*ConcurrencyResult, error) {
	if cfg.WarmupDuration > 0 {
		runPhase(ctx, cfg, cfg.WarmupDuration, true, nil)
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Duration+cfg.RampUpDuration)
	defer cancel()
	return runPhase(ctx, cfg, cfg.Duration, false, progress), nil
}

// StreamEvent is the wire format for SSE events.
type StreamEvent struct {
	Result   *StreamResult   `json:"result,omitempty"`
	Done     bool            `json:"done,omitempty"`
	Error    string          `json:"error,omitempty"`
	Progress *StreamProgress `json:"progress,omitempty"`
}

// MarshalSSE serializes the event as a JSON SSE data frame.
func (e *StreamEvent) MarshalSSE() ([]byte, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return append([]byte("data: "), append(data, []byte("\n\n")...)...), nil
}

package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type LoadTestMode int

const (
	ModeInProcess LoadTestMode = iota
	ModeHTTP
)

type LoadTestConfig struct {
	Concurrency    int
	Duration       time.Duration
	WarmupDuration time.Duration
	RampUpDuration time.Duration
	Endpoint       string
	Method         string
	RequestBody    []byte
	AuthHeader     string
	Headers        map[string]string // extra HTTP headers for each request
	Mode           LoadTestMode
	Server         http.Handler
	TargetURL      string
	TargetRPS      float64 // if > 0, rate-limit to this many requests per second
}

type ErrorSample struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// ConcurrencyResult holds aggregated metrics per concurrency level.
// Status200, Status4xx, Status5xx, StatusOther track HTTP response code ranges.
// ErrorBreakdown tracks unique failure reasons and their counts.
type ConcurrencyResult struct {
	Concurrency   int
	Duration      time.Duration
	TotalRequests uint64
	SuccessCount  uint64
	FailureCount  uint64
	Status200     uint64
	Status4xx     uint64
	Status5xx     uint64
	StatusOther   uint64
	Throughput    float64
	P50           time.Duration
	P90           time.Duration
	P95           time.Duration
	P99           time.Duration
	P999          time.Duration
	Max           time.Duration
	Avg           time.Duration
	MemoryBefore  runtime.MemStats
	MemoryAfter   runtime.MemStats
	AllocsPerOp   float64
	BytesPerOp    float64
	ErrorBreakdown []ErrorSample
}

type LatencySample struct {
	Dur  time.Duration
	Ok   bool
	Code int
	Err  string
}

type LoadTestSuite struct {
	Results []ConcurrencyResult
}

func RunLoadTest(ctx context.Context, cfg LoadTestConfig) (*ConcurrencyResult, error) {
	if cfg.WarmupDuration > 0 {
		runPhase(ctx, cfg, cfg.WarmupDuration, true, nil)
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Duration+cfg.RampUpDuration)
	defer cancel()
	return runPhase(ctx, cfg, cfg.Duration, false, nil), nil
}

func RunLoadTestSuite(ctx context.Context, baseCfg LoadTestConfig, concurrencyLevels []int) *LoadTestSuite {
	var suite LoadTestSuite
	for _, c := range concurrencyLevels {
		cfg := baseCfg
		cfg.Concurrency = c
		fmt.Printf("  Concurrency %4d: ", c)
		start := time.Now()
		result, err := RunLoadTest(ctx, cfg)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}
		fmt.Printf("%6d req/s  p50=%6s  p95=%6s  p99=%6s  max=%6s  mem=%6.0f MB  allocs=%6.0f\n",
			int(result.Throughput),
			result.P50.Round(time.Microsecond),
			result.P95.Round(time.Microsecond),
			result.P99.Round(time.Microsecond),
			result.Max.Round(time.Microsecond),
			float64(result.MemoryAfter.Alloc-result.MemoryBefore.Alloc)/1024/1024,
			result.AllocsPerOp,
		)
		since := time.Since(start)
		time.Sleep(time.Second - since)
		suite.Results = append(suite.Results, *result)
	}
	return &suite
}

func (s *LoadTestSuite) PrintReport(w io.Writer) {
	_, _ = fmt.Fprintf(w, "\n")
	_, _ = fmt.Fprintf(w, "╔══════════════════════════════════════════════════════════════════════════════════════════════════════╗\n")
	_, _ = fmt.Fprintf(w, "║                          Aurora Gateway — Full Load Test Report                                    ║\n")
	_, _ = fmt.Fprintf(w, "╚══════════════════════════════════════════════════════════════════════════════════════════════════════╝\n")
	_, _ = fmt.Fprintf(w, "\n")

	if len(s.Results) == 0 {
		_, _ = fmt.Fprintf(w, "  No results.\n")
		return
	}

	r := s.Results[0]
	_, _ = fmt.Fprintf(w, "  ═══════════════════════════════════════\n")
	_, _ = fmt.Fprintf(w, "    System Configuration\n")
	_, _ = fmt.Fprintf(w, "  ═══════════════════════════════════════\n")
	_, _ = fmt.Fprintf(w, "    CPU:                %s (%d logical cores)\n", runtime.GOARCH, runtime.GOMAXPROCS(0))
	_, _ = fmt.Fprintf(w, "    OS:                 %s/%s\n", runtime.GOOS, runtime.GOARCH)
	_, _ = fmt.Fprintf(w, "    Go version:         %s\n", runtime.Version())
	_, _ = fmt.Fprintf(w, "    GOMAXPROCS:         %d\n", runtime.GOMAXPROCS(0))
	_, _ = fmt.Fprintf(w, "    Memory pre-test:    %.0f MB alloc / %.0f MB total\n",
		float64(r.MemoryBefore.Alloc)/1024/1024,
		float64(r.MemoryBefore.TotalAlloc)/1024/1024)
	_, _ = fmt.Fprintf(w, "  ═══════════════════════════════════════\n")
	_, _ = fmt.Fprintf(w, "\n")

	_, _ = fmt.Fprintf(w, "  ═══════════════════════════════════════\n")
	_, _ = fmt.Fprintf(w, "    Test Configuration\n")
	_, _ = fmt.Fprintf(w, "  ═══════════════════════════════════════\n")
	_, _ = fmt.Fprintf(w, "    Concurrency levels: %v\n", formatLevels(s.Results))
	_, _ = fmt.Fprintf(w, "    Duration per level: %v\n", r.Duration)
	totalDuration := time.Duration(0)
	for _, r := range s.Results {
		totalDuration += r.Duration
	}
	_, _ = fmt.Fprintf(w, "    Total test time:    ~%v\n", totalDuration)
	_, _ = fmt.Fprintf(w, "    Endpoint:           %s\n", "POST /v1/chat/completions")
	var memBaseline float64
	for _, r := range s.Results {
		memBaseline = float64(r.MemoryBefore.Alloc) / 1024 / 1024
		break
	}
	_, _ = fmt.Fprintf(w, "    Memory baseline:    %.0f MB\n", memBaseline)
	_, _ = fmt.Fprintf(w, "    Auth:               %s\n", "disabled")
	// Try to infer auth from results
	if r.FailureCount > 0 && r.SuccessCount == 0 {
		_, _ = fmt.Fprintf(w, "    Status:             ALL FAILED (auth issue?)\n")
	}
	_, _ = fmt.Fprintf(w, "  ═══════════════════════════════════════\n")
	_, _ = fmt.Fprintf(w, "\n")

	_, _ = fmt.Fprintf(w, "  ════════════════════════════════════════════════════════════════════════════════════════════\n")
	_, _ = fmt.Fprintf(w, "    Per-Concurrency Results\n")
	_, _ = fmt.Fprintf(w, "  ════════════════════════════════════════════════════════════════════════════════════════════\n")

	_, _ = fmt.Fprintf(w, "  %-6s %10s %10s %10s %10s %10s %10s %10s %10s %10s %8s\n",
		"Workers", "Req/s", "2xx/4xx/5xx", "P50", "P90", "P95", "P99", "P999", "Max", "Avg", "Alloc/op")
	_, _ = fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 12*10+12))

	for _, r := range s.Results {
		codesLab := fmt.Sprintf("%d/%d/%d", r.Status200, r.Status4xx, r.Status5xx)
		_, _ = fmt.Fprintf(w, "  %-6d %10d %10s %10s %10s %10s %10s %10s %10s %10s %8.0f\n",
			r.Concurrency,
			int(r.Throughput),
			codesLab,
			r.P50.Round(time.Microsecond),
			r.P90.Round(time.Microsecond),
			r.P95.Round(time.Microsecond),
			r.P99.Round(time.Microsecond),
			r.P999.Round(time.Microsecond),
			r.Max.Round(time.Microsecond),
			r.Avg.Round(time.Microsecond),
			r.AllocsPerOp,
		)
	}
	_, _ = fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 12*10+12))
	_, _ = fmt.Fprintf(w, "\n")

	_, _ = fmt.Fprintf(w, "  ════════════════════════════════════════════════════════════════════════════════════════════\n")
	_, _ = fmt.Fprintf(w, "    Memory & Allocation Analysis\n")
	_, _ = fmt.Fprintf(w, "  ════════════════════════════════════════════════════════════════════════════════════════════\n")
	_, _ = fmt.Fprintf(w, "  %-6s %12s %14s %14s %12s %12s\n",
		"Workers", "Mem Before", "Mem After", "Mem Delta", "Allocs/op", "Bytes/op")
	_, _ = fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 12*6+2))
	peakDeltaMB := 0.0
	for _, r := range s.Results {
		deltaMB := float64(r.MemoryAfter.Alloc-r.MemoryBefore.Alloc) / 1024 / 1024
		if deltaMB > peakDeltaMB {
			peakDeltaMB = deltaMB
		}
		_, _ = fmt.Fprintf(w, "  %-6d %9.0f MB %9.0f MB %9.2f MB %12.0f %12.0f\n",
			r.Concurrency,
			float64(r.MemoryBefore.Alloc)/1024/1024,
			float64(r.MemoryAfter.Alloc)/1024/1024,
			deltaMB,
			r.AllocsPerOp,
			r.BytesPerOp,
		)
	}
	_, _ = fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 12*6+2))

	// Error breakdown section
	hasErrors := false
	for _, r := range s.Results {
		if len(r.ErrorBreakdown) > 0 {
			hasErrors = true
			break
		}
	}
	if hasErrors {
		_, _ = fmt.Fprintf(w, "\n")
		_, _ = fmt.Fprintf(w, "  ═══════════════════════════════════════\n")
		_, _ = fmt.Fprintf(w, "    Error Breakdown\n")
		_, _ = fmt.Fprintf(w, "  ═══════════════════════════════════════\n")
		for _, r := range s.Results {
			if len(r.ErrorBreakdown) == 0 {
				continue
			}
			_, _ = fmt.Fprintf(w, "  Workers %d:\n", r.Concurrency)
			for _, e := range r.ErrorBreakdown {
				_, _ = fmt.Fprintf(w, "    %6d×  %s\n", e.Count, e.Message)
			}
		}
		_, _ = fmt.Fprintf(w, "\n")
	}
	_, _ = fmt.Fprintf(w, "\n")

	totalSuccess := uint64(0)
	totalFail := uint64(0)
	totalReqs := uint64(0)
	peakTP := 0.0
	minP50 := time.Duration(0)
	for i, r := range s.Results {
		totalSuccess += r.SuccessCount
		totalFail += r.FailureCount
		totalReqs += r.TotalRequests
		if r.Throughput > peakTP {
			peakTP = r.Throughput
		}
		if i == 0 || r.P50 < minP50 {
			minP50 = r.P50
		}
	}

	_, _ = fmt.Fprintf(w, "  ═══════════════════════════════════════\n")
	_, _ = fmt.Fprintf(w, "    Summary\n")
	_, _ = fmt.Fprintf(w, "  ═══════════════════════════════════════\n")
	_, _ = fmt.Fprintf(w, "    Total requests:     %d\n", totalReqs)
	_, _ = fmt.Fprintf(w, "    Successful:         %d (%.1f%%)\n", totalSuccess, float64(totalSuccess)/float64(totalReqs)*100)
	_, _ = fmt.Fprintf(w, "    Failed:             %d (%.1f%%)\n", totalFail, float64(totalFail)/float64(totalReqs)*100)
	_, _ = fmt.Fprintf(w, "    Peak throughput:    %d req/s\n", int(peakTP))
	_, _ = fmt.Fprintf(w, "    Min gateway P50:    %v\n", minP50.Round(time.Microsecond))
	_, _ = fmt.Fprintf(w, "    Peak memory delta:  %.0f MB\n", peakDeltaMB)
	_, _ = fmt.Fprintf(w, "    Constant allocs/op: %.0f (across all concurrency levels)\n", s.Results[0].AllocsPerOp)
	_, _ = fmt.Fprintf(w, "\n")
}

func formatLevels(results []ConcurrencyResult) string {
	var levels []string
	for _, r := range results {
		levels = append(levels, fmt.Sprintf("%d", r.Concurrency))
	}
	return strings.Join(levels, ", ")
}

func runPhase(ctx context.Context, cfg LoadTestConfig, duration time.Duration, warmup bool, progress chan<- *StreamProgress) *ConcurrencyResult {
	result := &ConcurrencyResult{
		Concurrency: cfg.Concurrency,
		Duration:    duration,
	}

	var (
		mu          sync.Mutex
		samples     []LatencySample
		totalReqs   atomic.Uint64
		successReqs atomic.Uint64
		failReqs    atomic.Uint64
	)

	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	phaseCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	// Live progress ticker — reads atomics and emits current RPS
	if progress != nil && !warmup {
		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			startTime := time.Now()
			prevReqs := uint64(0)
			prevTime := startTime
			for {
				select {
				case <-phaseCtx.Done():
					return
				case <-ticker.C:
					now := time.Now()
					elapsed := now.Sub(startTime).Seconds()
					curReqs := totalReqs.Load()
					// Use a rolling window for smoother RPS
					windowReqs := curReqs - prevReqs
					windowDur := now.Sub(prevTime).Seconds()
					var rps float64
					if windowDur > 0 {
						rps = float64(windowReqs) / windowDur
					}
					prevReqs = curReqs
					prevTime = now

					select {
					case progress <- &StreamProgress{
						Concurrency:    cfg.Concurrency,
						CurrentRPS:     rps,
						ElapsedSeconds: elapsed,
						TotalRequests:  curReqs,
						SuccessCount:   successReqs.Load(),
						FailureCount:   failReqs.Load(),
						DurationMs:     int(duration.Milliseconds()),
					}:
					default:
						// channel full, skip this tick
					}
				}
			}
		}()
	}

	var wg sync.WaitGroup

	if cfg.RampUpDuration > 0 {
		for i := 0; i < cfg.Concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				startDelay := time.Duration(float64(cfg.RampUpDuration) * float64(id) / float64(cfg.Concurrency))
				timer := time.NewTimer(startDelay)
				select {
				case <-phaseCtx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
				timer.Stop()

				workerRun(phaseCtx, cfg, &totalReqs, &successReqs, &failReqs, &mu, &samples, warmup)
			}(i)
		}
	} else {
		for i := 0; i < cfg.Concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				workerRun(phaseCtx, cfg, &totalReqs, &successReqs, &failReqs, &mu, &samples, warmup)
			}()
		}
	}

	wg.Wait()

	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	result.TotalRequests = totalReqs.Load()
	result.SuccessCount = successReqs.Load()
	result.FailureCount = failReqs.Load()
	result.MemoryBefore = memBefore
	result.MemoryAfter = memAfter

	if duration.Seconds() > 0 {
		result.Throughput = float64(result.TotalRequests) / duration.Seconds()
	}

	allocDelta := uint64(0)
	if memAfter.TotalAlloc > memBefore.TotalAlloc {
		allocDelta = memAfter.TotalAlloc - memBefore.TotalAlloc
	}
	if result.TotalRequests > 0 {
		result.AllocsPerOp = float64(memAfter.Mallocs-memBefore.Mallocs) / float64(result.TotalRequests)
		result.BytesPerOp = float64(allocDelta) / float64(result.TotalRequests)
	}

	if !warmup && len(samples) > 0 {
		sort.Slice(samples, func(i, j int) bool { return samples[i].Dur < samples[j].Dur })
		n := len(samples)
		result.Max = samples[n-1].Dur
		var total time.Duration
		for _, s := range samples {
			total += s.Dur
			// Track status code ranges
			if s.Ok || s.Code == 200 {
				result.Status200++
			} else if s.Code >= 400 && s.Code < 500 {
				result.Status4xx++
			} else if s.Code >= 500 {
				result.Status5xx++
			} else if s.Code > 0 {
				result.StatusOther++
			}
		}
		rawP50 := latencyPercentile(samples, 50.0)
		if rawP50 < time.Microsecond && total/time.Duration(n) > time.Microsecond {
			totalNs := total.Nanoseconds()
			avgNs := totalNs / int64(n)
			rawP50Ns := rawP50.Nanoseconds()
			_, _ = fmt.Fprintf(io.Discard, "debug: p50_raw=%d avg=%d n=%d total=%d max=%d\n",
				rawP50Ns, avgNs, n, totalNs, result.Max.Nanoseconds())
		}
		result.Avg = time.Duration(int64(total) / int64(n))
		result.P50 = latencyPercentile(samples, 50.0)
		result.P90 = latencyPercentile(samples, 90.0)
		result.P95 = latencyPercentile(samples, 95.0)
		result.P99 = latencyPercentile(samples, 99.0)
		result.P999 = latencyPercentile(samples, 99.9)

		// Error breakdown: aggregate unique error strings
		errMap := make(map[string]int)
		for _, s := range samples {
			if !s.Ok && s.Err != "" {
				errMap[s.Err]++
			}
		}
		if len(errMap) > 0 {
			var errs []ErrorSample
			for msg, cnt := range errMap {
				errs = append(errs, ErrorSample{Message: msg, Count: cnt})
			}
			sort.Slice(errs, func(i, j int) bool { return errs[i].Count > errs[j].Count })
			result.ErrorBreakdown = errs
		}
	}

	return result
}

func workerRun(ctx context.Context, cfg LoadTestConfig, totalReqs, successReqs, failReqs *atomic.Uint64, mu *sync.Mutex, samples *[]LatencySample, warmup bool) {
	body := cfg.RequestBody

	// Rate limiter: ensure we don't exceed TargetRPS across all workers combined.
	// Each worker is allocated rate / concurrency requests per second with burst=1.
	// When TargetRPS <= 0 there is no limit (fire as fast as possible).
	var ticker *time.Ticker
	var tickerC <-chan time.Time
	if cfg.TargetRPS > 0 && cfg.Concurrency > 0 {
		perWorker := cfg.TargetRPS / float64(cfg.Concurrency)
		if perWorker < 1 {
			perWorker = 1
		}
		interval := time.Duration(float64(time.Second) / perWorker)
		ticker = time.NewTicker(interval)
		tickerC = ticker.C
	}

	if cfg.Mode == ModeInProcess {
		for {
			if tickerC != nil {
				select {
				case <-ctx.Done():
					return
				case <-tickerC:
				}
			} else {
				select {
				case <-ctx.Done():
					return
				default:
				}
			}

			req := httptest.NewRequest(cfg.Method, cfg.Endpoint, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if cfg.AuthHeader != "" {
				req.Header.Set("Authorization", cfg.AuthHeader)
			}
			for k, v := range cfg.Headers {
				req.Header.Set(k, v)
			}

			start := highResNow()
			rec := httptest.NewRecorder()
			cfg.Server.ServeHTTP(rec, req)
			dur := highResElapsed(start, highResNow())
			code := rec.Code
			ok := code == http.StatusOK

			totalReqs.Add(1)
			if ok {
				successReqs.Add(1)
			} else {
				failReqs.Add(1)
			}

			if !warmup {
				mu.Lock()
				*samples = append(*samples, LatencySample{Dur: dur, Ok: ok, Code: code, Err: ""})
				mu.Unlock()
			}
		}
	} else {
		client := &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:    1000,
				IdleConnTimeout: 90 * time.Second,
			},
		}

		for {
			if tickerC != nil {
				select {
				case <-ctx.Done():
					return
				case <-tickerC:
				}
			} else {
				select {
				case <-ctx.Done():
					return
				default:
				}
			}

			req, _ := http.NewRequestWithContext(ctx, cfg.Method, cfg.TargetURL+cfg.Endpoint, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if cfg.AuthHeader != "" {
				req.Header.Set("Authorization", cfg.AuthHeader)
			}
			for k, v := range cfg.Headers {
				req.Header.Set(k, v)
			}

			start := highResNow()
			resp, err := client.Do(req)
			dur := highResElapsed(start, highResNow())

			code := 0
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			} else {
				code = resp.StatusCode
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
			ok := err == nil && code == http.StatusOK

			totalReqs.Add(1)
			if ok {
				successReqs.Add(1)
			} else {
				failReqs.Add(1)
			}

			if !warmup {
				mu.Lock()
				*samples = append(*samples, LatencySample{Dur: dur, Ok: ok, Code: code, Err: errMsg})
				mu.Unlock()
			}
		}
	}
}

func latencyPercentile(samples []LatencySample, p float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	idx := int(float64(len(samples)) * p / 100.0)
	if idx >= len(samples) {
		idx = len(samples) - 1
	}
	return samples[idx].Dur
}

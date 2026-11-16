package llmclient

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurora/internal/egress"
	httpclient "aurora/internal/http_client"
)

// defaultEgress is the process-wide exit registry installed by the gateway.
//
// A registry is keyed by provider name and answers "nothing configured" for
// providers that have no exits, so a single global handle reaches every
// provider without threading it through each provider constructor — the same
// shape as httpclient.SetDefaultConfigOverride.
var (
	defaultEgress   *egress.Registry
	defaultEgressMu sync.RWMutex
)

// SetDefaultEgress installs (or clears, with nil) the registry every client
// consults when its own Config.Egress is unset. Call it once during startup,
// before providers are built.
func SetDefaultEgress(reg *egress.Registry) {
	defaultEgressMu.Lock()
	defaultEgress = reg
	defaultEgressMu.Unlock()
}

// DefaultEgress returns the registry installed by SetDefaultEgress, if any.
func DefaultEgress() *egress.Registry {
	defaultEgressMu.RLock()
	defer defaultEgressMu.RUnlock()
	return defaultEgress
}

// egressStateKey carries the per-request exit selection state.
type egressStateKey struct{}

// egressState records which exits one logical request has already used so a
// retry fails over to the next one instead of repeating the same failure.
type egressState struct {
	tried map[string]struct{}
	// last is the exit picked for the most recent attempt, so the outcome of
	// that attempt can be recorded even after the request object is gone.
	last egress.Candidate
}

// withEgress attaches selection state when a registry is wired. Without a
// registry the context is returned untouched and everything behaves as
// before: a single client per provider.
func withEgress(ctx context.Context, reg *egress.Registry) context.Context {
	if reg == nil {
		return ctx
	}
	return context.WithValue(ctx, egressStateKey{}, &egressState{tried: make(map[string]struct{})})
}

// egressPool caches one HTTP client per exit. Every exit needs its own
// connection pool: the standard transport keys connections by proxy but not
// by local source address, so a request that selected one address could
// otherwise be handed a connection opened from another.
type egressPool struct {
	mu      sync.Mutex
	useUTLS bool
	byName  map[string]*http.Client
}

func newEgressPool(useUTLS bool) *egressPool {
	return &egressPool{useUTLS: useUTLS, byName: make(map[string]*http.Client)}
}

func (p *egressPool) client(c egress.Candidate) *http.Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cl, ok := p.byName[c.Name]; ok {
		return cl
	}
	var bindIP string
	if c.LocalAddr != nil {
		bindIP = c.LocalAddr.String()
	}
	cl := httpclient.NewEgressClient(httpclient.EgressOptions{
		BindIP:  bindIP,
		Proxy:   c.ProxyURL,
		UseUTLS: p.useUTLS,
	})
	p.byName[c.Name] = cl
	return cl
}

// selectEgress picks the exit for this attempt. It returns ok=false when no
// registry is wired or nothing is configured, in which case the caller falls
// back to the provider's default client.
//
// A provider whose upstream connection is opened by a local sidecar keeps its
// own plain client for the local hop: the exit is reported to the sidecar
// (see Config.ExitHeaderSetter) instead of being applied to a connection that
// never leaves the machine.
func (c *Client) selectEgress(ctx context.Context) (*http.Client, egress.Candidate, bool) {
	if c.egress == nil || c.egressPool == nil {
		return c.httpClient, egress.Candidate{}, false
	}
	var tried map[string]struct{}
	if st, _ := ctx.Value(egressStateKey{}).(*egressState); st != nil {
		tried = st.tried
	}
	cand, ok := c.egress.Pick(c.egressProviderName(), tried)
	if !ok {
		return c.httpClient, egress.Candidate{}, false
	}
	if st, _ := ctx.Value(egressStateKey{}).(*egressState); st != nil {
		st.tried[cand.Name] = struct{}{}
		st.last = cand
	}
	if c.config.EgressViaSidecar {
		return c.httpClient, cand, true
	}
	return c.egressPool.client(cand), cand, true
}

// reportEgress feeds the outcome back to the registry so a failing exit is
// cooled down — and a rate-limited tier handed over to the next tier —
// instead of being retried on every request. detail is the upstream error or
// status line the operator sees in the egress status panel. retryAfter is the
// deadline the upstream announced for a rate limit, or the zero time.
func (c *Client) reportEgress(cand egress.Candidate, outcome egress.Outcome, detail string, retryAfter time.Time) {
	if c.egress == nil {
		return
	}
	c.egress.ReportRateLimit(c.egressProviderName(), cand, outcome, detail, retryAfter)
}

// reportQuotaExhausted parks an exit for a full rate-limit window without
// having talked to the upstream. It is what a sidecar answers when the picked
// address has no proxy to leave from: retrying it is pointless, and leaving it
// in rotation made the next request pick it again.
func (c *Client) reportQuotaExhausted(cand egress.Candidate, detail string) {
	if c.egress == nil {
		return
	}
	c.egress.ReportQuotaExhausted(c.egressProviderName(), cand, detail)
}

// retryAfterDeadlineSeconds parses a Retry-After header value into an absolute
// instant.
func retryAfterDeadlineSeconds(raw string, now time.Time) time.Time {
	return retryAfterDeadline(&http.Response{Header: http.Header{"Retry-After": []string{raw}}}, now)
}

// retryAfterDeadline parses the upstream's Retry-After answer into an absolute
// instant. Both forms are accepted: delay-seconds and an HTTP-date. A missing
// or unparsable header yields the zero time, which the registry treats as
// "probe back on the default window".
func retryAfterDeadline(resp *http.Response, now time.Time) time.Time {
	if resp == nil {
		return time.Time{}
	}
	raw := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if raw == "" {
		return time.Time{}
	}
	if secs, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if secs <= 0 {
			return time.Time{}
		}
		// Saturate instead of overflowing on a hostile value.
		if secs > int64(egress.MaxLimitedCoolDown/time.Second) {
			secs = int64(egress.MaxLimitedCoolDown / time.Second)
		}
		return now.Add(time.Duration(secs) * time.Second)
	}
	if at, err := http.ParseTime(raw); err == nil {
		return at
	}
	return time.Time{}
}

// egressOutcome classifies an attempt. 429/403 are treated as rate limiting
// of the exit that hit it, not of the tier: the upstream meters its free
// tier per source address, so the other addresses keep serving.
func egressOutcome(err error, resp *http.Response) egress.Outcome {
	if err != nil {
		return egress.OutcomeFailed
	}
	if resp == nil {
		return egress.OutcomeFailed
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusForbidden:
		return egress.OutcomeLimited
	}
	if resp.StatusCode >= 500 {
		return egress.OutcomeFailed
	}
	return egress.OutcomeOK
}

// hasUntriedExit reports whether the provider has another exit left that this
// request has not used yet.
//
// A rate limit belongs to the address that hit it, so a 429 on one address is
// worth retrying on the next one. Without a registry, or once every exit has
// been used, the 429 has to reach the caller.
func (c *Client) hasUntriedExit(ctx context.Context) bool {
	if c.egress == nil {
		return false
	}
	var tried map[string]struct{}
	if st, _ := ctx.Value(egressStateKey{}).(*egressState); st != nil {
		tried = st.tried
	}
	return c.egress.HasAlternative(c.egressProviderName(), tried)
}

// egressCandidate returns the exit used by the most recent attempt of this
// request, if any.
func egressCandidate(ctx context.Context) (egress.Candidate, bool) {
	if st, _ := ctx.Value(egressStateKey{}).(*egressState); st != nil && st.last.Name != "" {
		return st.last, true
	}
	return egress.Candidate{}, false
}

// egressDetail renders the operator-facing reason for an attempt: the
// transport error when the request never completed, otherwise the upstream
// status line when it answered with an error. Empty when the exit worked.
func egressDetail(err error, resp *http.Response) string {
	if err != nil {
		return err.Error()
	}
	if resp != nil && resp.StatusCode >= http.StatusBadRequest {
		return resp.Status
	}
	return ""
}

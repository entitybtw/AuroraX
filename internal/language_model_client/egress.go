package llmclient

import (
	"context"
	"net/http"
	"sync"

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
func (c *Client) selectEgress(ctx context.Context) (*http.Client, egress.Candidate, bool) {
	if c.egress == nil || c.egressPool == nil {
		return c.httpClient, egress.Candidate{}, false
	}
	var tried map[string]struct{}
	if st, _ := ctx.Value(egressStateKey{}).(*egressState); st != nil {
		tried = st.tried
	}
	cand, ok := c.egress.Pick(c.config.ProviderName, tried)
	if !ok {
		return c.httpClient, egress.Candidate{}, false
	}
	if tried != nil {
		tried[cand.Name] = struct{}{}
	}
	return c.egressPool.client(cand), cand, true
}

// reportEgress feeds the outcome back to the registry so a failing exit is
// cooled down — and a rate-limited tier handed over to the fallback tier —
// instead of being retried on every request.
func (c *Client) reportEgress(cand egress.Candidate, outcome egress.Outcome) {
	if c.egress == nil {
		return
	}
	c.egress.Report(c.config.ProviderName, cand, outcome)
}

// egressOutcome classifies an attempt. 429/403 are treated as rate limiting
// of the whole exit tier rather than a fault of the individual address.
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

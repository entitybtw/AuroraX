// Package pool implements load-balanced provider pools for the gateway.
//
// A pool is a logical group of one or more configured providers that share
// the same upstream type and can serve the same models. Requests addressed
// to a pool selector (e.g. "jina-pool/jina-embeddings-v3") are dispatched
// to a single concrete member chosen by round-robin rotation.
//
// Health-aware: if a member's circuit breaker is open or it has been
// administratively disabled, it is skipped during selection. Pools never
// route to an unhealthy member; if all members are unhealthy the pool
// returns an error so callers can decide whether to wait for recovery.
//
// Failover: callers (Router) treat the chosen member as a candidate and
// can ask the pool for the next eligible member when a transient upstream
// error occurs. The pool excludes already-tried members for the rest of
// the request.
package pool

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
)

// Capability represents a model-processing capability that a pool member
// supports. The router uses this to filter members when dispatching requests
// for different model types (chat, embedding, reranker, etc.).
type Capability string

const (
	CapChat       Capability = "chat"
	CapEmbedding  Capability = "embedding"
	CapRerank     Capability = "reranker"
	CapResponses  Capability = "responses"
	CapFiles      Capability = "files"
	CapBatches    Capability = "batches"
)

// Strategy enumerates supported load-balancing strategies.
type Strategy string

const (
	StrategyRoundRobin Strategy = "round_robin"
)

// ParseStrategy normalizes a string to a Strategy. Empty defaults to round_robin.
func ParseStrategy(s string) (Strategy, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return StrategyRoundRobin, nil
	}
	if s == string(StrategyRoundRobin) {
		return StrategyRoundRobin, nil
	}
	return "", fmt.Errorf("unsupported pool strategy %q (OSS edition supports: round_robin)", s)
}

// HealthChecker reports whether a configured provider name is currently
// eligible for routing. Implementations typically delegate to the registry's
// circuit breaker / availability state.
type HealthChecker interface {
	IsProviderHealthy(providerName string) bool
}

// alwaysHealthy is a default health checker used when no implementation is
// supplied. It treats every member as eligible.
type alwaysHealthy struct{}

func (alwaysHealthy) IsProviderHealthy(string) bool { return true }

// Pool is a load-balanced group of provider instances.
//
// A Pool is safe for concurrent use. It does not reach into the registry to
// dispatch requests — the caller (Router) does the actual provider call. The
// pool only owns selection state (round-robin index).
type Pool struct {
	name     string
	strategy Strategy
	members  []*Member
	memberIx map[string]int

	rrIndex uint64
	health  HealthChecker
}

// Member is a single provider instance inside a pool.
type Member struct {
	ProviderName string
	Capabilities []Capability

	totalRequests int64
	totalErrors   int64
}

// SupportsCapability reports whether this member can handle the given capability.
func (m *Member) SupportsCapability(c Capability) bool {
	if len(m.Capabilities) == 0 {
		return true // no capability constraint = assume all
	}
	for _, cap := range m.Capabilities {
		if cap == c {
			return true
		}
	}
	return false
}

// Config is the validated input for NewPool.
type Config struct {
	Name     string
	Strategy Strategy
	Members  []MemberConfig
	Health   HealthChecker
}

// MemberConfig is the per-member input to NewPool.
type MemberConfig struct {
	ProviderName string
	Capabilities []Capability
}

// NewPool constructs a pool from a validated Config.
func NewPool(cfg Config) (*Pool, error) {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		return nil, fmt.Errorf("pool name is required")
	}
	if len(cfg.Members) == 0 {
		return nil, fmt.Errorf("pool %q: at least one member is required", name)
	}

	strategy := cfg.Strategy
	if strategy == "" {
		strategy = StrategyRoundRobin
	}

	members := make([]*Member, 0, len(cfg.Members))
	memberIx := make(map[string]int, len(cfg.Members))
	for i, mc := range cfg.Members {
		providerName := strings.TrimSpace(mc.ProviderName)
		if providerName == "" {
			return nil, fmt.Errorf("pool %q: member %d has empty provider name", name, i)
		}
		if _, dup := memberIx[providerName]; dup {
			return nil, fmt.Errorf("pool %q: duplicate member %q", name, providerName)
		}
		capabilities := make([]Capability, len(mc.Capabilities))
		copy(capabilities, mc.Capabilities)
		members = append(members, &Member{ProviderName: providerName, Capabilities: capabilities})
		memberIx[providerName] = len(members) - 1
	}

	health := cfg.Health
	if health == nil {
		health = alwaysHealthy{}
	}

	return &Pool{
		name:     name,
		strategy: strategy,
		members:  members,
		memberIx: memberIx,
		health:   health,
	}, nil
}

func (p *Pool) Name() string { return p.name }

func (p *Pool) Strategy() Strategy { return p.strategy }

// Members returns a defensive snapshot of member metadata for diagnostics.
func (p *Pool) Members() []MemberSnapshot {
	out := make([]MemberSnapshot, 0, len(p.members))
	for _, m := range p.members {
		caps := make([]Capability, len(m.Capabilities))
		copy(caps, m.Capabilities)
		out = append(out, MemberSnapshot{
			ProviderName:  m.ProviderName,
			TotalRequests: atomic.LoadInt64(&m.totalRequests),
			TotalErrors:   atomic.LoadInt64(&m.totalErrors),
			Healthy:       p.health.IsProviderHealthy(m.ProviderName),
			Capabilities:  caps,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ProviderName < out[j].ProviderName })
	return out
}

// MemberSnapshot is a read-only view of a member used by the dashboard /
// diagnostics endpoints.
type MemberSnapshot struct {
	ProviderName  string       `json:"provider_name"`
	TotalRequests int64        `json:"total_requests"`
	TotalErrors   int64        `json:"total_errors"`
	Healthy       bool         `json:"healthy"`
	Capabilities  []Capability `json:"capabilities,omitempty"`
}

// Pick chooses the next eligible member for a request, excluding any names in
// `tried`. If `cap` is non-empty, only members that support the given
// capability are considered. Returns the chosen provider name and a release
// callback that the caller MUST invoke when the request completes (success or
// failure).
func (p *Pool) Pick(tried map[string]struct{}, cap ...Capability) (providerName string, release func(success bool), err error) {
	eligible := p.eligibleMembers(tried, cap...)
	if len(eligible) == 0 {
		if len(p.members) == 0 {
			return "", noopRelease, fmt.Errorf("pool %q has no members", p.name)
		}
		return "", noopRelease, fmt.Errorf("pool %q: no healthy members available", p.name)
	}

	chosen := p.choose(eligible)
	atomic.AddInt64(&chosen.totalRequests, 1)
	released := false
	return chosen.ProviderName, func(success bool) {
		if released {
			return
		}
		released = true
		if !success {
			atomic.AddInt64(&chosen.totalErrors, 1)
		}
	}, nil
}

// HasMember reports whether the given provider instance name is registered in
// this pool.
func (p *Pool) HasMember(providerName string) bool {
	_, ok := p.memberIx[strings.TrimSpace(providerName)]
	return ok
}

func (p *Pool) eligibleMembers(tried map[string]struct{}, cap ...Capability) []*Member {
	requireCap := len(cap) > 0 && cap[0] != ""
	out := make([]*Member, 0, len(p.members))
	for _, m := range p.members {
		if _, alreadyTried := tried[m.ProviderName]; alreadyTried {
			continue
		}
		if !p.health.IsProviderHealthy(m.ProviderName) {
			continue
		}
		if requireCap && !m.SupportsCapability(cap[0]) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func (p *Pool) choose(eligible []*Member) *Member {
	if len(eligible) == 1 {
		return eligible[0]
	}
	idx := atomic.AddUint64(&p.rrIndex, 1) - 1
	return eligible[idx%uint64(len(eligible))]
}

func noopRelease(bool) {}

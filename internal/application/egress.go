package app

import (
	"encoding/json"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"aurora/configuration"
	"aurora/internal/addon"
	"aurora/internal/egress"
	"aurora/internal/hooks"
)

// egressHookHook is the runtime-addon hook that lets an extension contribute
// its own exits (for example the endpoints behind a subscription) to a
// provider's rotation.
const egressHook = "EgressCandidates"

// egressCacheTTL bounds how often the extension hook is interpreted during
// selection. Picks happen per attempt; the addon keeps its own state and only
// needs to be consulted occasionally.
const egressCacheTTL = 3 * time.Second

// syncEgress rebuilds the preferred-tier candidate set for every provider
// from its configured source addresses and drops state for providers that no
// longer exist. Called after each provider (re)build so editing the list from
// the dashboard takes effect without a restart.
func (a *App) syncEgress() {
	if a == nil || a.egress == nil {
		return
	}
	rawProviders := a.runtimeRawProviders()
	seen := make(map[string]struct{}, len(rawProviders))
	for name, raw := range rawProviders {
		seen[name] = struct{}{}
		cands := egressCandidates(raw)
		if len(cands) == 0 {
			// Still records the disabled set: a provider whose only exits come
			// from an extension keeps its operator-marked "off" list.
			a.egress.Remove(name)
		} else {
			a.egress.SetCore(name, egress.ParseStrategy(raw.EgressStrategy), cands)
		}
		a.egress.SetDisabled(name, raw.EgressDisabled)
	}
	for _, name := range a.egress.Providers() {
		if _, ok := seen[name]; !ok {
			a.egress.Remove(name)
		}
	}
}

// egressCandidates turns a provider's configured source addresses into
// preferred-tier exits. Unparseable entries are skipped rather than failing
// the whole provider.
func egressCandidates(raw config.RawProviderConfig) []egress.Candidate {
	var out []egress.Candidate
	seen := make(map[string]struct{}, len(raw.BindIPs)+1)
	add := func(addr string) {
		ip := net.ParseIP(strings.TrimSpace(addr))
		if ip == nil {
			return
		}
		if _, ok := seen[ip.String()]; ok {
			return
		}
		seen[ip.String()] = struct{}{}
		out = append(out, egress.Candidate{
			Name:      "ip:" + ip.String(),
			LocalAddr: ip,
			Source:    egress.SourceConfig,
		})
	}
	// bind_ip is the legacy single-address form and is still what most
	// providers carry — folding it in keeps the preferred tier populated
	// instead of silently rotating nothing.
	add(raw.BindIP)
	for _, addr := range raw.BindIPs {
		add(addr)
	}
	return out
}

// hookEgressEntry is one cached answer from the extension hook.
type hookEgressEntry struct {
	until time.Time
	cands []egress.Candidate
}

// hookEgressSource adapts the runtime hook bridge to egress.Source, caching
// answers so a per-attempt pick does not re-interpret the addon every time.
type hookEgressSource struct {
	hooks   *hooks.Bridge
	ttl     time.Duration
	mu      sync.Mutex
	entries map[string]hookEgressEntry
}

func newHookEgressSource(b *hooks.Bridge) *hookEgressSource {
	return &hookEgressSource{
		hooks:   b,
		ttl:     egressCacheTTL,
		entries: make(map[string]hookEgressEntry),
	}
}

func (s *hookEgressSource) get(provider string) []egress.Candidate {
	if s == nil || s.hooks == nil {
		return nil
	}
	now := time.Now()
	s.mu.Lock()
	entry, ok := s.entries[provider]
	s.mu.Unlock()
	if ok && now.Before(entry.until) {
		return entry.cands
	}

	raw, err := s.hooks.Call(addon.KindRuntime, egressHook, map[string]any{"provider": provider})
	if err != nil {
		// Cache a miss too, otherwise an extension that exports nothing would
		// be re-consulted on every attempt.
		s.store(provider, hookEgressEntry{until: now.Add(s.ttl)})
		return nil
	}
	cands := parseEgressCandidates(raw)
	s.store(provider, hookEgressEntry{until: now.Add(s.ttl), cands: cands})
	return cands
}

func (s *hookEgressSource) store(provider string, entry hookEgressEntry) {
	s.mu.Lock()
	s.entries[provider] = entry
	s.mu.Unlock()
}

// invalidate drops the cache so the next pick consults the addon again. Called
// when an extension is applied, updated or its settings change.
func (s *hookEgressSource) invalidate() {
	s.mu.Lock()
	s.entries = make(map[string]hookEgressEntry)
	s.mu.Unlock()
}

// poolsContaining lists the pools a provider belongs to, excluding a pool
// that happens to share the provider's name. Pool selection resolves to a
// member provider before a request reaches the egress registry, so an
// extension binding written against a pool name is only ever seen here.
func poolsContaining(pools map[string]config.RawPoolConfig, provider string) []string {
	want := strings.TrimSpace(provider)
	if want == "" {
		return nil
	}
	var out []string
	for name, raw := range pools {
		if strings.EqualFold(strings.TrimSpace(name), want) {
			continue
		}
		for _, member := range raw.Members {
			if strings.EqualFold(strings.TrimSpace(member), want) {
				out = append(out, name)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// extensionCandidates is what the egress registry consults for a provider. It
// asks the hook about the provider itself and about every pool containing it,
// so one apply_to list can name providers and pools together.
func (a *App) extensionCandidates(provider string) []egress.Candidate {
	if a == nil || a.egressSource == nil {
		return nil
	}
	cands := a.egressSource.get(provider)
	for _, pool := range poolsContaining(a.runtimeRawPools(), provider) {
		cands = append(cands, a.egressSource.get(pool)...)
	}
	return cands
}

// egressCandidateDTO is one entry of an addon's EgressCandidates reply.
type egressCandidateDTO struct {
	Name    string `json:"name"`
	Proxy   string `json:"proxy"`
	BindIP  string `json:"bind_ip"`
	LocalIP string `json:"local_addr"`
	Weight  int    `json:"weight"`
	// Tier is optional. Absent means "no opinion" and the gateway keeps the
	// documented default (1 = fallback). An explicit 0 promotes the exits into
	// the same rotation as the provider's own source addresses, and -1 puts
	// them ahead of those addresses.
	Tier *int `json:"tier"`
}

// parseEgressCandidates decodes the hook reply. Both a bare array and the
// {"candidates": [...]} envelope are accepted so addons can be terse.
func parseEgressCandidates(raw string) []egress.Candidate {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var envelope struct {
		Candidates []egressCandidateDTO `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err == nil && len(envelope.Candidates) > 0 {
		return convertEgressCandidates(envelope.Candidates)
	}
	var bare []egressCandidateDTO
	if err := json.Unmarshal([]byte(raw), &bare); err != nil {
		return nil
	}
	return convertEgressCandidates(bare)
}

func convertEgressCandidates(in []egressCandidateDTO) []egress.Candidate {
	out := make([]egress.Candidate, 0, len(in))
	for _, dto := range in {
		tier := egress.TierFallback
		if dto.Tier != nil {
			tier = egress.Tier(*dto.Tier)
			// Below TierExclusive would only ever sort first and mean nothing
			// the operator could reason about, so it is floored there.
			if tier < egress.TierExclusive {
				tier = egress.TierExclusive
			}
		}
		c := egress.Candidate{
			Name:   strings.TrimSpace(dto.Name),
			Weight: dto.Weight,
			Tier:   tier,
			Source: egress.SourceExtension,
		}
		if p := strings.TrimSpace(dto.Proxy); p != "" {
			u, err := url.Parse(p)
			if err != nil {
				continue
			}
			c.ProxyURL = u
		}
		if addr := firstNonEmpty(dto.LocalIP, dto.BindIP); addr != "" {
			ip := net.ParseIP(addr)
			if ip == nil {
				continue
			}
			c.LocalAddr = ip
		}
		if c.Name == "" {
			// Leave it empty: egress derives a stable name from the address.
			if c.ProxyURL != nil {
				c.Name = "proxy:" + c.ProxyURL.String()
			} else if c.LocalAddr != nil {
				c.Name = "ip:" + c.LocalAddr.String()
			}
		}
		if c.Name == "" {
			continue
		}
		out = append(out, c)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

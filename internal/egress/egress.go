// Package egress decides where a provider's outbound traffic leaves from.
//
// One logical provider (single API key, single base URL) can have several
// exits organised in tiers:
//
//   - tier 0 — local source addresses. Preferred: as long as they answer,
//     traffic leaves directly from them.
//   - tier 1 and up — endpoints contributed by an extension, e.g. a VPN
//     subscription. Reached when the preferred tier is unusable: every
//     address is cooling down after a hard failure, or the tier was marked
//     rate-limited (429/403).
//
// A tier that reports a rate limit is demoted for a while and then probes
// back automatically, so the gateway returns to direct egress on its own.
//
// The core owns tier 0; extensions own the rest through a hook source, so the
// core never learns anything about what an extension is actually doing.
package egress

import (
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Strategy selects among the healthy candidates of one tier.
type Strategy string

const (
	// StrategyRoundRobin walks the candidate list in order (default).
	StrategyRoundRobin Strategy = "round_robin"
	// StrategyRandom picks uniformly at random.
	StrategyRandom Strategy = "random"
	// StrategyWeighted honours Candidate.Weight (missing = 1).
	StrategyWeighted Strategy = "weighted"
	// StrategyFirst always tries the first healthy candidate — useful when
	// the list is already ordered by preference (best latency first).
	StrategyFirst Strategy = "first"
)

// ParseStrategy maps a config/UI string onto a Strategy, defaulting to
// round-robin so a typo never silently disables selection.
func ParseStrategy(s string) Strategy {
	switch Strategy(strings.ToLower(strings.TrimSpace(s))) {
	case StrategyRandom:
		return StrategyRandom
	case StrategyWeighted:
		return StrategyWeighted
	case StrategyFirst:
		return StrategyFirst
	default:
		return StrategyRoundRobin
	}
}

// Tier groups candidates by preference. 0 is the preferred tier.
type Tier int

// TierPreferred is the local source-address tier: the provider's own
// configured addresses plus any extension exit that asked to join them.
const TierPreferred Tier = 0

// TierFallback is the default tier for an extension's exits: they serve once
// the preferred tier stops answering or gets rate-limited, unless the
// extension asks to join the preferred rotation with tier 0.
const TierFallback Tier = 1

// TierExclusive sits before TierPreferred, so an extension that asks for it
// is used ahead of the provider's own source addresses and falls back to
// them only when the exclusive tier is unusable.
const TierExclusive Tier = -1

// Outcome is what an attempt through a candidate actually did.
type Outcome int

const (
	// OutcomeOK — the exit worked.
	OutcomeOK Outcome = iota
	// OutcomeFailed — connection or upstream failure; the candidate itself
	// is at fault and cools down.
	OutcomeFailed
	// OutcomeLimited — the upstream rate-limited us (429/403). The limit is
	// scoped to the address that hit it, so the exit itself cools down while
	// its neighbours in the same tier keep serving.
	OutcomeLimited
)

// String renders an outcome for status payloads.
func (o Outcome) String() string {
	switch o {
	case OutcomeOK:
		return "ok"
	case OutcomeFailed:
		return "failed"
	case OutcomeLimited:
		return "limited"
	default:
		return ""
	}
}

// Candidate is one way out for a provider. Direct is implied when LocalAddr
// is set and ProxyURL is nil; a proxy candidate sets ProxyURL only.
type Candidate struct {
	// Name is a stable identifier used for health bookkeeping and status.
	Name string
	// LocalAddr binds outbound connections to this source address.
	LocalAddr net.IP
	// ProxyURL routes the connection through this proxy (http/https/socks5).
	ProxyURL *url.URL
	// Weight biases StrategyWeighted. Missing = 1.
	Weight int
	// Tier orders this candidate against others. 0 = preferred.
	Tier Tier
	// Source tells the status panel whether the exit came from the provider's
	// own configuration or from an extension that contributed it.
	Source string
}

// Where a candidate came from.
const (
	// SourceConfig is a source address declared on the provider itself.
	SourceConfig = "config"
	// SourceExtension is an exit contributed by an extension (VPN endpoints).
	SourceExtension = "extension"
)

// Proxy reports whether the candidate routes through a proxy.
func (c Candidate) Proxy() bool { return c.ProxyURL != nil }

func (c Candidate) weight() int {
	if c.Weight <= 0 {
		return 1
	}
	return c.Weight
}

// Source supplies extension-provided candidates for a provider. It is called
// on every selection, so an implementation should answer from its own cached
// state rather than doing I/O.
type Source func(provider string) []Candidate

// badAfter hard failures mark a candidate unavailable...
const badAfter = 3

// ...for this long.
const coolDown = 30 * time.Second

// A rate-limited tier probes back after this window.
const limitedCoolDown = 45 * time.Second

// A rate-limited exit stays out of rotation for this long.
//
// The window is per exit, not per tier: an upstream that meters its free
// tier per source address hands out a quota to every address it sees, so
// cooling down the whole tier would throw away the addresses that still have
// quota left. 45s keeps an address out long enough for a short window to
// roll over, without parking it for the whole window when the operator has
// many addresses.
const limitedExitCoolDown = 45 * time.Second

type healthEntry struct {
	failures  int
	downUntil time.Time
	// limitedUntil is set when the upstream rate-limited this exit. The exit
	// stays out of rotation until the window passes, while the other exits
	// of its tier keep serving.
	limitedUntil time.Time
	// requests counts attempts through this exit, ok the ones that worked.
	requests uint64
	ok       uint64
	// lastOutcome/lastError/lastAt record the most recent attempt so the
	// status panel can show why an exit is being skipped, not just that it is.
	lastOutcome Outcome
	lastError   string
	lastAt      time.Time
}

type providerState struct {
	strategy Strategy
	core     []Candidate
	rr       uint64
	health   map[string]*healthEntry
	// disabled holds exits the operator turned off from the dashboard. They
	// stay visible in the status panel but are never selected.
	disabled map[string]bool
	// limitedUntil is keyed by tier: while set, that tier is passed over in
	// favour of a fallback tier.
	limitedUntil map[Tier]time.Time
}

// Registry tracks per-provider egress candidates. It is safe for concurrent
// use.
type Registry struct {
	mu         sync.RWMutex
	byProvider map[string]*providerState
	ext        Source
	now        func() time.Time
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byProvider: make(map[string]*providerState),
		now:        time.Now,
	}
}

// SetExtensionSource installs the callback that contributes extension-owned
// candidates (VPN endpoints, …). Passing nil disables it.
func (r *Registry) SetExtensionSource(src Source) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.ext = src
	r.mu.Unlock()
}

// SetCore replaces the preferred-tier candidates for a provider. Health that
// still applies is kept so a config reload does not clear a cool-down that
// has not expired yet.
func (r *Registry) SetCore(provider string, strategy Strategy, cands []Candidate) {
	if r == nil || strings.TrimSpace(provider) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.byProvider[provider]
	if st == nil {
		st = r.newState()
		r.byProvider[provider] = st
	}
	normalised := dedupe(cands)
	for i := range normalised {
		normalised[i].Tier = TierPreferred
		if normalised[i].Source == "" {
			normalised[i].Source = SourceConfig
		}
	}
	st.strategy = strategy
	st.core = normalised
	if st.disabled == nil {
		st.disabled = make(map[string]bool)
	}
	kept := make(map[string]*healthEntry, len(st.core))
	for _, c := range st.core {
		if h, ok := st.health[c.Name]; ok {
			kept[c.Name] = h
		}
	}
	st.health = kept
}

func (r *Registry) newState() *providerState {
	return &providerState{
		health:       make(map[string]*healthEntry),
		disabled:     make(map[string]bool),
		limitedUntil: make(map[Tier]time.Time),
	}
}

// Remove drops a provider's state (called when a provider disappears).
func (r *Registry) Remove(provider string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.byProvider, provider)
	r.mu.Unlock()
}

// SetDisabled replaces the set of exits an operator turned off for a provider.
// Disabled exits stay visible in the status panel but are never selected.
// Passing an empty set re-enables everything.
func (r *Registry) SetDisabled(provider string, names []string) {
	if r == nil || strings.TrimSpace(provider) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.byProvider[provider]
	if st == nil {
		st = r.newState()
		r.byProvider[provider] = st
	}
	next := make(map[string]bool, len(names))
	for _, n := range names {
		if n = strings.TrimSpace(n); n != "" {
			next[n] = true
		}
	}
	st.disabled = next
}

// Report records what an attempt through an exit did. detail is the upstream
// error or status line shown in the status panel; pass "" when there is none.
//
//   - OutcomeOK clears the exit's failures and its rate-limit cool-down.
//   - OutcomeFailed cools down the exit itself, so its neighbours in the
//     same tier keep serving.
//   - OutcomeLimited cools down the exit itself as well: a rate limit is
//     scoped to the address that hit it, so the other addresses of the tier
//     keep serving until their own quota runs out. The tier is demoted only
//     once every one of its exits is rate-limited, which is the point where
//     the next tier genuinely has to take over.
func (r *Registry) Report(provider string, c Candidate, outcome Outcome, detail string) {
	if r == nil || c.Name == "" {
		return
	}
	// The extension hook is called outside the lock: it may reach back into
	// the registry, and holding the write lock here would deadlock.
	exits := r.allCandidates(provider)

	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.byProvider[provider]
	if st == nil {
		return
	}
	now := r.now()
	h := st.health[c.Name]
	if h == nil {
		h = &healthEntry{}
		st.health[c.Name] = h
	}
	h.requests++
	h.lastOutcome = outcome
	h.lastError = detail
	h.lastAt = now
	switch outcome {
	case OutcomeOK:
		h.ok++
		h.failures = 0
		h.downUntil = time.Time{}
		h.limitedUntil = time.Time{}
		delete(st.limitedUntil, c.Tier)
	case OutcomeFailed:
		h.failures++
		if h.failures >= badAfter {
			h.downUntil = now.Add(coolDown)
		}
	case OutcomeLimited:
		h.failures = 0
		h.downUntil = time.Time{}
		// The limit belongs to this address, not to the tier: cool the exit
		// down and let its neighbours keep serving.
		h.limitedUntil = now.Add(limitedExitCoolDown)
		// Only when every exit of the tier is rate-limited is there nothing
		// left to gain by trying another address of the same tier, and the
		// fallback tier takes over until the windows roll over.
		if tierFullyLimited(st, exits, c.Tier, now) {
			st.limitedUntil[c.Tier] = now.Add(limitedCoolDown)
		}
	}
}

// allCandidates lists the exits of a provider: the core ones plus the ones
// an extension contributed, with duplicates removed.
func (r *Registry) allCandidates(provider string) []Candidate {
	r.mu.RLock()
	st := r.byProvider[provider]
	var core []Candidate
	if st != nil {
		core = dedupe(st.core)
	}
	ext := r.ext
	r.mu.RUnlock()

	all := dedupe(append([]Candidate{}, core...))
	if ext != nil {
		all = dedupe(append(all, ext(provider)...))
	}
	return all
}

// tierFullyLimited reports whether every enabled exit of a tier is currently
// rate-limited, which is the moment the tier as a whole has to give way.
func tierFullyLimited(st *providerState, exits []Candidate, tier Tier, now time.Time) bool {
	total := 0
	for _, e := range exits {
		if e.Tier != tier {
			continue
		}
		if st.disabled[e.Name] {
			continue
		}
		total++
		h := st.health[e.Name]
		if h == nil {
			// Never attempted: the tier still has an untouched exit.
			return false
		}
		if h.limitedUntil.IsZero() || !now.Before(h.limitedUntil) {
			return false
		}
	}
	return total > 0
}

// available reports whether a candidate may be tried right now.
func (st *providerState) available(name string, now time.Time) bool {
	h := st.health[name]
	if h == nil {
		return true
	}
	if !h.downUntil.IsZero() && now.Before(h.downUntil) {
		return false
	}
	// A rate-limited address is out of rotation until its window passes. The
	// other addresses of the tier are unaffected — that is the whole point of
	// cooling the exit down instead of the tier.
	if !h.limitedUntil.IsZero() && now.Before(h.limitedUntil) {
		return false
	}
	return true
}

func (st *providerState) tierLimited(t Tier, now time.Time) bool {
	until, ok := st.limitedUntil[t]
	return ok && now.Before(until)
}

// Pick returns the next candidate for a provider. Names in exclude (already
// tried during this request) are avoided unless every candidate has been
// tried, which is what turns a retry into a failover to another exit.
//
// The lowest usable tier wins: tier 0 — the configured source addresses plus
// any extension exit that asked to join them — while it answers, and the
// fallback tier once that tier fails or gets rate-limited.
func (r *Registry) Pick(provider string, exclude map[string]struct{}) (Candidate, bool) {
	if r == nil {
		return Candidate{}, false
	}
	now := r.now()

	r.mu.RLock()
	st := r.byProvider[provider]
	ext := r.ext
	var (
		core     []Candidate
		strategy = StrategyRoundRobin
		rr       uint64
		disabled map[string]bool
	)
	if st != nil {
		core = dedupe(st.core)
		strategy = st.strategy
		disabled = st.disabled
		// Advance the rotation counter while the state is still guarded.
		rr = atomic.AddUint64(&st.rr, 1)
	}
	r.mu.RUnlock()

	// An extension that returns tier 0 explicitly joins the preferred
	// rotation — the extended multi-IP set — instead of waiting for the local
	// addresses to fail. Entries that do not say which tier they want stay in
	// the fallback tier.
	var all []Candidate
	all = dedupe(append(all, core...))
	if ext != nil {
		all = append(all, ext(provider)...)
	}
	all = dedupe(all)
	all = filterDisabled(all, disabled)
	if len(all) == 0 {
		return Candidate{}, false
	}

	usable := make([]Candidate, 0, len(all))
	for _, c := range all {
		if _, skip := exclude[c.Name]; skip {
			continue
		}
		if st != nil && !st.available(c.Name, now) {
			continue
		}
		usable = append(usable, c)
	}
	if len(usable) == 0 {
		// Everything was tried or cooling down — fall back to the full set
		// rather than failing the request outright.
		usable = all
	}

	chosen := chooseTier(usable, func(t Tier) bool {
		return st == nil || !st.tierLimited(t, now)
	})
	return selectOne(chosen, strategy, rr), true
}

// chooseTier narrows candidates to the lowest tier that still passes the
// gate. If no gated tier is left (everything demoted or filtered), the lowest
// tier overall is used so a request is never left without an exit.
func chooseTier(cands []Candidate, allowed func(Tier) bool) []Candidate {
	byTier := make(map[Tier][]Candidate, 2)
	var order []Tier
	for _, c := range cands {
		if _, ok := byTier[c.Tier]; !ok {
			order = append(order, c.Tier)
		}
		byTier[c.Tier] = append(byTier[c.Tier], c)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	for _, t := range order {
		if allowed(t) && len(byTier[t]) > 0 {
			return byTier[t]
		}
	}
	lowest := order[0]
	return byTier[lowest]
}

func selectOne(cands []Candidate, strategy Strategy, rr uint64) Candidate {
	if len(cands) == 1 {
		return cands[0]
	}
	switch strategy {
	case StrategyFirst:
		return cands[0]
	case StrategyRandom:
		return cands[rand.Intn(len(cands))]
	case StrategyWeighted:
		total := 0
		for _, c := range cands {
			total += c.weight()
		}
		pick := rand.Intn(total)
		for _, c := range cands {
			pick -= c.weight()
			if pick < 0 {
				return c
			}
		}
		return cands[len(cands)-1]
	default: // round-robin
		return cands[int(rr-1)%len(cands)]
	}
}

// HasAlternative reports whether the provider still has an exit that was not
// tried yet during this request and is usable right now.
//
// It is what tells a rate limit apart from a dead end: a 429 on one address
// is worth retrying on the next one, but only while another address is left
// to try. With every exit used up the request has to surface the 429.
func (r *Registry) HasAlternative(provider string, exclude map[string]struct{}) bool {
	if r == nil {
		return false
	}
	now := r.now()

	r.mu.RLock()
	st := r.byProvider[provider]
	ext := r.ext
	var (
		core     []Candidate
		disabled map[string]bool
	)
	if st != nil {
		core = dedupe(st.core)
		disabled = st.disabled
	}
	r.mu.RUnlock()

	all := dedupe(append([]Candidate{}, core...))
	if ext != nil {
		all = dedupe(append(all, ext(provider)...))
	}
	all = filterDisabled(all, disabled)

	for _, c := range all {
		if _, skip := exclude[c.Name]; skip {
			continue
		}
		if st != nil && !st.available(c.Name, now) {
			continue
		}
		// A tier that was demoted wholesale is passed over as well.
		if st != nil && st.tierLimited(c.Tier, now) {
			continue
		}
		return true
	}
	return false
}

// ExitCount reports how many exits the provider currently has.
//
// The retry budget is stretched to match it: a rate limit belongs to one
// address, so a request has to be able to walk the whole pool before it
// gives up. With a fixed budget of four attempts and six addresses, a
// request could run out of attempts while live addresses were still left.
func (r *Registry) ExitCount(provider string) int {
	if r == nil {
		return 0
	}
	return len(r.Candidates(provider))
}

// Candidates returns a snapshot of the current candidate set for a provider,
// used by the status endpoints.
func (r *Registry) Candidates(provider string) []Candidate {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	st := r.byProvider[provider]
	ext := r.ext
	var (
		disabled map[string]bool
		core     []Candidate
	)
	if st != nil {
		disabled = st.disabled
		core = dedupe(st.core)
	}
	r.mu.RUnlock()
	var out []Candidate
	if core != nil {
		out = core
	}
	if ext != nil {
		out = append(out, ext(provider)...)
	}
	return filterDisabled(dedupe(out), disabled)
}

// filterDisabled drops the exits an operator turned off.
func filterDisabled(in []Candidate, disabled map[string]bool) []Candidate {
	if len(disabled) == 0 || len(in) == 0 {
		return in
	}
	out := make([]Candidate, 0, len(in))
	for _, c := range in {
		if disabled[c.Name] {
			continue
		}
		out = append(out, c)
	}
	return out
}

// ExitStatus is the operator-facing view of one exit: what it is, where it
// goes, and why it is or is not being used right now.
type ExitStatus struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	// Kind is "ip" for a local source address and "proxy" for a tunnelled exit.
	Kind   string `json:"kind"`
	Tier   Tier   `json:"tier"`
	// Address is the local source address, Proxy the tunnel endpoint.
	Address string `json:"address,omitempty"`
	Proxy   string `json:"proxy,omitempty"`
	Weight  int    `json:"weight,omitempty"`
	// Available is false while the exit is cooling down after failures.
	Available bool `json:"available"`
	// Disabled is true when the operator turned this exit off.
	Disabled bool `json:"disabled"`
	// TierLimited is true while the exit's whole tier is rate-limited.
	TierLimited bool `json:"tier_limited"`

	Failures int    `json:"failures"`
	Requests uint64 `json:"requests"`
	OK       uint64 `json:"ok"`

	CooldownUntil *time.Time `json:"cooldown_until,omitempty"`
	LastOutcome   string     `json:"last_outcome,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	LastAt        *time.Time `json:"last_at,omitempty"`
}

// ProviderEgressStatus is every exit of one provider plus how they are
// selected.
type ProviderEgressStatus struct {
	Provider string       `json:"provider"`
	Strategy Strategy     `json:"strategy"`
	Exits    []ExitStatus `json:"exits"`
}

// Status reports every exit of a provider, including the ones an operator
// disabled, with the health recorded for each. It returns false when the
// provider has no configured exits at all.
func (r *Registry) Status(provider string) (ProviderEgressStatus, bool) {
	if r == nil {
		return ProviderEgressStatus{}, false
	}
	now := r.now()

	r.mu.RLock()
	st := r.byProvider[provider]
	ext := r.ext
	var (
		strategy = StrategyRoundRobin
		disabled = make(map[string]bool)
		limited  = make(map[Tier]time.Time)
		health   = make(map[string]healthEntry)
		core     []Candidate
	)
	if st != nil {
		core = dedupe(st.core)
		strategy = st.strategy
		for k, v := range st.disabled {
			disabled[k] = v
		}
		for k, v := range st.limitedUntil {
			limited[k] = v
		}
		for k, v := range st.health {
			if v != nil {
				health[k] = *v
			}
		}
	}
	r.mu.RUnlock()

	var all []Candidate
	all = append(all, core...)
	if ext != nil {
		all = append(all, ext(provider)...)
	}
	all = dedupe(all)
	if len(all) == 0 {
		return ProviderEgressStatus{Provider: provider, Strategy: strategy, Exits: []ExitStatus{}}, false
	}

	out := ProviderEgressStatus{
		Provider: provider,
		Strategy: strategy,
		Exits:    make([]ExitStatus, 0, len(all)),
	}
	for _, c := range all {
		exit := ExitStatus{
			Name:     c.Name,
			Source:   c.Source,
			Tier:     c.Tier,
			Weight:   c.Weight,
			Disabled: disabled[c.Name],
		}
		if exit.Source == "" {
			exit.Source = SourceConfig
		}
		if c.LocalAddr != nil {
			exit.Kind = "ip"
			exit.Address = c.LocalAddr.String()
		} else {
			exit.Kind = "proxy"
			if c.ProxyURL != nil {
				exit.Proxy = c.ProxyURL.String()
			}
		}
		if h, ok := health[c.Name]; ok {
			exit.Requests = h.requests
			exit.OK = h.ok
			exit.Failures = h.failures
			if !h.downUntil.IsZero() {
				until := h.downUntil
				exit.CooldownUntil = &until
			}
			if !h.lastAt.IsZero() {
				at := h.lastAt
				exit.LastAt = &at
			}
			exit.LastOutcome = h.lastOutcome.String()
			exit.LastError = h.lastError
		}
		if until, ok := limited[c.Tier]; ok && now.Before(until) {
			exit.TierLimited = true
		}
		// An exit is usable unless the operator turned it off or it is
		// cooling down; a rate-limited tier is reported separately because it
		// affects every exit in it.
		exit.Available = !exit.Disabled && (exit.CooldownUntil == nil || !now.Before(*exit.CooldownUntil))
		out.Exits = append(out.Exits, exit)
	}
	sort.SliceStable(out.Exits, func(i, j int) bool {
		if out.Exits[i].Tier != out.Exits[j].Tier {
			return out.Exits[i].Tier < out.Exits[j].Tier
		}
		return out.Exits[i].Name < out.Exits[j].Name
	})
	return out, true
}

// StatusAll reports the egress status of every provider that has exits.
// Providers whose only exits come from an extension have no registry state of
// their own, so callers that know the configured provider names should ask
// Status(name) for those directly.
func (r *Registry) StatusAll() []ProviderEgressStatus {
	if r == nil {
		return nil
	}
	names := r.Providers()
	out := make([]ProviderEgressStatus, 0, len(names))
	for _, n := range names {
		if st, ok := r.Status(n); ok {
			out = append(out, st)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out
}

// Providers lists every provider with configured egress state.
func (r *Registry) Providers() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byProvider))
	for k := range r.byProvider {
		out = append(out, k)
	}
	return out
}

// Healthy reports whether a candidate is currently usable — surfaced in
// status payloads so an operator can see why an exit is being skipped.
func (r *Registry) Healthy(provider, name string) bool {
	if r == nil {
		return true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	st := r.byProvider[provider]
	if st == nil {
		return true
	}
	return st.available(name, r.now())
}

// SplitList parses a human-edited list value. It accepts a JSON-looking
// array, comma/semicolon/newline separated values, or a single value, and
// drops blanks — so an operator can type "a, b" anywhere a list is expected.
func SplitList(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	// Tolerate a JSON array pasted into a text field.
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		inner := strings.TrimSuffix(strings.TrimPrefix(v, "["), "]")
		return splitDelim(inner, ',')
	}
	return splitDelim(v, ',')
}

func splitDelim(s string, sep rune) []string {
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == sep || r == ';' || r == '\n' || r == '\r' || r == '\t'
	}) {
		part = strings.Trim(strings.TrimSpace(part), `"'`)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// MatchTargets reports whether any of the patterns matches any of the names.
//
// This is what makes a comma-separated target list ("zen, backup, mypool")
// declarative: an extension or egress pool names the providers and pools it
// applies to instead of relying on a single implicit match. "*" matches
// everything; a pattern ending in "*" is a prefix match.
func MatchTargets(patterns []string, names ...string) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "*" {
			return true
		}
		for _, n := range names {
			n = strings.TrimSpace(n)
			if n == "" {
				continue
			}
			if strings.EqualFold(p, n) {
				return true
			}
			if strings.HasSuffix(p, "*") && strings.HasPrefix(strings.ToLower(n), strings.ToLower(strings.TrimSuffix(p, "*"))) {
				return true
			}
		}
	}
	return false
}

func dedupe(in []Candidate) []Candidate {
	if len(in) == 0 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]Candidate, 0, len(in))
	for _, c := range in {
		if c.Name == "" {
			c.Name = defaultName(c)
		}
		if c.Name == "" {
			continue
		}
		if _, ok := seen[c.Name]; ok {
			continue
		}
		seen[c.Name] = struct{}{}
		out = append(out, c)
	}
	return out
}

func defaultName(c Candidate) string {
	if c.ProxyURL != nil {
		return fmt.Sprintf("proxy:%s", c.ProxyURL.String())
	}
	if c.LocalAddr != nil {
		return fmt.Sprintf("ip:%s", c.LocalAddr.String())
	}
	return ""
}

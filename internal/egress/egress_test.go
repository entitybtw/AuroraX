package egress

import (
	"net"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func ip(s string) net.IP { return net.ParseIP(s) }

func TestRoundRobinRotates(t *testing.T) {
	r := NewRegistry()
	r.SetCore("zen", StrategyRoundRobin, []Candidate{
		{Name: "a", LocalAddr: ip("10.0.0.1")},
		{Name: "b", LocalAddr: ip("10.0.0.2")},
		{Name: "c", LocalAddr: ip("10.0.0.3")},
	})

	var got []string
	for i := 0; i < 6; i++ {
		c, ok := r.Pick("zen", nil)
		if !ok {
			t.Fatal("Pick failed")
		}
		got = append(got, c.Name)
	}
	want := []string{"a", "b", "c", "a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// A retry must move to a different exit instead of hammering the same one.
func TestPickSkipsAlreadyTriedCandidates(t *testing.T) {
	r := NewRegistry()
	r.SetCore("zen", StrategyRoundRobin, []Candidate{
		{Name: "a", LocalAddr: ip("10.0.0.1")},
		{Name: "b", LocalAddr: ip("10.0.0.2")},
	})

	exclude := map[string]struct{}{"a": {}}
	c, ok := r.Pick("zen", exclude)
	if !ok || c.Name != "b" {
		t.Fatalf("Pick = %+v ok=%v, want b", c, ok)
	}

	// Once everything has been tried we fall back instead of failing.
	exclude = map[string]struct{}{"a": {}, "b": {}}
	if _, ok := r.Pick("zen", exclude); !ok {
		t.Fatal("expected a fallback candidate when all are excluded")
	}
}

func TestReportCoolsDownFailingCandidate(t *testing.T) {
	r := NewRegistry()
	now := time.Now()
	r.now = func() time.Time { return now }
	r.SetCore("zen", StrategyFirst, []Candidate{
		{Name: "a", LocalAddr: ip("10.0.0.1")},
		{Name: "b", LocalAddr: ip("10.0.0.2")},
	})

	for i := 0; i < badAfter; i++ {
		r.Report("zen", Candidate{Name: "a"}, OutcomeFailed, "")
	}
	if r.Healthy("zen", "a") {
		t.Fatal("a should be cooling down after repeated failures")
	}

	// "a" is skipped, so the first-strategy provider falls back to "b".
	c, _ := r.Pick("zen", nil)
	if c.Name != "b" {
		t.Fatalf("Pick = %q, want b while a is down", c.Name)
	}

	// A success re-arms it immediately.
	r.Report("zen", Candidate{Name: "a"}, OutcomeOK, "")
	if !r.Healthy("zen", "a") {
		t.Fatal("a should be healthy again after a success")
	}
}

func TestExtensionSourceMergesIntoPool(t *testing.T) {
	r := NewRegistry()
	r.SetCore("zen", StrategyRoundRobin, []Candidate{
		{Name: "ip1", LocalAddr: ip("10.0.0.1")},
	})
	proxy, _ := url.Parse("socks5://127.0.0.1:1080")
	r.SetExtensionSource(func(provider string) []Candidate {
		if provider != "zen" {
			return nil
		}
		return []Candidate{{Name: "vpn-node-1", ProxyURL: proxy, Tier: TierFallback}}
	})

	// The extension's endpoint is part of the pool, but only after the
	// preferred tier has been demoted by a rate limit.
	c, ok := r.Pick("zen", nil)
	if !ok || c.Name != "ip1" {
		t.Fatalf("Pick = %+v ok=%v, want ip1", c, ok)
	}
	r.Report("zen", c, OutcomeLimited, "")

	c, ok = r.Pick("zen", nil)
	if !ok || c.Name != "vpn-node-1" {
		t.Fatalf("Pick = %+v ok=%v, want the extension's endpoint", c, ok)
	}
	if c.Tier == TierPreferred {
		t.Fatalf("tier = %d, want the fallback tier", c.Tier)
	}

	// An unrelated provider must not see the extension's candidates.
	if got := r.Candidates("other"); len(got) != 0 {
		t.Fatalf("other provider candidates = %v, want none", got)
	}
}

func TestSetCoreKeepsHealthAcrossReload(t *testing.T) {
	r := NewRegistry()
	r.SetCore("zen", StrategyRoundRobin, []Candidate{{Name: "a"}, {Name: "b"}})
	r.Report("zen", Candidate{Name: "a"}, OutcomeFailed, "")
	r.Report("zen", Candidate{Name: "a"}, OutcomeFailed, "")
	r.Report("zen", Candidate{Name: "a"}, OutcomeFailed, "")

	r.SetCore("zen", StrategyRoundRobin, []Candidate{{Name: "a"}, {Name: "b"}})
	if r.Healthy("zen", "a") {
		t.Fatal("cool-down should survive a config reload")
	}
}

func TestSplitList(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"zen", []string{"zen"}},
		{"zen, backup", []string{"zen", "backup"}},
		{"zen;backup\nops", []string{"zen", "backup", "ops"}},
		{`["zen","backup"]`, []string{"zen", "backup"}},
		{" , ", nil},
	}
	for _, c := range cases {
		if got := SplitList(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitList(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestMatchTargets(t *testing.T) {
	cases := []struct {
		patterns []string
		names    []string
		want     bool
	}{
		{nil, []string{"zen"}, false},
		{[]string{"zen"}, []string{"zen"}, true},
		{[]string{"backup"}, []string{"zen"}, false},
		{[]string{"zen", "backup"}, []string{"ops"}, false},
		{[]string{"*"}, []string{"anything"}, true},
		{[]string{"zen-*"}, []string{"zen-main"}, true},
		{[]string{"ZEN"}, []string{"zen"}, true},
		{[]string{"mypool"}, []string{"zen", "mypool"}, true},
	}
	for _, c := range cases {
		if got := MatchTargets(c.patterns, c.names...); got != c.want {
			t.Errorf("MatchTargets(%v, %v) = %v, want %v", c.patterns, c.names, got, c.want)
		}
	}
}

func TestParseStrategyDefaultsToRoundRobin(t *testing.T) {
	for in, want := range map[string]Strategy{
		"":              StrategyRoundRobin,
		"random":        StrategyRandom,
		"weighted":      StrategyWeighted,
		"first":         StrategyFirst,
		"nonsense":      StrategyRoundRobin,
		" ROUND_ROBIN ": StrategyRoundRobin,
		"round_robin":   StrategyRoundRobin,
	} {
		if got := ParseStrategy(in); got != want {
			t.Errorf("ParseStrategy(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPickFailsWhenNothingConfigured(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Pick("unknown", nil); ok {
		t.Fatal("expected no candidate for an unconfigured provider")
	}
	var nilReg *Registry
	if _, ok := nilReg.Pick("zen", nil); ok {
		t.Fatal("nil registry must not pick")
	}
}

// Local source addresses are the default: the extension's endpoints are only
// reached once the preferred tier stops answering.
func TestPreferredTierWinsWhileHealthy(t *testing.T) {
	r := NewRegistry()
	r.SetCore("zen", StrategyRoundRobin, []Candidate{
		{Name: "ip1", LocalAddr: ip("10.0.0.1")},
		{Name: "ip2", LocalAddr: ip("10.0.0.2")},
	})
	proxy, _ := url.Parse("socks5://127.0.0.1:1080")
	r.SetExtensionSource(func(string) []Candidate {
		return []Candidate{{Name: "vpn1", ProxyURL: proxy, Tier: TierFallback}}
	})

	for i := 0; i < 6; i++ {
		c, ok := r.Pick("zen", nil)
		if !ok {
			t.Fatal("Pick failed")
		}
		if c.Tier != TierPreferred || c.Name == "vpn1" {
			t.Fatalf("Pick = %+v, want a local source address", c)
		}
	}
}

// A 429/403 demotes the whole preferred tier: the extension's endpoints take
// over, and direct egress returns by itself once the window closes.
func TestRateLimitedPreferredTierFallsBackToExtension(t *testing.T) {
	r := NewRegistry()
	now := time.Now()
	r.now = func() time.Time { return now }
	r.SetCore("zen", StrategyRoundRobin, []Candidate{{Name: "ip1", LocalAddr: ip("10.0.0.1")}})
	proxy, _ := url.Parse("socks5://127.0.0.1:1080")
	r.SetExtensionSource(func(string) []Candidate {
		return []Candidate{{Name: "vpn1", ProxyURL: proxy, Tier: TierFallback}}
	})

	c, _ := r.Pick("zen", nil)
	if c.Name != "ip1" {
		t.Fatalf("Pick = %q, want ip1 before any limit", c.Name)
	}
	r.Report("zen", c, OutcomeLimited, "")

	c, _ = r.Pick("zen", nil)
	if c.Name != "vpn1" || c.Tier == TierPreferred {
		t.Fatalf("after rate limit Pick = %+v, want the fallback tier", c)
	}

	// The window closes and the gateway probes the local address again.
	now = now.Add(limitedCoolDown + time.Second)
	c, _ = r.Pick("zen", nil)
	if c.Name != "ip1" {
		t.Fatalf("after cooldown Pick = %q, want ip1", c.Name)
	}
}

// A hard failure is the address's own fault, so the tier keeps serving with
// its remaining addresses instead of jumping straight to the fallback tier.
func TestHardFailureStaysInPreferredTier(t *testing.T) {
	r := NewRegistry()
	now := time.Now()
	r.now = func() time.Time { return now }
	r.SetCore("zen", StrategyRoundRobin, []Candidate{
		{Name: "ip1", LocalAddr: ip("10.0.0.1")},
		{Name: "ip2", LocalAddr: ip("10.0.0.2")},
	})
	proxy, _ := url.Parse("socks5://127.0.0.1:1080")
	r.SetExtensionSource(func(string) []Candidate {
		return []Candidate{{Name: "vpn1", ProxyURL: proxy, Tier: TierFallback}}
	})

	bad, _ := r.Pick("zen", nil)
	for i := 0; i < badAfter; i++ {
		r.Report("zen", bad, OutcomeFailed, "")
	}

	for i := 0; i < 4; i++ {
		c, _ := r.Pick("zen", nil)
		if c.Name == "vpn1" {
			t.Fatalf("Pick = %v, must not fall back to the VPN tier", c.Name)
		}
	}
}

// An extension that asks for tier 0 joins the provider's own rotation
// immediately instead of waiting for the local addresses to fail.
func TestExtensionTierZeroJoinsPreferredRotation(t *testing.T) {
	r := NewRegistry()
	r.SetCore("zen", StrategyRoundRobin, []Candidate{
		{Name: "ip1", LocalAddr: ip("10.0.0.1")},
		{Name: "ip2", LocalAddr: ip("10.0.0.2")},
	})
	proxy, _ := url.Parse("socks5://127.0.0.1:1080")
	r.SetExtensionSource(func(string) []Candidate {
		return []Candidate{{Name: "vpn1", ProxyURL: proxy, Tier: TierPreferred}}
	})

	seen := make(map[string]bool)
	for i := 0; i < 6; i++ {
		c, ok := r.Pick("zen", nil)
		if !ok {
			t.Fatal("Pick failed")
		}
		if c.Tier != TierPreferred {
			t.Fatalf("tier = %d, want the preferred tier", c.Tier)
		}
		seen[c.Name] = true
	}
	if len(seen) != 3 {
		t.Fatalf("picked %v, want ip1, ip2 and vpn1 all in rotation", seen)
	}
}

// Disabled exits stay reported (so the operator can see them) but are never
// selected.
func TestDisabledExitsAreHiddenFromPickButVisibleInStatus(t *testing.T) {
	r := NewRegistry()
	r.SetCore("zen", StrategyRoundRobin, []Candidate{
		{Name: "ip1", LocalAddr: ip("10.0.0.1")},
		{Name: "ip2", LocalAddr: ip("10.0.0.2")},
	})
	r.SetDisabled("zen", []string{"ip2"})

	for i := 0; i < 6; i++ {
		c, ok := r.Pick("zen", nil)
		if !ok {
			t.Fatal("Pick failed")
		}
		if c.Name == "ip2" {
			t.Fatalf("Pick = %q, a disabled exit must not be selected", c.Name)
		}
	}
	if got := len(r.Candidates("zen")); got != 1 {
		t.Fatalf("Candidates = %d, want 1 selectable exit", got)
	}

	st, ok := r.Status("zen")
	if !ok {
		t.Fatal("Status reported no exits")
	}
	if len(st.Exits) != 2 {
		t.Fatalf("exits = %d, want both the enabled and the disabled one", len(st.Exits))
	}
	for _, exit := range st.Exits {
		if exit.Name != "ip2" {
			continue
		}
		if !exit.Disabled || exit.Available {
			t.Fatalf("ip2 status = %+v, want disabled and unavailable", exit)
		}
	}

	// Clearing the set re-enables everything.
	r.SetDisabled("zen", nil)
	if got := len(r.Candidates("zen")); got != 2 {
		t.Fatalf("Candidates after re-enable = %d, want 2", got)
	}
}

// The status panel shows why an exit last failed, not just that it did.
func TestStatusCarriesLastOutcomeAndError(t *testing.T) {
	r := NewRegistry()
	now := time.Now()
	r.now = func() time.Time { return now }
	r.SetCore("zen", StrategyFirst, []Candidate{{Name: "ip1", LocalAddr: ip("10.0.0.1")}})
	for i := 0; i < badAfter; i++ {
		r.Report("zen", Candidate{Name: "ip1", Tier: TierPreferred}, OutcomeFailed, "connection reset by peer")
	}

	st, ok := r.Status("zen")
	if !ok || len(st.Exits) != 1 {
		t.Fatalf("Status = %+v ok=%v, want one exit", st, ok)
	}
	exit := st.Exits[0]
	if exit.Requests != badAfter || exit.OK != 0 {
		t.Fatalf("counters = requests:%d ok:%d, want %d/0", exit.Requests, exit.OK, badAfter)
	}
	if exit.LastOutcome != "failed" {
		t.Fatalf("last outcome = %q, want failed", exit.LastOutcome)
	}
	if exit.LastError != "connection reset by peer" {
		t.Fatalf("last error = %q, want the reported detail", exit.LastError)
	}
	if exit.CooldownUntil == nil || !exit.CooldownUntil.After(now) {
		t.Fatalf("cooldown = %v, want a future deadline after repeated failures", exit.CooldownUntil)
	}

	if all := r.StatusAll(); len(all) != 1 || all[0].Provider != "zen" {
		t.Fatalf("StatusAll = %+v, want the zen provider", all)
	}
}

// An exclusive tier is picked ahead of the provider's own source addresses,
// and falls back to them once it is rate-limited.
func TestExclusiveTierPrecedesPreferred(t *testing.T) {
	r := NewRegistry()
	now := time.Now()
	r.now = func() time.Time { return now }
	r.SetCore("zen", StrategyRoundRobin, []Candidate{
		{Name: "ip1", LocalAddr: ip("10.0.0.1")},
	})
	proxy, _ := url.Parse("socks5://127.0.0.1:1080")
	r.SetExtensionSource(func(string) []Candidate {
		return []Candidate{{Name: "vpn1", ProxyURL: proxy, Tier: TierExclusive}}
	})

	c, ok := r.Pick("zen", nil)
	if !ok || c.Name != "vpn1" || c.Tier != TierExclusive {
		t.Fatalf("Pick = %+v ok=%v, want the exclusive vpn exit", c, ok)
	}

	r.Report("zen", c, OutcomeLimited, "")
	c, _ = r.Pick("zen", nil)
	if c.Name != "ip1" {
		t.Fatalf("after a rate limit Pick = %+v, want the source address", c)
	}
}

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
		r.Report("zen", Candidate{Name: "a"}, OutcomeFailed)
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
	r.Report("zen", Candidate{Name: "a"}, OutcomeOK)
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
		return []Candidate{{Name: "vpn-node-1", ProxyURL: proxy}}
	})

	// The extension's endpoint is part of the pool, but only after the
	// preferred tier has been demoted by a rate limit.
	c, ok := r.Pick("zen", nil)
	if !ok || c.Name != "ip1" {
		t.Fatalf("Pick = %+v ok=%v, want ip1", c, ok)
	}
	r.Report("zen", c, OutcomeLimited)

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
	r.Report("zen", Candidate{Name: "a"}, OutcomeFailed)
	r.Report("zen", Candidate{Name: "a"}, OutcomeFailed)
	r.Report("zen", Candidate{Name: "a"}, OutcomeFailed)

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
		return []Candidate{{Name: "vpn1", ProxyURL: proxy}}
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
		return []Candidate{{Name: "vpn1", ProxyURL: proxy}}
	})

	c, _ := r.Pick("zen", nil)
	if c.Name != "ip1" {
		t.Fatalf("Pick = %q, want ip1 before any limit", c.Name)
	}
	r.Report("zen", c, OutcomeLimited)

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
		return []Candidate{{Name: "vpn1", ProxyURL: proxy}}
	})

	bad, _ := r.Pick("zen", nil)
	for i := 0; i < badAfter; i++ {
		r.Report("zen", bad, OutcomeFailed)
	}

	for i := 0; i < 4; i++ {
		c, _ := r.Pick("zen", nil)
		if c.Name == "vpn1" {
			t.Fatalf("Pick = %v, must not fall back to the VPN tier", c.Name)
		}
	}
}

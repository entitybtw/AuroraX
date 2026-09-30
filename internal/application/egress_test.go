package app

import (
	"testing"

	"aurora/configuration"
	"aurora/internal/egress"
)


func TestEgressCandidatesFoldsLegacyBindIP(t *testing.T) {
	// bind_ips is the newer field; a provider that only carries the singular
	// bind_ip must still get a preferred-tier exit instead of an empty set.
	cands := egressCandidates(config.RawProviderConfig{BindIP: "203.0.113.7"})
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate from bind_ip, got %d", len(cands))
	}
	if cands[0].Name != "ip:203.0.113.7" || cands[0].LocalAddr == nil {
		t.Fatalf("unexpected candidate %+v", cands[0])
	}
	if cands[0].Tier != egress.TierPreferred {
		t.Fatalf("expected preferred tier, got %d", cands[0].Tier)
	}
}

func TestEgressCandidatesDeduplicatesAndSkipsInvalid(t *testing.T) {
	cands := egressCandidates(config.RawProviderConfig{
		BindIP:  "203.0.113.7",
		BindIPs: []string{"203.0.113.7", "198.51.100.4", "not-an-ip", "  "},
	})
	if len(cands) != 2 {
		t.Fatalf("expected 2 unique valid candidates, got %d (%v)", len(cands), cands)
	}
}

// An addon that does not state a tier keeps the documented default (fallback);
// an explicit tier 0 promotes it into the provider's own rotation.
func TestParseEgressCandidatesTierDefaults(t *testing.T) {
	cands := parseEgressCandidates(`[
		{"name":"vpn:eu-1","proxy":"socks5://127.0.0.1:1080"},
		{"name":"vpn:eu-2","proxy":"socks5://127.0.0.1:1081","tier":0},
		{"name":"vpn:eu-3","proxy":"socks5://127.0.0.1:1082","tier":1}
	]`)
	if len(cands) != 3 {
		t.Fatalf("parsed %d candidates, want 3", len(cands))
	}
	want := []egress.Tier{egress.TierFallback, egress.TierPreferred, egress.TierFallback}
	for i, c := range cands {
		if c.Tier != want[i] {
			t.Fatalf("candidate %q tier = %d, want %d", c.Name, c.Tier, want[i])
		}
		if c.Source != egress.SourceExtension {
			t.Fatalf("candidate %q source = %q, want %q", c.Name, c.Source, egress.SourceExtension)
		}
	}
}

// A configured bind_ip is tagged as config-sourced so the status panel can
// tell the provider's own addresses apart from extension-supplied ones.
func TestEgressCandidatesAreConfigSourced(t *testing.T) {
	cands := egressCandidates(config.RawProviderConfig{BindIPs: []string{"203.0.113.7"}})
	if len(cands) != 1 || cands[0].Source != egress.SourceConfig {
		t.Fatalf("candidates = %+v, want one config-sourced exit", cands)
	}
}

// An addon may put its exits ahead of the provider's own addresses (vpn_only)
// or below the fallback tier; the gateway floors the value at TierExclusive.
func TestParseEgressCandidatesHonoursExclusiveTier(t *testing.T) {
	cands := parseEgressCandidates(`[
		{"name":"vpn:eu-1","proxy":"socks5://127.0.0.1:1080","tier":-1},
		{"name":"vpn:eu-2","proxy":"socks5://127.0.0.1:1081","tier":-99}
	]`)
	if len(cands) != 2 {
		t.Fatalf("parsed %d candidates, want 2", len(cands))
	}
	if cands[0].Tier != egress.TierExclusive {
		t.Fatalf("tier = %d, want TierExclusive", cands[0].Tier)
	}
	if cands[1].Tier != egress.TierExclusive {
		t.Fatalf("floored tier = %d, want TierExclusive", cands[1].Tier)
	}
}

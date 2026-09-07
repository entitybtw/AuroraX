package sessionhub

import (
	"net/http"
	"regexp"
	"testing"
)

func TestHubApply_MapGeneratesStableUniquePerProvider(t *testing.T) {
	cfg := &HubConfig{
		Enabled: true,
		Providers: map[string]ProviderRule{
			"acc1": {
				Enabled: true,
				Headers: []HeaderRule{{
					Name:   "x-opencode-session",
					Mode:   HeaderModeMap,
					Prefix: "ses_",
					Length: 28,
				}},
			},
			"acc2": {
				Enabled: true,
				Headers: []HeaderRule{{
					Name:   "x-opencode-session",
					Mode:   HeaderModeMap,
					Prefix: "ses_",
					Length: 28,
				}},
			},
		},
	}
	h := New(cfg)

	inbound := "ses_client_original_123"
	re := regexp.MustCompile(`^ses_[A-Za-z0-9]{28}$`)

	newReq := func() http.Header {
		hd := http.Header{}
		hd.Set("x-opencode-session", inbound)
		return hd
	}

	// First provider gets a unique session
	h1 := newReq()
	res1 := h.Apply(h1, "acc1")
	if res1 == nil {
		t.Fatal("expected a mapping for acc1")
	}
	c1 := h1.Get("x-opencode-session")
	if !re.MatchString(c1) {
		t.Fatalf("outbound for acc1 not alphanumeric 28: got %q", c1)
	}

	// Second provider must get a DIFFERENT session (so upstream sees distinct clients)
	h2 := newReq()
	h2.Set("x-opencode-session", inbound)
	h.Apply(h2, "acc2")
	c2 := h2.Get("x-opencode-session")
	if c1 == c2 {
		t.Fatalf("acc1 and acc2 share outbound session %q; want distinct", c1)
	}

	// Replay same inbound to acc1 must return the SAME outbound (stable mapping)
	h1b := newReq()
	h.Apply(h1b, "acc1")
	if got := h1b.Get("x-opencode-session"); got != c1 {
		t.Fatalf("acc1 unstable: first %q then %q", c1, got)
	}
}

func TestHubApply_NoRuleNoChange(t *testing.T) {
	h := New(&HubConfig{Enabled: true, Providers: map[string]ProviderRule{}})
	hd := http.Header{}
	hd.Set("x-opencode-session", "ses_inbound")
	res := h.Apply(hd, "nope")
	if res != nil {
		t.Fatalf("expected nil when no rule matches, got %v", res)
	}
	if hd.Get("x-opencode-session") != "ses_inbound" {
		t.Fatalf("header should be unchanged")
	}
}

func TestHubApply_PoolBoundRuleAppliesToMember(t *testing.T) {
	h := New(&HubConfig{
		Enabled: true,
		Providers: map[string]ProviderRule{
			"opencode-pool": {
				Enabled: true,
				Headers: []HeaderRule{{
					Name:   "x-opencode-session",
					Mode:   HeaderModeMap,
					Prefix: "ses_",
					Length: 28,
				}},
			},
		},
	})
	h.SetPoolMembership("opencode-pool", []string{"acc1", "acc2"})

	hd := http.Header{}
	hd.Set("x-opencode-session", "ses_inbound_pool")
	res := h.Apply(hd, "acc1") // member routed through pool
	if res == nil {
		t.Fatal("expected pool-bound rule to apply to member acc1")
	}
	out := hd.Get("x-opencode-session")
	re := regexp.MustCompile(`^ses_[A-Za-z0-9]{28}$`)
	if !re.MatchString(out) {
		t.Fatalf("expected generated session, got %q", out)
	}
}

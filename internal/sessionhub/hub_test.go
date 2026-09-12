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

func TestHubPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/rules.yaml"

	cfg := &HubConfig{
		Enabled: true,
		Providers: map[string]ProviderRule{
			"opencode-zen": {
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
	h := NewWithPersistence(path, path+".map", cfg)
	h.SetProviderRule("acc2", ProviderRule{
		Enabled: true,
		Headers: []HeaderRule{{Name: "user-agent", Mode: HeaderModeStatic, Value: "opencode-cli/1.0"}},
	})

	// Load into a fresh hub from disk
	h2 := NewWithPersistence(path, path+".map", nil)
	if _, ok := h2.Config().Providers["opencode-zen"]; !ok {
		t.Fatal("persisted rule opencode-zen missing after reload")
	}
	rule, ok := h2.Config().Providers["acc2"]
	if !ok || len(rule.Headers) == 0 || rule.Headers[0].Value != "opencode-cli/1.0" {
		t.Fatalf("persisted rule acc2 not correctly reloaded: %+v", h2.Config().Providers)
	}
}

func TestStoreMappingPersistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/mappings.json"

	s := NewStore(WithPersistence(path))
	if s.StorageMode() != "disk" {
		t.Fatalf("expected disk storage, got %s", s.StorageMode())
	}
	s.GetOrCreate("acc1", "ses_in", "ses_", 28)

	// Fresh store on same file restores mapping
	s2 := NewStore(WithPersistence(path))
	if got := s2.Get("acc1", "ses_in"); got == "" {
		t.Fatal("mapping not restored from disk")
	}
}

func TestStoreMemoryToggle(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/mappings.json"

	s := NewStore(WithPersistence(path))
	if s.StorageMode() != "disk" {
		t.Fatalf("expected disk, got %s", s.StorageMode())
	}
	s.SetPersists(false)
	if s.StorageMode() != "memory" {
		t.Fatalf("expected memory after disable, got %s", s.StorageMode())
	}
	s.SetPersists(true)
	if s.StorageMode() != "disk" {
		t.Fatalf("expected disk after enable, got %s", s.StorageMode())
	}
}

func TestHubApply_MapOrGenerate_WithSession(t *testing.T) {
	cfg := &HubConfig{
		Enabled: true,
		Providers: map[string]ProviderRule{
			"acc1": {
				Enabled: true,
				Headers: []HeaderRule{{
					Name:   "x-opencode-session",
					Mode:   HeaderModeMapOrGenerate,
					Prefix: "ses_",
					Length: 28,
				}},
			},
		},
	}
	h := New(cfg)

	inbound := "ses_client_existing"
	re := regexp.MustCompile(`^ses_[A-Za-z0-9]{28}$`)

	hd := http.Header{}
	hd.Set("x-opencode-session", inbound)
	res := h.Apply(hd, "acc1")
	if res == nil {
		t.Fatal("expected a mapping for acc1")
	}
	out := hd.Get("x-opencode-session")
	if !re.MatchString(out) {
		t.Fatalf("outbound not alphanumeric 28: got %q", out)
	}

	hd2 := http.Header{}
	hd2.Set("x-opencode-session", inbound)
	h.Apply(hd2, "acc1")
	if got := hd2.Get("x-opencode-session"); got != out {
		t.Fatalf("stable mapping expected: first %q then %q", out, got)
	}
}

func TestHubApply_MapOrGenerate_WithoutSession(t *testing.T) {
	cfg := &HubConfig{
		Enabled: true,
		Providers: map[string]ProviderRule{
			"acc1": {
				Enabled: true,
				Headers: []HeaderRule{{
					Name:   "x-opencode-session",
					Mode:   HeaderModeMapOrGenerate,
					Prefix: "ses_",
					Length: 28,
				}},
			},
		},
	}
	h := New(cfg)

	re := regexp.MustCompile(`^ses_[A-Za-z0-9]{28}$`)

	hd := http.Header{}
	res := h.Apply(hd, "acc1")
	if res == nil {
		t.Fatal("expected a generated value for acc1")
	}
	out := hd.Get("x-opencode-session")
	if !re.MatchString(out) {
		t.Fatalf("generated value not alphanumeric 28: got %q", out)
	}

	hd2 := http.Header{}
	h.Apply(hd2, "acc1")
	out2 := hd2.Get("x-opencode-session")
	if out == out2 {
		t.Fatalf("generate mode should produce fresh values, got same %q twice", out)
	}
}

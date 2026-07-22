package pool

import (
	"testing"
)

type stubHealth struct {
	unhealth map[string]bool
}

func (s *stubHealth) IsProviderHealthy(name string) bool {
	if s.unhealth == nil {
		return true
	}
	return !s.unhealth[name]
}

func (s *stubHealth) markUnhealthy(name string) {
	if s.unhealth == nil {
		s.unhealth = map[string]bool{}
	}
	s.unhealth[name] = true
}

func mustNewPool(t *testing.T, cfg Config) *Pool {
	t.Helper()
	p, err := NewPool(cfg)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	return p
}

func TestParseStrategy(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Strategy
		wantErr bool
	}{
		{"empty defaults to round_robin", "", StrategyRoundRobin, false},
		{"round_robin", "round_robin", StrategyRoundRobin, false},
		{"unknown rejected", "magic", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseStrategy(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr=%v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewPool_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"empty name", Config{Members: []MemberConfig{{ProviderName: "a"}}}, true},
		{"no members", Config{Name: "p"}, true},
		{"empty member name", Config{Name: "p", Members: []MemberConfig{{ProviderName: ""}}}, true},
		{"duplicate member", Config{Name: "p", Members: []MemberConfig{{ProviderName: "a"}, {ProviderName: "a"}}}, true},
		{"valid", Config{Name: "p", Members: []MemberConfig{{ProviderName: "a"}, {ProviderName: "b"}}}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewPool(tc.cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("NewPool err = %v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestPool_RoundRobin_RotatesAcrossMembers(t *testing.T) {
	p := mustNewPool(t, Config{
		Name:    "rr",
		Members: []MemberConfig{{ProviderName: "a"}, {ProviderName: "b"}, {ProviderName: "c"}},
	})

	got := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		name, release, err := p.Pick(map[string]struct{}{})
		if err != nil {
			t.Fatalf("Pick: %v", err)
		}
		release(true)
		got = append(got, name)
	}

	counts := map[string]int{}
	for _, n := range got {
		counts[n]++
	}
	for _, n := range []string{"a", "b", "c"} {
		if counts[n] != 2 {
			t.Errorf("member %q hit %d times, want 2 (got sequence %v)", n, counts[n], got)
		}
	}
}

func TestPool_HealthAware_SkipsUnhealthy(t *testing.T) {
	health := &stubHealth{}
	health.markUnhealthy("b")
	p := mustNewPool(t, Config{
		Name:    "h",
		Members: []MemberConfig{{ProviderName: "a"}, {ProviderName: "b"}, {ProviderName: "c"}},
		Health:  health,
	})

	for i := 0; i < 10; i++ {
		name, release, err := p.Pick(map[string]struct{}{})
		if err != nil {
			t.Fatalf("Pick: %v", err)
		}
		release(true)
		if name == "b" {
			t.Fatalf("picked unhealthy member %q", name)
		}
	}
}

func TestPool_Pick_ExcludesTried(t *testing.T) {
	p := mustNewPool(t, Config{
		Name:    "ex",
		Members: []MemberConfig{{ProviderName: "a"}, {ProviderName: "b"}, {ProviderName: "c"}},
	})

	tried := map[string]struct{}{"a": {}, "b": {}}
	name, release, err := p.Pick(tried)
	if err != nil {
		t.Fatalf("Pick: %v", err)
	}
	release(true)
	if name != "c" {
		t.Fatalf("Pick = %q, want c", name)
	}
}

func TestPool_AllUnhealthy_ReturnsError(t *testing.T) {
	health := &stubHealth{}
	health.markUnhealthy("a")
	health.markUnhealthy("b")
	p := mustNewPool(t, Config{
		Name:    "dead",
		Members: []MemberConfig{{ProviderName: "a"}, {ProviderName: "b"}},
		Health:  health,
	})

	_, _, err := p.Pick(map[string]struct{}{})
	if err == nil {
		t.Fatal("expected error when all members are unhealthy")
	}
}

func TestPool_Members_SnapshotReturnsMetadata(t *testing.T) {
	p := mustNewPool(t, Config{
		Name:    "snap",
		Members: []MemberConfig{{ProviderName: "a"}, {ProviderName: "b"}},
	})

	members := p.Members()
	if len(members) != 2 {
		t.Fatalf("got %d members, want 2", len(members))
	}
}

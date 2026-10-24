package llmclient

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"aurora/internal/egress"
)

// sourceIPs records which address each request arrived from.
func echoSource(t *testing.T) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			mu.Lock()
			seen = append(seen, host)
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

// The exit registry is consulted per logical request, so consecutive requests
// walk the configured source addresses.
func TestEgressRotatesSourceAddressesPerRequest(t *testing.T) {
	srv, _ := echoSource(t)

	reg := egress.NewRegistry()
	reg.SetCore("rotating", egress.StrategyRoundRobin, []egress.Candidate{
		{Name: "ip1", LocalAddr: net.ParseIP("127.0.0.1")},
		{Name: "ip2", LocalAddr: net.ParseIP("127.0.0.2")},
	})

	cfg := DefaultConfig("rotating", srv.URL)
	cfg.Retry.MaxRetries = 0
	cfg.Egress = reg
	client := New(cfg, nil)

	used := map[string]bool{}
	for i := 0; i < 4; i++ {
		if _, err := client.DoRaw(context.Background(), Request{Method: http.MethodGet, Endpoint: "/x"}); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	// One pooled client per exit: two entries means both addresses served a
	// request rather than the first one being pinned forever.
	client.egressPool.mu.Lock()
	for name := range client.egressPool.byName {
		used[name] = true
	}
	client.egressPool.mu.Unlock()
	if len(used) != 2 {
		t.Fatalf("exercises exits = %v, want both configured addresses", used)
	}
}

// A source address that cannot be bound must not consume the whole retry
// budget: the next attempt moves to the next exit and the request succeeds.
func TestEgressFailsOverToNextExitOnRetry(t *testing.T) {
	srv, _ := echoSource(t)

	reg := egress.NewRegistry()
	// 203.0.113.1 is not local, so binding it fails immediately.
	reg.SetCore("failing", egress.StrategyFirst, []egress.Candidate{
		{Name: "bad", LocalAddr: net.ParseIP("203.0.113.1")},
		{Name: "good", LocalAddr: net.ParseIP("127.0.0.1")},
	})

	cfg := DefaultConfig("failing", srv.URL)
	cfg.Retry.MaxRetries = 2
	cfg.Egress = reg
	client := New(cfg, nil)

	if _, err := client.DoRaw(context.Background(), Request{Method: http.MethodGet, Endpoint: "/x"}); err != nil {
		t.Fatalf("request should have failed over to a healthy exit: %v", err)
	}
}

// A rate limit is the answer about the request's own quota. When the walk
// over the exits ends on a server error — a dead tunnel, a broken proxy —
// the rate limit that was seen on the way is what the caller gets: a spent
// quota is actionable, a gateway timeout is not.
func TestEgressRateLimitSurvivesBrokenExitAfterwards(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = "unknown"
		}
		mu.Lock()
		seen[host]++
		mu.Unlock()
		switch host {
		case "127.0.0.1":
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
		default:
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"broken exit"}`))
		}
	}))
	defer srv.Close()

	reg := egress.NewRegistry()
	reg.SetCore("mixed", egress.StrategyFirst, []egress.Candidate{
		{Name: "limited", LocalAddr: net.ParseIP("127.0.0.1")},
		{Name: "broken", LocalAddr: net.ParseIP("127.0.0.2")},
	})

	cfg := DefaultConfig("mixed", srv.URL)
	cfg.Retry.MaxRetries = 0
	cfg.Egress = reg
	client := New(cfg, nil)

	_, err := client.DoRaw(context.Background(), Request{Method: http.MethodGet, Endpoint: "/x"})
	if err == nil {
		t.Fatal("the request should have failed: every exit is unusable")
	}
	if got := extractStatusCode(err); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d: the rate limit must not be masked by the broken exit", got, http.StatusTooManyRequests)
	}
}

// The same holds for a passthrough request: the answer the client receives is
// the rate limit, not the server error of the exit that was tried last.
func TestEgressRateLimitSurvivesBrokenExitAfterwardsInPassthrough(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = "unknown"
		}
		mu.Lock()
		seen[host]++
		mu.Unlock()
		switch host {
		case "127.0.0.1":
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
		default:
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"broken exit"}`))
		}
	}))
	defer srv.Close()

	reg := egress.NewRegistry()
	reg.SetCore("mixed", egress.StrategyFirst, []egress.Candidate{
		{Name: "limited", LocalAddr: net.ParseIP("127.0.0.1")},
		{Name: "broken", LocalAddr: net.ParseIP("127.0.0.2")},
	})

	cfg := DefaultConfig("mixed", srv.URL)
	cfg.Retry.MaxRetries = 0
	cfg.Egress = reg
	client := New(cfg, nil)

	resp, err := client.DoPassthrough(context.Background(), Request{Method: http.MethodGet, Endpoint: "/x"})
	if err != nil {
		t.Fatalf("the passthrough should answer with the rate limit: %v", err)
	}
	if resp == nil {
		t.Fatal("the passthrough returned no response")
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d: the rate limit must not be masked by the broken exit", resp.StatusCode, http.StatusTooManyRequests)
	}
}

// A 429 from one exit is scoped to that address: the request is retried on
// another exit of the same provider instead of being handed to the caller.
func TestEgressRateLimitFailsOverToNextExitInRequest(t *testing.T) {	var mu sync.Mutex
	seen := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = "unknown"
		}
		mu.Lock()
		seen[host]++
		mu.Unlock()
		// The first address is rate limited, the second one answers.
		if host == "127.0.0.1" {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	reg := egress.NewRegistry()
	reg.SetCore("limited", egress.StrategyFirst, []egress.Candidate{
		{Name: "first", LocalAddr: net.ParseIP("127.0.0.1")},
		{Name: "second", LocalAddr: net.ParseIP("127.0.0.2")},
	})

	cfg := DefaultConfig("limited", srv.URL)
	cfg.Retry.MaxRetries = 0
	cfg.Egress = reg
	client := New(cfg, nil)

	// The 429 is absorbed: the request succeeds on the second exit.
	if _, err := client.DoRaw(context.Background(), Request{Method: http.MethodGet, Endpoint: "/x"}); err != nil {
		t.Fatalf("request should have failed over to the next exit: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if seen["127.0.0.1"] == 0 {
		t.Fatal("the rate-limited exit was never tried")
	}
	if seen["127.0.0.2"] == 0 {
		t.Fatal("the request never reached the second exit")
	}
}

// Without a registry nothing changes: the client keeps using its single
// transport.
func TestEgressDisabledFallsBackToDefaultClient(t *testing.T) {
	srv, _ := echoSource(t)
	cfg := DefaultConfig("plain", srv.URL)
	cfg.Retry.MaxRetries = 0
	client := New(cfg, nil)

	if client.egress != nil {
		t.Fatal("no registry should be attached")
	}
	if _, err := client.DoRaw(context.Background(), Request{Method: http.MethodGet, Endpoint: "/x"}); err != nil {
		t.Fatalf("request: %v", err)
	}
}

// A named provider of a shared type looks its exits up under its own name.
// The registry is filled per provider ("opencode-zen"), so a client that
// asked for "vllm" — the provider type — would find nothing and silently
// keep using a single fixed client: no rotation, and none of the exits an
// extension contributed.
func TestEgressLooksUpExitsByProviderNameNotType(t *testing.T) {
	srv, _ := echoSource(t)

	reg := egress.NewRegistry()
	reg.SetCore("opencode-zen", egress.StrategyRoundRobin, []egress.Candidate{
		{Name: "ip1", LocalAddr: net.ParseIP("127.0.0.1")},
		{Name: "ip2", LocalAddr: net.ParseIP("127.0.0.2")},
	})

	cfg := DefaultConfig("vllm", srv.URL)
	cfg.EgressProviderName = "opencode-zen"
	cfg.Retry.MaxRetries = 0
	cfg.Egress = reg
	client := New(cfg, nil)

	for i := 0; i < 4; i++ {
		if _, err := client.DoRaw(context.Background(), Request{Method: http.MethodGet, Endpoint: "/x"}); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}

	// Both exits must have served traffic: picking the wrong key leaves the
	// registry empty and every request on the pinned default client.
	status, ok := reg.Status("opencode-zen")
	if !ok {
		t.Fatal("registry has no status for opencode-zen")
	}
	seen := map[string]bool{}
	for _, e := range status.Exits {
		if e.Requests > 0 {
			seen[e.Name] = true
		}
	}
	if len(seen) != 2 {
		t.Fatalf("expected both exits to be used, got %v", seen)
	}
}

// Without an explicit name the client falls back to the provider name, which
// is what the built-in providers rely on.
func TestEgressProviderNameDefaultsToProviderName(t *testing.T) {
	c := &Client{config: Config{ProviderName: "openai"}}
	if got := c.egressProviderName(); got != "openai" {
		t.Fatalf("expected %q, got %q", "openai", got)
	}
	c = &Client{config: Config{ProviderName: "vllm", EgressProviderName: "opencode-zen"}}
	if got := c.egressProviderName(); got != "opencode-zen" {
		t.Fatalf("expected %q, got %q", "opencode-zen", got)
	}
}

// A provider whose upstream connection is opened by a sidecar cannot apply
// the exit to its own hop: the gateway only reaches the sidecar, so the exit
// is handed over and the local connection stays plain.
func TestEgressViaSidecarHandsTheExitOver(t *testing.T) {
	var got []egress.Candidate
	srv, _ := echoSource(t)

	reg := egress.NewRegistry()
	reg.SetCore("opencode-zen", egress.StrategyRoundRobin, []egress.Candidate{
		{Name: "ip1", LocalAddr: net.ParseIP("127.0.0.1")},
	})

	cfg := DefaultConfig("vllm", srv.URL)
	cfg.EgressProviderName = "opencode-zen"
	cfg.EgressViaSidecar = true
	cfg.Retry.MaxRetries = 0
	cfg.Egress = reg
	cfg.ExitHeaderSetter = func(_ *http.Request, cand egress.Candidate) {
		got = append(got, cand)
	}
	client := New(cfg, nil)

	if _, err := client.DoRaw(context.Background(), Request{Method: http.MethodGet, Endpoint: "/x"}); err != nil {
		t.Fatalf("request: %v", err)
	}
	if len(got) != 1 || got[0].Name != "ip1" {
		t.Fatalf("expected the exit to be handed over, got %+v", got)
	}
}

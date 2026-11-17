package tlsfront

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// echoServer answers with the request it received so a test can assert what
// actually reached the upstream.
func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen := map[string]string{
			"method": r.Method,
			"path":   r.URL.Path,
			"body":   string(body),
		}
		for _, name := range []string{"X-Aurora-Upstream", "X-Aurora-Bind-Ip", "X-Opencode-Session"} {
			seen[strings.ToLower(name)] = r.Header.Get(name)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(seen)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestFront(t *testing.T, presets ...string) *Front {
	t.Helper()
	f, err := New(Config{Presets: presets, InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := f.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// TestFrontForwardsRequest checks that the front keeps the upstream answer
// intact, including the request body, and that the control headers stay local.
func TestFrontForwardsRequest(t *testing.T) {
	upstream := echoServer(t)
	f := newTestFront(t, "chrome")

	body := `{"model":"m","messages":[]}`
	req, err := http.NewRequest(http.MethodPost, "http://"+f.Addr()+"/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set(UpstreamHeader, upstream.URL)
	req.Header.Set("X-Aurora-Bind-Ip", "127.0.0.1")
	req.Header.Set("X-Opencode-Session", "ses_test")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, raw)
	}

	var seen map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&seen); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if seen["method"] != http.MethodPost || seen["path"] != "/chat/completions" {
		t.Fatalf("upstream saw %s %s", seen["method"], seen["path"])
	}
	if seen["body"] != body {
		t.Fatalf("upstream body = %q, want %q", seen["body"], body)
	}
	if seen["x-opencode-session"] != "ses_test" {
		t.Fatalf("identity header was dropped: %q", seen["x-opencode-session"])
	}
	if seen["x-aurora-upstream"] != "" || seen["x-aurora-bind-ip"] != "" {
		t.Fatalf("control headers leaked upstream: %v", seen)
	}
	if got := resp.Header.Get(PresetHeader); got != "chrome" {
		t.Fatalf("preset header = %q, want chrome", got)
	}
}

// TestFrontRotatesPresets checks the round-robin order: a different
// fingerprint per request, cycling through the configured list.
func TestFrontRotatesPresets(t *testing.T) {
	upstream := echoServer(t)
	f := newTestFront(t, "chrome", "firefox", "safari")
	want := []string{"chrome", "firefox", "safari", "chrome"}

	for i, expected := range want {
		req, err := http.NewRequest(http.MethodGet, "http://"+f.Addr()+"/models", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set(UpstreamHeader, upstream.URL)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: status %d", i, resp.StatusCode)
		}
		if got := resp.Header.Get(PresetHeader); got != expected {
			t.Fatalf("request %d: preset = %q, want %q", i, got, expected)
		}
	}
}

// TestFrontRejectsMissingUpstream checks the front fails loudly instead of
// silently posting somewhere unexpected.
func TestFrontRejectsMissingUpstream(t *testing.T) {
	f := newTestFront(t, "chrome")
	resp, err := http.Get("http://" + f.Addr() + "/chat/completions")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestNewRejectsUnknownPreset keeps a typo in configuration from failing every
// request at runtime.
func TestNewRejectsUnknownPreset(t *testing.T) {
	if _, err := New(Config{Presets: []string{"chrome", "netscape"}}); err == nil {
		t.Fatal("expected an error for an unknown preset")
	}
}

// TestJoinUpstream keeps base paths and queries intact.
func TestJoinUpstream(t *testing.T) {
	got, err := joinUpstream("https://opencode.ai/inference/v1", &url.URL{Path: "/chat/completions", RawQuery: "a=1"})
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if got.String() != "https://opencode.ai/inference/v1/chat/completions?a=1" {
		t.Fatalf("got %s", got)
	}
}

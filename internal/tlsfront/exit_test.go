package tlsfront

import (
	"net/http"
	"testing"
)

// TestFrontAppliesSourceExit checks that the exit the gateway picked for this
// attempt is honoured by the front itself. The upstream is answered by a
// loopback server, so the assertion is that the front dials *through* the exit
// it was given (here: a proxy that is not listening, which must fail) instead
// of quietly ignoring it and connecting directly.
func TestFrontAppliesSourceExit(t *testing.T) {
	upstream := echoServer(t)
	f := newTestFront(t, "chrome")

	req, err := http.NewRequest(http.MethodGet, "http://"+f.Addr()+"/models", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set(UpstreamHeader, upstream.URL)
	// A dead proxy: if the front ignored the exit header the request would
	// succeed directly and this test would silently stop covering it.
	req.Header.Set("X-Aurora-Exit-Proxy", "http://127.0.0.1:1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (the exit must be used)", resp.StatusCode)
	}
}

// TestFrontRejectsNonHTTPUpstream keeps a scheme the front cannot dial from
// turning into a confusing uTLS error.
func TestFrontRejectsMalformedUpstream(t *testing.T) {
	f := newTestFront(t, "chrome")
	req, err := http.NewRequest(http.MethodGet, "http://"+f.Addr()+"/models", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set(UpstreamHeader, "/not-absolute")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

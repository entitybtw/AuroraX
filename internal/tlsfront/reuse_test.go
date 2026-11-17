package tlsfront

import (
	"net"
	"net/http"
	"testing"
)

// TestStartOrReuse_ReusesAnotherFront is the restart scenario: a container
// that is still shutting down (or a front left behind) holds the fixed
// loopback port. Failing startup took the whole gateway down, so a verified
// front must be reused instead.
func TestStartOrReuse_ReusesAnotherFront(t *testing.T) {
	first, err := New(Config{Presets: []string{"chrome"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	addr, err := first.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer first.Close()

	second, err := New(Config{Presets: []string{"chrome", "firefox"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := second.StartOrReuse(addr)
	if err != nil {
		t.Fatalf("StartOrReuse: %v", err)
	}
	if got != addr {
		t.Fatalf("addr = %q, want the existing %q", got, addr)
	}
	if second.Addr() != "" {
		t.Fatalf("second front bound %q, it should have kept the existing one", second.Addr())
	}
}

// TestStartOrReuse_RefusesAStranger keeps the reuse check strict: a listener
// that does not answer like a front is an error, not something to talk to.
func TestStartOrReuse_RefusesAStranger(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	front, err := New(Config{Presets: []string{"chrome"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := front.StartOrReuse(ln.Addr().String()); err == nil {
		t.Fatal("expected an error when the port is held by something that is not a front")
	}
}

// TestLooksLikeFront_MissingUpstreamIsNotAConflict: an empty port means no
// front is running, so startup should proceed normally.
func TestLooksLikeFront_EmptyPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	if LooksLikeFront(addr) {
		t.Fatalf("%s reported as a front with nothing listening", addr)
	}
}
